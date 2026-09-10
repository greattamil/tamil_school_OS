package bulkimport

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

const maxUploadBytes = 10 << 20 // 10 MB: comfortably above a 1,500-student CSV

type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/imports/students/preview", h.preview)
	mux.HandleFunc("POST /api/v1/imports/students/commit", h.commit)
}

func (h *Handlers) preview(w http.ResponseWriter, r *http.Request) {
	file, err := openUploadedFile(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", err.Error())
		return
	}
	defer file.Close()

	result, err := h.svc.Preview(r.Context(), file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_csv", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) commit(w http.ResponseWriter, r *http.Request) {
	academicYearID, err := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "academic_year_id query param is required")
		return
	}

	file, err := openUploadedFile(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", err.Error())
		return
	}
	defer file.Close()

	var decisions []GuardianDecision
	if raw := r.FormValue("guardian_decisions"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &decisions); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_guardian_decisions", "guardian_decisions must be a JSON array of {mobile, action}")
			return
		}
	}

	result, err := h.svc.Commit(r.Context(), file, academicYearID, decisions)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_csv", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func openUploadedFile(r *http.Request) (multipartFile, error) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return nil, err
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	return file, nil
}

// multipartFile is the subset of multipart.File this package needs, named locally
// so callers don't have to import mime/multipart just to pass the value through.
type multipartFile interface {
	Read(p []byte) (n int, err error)
	Close() error
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
