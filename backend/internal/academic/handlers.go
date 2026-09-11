package academic

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

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
	mux.HandleFunc("POST /api/v1/academic-years", h.createYear)
	mux.HandleFunc("GET /api/v1/academic-years", h.listYears)
	mux.HandleFunc("POST /api/v1/academic-years/{id}/transition", h.transitionYear)
	mux.HandleFunc("POST /api/v1/classes", h.createClass)
	mux.HandleFunc("GET /api/v1/classes", h.listClasses)
	mux.HandleFunc("POST /api/v1/sections", h.createSection)
	mux.HandleFunc("GET /api/v1/sections", h.listSections)
	mux.HandleFunc("PUT /api/v1/academic-calendar", h.setCalendarDays)
	mux.HandleFunc("GET /api/v1/academic-calendar", h.listCalendarDays)
}

type createYearRequest struct {
	Label     string `json:"label"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (h *Handlers) createYear(w http.ResponseWriter, r *http.Request) {
	var req createYearRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	start, err1 := time.Parse("2006-01-02", req.StartDate)
	end, err2 := time.Parse("2006-01-02", req.EndDate)
	if req.Label == "" || err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "label, start_date and end_date (YYYY-MM-DD) are required")
		return
	}

	year, err := h.repo.CreateYear(r.Context(), req.Label, start, end)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, year)
}

func (h *Handlers) listYears(w http.ResponseWriter, r *http.Request) {
	years, err := h.repo.ListYears(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": years})
}

type transitionRequest struct {
	NewState YearState `json:"new_state"`
	Reason   string    `json:"reason"`
}

func (h *Handlers) transitionYear(w http.ResponseWriter, r *http.Request) {
	yearID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid academic year id")
		return
	}
	var req transitionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.NewState == YearLocked && req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason_required", "a reason is required to lock or reopen an academic year")
		return
	}

	actorID, _ := tenancy.UserID(r.Context())
	role, _ := tenancy.Role(r.Context())

	// Reopening a locked year requires correspondent authority (PRD 3.1). Other
	// transitions (active -> soft_closing -> locked) are also office-admin
	// permitted per the role matrix (PRD 2.2: "Configure fee structure" and
	// closely-held settings are correspondent-only; year state is treated the
	// same way here since it governs whether fee and mark records are writable).
	if role != "correspondent" {
		writeError(w, http.StatusForbidden, "forbidden", "only the correspondent may change an academic year's state")
		return
	}

	if err := h.repo.TransitionState(r.Context(), yearID, req.NewState, req.Reason, actorID); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createClassRequest struct {
	AcademicYearID uuid.UUID `json:"academic_year_id"`
	Name           string    `json:"name"`
	Sequence       int       `json:"sequence"`
}

func (h *Handlers) createClass(w http.ResponseWriter, r *http.Request) {
	var req createClassRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AcademicYearID == uuid.Nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "academic_year_id and name are required")
		return
	}
	class, err := h.repo.CreateClass(r.Context(), req.AcademicYearID, req.Name, req.Sequence)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, class)
}

func (h *Handlers) listClasses(w http.ResponseWriter, r *http.Request) {
	yearID, err := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "academic_year_id query param is required")
		return
	}
	classes, err := h.repo.ListClasses(r.Context(), yearID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": classes})
}

type createSectionRequest struct {
	ClassID uuid.UUID `json:"class_id"`
	Name    string    `json:"name"`
}

func (h *Handlers) createSection(w http.ResponseWriter, r *http.Request) {
	var req createSectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ClassID == uuid.Nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "class_id and name are required")
		return
	}
	section, err := h.repo.CreateSection(r.Context(), req.ClassID, req.Name)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, section)
}

func (h *Handlers) listSections(w http.ResponseWriter, r *http.Request) {
	classID, err := uuid.Parse(r.URL.Query().Get("class_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_class_id", "class_id query param is required")
		return
	}
	sections, err := h.repo.ListSections(r.Context(), classID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sections})
}

type setCalendarDaysRequest struct {
	Days []struct {
		Date    string  `json:"date"`
		DayType DayType `json:"day_type"`
		Note    string  `json:"note"`
	} `json:"days"`
}

// setCalendarDays upserts the day_type for one or more dates in a single call
// (PRD 3.1: setting a year's calendar up front is naturally many dates at once;
// amending a single declared holiday later is the len==1 case of the same call).
func (h *Handlers) setCalendarDays(w http.ResponseWriter, r *http.Request) {
	role, _ := tenancy.Role(r.Context())
	if role != "correspondent" && role != "office_admin" {
		writeError(w, http.StatusForbidden, "forbidden", "only correspondent or office admin may set the academic calendar")
		return
	}

	var req setCalendarDaysRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Days) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_body", "days must not be empty")
		return
	}
	days := make([]CalendarDay, len(req.Days))
	for i, d := range req.Days {
		date, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_date", "each date must be YYYY-MM-DD")
			return
		}
		switch d.DayType {
		case DayRegularWorking, DayHoliday, DayCompensatoryWorking, DayHalf, DayExam:
		default:
			writeError(w, http.StatusBadRequest, "invalid_day_type", "unrecognized day_type")
			return
		}
		days[i] = CalendarDay{Date: date, DayType: d.DayType, Note: d.Note}
	}

	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.SetCalendarDays(r.Context(), days, actorID); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) listCalendarDays(w http.ResponseWriter, r *http.Request) {
	from, err1 := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	to, err2 := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "invalid_range", "from and to (YYYY-MM-DD) are required")
		return
	}
	days, err := h.repo.ListCalendarDays(r.Context(), from, to)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": days})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrDuplicateLabel):
		writeError(w, http.StatusConflict, "duplicate", "already exists")
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_transition", "invalid academic year state transition")
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
