package domain

import (
	"time"

	"github.com/google/uuid"
)

type IngestStatus string

const (
	IngestPending    IngestStatus = "pending"
	IngestProcessing IngestStatus = "processing"
	IngestReady      IngestStatus = "ready"
	IngestFailed     IngestStatus = "failed"
)

type FileAsset struct {
	ID           uuid.UUID
	CourseID     uuid.UUID
	FileName     string
	MinioKey     string
	IngestStatus IngestStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
