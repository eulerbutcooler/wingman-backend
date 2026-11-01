package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type LessonPlanBacklogItem struct {
	CourseID     uuid.UUID `json:"course_id"`
	InstructorID uuid.UUID `json:"instructor_id"`
}

type Lesson struct {
	ID               uuid.UUID       `json:"id"`
	CourseID         uuid.UUID       `json:"course_id"`
	Title            string          `json:"title"`
	OrderIdx         int             `json:"order_idx"`
	Keywords         json.RawMessage `json:"keywords,omitempty"`
	SyllabusRowIndex *int            `json:"syllabus_row_index,omitempty"`
	LessonPlanKey    string          `json:"-"`
	LessonPlanStatus string          `json:"lesson_plan_status,omitempty"`
	LessonPlanError  string          `json:"lesson_plan_error,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
