package port

import (
	"context"

	"github.com/eulerbutcooler/wingman-backend/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEnrollmentID(ctx context.Context, enrollmentID string) (*domain.User, error)
}
