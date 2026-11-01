package port

import (
	"context"
	"io"
)

type RagClient interface {
	ChatStream(ctx context.Context, req ChatRequest) (io.ReadCloser, error)
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	GradeAnswer(ctx context.Context, req GradeRequest) (*GradeResponse, error)
	ExtractSyllabus(ctx context.Context, req SyllabusExtractRequest) (*SyllabusExtractResponse, error)
	ListSyllabusSubjects(ctx context.Context, req SyllabusSubjectsRequest) (*SyllabusSubjectsResponse, error)
	GenerateLessonPlans(ctx context.Context, req LessonPlanGenerateRequest) (*LessonPlanGenerateResponse, error)
	DeleteCourse(ctx context.Context, courseID string) error
	DeleteFileVectors(ctx context.Context, fileID string) error
}

type SyllabusExtractRequest struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	FileName    string `json:"file_name"`
	SubjectCode string `json:"subject_code,omitempty"`
	SubjectName string `json:"subject_name,omitempty"`
}

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

type SyllabusExtractResponse struct {
	Topics      []string      `json:"topics"`
	Rows        []SyllabusRow `json:"rows"`
	SubjectCode string        `json:"subject_code,omitempty"`
	SubjectName string        `json:"subject_name,omitempty"`
	Section     string        `json:"section,omitempty"`
}

type SyllabusSubjectsRequest struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	FileName string `json:"file_name"`
}

type SyllabusSubject struct {
	Code   string        `json:"code"`
	Name   string        `json:"name"`
	Rows   []SyllabusRow `json:"rows"`
	Topics []string      `json:"topics"`
}

type SyllabusSubjectsResponse struct {
	Subjects []SyllabusSubject `json:"subjects"`
}

type LessonPlanGenerateRequest struct {
	Bucket         string `json:"bucket"`
	Key            string `json:"key"`
	FileName       string `json:"file_name"`
	SubjectCode    string `json:"subject_code"`
	SubjectName    string `json:"subject_name"`
	ClassName      string `json:"class_name,omitempty"`
	SessionMinutes int    `json:"session_minutes,omitempty"`
}

type LessonPlanPDF struct {
	RowIndex     int    `json:"row_index"`
	LessonPlanNo string `json:"lesson_plan_no"`
	Title        string `json:"title"`
	PDFBase64    string `json:"pdf_base64"`
}

type LessonPlanGenerateResponse struct {
	SubjectCode string          `json:"subject_code"`
	SubjectName string          `json:"subject_name"`
	Plans       []LessonPlanPDF `json:"plans"`
}

type ChatRequest struct {
	CourseIDs []string      `json:"course_ids"`
	Query     string        `json:"query"`
	Stream    bool          `json:"stream"`
	History   []ChatMessage `json:"history"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Answer    string         `json:"answer"`
	Citations []CitationItem `json:"citations"`
}

type CitationItem struct {
	FileName string   `json:"file_name"`
	FileID   string   `json:"file_id"`
	Score    *float64 `json:"score,omitempty"`
}

type GradeRequest struct {
	Question        string `json:"question"`
	ReferenceAnswer string `json:"reference_answer"`
	UserAnswer      string `json:"user_answer"`
}

type GradeResponse struct {
	IsCorrect   bool    `json:"is_correct"`
	Score       float64 `json:"score"`
	Explanation string  `json:"explanation"`
}
