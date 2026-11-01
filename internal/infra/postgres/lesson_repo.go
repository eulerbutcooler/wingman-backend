package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/infra/postgres/gen"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type lessonRepo struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func NewLessonRepo(pool *pgxpool.Pool) port.LessonRepository {
	return &lessonRepo{pool: pool, q: gen.New(pool)}
}

func (r *lessonRepo) Create(ctx context.Context, l *domain.Lesson) error {
	row, err := r.q.CreateLesson(ctx, gen.CreateLessonParams{
		CourseID: l.CourseID,
		Title:    l.Title,
		OrderIdx: int32(l.OrderIdx),
	})
	if err != nil {
		return err
	}
	l.ID = row.ID
	l.CreatedAt = row.CreatedAt
	l.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *lessonRepo) CreateSyllabusLesson(ctx context.Context, l *domain.Lesson) error {
	var rowIndex pgtype.Int4
	if l.SyllabusRowIndex != nil {
		rowIndex = pgtype.Int4{Int32: int32(*l.SyllabusRowIndex), Valid: true}
	}
	row, err := r.q.CreateSyllabusLesson(ctx, gen.CreateSyllabusLessonParams{
		CourseID:         l.CourseID,
		Title:            l.Title,
		OrderIdx:         int32(l.OrderIdx),
		SyllabusRowIndex: rowIndex,
	})
	if err != nil {
		return err
	}
	*l = *toDomainLesson(row)
	return nil
}

func (r *lessonRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Lesson, error) {
	row, err := r.q.GetLessonByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomainLesson(row), nil
}

func (r *lessonRepo) GetMaterialsLesson(
	ctx context.Context,
	courseID uuid.UUID,
) (*domain.Lesson, error) {
	row, err := r.q.GetMaterialsLesson(ctx, courseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomainLesson(row), nil
}

func (r *lessonRepo) ListByCourse(ctx context.Context, courseID uuid.UUID) ([]domain.Lesson, error) {
	rows, err := r.q.ListLessonsByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	lessons := make([]domain.Lesson, len(rows))
	for i, row := range rows {
		lessons[i] = *toDomainLesson(row)
	}
	return lessons, nil
}

func (r *lessonRepo) ListAllByCourse(
	ctx context.Context,
	courseID uuid.UUID,
) ([]domain.Lesson, error) {
	rows, err := r.q.ListAllLessonsByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	lessons := make([]domain.Lesson, len(rows))
	for i, row := range rows {
		lessons[i] = *toDomainLesson(row)
	}
	return lessons, nil
}

func (r *lessonRepo) ListLessonPlanBacklog(
	ctx context.Context,
) ([]domain.LessonPlanBacklogItem, error) {
	rows, err := r.q.ListLessonPlanBacklog(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.LessonPlanBacklogItem, len(rows))
	for i, row := range rows {
		items[i] = domain.LessonPlanBacklogItem{
			CourseID:     row.CourseID,
			InstructorID: row.InstructorID,
		}
	}
	return items, nil
}

func (r *lessonRepo) Update(ctx context.Context, l *domain.Lesson) error {
	row, err := r.q.UpdateLesson(ctx, gen.UpdateLessonParams{
		ID:       l.ID,
		Title:    l.Title,
		OrderIdx: int32(l.OrderIdx),
	})
	if err != nil {
		return err
	}
	l.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *lessonRepo) UpdateKeywords(ctx context.Context, id uuid.UUID, keywords []byte) error {
	_, err := r.q.UpdateLessonKeywords(ctx, gen.UpdateLessonKeywordsParams{
		ID:       id,
		Keywords: keywords,
	})
	return err
}

func (r *lessonRepo) SetCourseLessonPlanStatus(
	ctx context.Context,
	courseID uuid.UUID,
	status string,
	errMsg string,
) error {
	return r.q.SetCourseLessonPlanStatus(ctx, gen.SetCourseLessonPlanStatusParams{
		CourseID:         courseID,
		LessonPlanStatus: status,
		Column3:          errMsg,
	})
}

func (r *lessonRepo) UpdateLessonPlan(
	ctx context.Context,
	lessonID uuid.UUID,
	key string,
	status string,
	errMsg string,
) (*domain.Lesson, error) {
	row, err := r.q.UpdateLessonPlan(ctx, gen.UpdateLessonPlanParams{
		ID:               lessonID,
		LessonPlanKey:    pgtype.Text{String: key, Valid: key != ""},
		LessonPlanStatus: status,
		Column4:          errMsg,
	})
	if err != nil {
		return nil, err
	}
	return toDomainLesson(row), nil
}

func (r *lessonRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteLesson(ctx, id)
}

func (r *lessonRepo) CreateTopics(ctx context.Context, lessonID uuid.UUID, topics []domain.LessonTopic) error {
	_, _ = r.pool.Exec(ctx, `DELETE FROM lesson_topics WHERE lesson_id = $1`, lessonID)

	for i, t := range topics {
		slidesJSON, err := json.Marshal(t.Slides)
		if err != nil {
			return err
		}
		_, err = r.pool.Exec(ctx, `
			INSERT INTO lesson_topics (lesson_id, title, order_index, slides)
			VALUES ($1, $2, $3, $4)
		`, lessonID, t.Title, i, slidesJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *lessonRepo) ListTopicsByLesson(ctx context.Context, lessonID uuid.UUID) ([]domain.LessonTopic, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, lesson_id, title, order_index, slides, created_at, updated_at
		FROM lesson_topics
		WHERE lesson_id = $1
		ORDER BY order_index ASC
	`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topics := make([]domain.LessonTopic, 0)
	for rows.Next() {
		var t domain.LessonTopic
		var slidesBytes []byte
		if err := rows.Scan(&t.ID, &t.LessonID, &t.Title, &t.OrderIndex, &slidesBytes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if len(slidesBytes) > 0 {
			_ = json.Unmarshal(slidesBytes, &t.Slides)
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func toDomainLesson(l gen.Lesson) *domain.Lesson {
	lesson := &domain.Lesson{
		ID:               l.ID,
		CourseID:         l.CourseID,
		Title:            l.Title,
		OrderIdx:         int(l.OrderIdx),
		Keywords:         l.Keywords,
		LessonPlanStatus: l.LessonPlanStatus,
		CreatedAt:        l.CreatedAt,
		UpdatedAt:        l.UpdatedAt,
	}
	if l.SyllabusRowIndex.Valid {
		index := int(l.SyllabusRowIndex.Int32)
		lesson.SyllabusRowIndex = &index
	}
	if l.LessonPlanKey.Valid {
		lesson.LessonPlanKey = l.LessonPlanKey.String
	}
	if l.LessonPlanError.Valid {
		lesson.LessonPlanError = l.LessonPlanError.String
	}
	return lesson
}
