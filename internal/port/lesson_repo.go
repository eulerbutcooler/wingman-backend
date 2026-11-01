package port

import (
	"context"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/google/uuid"
)

type LessonRepository interface {
	Create(ctx context.Context, lesson *domain.Lesson) error
	CreateSyllabusLesson(ctx context.Context, lesson *domain.Lesson) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Lesson, error)
	GetMaterialsLesson(ctx context.Context, courseID uuid.UUID) (*domain.Lesson, error)
	ListByCourse(ctx context.Context, courseID uuid.UUID) ([]domain.Lesson, error)
	ListAllByCourse(ctx context.Context, courseID uuid.UUID) ([]domain.Lesson, error)
	ListLessonPlanBacklog(ctx context.Context) ([]domain.LessonPlanBacklogItem, error)
	Update(ctx context.Context, lesson *domain.Lesson) error
	UpdateKeywords(ctx context.Context, id uuid.UUID, keywords []byte) error
	SetCourseLessonPlanStatus(
		ctx context.Context,
		courseID uuid.UUID,
		status string,
		errMsg string,
	) error
	UpdateLessonPlan(
		ctx context.Context,
		lessonID uuid.UUID,
		key string,
		status string,
		errMsg string,
	) (*domain.Lesson, error)
	CreateTopics(ctx context.Context, lessonID uuid.UUID, topics []domain.LessonTopic) error
	ListTopicsByLesson(ctx context.Context, lessonID uuid.UUID) ([]domain.LessonTopic, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
