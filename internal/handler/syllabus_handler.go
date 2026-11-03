package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxSyllabusBytes = 25 << 20 // 25 MB

type SyllabusHandler struct {
	svc *service.SyllabusService
}

func NewSyllabusHandler(svc *service.SyllabusService) *SyllabusHandler {
	return &SyllabusHandler{svc: svc}
}

func (h *SyllabusHandler) Get(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	syllabus, err := h.svc.Get(r.Context(), courseID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, syllabus)
}

// Import parses a multi-subject master syllabus and creates all subjects at
// once. Lesson plans, slides and question banks are queued in the background.
func (h *SyllabusHandler) Import(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSyllabusBytes+(1<<20))
	if err := r.ParseMultipartForm(maxSyllabusBytes); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("syllabus must be 25 MB or smaller"))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("syllabus")
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("missing syllabus file"))
		return
	}
	defer file.Close()

	if header.Size > maxSyllabusBytes {
		apierr.WriteJSON(w, apierr.BadRequest("syllabus must be 25 MB or smaller"))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("could not read syllabus file"))
		return
	}

	generate := true
	if raw := strings.ToLower(strings.TrimSpace(r.FormValue("generate"))); raw != "" {
		generate = raw != "false" && raw != "0" && raw != "no"
	}

	claims := GetClaims(r)
	result, err := h.svc.ImportAll(
		r.Context(),
		claims.UserID,
		header.Filename,
		strings.TrimSpace(r.FormValue("rank")),
		generate,
		data,
	)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, result)
}

func (h *SyllabusHandler) ViewURL(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	url, err := h.svc.GetFileURL(r.Context(), courseID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"url": url})
}

func (h *SyllabusHandler) Upload(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSyllabusBytes+(1<<20))
	if err := r.ParseMultipartForm(maxSyllabusBytes); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("syllabus must be 25 MB or smaller"))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("syllabus")
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("missing syllabus file"))
		return
	}
	defer file.Close()

	if header.Size > maxSyllabusBytes {
		apierr.WriteJSON(w, apierr.BadRequest("syllabus must be 25 MB or smaller"))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("could not read syllabus file"))
		return
	}

	claims := GetClaims(r)
	subjectCode := strings.TrimSpace(r.FormValue("subject_code"))
	generate := true
	if raw := strings.ToLower(strings.TrimSpace(r.FormValue("generate"))); raw != "" {
		generate = raw != "false" && raw != "0" && raw != "no"
	}
	syllabus, err := h.svc.Upload(
		r.Context(),
		courseID,
		claims.UserID,
		header.Filename,
		subjectCode,
		generate,
		data,
	)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, syllabus)
}
