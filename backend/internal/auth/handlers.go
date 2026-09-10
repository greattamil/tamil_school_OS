package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/staff/login", h.staffLogin)
	mux.HandleFunc("POST /api/v1/auth/parent/otp/request", h.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/parent/otp/verify", h.verifyOTP)
	mux.HandleFunc("POST /api/v1/auth/select-school", h.selectSchool)
}

type staffLoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	DeviceID   string `json:"device_id"`
}

func (h *Handlers) staffLogin(w http.ResponseWriter, r *http.Request) {
	var req staffLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	tokens, err := h.svc.StaffLogin(r.Context(), req.Identifier, req.Password, req.DeviceID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeTokens(w, tokens)
}

type otpRequestRequest struct {
	Mobile string `json:"mobile"`
}

func (h *Handlers) requestOTP(w http.ResponseWriter, r *http.Request) {
	var req otpRequestRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	code, err := h.svc.RequestParentOTP(r.Context(), req.Mobile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not issue otp")
		return
	}

	resp := map[string]any{"status": "sent"}
	if devCode := code; devCode != "" {
		// Local/dev convenience only -- real dispatch is via the school's SMS
		// gateway (see Service.RequestParentOTP). Never expose this outside dev.
		resp["dev_only_code"] = devCode
	}
	writeJSON(w, http.StatusOK, resp)
}

type otpVerifyRequest struct {
	Mobile   string `json:"mobile"`
	Code     string `json:"code"`
	DeviceID string `json:"device_id"`
}

func (h *Handlers) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var req otpVerifyRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	tokens, err := h.svc.VerifyParentOTP(r.Context(), req.Mobile, req.Code, req.DeviceID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeTokens(w, tokens)
}

type selectSchoolRequest struct {
	RefreshToken string    `json:"refresh_token"`
	SchoolID     uuid.UUID `json:"school_id"`
	DeviceID     string    `json:"device_id"`
}

func (h *Handlers) selectSchool(w http.ResponseWriter, r *http.Request) {
	var req selectSchoolRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	tokens, err := h.svc.SelectSchool(r.Context(), req.RefreshToken, req.SchoolID, req.DeviceID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeTokens(w, tokens)
}

func writeTokens(w http.ResponseWriter, tokens TokenPair) {
	schools := make([]map[string]any, 0, len(tokens.Schools))
	for _, s := range tokens.Schools {
		schools = append(schools, map[string]any{
			"school_id":   s.SchoolID,
			"school_name": s.SchoolName,
			"role":        s.Role,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"schools":       schools,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, ErrOTPInvalid):
		writeError(w, http.StatusUnauthorized, "invalid_otp", "invalid or expired otp")
	case errors.Is(err, ErrOTPAttemptsExceeded):
		writeError(w, http.StatusTooManyRequests, "otp_attempts_exceeded", "too many otp attempts")
	case errors.Is(err, ErrSessionRevoked):
		writeError(w, http.StatusUnauthorized, "session_revoked", "session revoked")
	case errors.Is(err, ErrNoSchoolAccess):
		writeError(w, http.StatusForbidden, "no_school_access", "no role at requested school")
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
