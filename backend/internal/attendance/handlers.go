package attendance

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
	mux.HandleFunc("POST /api/v1/sections/{id}/attendance/sync", h.sync)
	mux.HandleFunc("GET /api/v1/sections/{id}/attendance", h.get)
	mux.HandleFunc("GET /api/v1/attendance/summary", h.dailySummary)
	mux.HandleFunc("GET /api/v1/students/{id}/attendance-percentage", h.percentage)
	mux.HandleFunc("GET /api/v1/sections/{id}/attendance/consecutive-absences", h.consecutiveAbsences)
}

type syncRequest struct {
	Date  string      `json:"date"`
	Edits []EntryEdit `json:"edits"`
}

func (h *Handlers) sync(w http.ResponseWriter, r *http.Request) {
	sectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid section id")
		return
	}
	var req syncRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
		return
	}
	if len(req.Edits) == 0 {
		writeError(w, http.StatusBadRequest, "no_edits", "edits must not be empty")
		return
	}

	result, err := h.repo.Sync(r.Context(), sectionID, date, req.Edits)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	sectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid section id")
		return
	}
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = today().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
		return
	}

	entries, err := h.repo.GetRegisterEntries(r.Context(), sectionID, date)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": dateStr, "entries": entries})
}

func (h *Handlers) dailySummary(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = today().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
		return
	}

	var sectionID *uuid.UUID
	if s := r.URL.Query().Get("section_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_section_id", "invalid section_id")
			return
		}
		sectionID = &id
	}

	rows, err := h.repo.DailySummary(r.Context(), sectionID, date)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": dateStr, "sections": rows})
}

func (h *Handlers) percentage(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	from, to, err := parseDateRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date_range", err.Error())
		return
	}

	present, total, err := h.repo.AttendancePercentageForStudent(r.Context(), studentID, from, to)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	pct := 0.0
	if total > 0 {
		pct = float64(present) / float64(total) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"present_days": present, "total_days": total, "percentage": pct,
	})
}

func (h *Handlers) consecutiveAbsences(w http.ResponseWriter, r *http.Request) {
	sectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid section id")
		return
	}
	threshold := 3
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.Atoi(t); err == nil {
			threshold = v
		}
	}

	rows, err := h.repo.ConsecutiveAbsences(r.Context(), sectionID, threshold)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"threshold": threshold, "items": rows})
}

func parseDateRange(r *http.Request) (from, to time.Time, err error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, errors.New("from and to query params (YYYY-MM-DD) are required")
	}
	from, err = time.Parse("2006-01-02", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must be YYYY-MM-DD")
	}
	to, err = time.Parse("2006-01-02", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("to must be YYYY-MM-DD")
	}
	return from, to, nil
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrPastDateRequiresOfficeAuthority):
		writeError(w, http.StatusForbidden, "past_date_requires_office_authority", err.Error())
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
