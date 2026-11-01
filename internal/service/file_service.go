package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

const viewURLExpiry = 1 * time.Hour

type FileService struct {
	files     port.FileRepository
	lessons   port.LessonRepository
	courses   port.CourseRepository
	storage   port.ObjectStorage
	ragClient port.RagClient
}

func NewFileService(
	files port.FileRepository,
	lessons port.LessonRepository,
	courses port.CourseRepository,
	storage port.ObjectStorage,
	ragClient port.RagClient,
) *FileService {
	return &FileService{
		files:     files,
		lessons:   lessons,
		courses:   courses,
		storage:   storage,
		ragClient: ragClient,
	}
}

// GetIngestStatus is intentionally uncached. The frontend polls this while a
// file is being ingested, and a TTL cache here makes completed files look
// "processing" long after the worker has finished.
func (s *FileService) GetIngestStatus(ctx context.Context, fileID uuid.UUID) (domain.IngestStatus, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	return file.IngestStatus, nil
}

func (s *FileService) GetViewURL(ctx context.Context, fileID uuid.UUID) (string, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	return s.storage.PresignView(ctx, file.MinioKey, file.FileName, viewURLExpiry)
}

// ListByLesson is intentionally uncached. It is the endpoint the upload UI
// polls for ingest progress; caching it for two minutes hid newly uploaded
// files and kept finished files stuck on "processing".
func (s *FileService) ListByLesson(ctx context.Context, lessonID uuid.UUID) ([]domain.FileAsset, error) {
	return s.files.ListByLesson(ctx, lessonID)
}

// Delete removes a file's vectors, stored object, and database row. Vectors go
// first: if Qdrant is unavailable we keep the row so the instructor can retry
// instead of leaving chat able to cite a file that no longer exists.
func (s *FileService) Delete(ctx context.Context, fileID, instructorID uuid.UUID) error {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return err
	}

	lesson, err := s.lessons.GetByID(ctx, file.LessonID)
	if err != nil {
		return err
	}

	course, err := s.courses.GetByID(ctx, lesson.CourseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}

	// Videos are stored as reference material and are not indexed in Qdrant.
	if file.FileType != domain.FileTypeVideo {
		if err := s.ragClient.DeleteFileVectors(ctx, fileID.String()); err != nil {
			return fmt.Errorf("delete file vectors: %w", err)
		}
	}

	if err := s.storage.Delete(ctx, file.MinioKey); err != nil {
		return fmt.Errorf("delete stored object: %w", err)
	}

	return s.files.Delete(ctx, fileID)
}
