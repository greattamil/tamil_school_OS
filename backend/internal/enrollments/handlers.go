package enrollments

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

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/students/{id}/enrollments", h.create)
	mux.HandleFunc("GET /api/v1/students/{id}/enrollments", h.listForStudent)
	mux.HandleFunc("POST /api/v1/students/{id}/enrollments/transfer", h.transfer)
	mux.HandleFunc("GET /api/v1/sections/{id}/roster", h.roster)
}

type createRequest struct {
	AcademicYearID uuid.UUID `json:"academic_year_id"`
	ClassID        uuid.UUID `json:"class_id"`
	SectionID      uuid.UUID `json:"section_id"`
	RollNumber     *string   `json:"roll_number"`
	Medium         string    `json:"medium"`
	GroupCode      string    `json:"group_code"`
	StartDate      string    `json:"start_date"`
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	var req createRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil || req.AcademicYearID == uuid.Nil || req.ClassID == uuid.Nil || req.SectionID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "academic_year_id, class_id, section_id and start_date (YYYY-MM-DD) are required")
		return
	}

	e, err := h.repo.Create(r.Context(), CreateInput{
		StudentID:      studentID,
		AcademicYearID: req.AcademicYearID,
		ClassID:        req.ClassID,
		SectionID:      req.SectionID,
		RollNumber:     req.RollNumber,
		Medium:         req.Medium,
		GroupCode:      req.GroupCode,
		StartDate:      start,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (h *Handlers) listForStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	list, err := h.repo.ListForStudent(r.Context(), studentID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

type transferRequest struct {
	NewClassID   uuid.UUID `json:"new_class_id"`
	NewSectionID uuid.UUID `json:"new_section_id"`
	TransferDate string    `json:"transfer_date"`
	RollNumber   *string   `json:"roll_number"`
}

func (h *Handlers) transfer(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	var req transferRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.NewClassID == uuid.Nil || req.NewSectionID == uuid.Nil || req.TransferDate == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "new_class_id, new_section_id and transfer_date are required")
		return
	}

	e, err := h.repo.TransferSection(r.Context(), studentID, req.NewClassID, req.NewSectionID, req.TransferDate, req.RollNumber)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (h *Handlers) roster(w http.ResponseWriter, r *http.Request) {
	sectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid section id")
		return
	}
	asOf := r.URL.Query().Get("date")
	if asOf == "" {
		asOf = time.Now().Format("2006-01-02")
	}

	list, err := h.repo.RosterAsOf(r.Context(), sectionID, asOf)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "as_of": asOf})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "enrollment not found")
	case errors.Is(err, ErrOverlappingEnrollment):
		writeError(w, http.StatusConflict, "overlapping_enrollment", "this overlaps an existing enrollment for the student")
	case errors.Is(err, db.ErrNoTenant):
		writeError(w, http.StatusForbidden, "no_tenant", "request is not scoped to a school")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
