package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"school-erp/backend/internal/db"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// Register wires these routes behind httpmw.RequireAuth in cmd/api/main.go -- every
// call here relies on tenancy.SchoolID already being set on the request context.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/students", h.list)
	mux.HandleFunc("POST /api/v1/students", h.create)
	mux.HandleFunc("GET /api/v1/students/{id}", h.get)
}

type createStudentRequest struct {
	AdmissionNumber string  `json:"admission_number"`
	NameEnglish     string  `json:"name_english"`
	NameTamil       *string `json:"name_tamil"`
	DateOfBirth     string  `json:"date_of_birth"` // YYYY-MM-DD
	Gender          string  `json:"gender"`
	MotherTongue    *string `json:"mother_tongue"`
	BloodGroup      *string `json:"blood_group"`
	Address         *string `json:"address"`
	PreviousSchool  *string `json:"previous_school"`
	RTEQuota        bool    `json:"rte_quota"`
	AdmissionDate   string  `json:"admission_date"` // YYYY-MM-DD
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createStudentRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date_of_birth", "date_of_birth must be YYYY-MM-DD")
		return
	}
	admissionDate, err := time.Parse("2006-01-02", req.AdmissionDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_admission_date", "admission_date must be YYYY-MM-DD")
		return
	}
	if req.AdmissionNumber == "" || req.NameEnglish == "" || req.Gender == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "admission_number, name_english and gender are required")
		return
	}

	student, err := h.repo.Create(r.Context(), CreateStudentInput{
		AdmissionNumber: req.AdmissionNumber,
		NameEnglish:     req.NameEnglish,
		NameTamil:       req.NameTamil,
		DateOfBirth:     dob,
		Gender:          req.Gender,
		MotherTongue:    req.MotherTongue,
		BloodGroup:      req.BloodGroup,
		Address:         req.Address,
		PreviousSchool:  req.PreviousSchool,
		RTEQuota:        req.RTEQuota,
		AdmissionDate:   admissionDate,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, student)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}

	student, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, student)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit := 50

	items, err := h.repo.List(r.Context(), cursor, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = items[len(items)-1].AdmissionNumber
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "student not found")
	case errors.Is(err, ErrDuplicateAdmissionNumber):
		writeError(w, http.StatusConflict, "duplicate_admission_number", "admission number already in use")
	case errors.Is(err, db.ErrNoTenant):
		writeError(w, http.StatusForbidden, "no_tenant", "request is not scoped to a school")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
