package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type syllabusRepo struct {
	pool *pgxpool.Pool
}

func NewSyllabusRepo(pool *pgxpool.Pool) port.SyllabusRepository {
	return &syllabusRepo{pool: pool}
}

func (r *syllabusRepo) Upsert(ctx context.Context, syllabus *domain.CourseSyllabus) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO course_syllabus (
			course_id, file_name, minio_key, subject_code, subject_name,
			status, topics, rows, error
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), 'processing', '[]'::jsonb, '[]'::jsonb, NULL)
		ON CONFLICT (course_id) DO UPDATE
		SET file_name = EXCLUDED.file_name,
		    minio_key = EXCLUDED.minio_key,
		    subject_code = EXCLUDED.subject_code,
		    subject_name = EXCLUDED.subject_name,
		    status = 'processing',
		    topics = '[]'::jsonb,
		    rows = '[]'::jsonb,
		    error = NULL,
		    updated_at = now()
		RETURNING created_at, updated_at
	`, syllabus.CourseID, syllabus.FileName, syllabus.MinioKey, syllabus.SubjectCode, syllabus.SubjectName)
	if err := row.Scan(&syllabus.CreatedAt, &syllabus.UpdatedAt); err != nil {
		return err
	}
	syllabus.Status = domain.SyllabusProcessing
	syllabus.Topics = []string{}
	syllabus.Rows = []domain.SyllabusRow{}
	return nil
}

func (r *syllabusRepo) GetByCourse(ctx context.Context, courseID uuid.UUID) (*domain.CourseSyllabus, error) {
	var syllabus domain.CourseSyllabus
	var topicsBytes, rowsBytes []byte
	var errText, subjectCode, subjectName *string
	err := r.pool.QueryRow(ctx, `
		SELECT course_id, file_name, minio_key, subject_code, subject_name,
		       status, topics, rows, error, created_at, updated_at
		FROM course_syllabus
		WHERE course_id = $1
	`, courseID).Scan(
		&syllabus.CourseID,
		&syllabus.FileName,
		&syllabus.MinioKey,
		&subjectCode,
		&subjectName,
		&syllabus.Status,
		&topicsBytes,
		&rowsBytes,
		&errText,
		&syllabus.CreatedAt,
		&syllabus.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if subjectCode != nil {
		syllabus.SubjectCode = *subjectCode
	}
	if subjectName != nil {
		syllabus.SubjectName = *subjectName
	}
	if len(topicsBytes) > 0 {
		if err := json.Unmarshal(topicsBytes, &syllabus.Topics); err != nil {
			return nil, err
		}
	}
	if len(rowsBytes) > 0 {
		if err := json.Unmarshal(rowsBytes, &syllabus.Rows); err != nil {
			return nil, err
		}
	}
	if errText != nil {
		syllabus.Error = *errText
	}
	return &syllabus, nil
}

func (r *syllabusRepo) UpdateStatus(
	ctx context.Context,
	courseID uuid.UUID,
	status domain.SyllabusStatus,
	subjectCode string,
	subjectName string,
	topics []string,
	rows []domain.SyllabusRow,
	errMsg string,
) error {
	if topics == nil {
		topics = []string{}
	}
	if rows == nil {
		rows = []domain.SyllabusRow{}
	}
	topicsJSON, err := json.Marshal(topics)
	if err != nil {
		return err
	}
	rowsJSON, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE course_syllabus
		SET status = $2,
		    subject_code = NULLIF($3, ''),
		    subject_name = NULLIF($4, ''),
		    topics = $5,
		    rows = $6,
		    error = NULLIF($7, ''),
		    updated_at = now()
		WHERE course_id = $1
	`, courseID, status, subjectCode, subjectName, topicsJSON, rowsJSON, errMsg)
	return err
}
