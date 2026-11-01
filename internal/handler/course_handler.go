package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/Amanyd/backend/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CourseHandler struct {
	svc *service.CourseService
}

type createCourseRequest struct {
	Title       string `json:"title"       validate:"required,min=2"`
	Description string `json:"description" validate:"required"`
	Rank        string `json:"rank"`
}

func (h *CourseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid json"))
		return
	}
	if err := validator.ValidateStruct(req); err != nil {
		apierr.WriteJSON(w, err)
		return
	}

	if req.Rank == "" {
		req.Rank = "officer"
	}

	claims := GetClaims(r)
	course, err := h.svc.Create(r.Context(), req.Title, req.Description, req.Rank, claims.UserID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusCreated, course)
}

func (h *CourseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	course, err := h.svc.GetByID(r.Context(), id, string(claims.Role), claims.UserID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, course)
}

func (h *CourseHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r)

	allCourses, err := h.svc.ListAll(r.Context())
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}

	if claims.Role == "instructor" {
		apierr.WriteData(w, http.StatusOK, allCourses)
		return
	}

	published := make([]domain.Course, 0, len(allCourses))
	for _, c := range allCourses {
		if c.Published {
			published = append(published, c)
		}
	}
	apierr.WriteData(w, http.StatusOK, published)
}

type updateCourseRequest struct {
	Title       string `json:"title"       validate:"required,min=2"`
	Description string `json:"description" validate:"required"`
	Rank        string `json:"rank"`
}

func (h *CourseHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	var req updateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid json"))
		return
	}
	if err := validator.ValidateStruct(req); err != nil {
		apierr.WriteJSON(w, err)
		return
	}

	if req.Rank == "" {
		req.Rank = "officer"
	}

	claims := GetClaims(r)
	course, err := h.svc.Update(r.Context(), id, claims.UserID, req.Title, req.Description, req.Rank)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, course)
}

const maxThumbnailBytes = 2 << 20

func (h *CourseHandler) UploadThumbnail(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailBytes+(1<<20))
	if err := r.ParseMultipartForm(maxThumbnailBytes); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("thumbnail must be 2 MB or smaller"))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("thumbnail file is required"))
		return
	}
	defer file.Close()

	if header.Size > maxThumbnailBytes {
		apierr.WriteJSON(w, apierr.BadRequest("thumbnail must be 2 MB or smaller"))
		return
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		apierr.WriteJSON(w, apierr.Internal("could not read thumbnail"))
		return
	}

	contentType := http.DetectContentType(buf[:n])
	if len(buf) >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		contentType = "image/webp"
	}

	var ext string
	switch contentType {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	default:
		apierr.WriteJSON(w, apierr.BadRequest("thumbnail must be PNG, JPG, or WEBP"))
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		apierr.WriteJSON(w, apierr.Internal("could not read thumbnail"))
		return
	}

	claims := GetClaims(r)
	course, err := h.svc.UploadThumbnail(
		r.Context(),
		courseID,
		claims.UserID,
		file,
		header.Size,
		contentType,
		ext,
	)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}

	apierr.WriteData(w, http.StatusOK, course)
}

func (h *CourseHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.Finalize(r.Context(), id, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"status": "finalized"})
}

func (h *CourseHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.Publish(r.Context(), id, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"status": "published"})
}

// GenerateContent creates slides, question banks and quizzes without
// publishing the subject. The instructor uploads supporting PDFs/PPTs first.
func (h *CourseHandler) GenerateContent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.GenerateContent(r.Context(), id, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusAccepted, map[string]string{"status": "generating"})
}

func (h *CourseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.Delete(r.Context(), id, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
