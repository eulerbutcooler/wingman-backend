package domain

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Rank         string    `json:"rank"`
	InstructorID uuid.UUID `json:"instructor_id"`
	Published    bool      `json:"published"`
	ThumbnailKey string    `json:"thumbnail_key,omitempty"`
	// ThumbnailURL is a short-lived presigned URL derived from ThumbnailKey.
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
