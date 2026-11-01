package handler

import (
	"net/http"

	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FileHandler struct {
	svc *service.FileService
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

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "fileId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid file id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.Delete(r.Context(), fileID, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
