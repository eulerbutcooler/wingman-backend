package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/Amanyd/backend/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FileHandler struct {
	svc *service.FileService
}

type initUploadRequest struct {
	FileName string `json:"file_name" validate:"required"`
	FileType string `json:"file_type" validate:"required,oneof=pdf ppt docx"`
}

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid lesson id"))
		return
	}

	var req initUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid json"))
		return
	}
	if err := validator.ValidateStruct(req); err != nil {
		apierr.WriteJSON(w, err)
		return
	}

	claims := GetClaims(r)
	result, err := h.svc.InitUpload(r.Context(), lessonID, req.FileName, domain.FileType(req.FileType), claims.UserID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusCreated, result)
}

func (h *FileHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "fileId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid file id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.ConfirmUpload(r.Context(), fileID, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"status": "processing"})
}

func (h *FileHandler) IngestStatus(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "fileId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid file id"))
		return
	}

	status, err := h.svc.GetIngestStatus(r.Context(), fileID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"status": string(status)})
}

func (h *FileHandler) ViewURL(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "fileId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid file id"))
		return
	}

	url, err := h.svc.GetViewURL(r.Context(), fileID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"url": url})
}

func (h *FileHandler) ListByLesson(w http.ResponseWriter, r *http.Request) {
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid lesson id"))
		return
	}

	files, err := h.svc.ListByLesson(r.Context(), lessonID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, files)
}
