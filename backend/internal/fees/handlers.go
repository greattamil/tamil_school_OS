package fees

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/fees/dues/import", h.importCSV)
	mux.HandleFunc("GET /api/v1/fees/dues", h.list)
	mux.HandleFunc("GET /api/v1/students/{id}/dues", h.forStudent)
}

func (h *Handlers) importCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "file is required")
		return
	}
	defer file.Close()

	actorID, _ := tenancy.UserID(r.Context())
	result, err := h.repo.ImportCSV(r.Context(), actorID, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_csv", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.List(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) forStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	snapshot, err := h.repo.ForStudent(r.Context(), studentID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusOK, map[string]any{"outstanding_amount_paise": 0, "message": "no dues on file"})
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
