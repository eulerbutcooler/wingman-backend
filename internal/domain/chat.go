package domain

import (
	"time"

	"github.com/google/uuid"
)

type ChatSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CourseID  *uuid.UUID
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

type Citation struct {
	FileName string
	FileID   string
	Score    *float64
}

type Message struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	Role      MessageRole
	Content   string
	Citations []Citation
	CreatedAt time.Time
}
