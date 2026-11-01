package port

import (
	"context"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/google/uuid"
)

type SyllabusRepository interface {
	Upsert(ctx context.Context, syllabus *domain.CourseSyllabus) error
	GetByCourse(ctx context.Context, courseID uuid.UUID) (*domain.CourseSyllabus, error)
	UpdateStatus(
		ctx context.Context,
		courseID uuid.UUID,
		status domain.SyllabusStatus,
		subjectCode string,
		subjectName string,
		topics []string,
		rows []domain.SyllabusRow,
		errMsg string,
	) error
}
