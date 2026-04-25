package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/pkg/apierr"
	jwtpkg "github.com/Amanyd/backend/pkg/jwt"
	"github.com/Amanyd/backend/pkg/logger"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func JWTAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				apierr.WriteJSON(w, apierr.Unauthorized("missing authorization header"))
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			claims, err := jwtpkg.ValidateToken(token, secret)
			if err != nil {
				apierr.WriteJSON(w, apierr.Unauthorized("invalid token"))
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				apierr.WriteJSON(w, apierr.Unauthorized("invalid token claims"))
				return
			}

			ctx := SetClaims(r.Context(), AuthClaims{
				UserID: userID,
				Role:   domain.Role(claims.Role),
				Rank:   claims.Rank,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RBACMiddleware(required domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims.Role != required {
				apierr.WriteJSON(w, apierr.Forbidden("insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := middleware.GetReqID(r.Context())

			reqLog := log.With(
				zap.String("request_id", reqID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)

			start := time.Now()
			ctx := logger.WithCtx(r.Context(), reqLog)
			next.ServeHTTP(w, r.WithContext(ctx))

			reqLog.Info("request completed", zap.Duration("duration", time.Since(start)))
		})
	}
}
