package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

const viewURLExpiry = 1 * time.Hour

// Cache keys used by FileService. Exported so other components (e.g. the
// ingest-done worker) can invalidate the same entries after writes instead
// of serving stale data until the TTL expires.
const (
	CacheKeyFileStatus  = "file:status:"
	CacheKeyFilesLesson = "files:lesson:"
)

type FileService struct {
	files   port.FileRepository
	storage port.ObjectStorage
	cache   port.Cache
}

func NewFileService(files port.FileRepository, storage port.ObjectStorage, cache port.Cache) *FileService {
	return &FileService{files: files, storage: storage, cache: cache}
}

func (s *FileService) GetIngestStatus(ctx context.Context, fileID uuid.UUID) (domain.IngestStatus, error) {
	key := CacheKeyFileStatus + fileID.String()

	if cached, err := s.cache.Get(ctx, key); err == nil {
		return domain.IngestStatus(cached), nil
	}

	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}

	s.cache.Set(ctx, key, string(file.IngestStatus), 30*time.Second)
	return file.IngestStatus, nil
}

func (s *FileService) GetViewURL(ctx context.Context, fileID uuid.UUID) (string, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	return s.storage.PresignView(ctx, file.MinioKey, viewURLExpiry)
}

func (s *FileService) ListByLesson(ctx context.Context, lessonID uuid.UUID) ([]domain.FileAsset, error) {
	key := CacheKeyFilesLesson + lessonID.String()

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var files []domain.FileAsset
		if json.Unmarshal([]byte(cached), &files) == nil {
			return files, nil
		}
	}

	files, err := s.files.ListByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(files); err == nil {
		s.cache.Set(ctx, key, string(data), 2*time.Minute)
	}
	return files, nil
}
