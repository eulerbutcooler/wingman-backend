package handler

import (
	"net/http"

	"github.com/Amanyd/backend/pkg/apierr"
)

type HealthHandler struct{}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	apierr.WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
}
