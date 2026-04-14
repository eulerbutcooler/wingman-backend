package port

import (
	"context"

	"github.com/eulerbutcooler/wingman-backend/internal/domain"
	"github.com/google/uuid"
)

type AnalyticsRepo interface {
	RecordEvent(ctx context.Context, event *domain.Event) error
	GetCourseMetrics(ctx context.Context, courseID uuid.UUID) (*domain.Metric, error)
	GetStudentScores(ctx context.Context, courseID uuid.UUID) ([]domain.StudentScore, error)
	GetOverview(ctx context.Context, instructorID uuid.UUID) (*domain.Overview, error)
}
