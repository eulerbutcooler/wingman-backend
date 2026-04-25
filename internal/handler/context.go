package handler

import (
	"context"
	"net/http"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/google/uuid"
)

type ctxKey string

const claimsKey ctxKey = "claims"

type AuthClaims struct {
	UserID uuid.UUID
	Role   domain.Role
	Rank   string
}

func SetClaims(ctx context.Context, c AuthClaims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func GetClaims(r *http.Request) AuthClaims {
	return r.Context().Value(claimsKey).(AuthClaims)
}
