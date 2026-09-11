package guardians

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
	mux.HandleFunc("POST /api/v1/guardians", h.create)
	mux.HandleFunc("GET /api/v1/guardians/{id}", h.get)
	mux.HandleFunc("GET /api/v1/guardians", h.findByMobile)
	mux.HandleFunc("GET /api/v1/parent/children", h.listChildren)
	mux.HandleFunc("POST /api/v1/students/{id}/guardians", h.linkToStudent)
	mux.HandleFunc("GET /api/v1/students/{id}/guardians", h.listForStudent)
	mux.HandleFunc("PUT /api/v1/guardians/{id}/notification-preferences", h.setNotificationPreferences)
}

type createGuardianRequest struct {
	Name       string  `json:"name"`
	Mobile     string  `json:"mobile"`
	Email      *string `json:"email"`
	Occupation *string `json:"occupation"`
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createGuardianRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.Mobile == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "name and mobile are required")
		return
	}
	g, err := h.repo.Create(r.Context(), req.Name, req.Mobile, req.Email, req.Occupation)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid guardian id")
		return
	}
	g, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// findByMobile backs the "detect existing guardian by mobile, offer to link"
// workflow (PRD 4.1.2). Empty result is not an error -- it means create a new one.
func (h *Handlers) findByMobile(w http.ResponseWriter, r *http.Request) {
	mobile := r.URL.Query().Get("mobile")
	if mobile == "" {
		writeError(w, http.StatusBadRequest, "missing_mobile", "mobile query param is required")
		return
	}
	list, err := h.repo.FindByMobile(r.Context(), mobile)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

type linkRequest struct {
	GuardianID       uuid.UUID    `json:"guardian_id"`
	Relationship     Relationship `json:"relationship"`
	IsPrimaryContact bool         `json:"is_primary_contact"`
	IsFeeResponsible bool         `json:"is_fee_responsible"`
}

func (h *Handlers) linkToStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	var req linkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.GuardianID == uuid.Nil || req.Relationship == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "guardian_id and relationship are required")
		return
	}

	err = h.repo.LinkToStudent(r.Context(), StudentGuardianLink{
		StudentID:        studentID,
		GuardianID:       req.GuardianID,
		Relationship:     req.Relationship,
		IsPrimaryContact: req.IsPrimaryContact,
		IsFeeResponsible: req.IsFeeResponsible,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "linked"})
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

func (h *Handlers) listChildren(w http.ResponseWriter, r *http.Request) {
	userID, _ := tenancy.UserID(r.Context())
	children, err := h.repo.ListChildrenForUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": children})
}

// setNotificationPreferences answers PRD 4.5.2. Authorized for office roles
// (managing on a family's behalf, e.g. by phone request) or the guardian's
// own linked account (parent-app self-service) -- verified against the
// actual row in the repository, not just trusted from the role claim.
func (h *Handlers) setNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	guardianID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid guardian id")
		return
	}
	var prefs NotificationPreferences
	if !decodeJSON(w, r, &prefs) {
		return
	}
	role, _ := tenancy.Role(r.Context())
	isOffice := role == "correspondent" || role == "office_admin"
	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.SetNotificationPreferences(r.Context(), guardianID, prefs, actorID, isOffice); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "guardian not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "not authorized to change this guardian's preferences")
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
