package handler

import (
	"net/http"

	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LessonPlanHandler struct {
	svc *service.LessonPlanService
}

func NewLessonPlanHandler(svc *service.LessonPlanService) *LessonPlanHandler {
	return &LessonPlanHandler{svc: svc}
}

func (h *LessonPlanHandler) GetURL(w http.ResponseWriter, r *http.Request) {
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid lesson id"))
		return
	}

	url, err := h.svc.GetURL(r.Context(), lessonID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, map[string]string{"url": url})
}

func (h *LessonPlanHandler) Generate(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(chi.URLParam(r, "courseId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
		return
	}

	claims := GetClaims(r)
	if err := h.svc.StartGeneration(r.Context(), courseID, claims.UserID); err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusAccepted, map[string]string{"status": "generating"})
}
