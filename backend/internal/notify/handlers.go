package notify

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
	mux.HandleFunc("POST /api/v1/notices", h.compose)
	mux.HandleFunc("GET /api/v1/notices", h.inbox)
	mux.HandleFunc("POST /api/v1/notices/{id}/read", h.markRead)
	mux.HandleFunc("POST /api/v1/device-tokens", h.registerToken)
}

type composeRequest struct {
	Kind          Kind        `json:"kind"`
	Title         string      `json:"title"`
	BodyEN        string      `json:"body_en"`
	BodyTA        string      `json:"body_ta"`
	TargetType    TargetType  `json:"target_type"`
	ClassID       *uuid.UUID  `json:"class_id"`
	SectionID     *uuid.UUID  `json:"section_id"`
	StudentID     *uuid.UUID  `json:"student_id"`
	CustomUserIDs []uuid.UUID `json:"custom_user_ids"`
	IsEmergency   bool        `json:"is_emergency"`
}

func (h *Handlers) compose(w http.ResponseWriter, r *http.Request) {
	role, _ := tenancy.Role(r.Context())
	if role != "correspondent" && role != "office_admin" && role != "teacher" {
		writeError(w, http.StatusForbidden, "forbidden", "role may not compose notices")
		return
	}

	var req composeRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	if req.Title == "" || req.BodyEN == "" || req.Kind == "" || req.TargetType == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "kind, title, body_en and target_type are required")
		return
	}
	// Class teachers may only reach their own section (PRD 2.2 role matrix);
	// enforcing exactly which section is "their own" requires the
	// section_teachers assignment, deferred here -- reject whole-school/class
	// targeting from a teacher role as the safe subset of that rule for now.
	if role == "teacher" && req.TargetType != TargetSection && req.TargetType != TargetStudent {
		writeError(w, http.StatusForbidden, "forbidden", "class teachers may only send to their own section or an individual student")
		return
	}
	if req.IsEmergency && role != "correspondent" && role != "office_admin" {
		writeError(w, http.StatusForbidden, "forbidden", "emergency broadcasts are restricted to correspondent and office admin")
		return
	}

	actorID, _ := tenancy.UserID(r.Context())
	notificationID, count, err := h.repo.Compose(r.Context(), ComposeInput{
		Kind: req.Kind, Title: req.Title, BodyEN: req.BodyEN, BodyTA: req.BodyTA,
		TargetType: req.TargetType, ClassID: req.ClassID, SectionID: req.SectionID,
		StudentID: req.StudentID, CustomUserIDs: req.CustomUserIDs, IsEmergency: req.IsEmergency,
	}, actorID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"notification_id": notificationID, "recipient_count": count})
}

func (h *Handlers) inbox(w http.ResponseWriter, r *http.Request) {
	userID, _ := tenancy.UserID(r.Context())
	items, err := h.repo.Inbox(r.Context(), userID, 50)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) markRead(w http.ResponseWriter, r *http.Request) {
	notificationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid notification id")
		return
	}
	userID, _ := tenancy.UserID(r.Context())
	if err := h.repo.MarkRead(r.Context(), userID, notificationID); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerTokenRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *Handlers) registerToken(w http.ResponseWriter, r *http.Request) {
	var req registerTokenRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "token is required")
		return
	}
	if req.Platform == "" {
		req.Platform = "android"
	}
	userID, _ := tenancy.UserID(r.Context())
	if err := h.repo.RegisterDeviceToken(r.Context(), userID, req.Token, req.Platform); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoRecipients):
		writeError(w, http.StatusUnprocessableEntity, "no_recipients", "this target has no reachable recipients")
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
