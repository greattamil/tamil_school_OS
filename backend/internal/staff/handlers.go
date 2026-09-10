package staff

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
	mux.HandleFunc("GET /api/v1/staff", h.list)
	mux.HandleFunc("POST /api/v1/staff", h.create)
	mux.HandleFunc("GET /api/v1/staff/{id}", h.get)
}

type createStaffRequest struct {
	Name          string  `json:"name"`
	Designation   *string `json:"designation"`
	Qualification *string `json:"qualification"`
	DateOfJoining *string `json:"date_of_joining"`
	IsTeaching    bool    `json:"is_teaching"`
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createStaffRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "name is required")
		return
	}

	var doj *time.Time
	if req.DateOfJoining != nil && *req.DateOfJoining != "" {
		t, err := time.Parse("2006-01-02", *req.DateOfJoining)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_date_of_joining", "date_of_joining must be YYYY-MM-DD")
			return
		}
		doj = &t
	}

	s, err := h.repo.Create(r.Context(), CreateStaffInput{
		Name:          req.Name,
		Designation:   req.Designation,
		Qualification: req.Qualification,
		DateOfJoining: doj,
		IsTeaching:    req.IsTeaching,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid staff id")
		return
	}
	s, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "staff not found")
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
