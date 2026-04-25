package handler

import (
	"errors"
	"net/http"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/pkg/apierr"
)

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apierr.NotFound(err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return apierr.Forbidden(err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return apierr.Unauthorized(err.Error())
	case errors.Is(err, domain.ErrConflict):
		return apierr.Conflict(err.Error())
	case errors.Is(err, domain.ErrBadInput):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal("internal error")
	}
}

func parseUUIDParam(r *http.Request, key string) (string, bool) {
	val := r.PathValue(key)
	if val == "" {
		return "", false
	}
	return val, true
}
