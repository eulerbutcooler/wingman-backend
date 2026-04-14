package domain

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	ID           uuid.UUID
	Title        string
	Description  string
	Rank         string
	InstructorID uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
