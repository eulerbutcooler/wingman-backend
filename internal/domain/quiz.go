package domain

import (
	"time"

	"github.com/google/uuid"
)

type QuizStatus string

const (
	QuizPending    QuizStatus = "pending"
	QuizGenerating QuizStatus = "generating"
	QuizReady      QuizStatus = "ready"
	QuizFailed     QuizStatus = "failed"
)

type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

type Quiz struct {
	ID         uuid.UUID
	CourseID   uuid.UUID
	Difficulty Difficulty
	Status     QuizStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type QuestionType string

const (
	QuestionMCQ       QuestionType = "mcq"
	QuestionOpenEnded QuestionType = "open_ended"
)

type Choice struct {
	Label string // "A", "B", "C", "D"
	Text  string
}

type Question struct {
	ID       uuid.UUID
	QuizID   uuid.UUID
	Type     QuestionType
	Question string
	Choices  []Choice
	Answer   string
	OrderIdx int
}

type Attempt struct {
	ID        uuid.UUID
	QuizID    uuid.UUID
	UserID    uuid.UUID
	Score     float64
	Total     int
	StartedAt time.Time
	EndedAt   *time.Time
}

type Answer struct {
	ID         uuid.UUID
	AttemptID  uuid.UUID
	QuestionID uuid.UUID
	UserAnswer string
	IsCorrect  bool
}
