package domain

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

type Event struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CourseID  *uuid.UUID
	Type      EventType
	Metadata  map[string]any
	CreatedAt time.Time
}

type Metric struct {
	CourseID      uuid.UUID
	TotalStudents int
	AvgQuizScore  float64
	TotalMessages int
	TotalFiles    int
}

type StudentScore struct {
	UserID   uuid.UUID
	Name     string
	Rank     string
	AvgScore float64
}

type Overview struct {
	TotalStudents int
	TotalCourses  int
	AvgScore      float64
}
