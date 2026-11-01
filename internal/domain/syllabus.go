package domain

import (
	"time"

	"github.com/google/uuid"
)

type SyllabusStatus string

const (
	SyllabusPending    SyllabusStatus = "pending"
	SyllabusProcessing SyllabusStatus = "processing"
	SyllabusReady      SyllabusStatus = "ready"
	SyllabusFailed     SyllabusStatus = "failed"
)

type SyllabusRow struct {
	Task         string `json:"task"`
	SubTask      string `json:"sub_task"`
	Objective    string `json:"objective"`
	Syllabus     string `json:"syllabus"`
	PeriodsT     string `json:"periods_t"`
	PeriodsP     string `json:"periods_p"`
	PeriodsF     string `json:"periods_f"`
	PeriodsTotal string `json:"periods_total"`
	SoK          string `json:"sok"`
	Remarks      string `json:"remarks"`
}

type ImportedSubject struct {
	CourseID    uuid.UUID `json:"course_id"`
	SubjectCode string    `json:"subject_code"`
	SubjectName string    `json:"subject_name"`
	LessonCount int       `json:"lesson_count"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
}

type SyllabusImportResult struct {
	FileName string            `json:"file_name"`
	Subjects []ImportedSubject `json:"subjects"`
}

// CourseSyllabus is the instructor-uploaded master syllabus and the generated
// topic outline (the "lesson plan") extracted from it.
type CourseSyllabus struct {
	CourseID    uuid.UUID      `json:"course_id"`
	FileName    string         `json:"file_name"`
	MinioKey    string         `json:"minio_key,omitempty"`
	SubjectCode string         `json:"subject_code,omitempty"`
	SubjectName string         `json:"subject_name,omitempty"`
	Status      SyllabusStatus `json:"status"`
	Topics      []string       `json:"topics"`
	Rows        []SyllabusRow  `json:"rows"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
