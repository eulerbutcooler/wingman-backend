package service

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

var subjectCodePattern = regexp.MustCompile(`\((\d+)\)`)

const syllabusContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

type SyllabusService struct {
	courses     port.CourseRepository
	lessons     port.LessonRepository
	syllabus    port.SyllabusRepository
	storage     port.ObjectStorage
	ragClient   port.RagClient
	lessonPlans *LessonPlanService
	bucket      string
}

func NewSyllabusService(
	courses port.CourseRepository,
	lessons port.LessonRepository,
	syllabus port.SyllabusRepository,
	storage port.ObjectStorage,
	ragClient port.RagClient,
	lessonPlans *LessonPlanService,
	bucket string,
) *SyllabusService {
	return &SyllabusService{
		courses:     courses,
		lessons:     lessons,
		syllabus:    syllabus,
		storage:     storage,
		ragClient:   ragClient,
		lessonPlans: lessonPlans,
		bucket:      bucket,
	}
}

func (s *SyllabusService) Get(ctx context.Context, courseID uuid.UUID) (*domain.CourseSyllabus, error) {
	return s.syllabus.GetByCourse(ctx, courseID)
}

// GetFileURL returns a short-lived URL for the original master-syllabus DOCX.
func (s *SyllabusService) GetFileURL(ctx context.Context, courseID uuid.UUID) (string, error) {
	syllabus, err := s.syllabus.GetByCourse(ctx, courseID)
	if err != nil {
		return "", err
	}
	if syllabus.MinioKey == "" {
		return "", domain.ErrNotFound
	}
	return s.storage.PresignView(ctx, syllabus.MinioKey, syllabus.FileName, time.Hour)
}

func (s *SyllabusService) Upload(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
	fileName string,
	subjectCode string,
	data []byte,
) (*domain.CourseSyllabus, error) {
	if strings.ToLower(filepath.Ext(fileName)) != ".docx" {
		return nil, fmt.Errorf("%w: master syllabus must be a .docx file", domain.ErrBadInput)
	}

	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course.InstructorID != instructorID {
		return nil, domain.ErrForbidden
	}

	key := fmt.Sprintf("courses/%s/syllabus/%s.docx", courseID.String(), uuid.NewString())
	if err := s.storage.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), syllabusContentType); err != nil {
		return nil, fmt.Errorf("upload syllabus: %w", err)
	}

	subjectCode = strings.TrimSpace(subjectCode)
	var subjectName string
	if subjectCode == "" {
		subjectCode, subjectName = subjectFromCourseTitle(course.Title)
	} else {
		subjectName = strings.TrimSpace(subjectCodePattern.ReplaceAllString(course.Title, ""))
		if subjectName == "" {
			subjectName = course.Title
		}
	}
	syllabus := &domain.CourseSyllabus{
		CourseID:    courseID,
		FileName:    fileName,
		MinioKey:    key,
		SubjectCode: subjectCode,
		SubjectName: subjectName,
	}
	if err := s.syllabus.Upsert(ctx, syllabus); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, fmt.Errorf("save syllabus: %w", err)
	}

	extracted, err := s.ragClient.ExtractSyllabus(ctx, port.SyllabusExtractRequest{
		Bucket:      s.bucket,
		Key:         key,
		FileName:    fileName,
		SubjectCode: subjectCode,
		SubjectName: subjectName,
	})
	if err != nil {
		msg := fmt.Sprintf("syllabus extraction failed: %v", err)
		_ = s.syllabus.UpdateStatus(
			ctx,
			courseID,
			domain.SyllabusFailed,
			subjectCode,
			subjectName,
			nil,
			nil,
			msg,
		)
		syllabus.Status = domain.SyllabusFailed
		syllabus.Error = msg
		return syllabus, nil
	}

	rows := toDomainSyllabusRows(extracted.Rows)

	if err := s.syllabus.UpdateStatus(
		ctx,
		courseID,
		domain.SyllabusReady,
		subjectCode,
		subjectName,
		extracted.Topics,
		rows,
		"",
	); err != nil {
		return nil, fmt.Errorf("save syllabus topics: %w", err)
	}
	syllabus.Status = domain.SyllabusReady
	syllabus.Topics = extracted.Topics
	syllabus.Rows = rows
	return syllabus, nil
}

// ImportAll parses a multi-subject master syllabus and creates one draft
// course per subject, with one lesson per detailed-syllabus row.
func (s *SyllabusService) ImportAll(
	ctx context.Context,
	instructorID uuid.UUID,
	fileName string,
	rank string,
	generate bool,
	data []byte,
) (*domain.SyllabusImportResult, error) {
	if strings.ToLower(filepath.Ext(fileName)) != ".docx" {
		return nil, fmt.Errorf("%w: master syllabus must be a .docx file", domain.ErrBadInput)
	}
	if rank == "" {
		rank = "officer"
	}

	tempKey := fmt.Sprintf(
		"syllabus-imports/%s/%s.docx",
		instructorID.String(),
		uuid.NewString(),
	)
	if err := s.storage.Upload(
		ctx,
		tempKey,
		bytes.NewReader(data),
		int64(len(data)),
		syllabusContentType,
	); err != nil {
		return nil, fmt.Errorf("upload master syllabus: %w", err)
	}
	defer func() {
		_ = s.storage.Delete(context.Background(), tempKey)
	}()

	subjectResponse, err := s.ragClient.ListSyllabusSubjects(
		ctx,
		port.SyllabusSubjectsRequest{
			Bucket:   s.bucket,
			Key:      tempKey,
			FileName: fileName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("detect subjects: %w", err)
	}
	if len(subjectResponse.Subjects) == 0 {
		return nil, fmt.Errorf("%w: no subjects found in master syllabus", domain.ErrBadInput)
	}

	existing, _ := s.courses.ListByInstructor(ctx, instructorID)
	existingByTitle := make(map[string]*domain.Course, len(existing))
	for index := range existing {
		existingByTitle[strings.ToLower(existing[index].Title)] = &existing[index]
	}

	result := &domain.SyllabusImportResult{
		FileName: fileName,
		Subjects: make([]domain.ImportedSubject, 0, len(subjectResponse.Subjects)),
	}

	for _, subject := range subjectResponse.Subjects {
		title := fmt.Sprintf("%s (%s)", subject.Name, subject.Code)
		course := existingByTitle[strings.ToLower(title)]
		status := "updated"
		if course == nil {
			status = "created"
			course = &domain.Course{
				Title:        title,
				Description:  fmt.Sprintf("Imported from %s", fileName),
				Rank:         rank,
				InstructorID: instructorID,
				Published:    false,
			}
			if err := s.courses.Create(ctx, course); err != nil {
				result.Subjects = append(result.Subjects, domain.ImportedSubject{
					SubjectCode: subject.Code,
					SubjectName: subject.Name,
					LessonCount: len(subject.Rows),
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}
		}

		imported := domain.ImportedSubject{
			CourseID:    course.ID,
			SubjectCode: subject.Code,
			SubjectName: subject.Name,
			Status:      status,
		}
		lessonCount, err := s.syncImportedSubject(
			ctx,
			course,
			subject,
			fileName,
			data,
			generate,
		)
		if err != nil {
			imported.Status = "failed"
			imported.Error = err.Error()
			result.Subjects = append(result.Subjects, imported)
			continue
		}
		imported.LessonCount = lessonCount
		result.Subjects = append(result.Subjects, imported)
	}

	return result, nil
}

// syncImportedSubject replaces a course's syllabus and keeps one lesson per
// detailed-syllabus row. Existing lessons are updated in place so uploaded
// files and progress survive a revised master syllabus.
func (s *SyllabusService) syncImportedSubject(
	ctx context.Context,
	course *domain.Course,
	subject port.SyllabusSubject,
	fileName string,
	data []byte,
	generate bool,
) (int, error) {
	oldKey := ""
	if old, err := s.syllabus.GetByCourse(ctx, course.ID); err == nil && old != nil {
		oldKey = old.MinioKey
	}

	key := fmt.Sprintf(
		"courses/%s/syllabus/%s.docx",
		course.ID.String(),
		uuid.NewString(),
	)
	if err := s.storage.Upload(
		ctx,
		key,
		bytes.NewReader(data),
		int64(len(data)),
		syllabusContentType,
	); err != nil {
		return 0, fmt.Errorf("upload syllabus copy: %w", err)
	}

	syllabus := &domain.CourseSyllabus{
		CourseID:    course.ID,
		FileName:    fileName,
		MinioKey:    key,
		SubjectCode: subject.Code,
		SubjectName: subject.Name,
	}
	if err := s.syllabus.Upsert(ctx, syllabus); err != nil {
		_ = s.storage.Delete(ctx, key)
		return 0, fmt.Errorf("save syllabus: %w", err)
	}

	rows := toDomainSyllabusRows(subject.Rows)
	if err := s.syllabus.UpdateStatus(
		ctx,
		course.ID,
		domain.SyllabusReady,
		subject.Code,
		subject.Name,
		subject.Topics,
		rows,
		"",
	); err != nil {
		return 0, fmt.Errorf("save syllabus topics: %w", err)
	}

	existingLessons, err := s.lessons.ListByCourse(ctx, course.ID)
	if err != nil {
		return 0, fmt.Errorf("list lessons: %w", err)
	}
	byRow := make(map[int]*domain.Lesson, len(existingLessons))
	for index := range existingLessons {
		lesson := &existingLessons[index]
		if lesson.SyllabusRowIndex != nil {
			byRow[*lesson.SyllabusRowIndex] = lesson
		}
		if _, ok := byRow[index]; !ok {
			byRow[index] = lesson
		}
	}

	for index, row := range subject.Rows {
		topic := strings.TrimSpace(row.Syllabus)
		if topic == "" {
			topic = strings.TrimSpace(row.SubTask)
		}
		title := fmt.Sprintf("LP %02d - %s", index+1, truncateText(topic, 180))

		if lesson, ok := byRow[index]; ok {
			lesson.Title = title
			lesson.OrderIdx = index
			if err := s.lessons.Update(ctx, lesson); err != nil {
				return index, fmt.Errorf("update lesson %d: %w", index+1, err)
			}
			// The old slides, keywords and PDF belong to the old syllabus.
			if err := s.lessons.CreateTopics(ctx, lesson.ID, nil); err != nil {
				return index, fmt.Errorf("clear lesson topics %d: %w", index+1, err)
			}
			_ = s.lessons.UpdateKeywords(ctx, lesson.ID, nil)
			_, _ = s.lessons.UpdateLessonPlan(ctx, lesson.ID, "", "pending", "")
			continue
		}

		rowIndex := index
		lesson := &domain.Lesson{
			CourseID:         course.ID,
			Title:            title,
			OrderIdx:         index,
			SyllabusRowIndex: &rowIndex,
		}
		if err := s.lessons.CreateSyllabusLesson(ctx, lesson); err != nil {
			return index, fmt.Errorf("create lesson %d: %w", index+1, err)
		}
	}

	if oldKey != "" && oldKey != key {
		_ = s.storage.Delete(ctx, oldKey)
	}

	// Lesson plans are independent of supporting material and can start
	// immediately. Slides, question banks and quizzes are deliberately
	// deferred until the instructor uploads PDFs/PPTs and triggers
	// "Generate content" from the subject builder.
	if generate {
		if err := s.lessonPlans.StartGeneration(
			ctx,
			course.ID,
			course.InstructorID,
		); err != nil {
			return len(subject.Rows), fmt.Errorf("queue lesson-plan generation: %w", err)
		}
	}

	return len(subject.Rows), nil
}

func toDomainSyllabusRows(rows []port.SyllabusRow) []domain.SyllabusRow {
	out := make([]domain.SyllabusRow, len(rows))
	for i, row := range rows {
		out[i] = domain.SyllabusRow{
			Task:         row.Task,
			SubTask:      row.SubTask,
			Objective:    row.Objective,
			Syllabus:     row.Syllabus,
			PeriodsT:     row.PeriodsT,
			PeriodsP:     row.PeriodsP,
			PeriodsF:     row.PeriodsF,
			PeriodsTotal: row.PeriodsTotal,
			SoK:          row.SoK,
			Remarks:      row.Remarks,
		}
	}
	return out
}

func truncateText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

func subjectFromCourseTitle(title string) (code string, name string) {
	name = strings.TrimSpace(title)
	if match := subjectCodePattern.FindStringSubmatch(title); len(match) > 1 {
		code = match[1]
		name = strings.TrimSpace(subjectCodePattern.ReplaceAllString(title, ""))
	}
	return code, name
}
