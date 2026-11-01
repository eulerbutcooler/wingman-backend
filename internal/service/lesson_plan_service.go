package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

const lessonPlanContentType = "application/pdf"

type LessonPlanService struct {
	courses  port.CourseRepository
	lessons  port.LessonRepository
	syllabus port.SyllabusRepository
	storage  port.ObjectStorage
	rag      port.RagClient
	bucket   string
	sem      chan struct{}
	mu       sync.Mutex
	active   map[uuid.UUID]bool
}

func NewLessonPlanService(
	courses port.CourseRepository,
	lessons port.LessonRepository,
	syllabus port.SyllabusRepository,
	storage port.ObjectStorage,
	rag port.RagClient,
	bucket string,
) *LessonPlanService {
	return &LessonPlanService{
		courses:  courses,
		lessons:  lessons,
		syllabus: syllabus,
		storage:  storage,
		rag:      rag,
		bucket:   bucket,
		// Temporary throughput experiment: four subjects at a time on the
		// 4-slot Modal llama.cpp server. Revert to 2 for the local 3-slot stack.
		sem:    make(chan struct{}, 4),
		active: make(map[uuid.UUID]bool),
	}
}

func (s *LessonPlanService) tryStart(courseID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[courseID] {
		return false
	}
	s.active[courseID] = true
	return true
}

func (s *LessonPlanService) finish(courseID uuid.UUID) {
	s.mu.Lock()
	delete(s.active, courseID)
	s.mu.Unlock()
}

// ResumeBacklog re-queues courses whose lesson-plan PDFs were marked
// generating/pending before a backend restart. The in-memory goroutine queue
// does not survive restarts, so this runs once at startup.
func (s *LessonPlanService) ResumeBacklog(ctx context.Context) {
	items, err := s.lessons.ListLessonPlanBacklog(ctx)
	if err != nil {
		fmt.Printf("lesson_plan_backlog_query_failed error=%v\n", err)
		return
	}
	if len(items) == 0 {
		return
	}
	fmt.Printf("lesson_plan_backlog_resume courses=%d\n", len(items))
	for _, item := range items {
		if err := s.StartGeneration(ctx, item.CourseID, item.InstructorID); err != nil {
			fmt.Printf(
				"lesson_plan_backlog_enqueue_failed course_id=%s error=%v\n",
				item.CourseID,
				err,
			)
		}
	}
}

// StartGeneration marks the course's plans as generating and runs the slow
// PDF generation in the background. The HTTP request returns immediately.
func (s *LessonPlanService) StartGeneration(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
) error {
	if !s.tryStart(courseID) {
		return nil
	}
	launched := false
	defer func() {
		if !launched {
			s.finish(courseID)
		}
	}()

	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}

	syllabus, err := s.syllabus.GetByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if syllabus.Status != domain.SyllabusReady {
		return fmt.Errorf("master syllabus is not ready")
	}
	if syllabus.MinioKey == "" {
		return fmt.Errorf("master syllabus file is missing")
	}

	lessons, err := s.lessons.ListByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if len(lessons) == 0 {
		return fmt.Errorf("course has no lessons")
	}

	if err := s.lessons.SetCourseLessonPlanStatus(
		ctx,
		courseID,
		"generating",
		"",
	); err != nil {
		return err
	}

	copied := *syllabus
	launched = true
	go func() {
		defer s.finish(courseID)
		s.generate(context.Background(), courseID, copied)
	}()
	return nil
}

func (s *LessonPlanService) generate(
	ctx context.Context,
	courseID uuid.UUID,
	syllabus domain.CourseSyllabus,
) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	response, err := s.rag.GenerateLessonPlans(ctx, port.LessonPlanGenerateRequest{
		Bucket:         s.bucket,
		Key:            syllabus.MinioKey,
		FileName:       syllabus.FileName,
		SubjectCode:    syllabus.SubjectCode,
		SubjectName:    syllabus.SubjectName,
		ClassName:      "Air Agniveer Apprentice (Engineering)",
		SessionMinutes: 80,
	})
	if err != nil {
		_ = s.lessons.SetCourseLessonPlanStatus(ctx, courseID, "failed", err.Error())
		return
	}

	lessons, err := s.lessons.ListByCourse(ctx, courseID)
	if err != nil {
		_ = s.lessons.SetCourseLessonPlanStatus(ctx, courseID, "failed", err.Error())
		return
	}

	// Prefer the explicit syllabus row index; fall back to lesson order for
	// courses created before the importer existed.
	lessonByRow := make(map[int]uuid.UUID, len(lessons))
	for index, lesson := range lessons {
		if lesson.SyllabusRowIndex != nil {
			lessonByRow[*lesson.SyllabusRowIndex] = lesson.ID
		}
		if _, ok := lessonByRow[index]; !ok {
			lessonByRow[index] = lesson.ID
		}
	}

	ready := make(map[uuid.UUID]bool, len(response.Plans))
	for _, plan := range response.Plans {
		lessonID, ok := lessonByRow[plan.RowIndex]
		if !ok {
			continue
		}
		pdfBytes, err := base64.StdEncoding.DecodeString(plan.PDFBase64)
		if err != nil {
			_, _ = s.lessons.UpdateLessonPlan(
				ctx,
				lessonID,
				"",
				"failed",
				fmt.Sprintf("decode PDF: %v", err),
			)
			continue
		}
		key := fmt.Sprintf(
			"courses/%s/lesson-plans/lp%02d.pdf",
			courseID.String(),
			plan.RowIndex+1,
		)
		if err := s.storage.Upload(
			ctx,
			key,
			bytes.NewReader(pdfBytes),
			int64(len(pdfBytes)),
			lessonPlanContentType,
		); err != nil {
			_, _ = s.lessons.UpdateLessonPlan(
				ctx,
				lessonID,
				"",
				"failed",
				fmt.Sprintf("upload PDF: %v", err),
			)
			continue
		}
		if _, err := s.lessons.UpdateLessonPlan(ctx, lessonID, key, "ready", ""); err != nil {
			continue
		}
		ready[lessonID] = true
	}

	for _, lesson := range lessons {
		if ready[lesson.ID] {
			continue
		}
		_, _ = s.lessons.UpdateLessonPlan(
			ctx,
			lesson.ID,
			"",
			"failed",
			"lesson plan was not generated",
		)
	}
}

func (s *LessonPlanService) GetURL(
	ctx context.Context,
	lessonID uuid.UUID,
) (string, error) {
	lesson, err := s.lessons.GetByID(ctx, lessonID)
	if err != nil {
		return "", err
	}
	if lesson.LessonPlanKey == "" {
		return "", domain.ErrNotFound
	}
	return s.storage.PresignView(
		ctx,
		lesson.LessonPlanKey,
		fmt.Sprintf("lesson-plan-%s.pdf", lesson.Title),
		time.Hour,
	)
}
