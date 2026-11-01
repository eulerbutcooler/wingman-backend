package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/infra/nats"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

type CourseService struct {
	courses   port.CourseRepository
	lessons   port.LessonRepository
	quizzes   port.QuizRepository
	files     port.FileRepository
	syllabus  port.SyllabusRepository
	queue     port.MessageQueue
	cache     port.Cache
	storage   port.ObjectStorage
	ragClient port.RagClient
}

func NewCourseService(courses port.CourseRepository, lessons port.LessonRepository, quizzes port.QuizRepository, files port.FileRepository, syllabus port.SyllabusRepository, queue port.MessageQueue, cache port.Cache, storage port.ObjectStorage, ragClient port.RagClient) *CourseService {
	return &CourseService{courses: courses, lessons: lessons, quizzes: quizzes, files: files, syllabus: syllabus, queue: queue, cache: cache, storage: storage, ragClient: ragClient}
}

func (s *CourseService) Create(ctx context.Context, title, desc, rank string, instructorID uuid.UUID) (*domain.Course, error) {
	course := &domain.Course{
		Title:        title,
		Description:  desc,
		Rank:         rank,
		InstructorID: instructorID,
		Published:    false,
	}
	if err := s.courses.Create(ctx, course); err != nil {
		return nil, err
	}
	s.cache.Delete(ctx, "courses:rank:"+rank)
	s.cache.Delete(ctx, "courses:all")
	return course, nil
}

func (s *CourseService) GetByID(ctx context.Context, courseID uuid.UUID, userRole string, userID uuid.UUID) (*domain.Course, error) {
	key := "course:" + courseID.String()

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var course domain.Course
		if json.Unmarshal([]byte(cached), &course) == nil {
			if !course.Published && userRole != "instructor" {
				return nil, domain.ErrNotFound
			}
			s.enrichThumbnailURL(ctx, &course)
			return &course, nil
		}
	}

	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if !course.Published && userRole != "instructor" {
		return nil, domain.ErrNotFound
	}

	s.enrichThumbnailURL(ctx, course)
	if data, err := json.Marshal(course); err == nil {
		s.cache.Set(ctx, key, string(data), 10*time.Minute)
	}
	return course, nil
}

func (s *CourseService) ListByRank(ctx context.Context, rank string) ([]domain.Course, error) {
	key := "courses:rank:" + rank

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var courses []domain.Course
		if json.Unmarshal([]byte(cached), &courses) == nil {
			s.enrichCourseThumbnails(ctx, courses)
			return courses, nil
		}
	}

	courses, err := s.courses.ListByRank(ctx, rank)
	if err != nil {
		return nil, err
	}

	s.enrichCourseThumbnails(ctx, courses)
	if data, err := json.Marshal(courses); err == nil {
		s.cache.Set(ctx, key, string(data), 5*time.Minute)
	}
	return courses, nil
}

func (s *CourseService) ListAll(ctx context.Context) ([]domain.Course, error) {
	key := "courses:all"

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var courses []domain.Course
		if json.Unmarshal([]byte(cached), &courses) == nil {
			s.enrichCourseThumbnails(ctx, courses)
			return courses, nil
		}
	}

	courses, err := s.courses.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	s.enrichCourseThumbnails(ctx, courses)
	if data, err := json.Marshal(courses); err == nil {
		s.cache.Set(ctx, key, string(data), 5*time.Minute)
	}
	return courses, nil
}

func (s *CourseService) ListByInstructor(ctx context.Context, instructorID uuid.UUID) ([]domain.Course, error) {
	courses, err := s.courses.ListByInstructor(ctx, instructorID)
	if err != nil {
		return nil, err
	}
	s.enrichCourseThumbnails(ctx, courses)
	return courses, nil
}

func (s *CourseService) Update(ctx context.Context, courseID, instructorID uuid.UUID, title, desc, rank string) (*domain.Course, error) {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course.InstructorID != instructorID {
		return nil, domain.ErrForbidden
	}

	oldRank := course.Rank
	course.Title = title
	course.Description = desc
	course.Rank = rank
	if err := s.courses.Update(ctx, course); err != nil {
		return nil, err
	}

	s.cache.Delete(ctx, "course:"+courseID.String())
	s.cache.Delete(ctx, "courses:rank:"+oldRank)
	s.cache.Delete(ctx, "courses:all")
	if rank != oldRank {
		s.cache.Delete(ctx, "courses:rank:"+rank)
	}
	s.enrichThumbnailURL(ctx, course)
	return course, nil
}

func (s *CourseService) UploadThumbnail(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
	reader io.Reader,
	size int64,
	contentType, ext string,
) (*domain.Course, error) {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course.InstructorID != instructorID {
		return nil, domain.ErrForbidden
	}

	key := fmt.Sprintf("courses/%s/thumbnail/%s.%s", courseID.String(), uuid.NewString(), ext)
	if err := s.storage.Upload(ctx, key, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("upload thumbnail: %w", err)
	}

	if err := s.courses.UpdateThumbnail(ctx, courseID, key); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, fmt.Errorf("update thumbnail: %w", err)
	}

	if course.ThumbnailKey != "" && course.ThumbnailKey != key {
		if err := s.storage.Delete(ctx, course.ThumbnailKey); err != nil {
			fmt.Printf("warning: failed to delete old thumbnail from minio: %v\n", err)
		}
	}

	course.ThumbnailKey = key
	course.ThumbnailURL = ""
	s.enrichThumbnailURL(ctx, course)
	s.invalidateCourseCaches(ctx, course)
	return course, nil
}

func (s *CourseService) Publish(ctx context.Context, courseID, instructorID uuid.UUID) error {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}
	if err := s.courses.Publish(ctx, courseID); err != nil {
		return err
	}
	s.cache.Delete(ctx, "course:"+courseID.String())
	s.cache.Delete(ctx, "courses:all")
	s.cache.Delete(ctx, "courses:rank:"+course.Rank)
	return nil
}

func (s *CourseService) Finalize(ctx context.Context, courseID, instructorID uuid.UUID) error {
	return s.generateContent(ctx, courseID, instructorID, true)
}

// GenerateContent creates the lesson quizzes and queues slide/question
// generation without publishing the course. Used by the master-syllabus
// importer so imported subjects stay as drafts until the instructor reviews them.
func (s *CourseService) GenerateContent(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
) error {
	return s.generateContent(ctx, courseID, instructorID, false)
}

func (s *CourseService) generateContent(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
	publish bool,
) error {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}

	if publish && !course.Published {
		if err := s.courses.Publish(ctx, courseID); err != nil {
			return err
		}
	}

	if !publish {
		existing, err := s.quizzes.ListQuizzesByCourse(ctx, courseID)
		if err != nil {
			return err
		}
		for _, quiz := range existing {
			if quiz.Status == domain.QuizGenerating {
				return fmt.Errorf(
					"%w: content generation is already running",
					domain.ErrConflict,
				)
			}
		}
	}

	// Delete old quizzes and regenerate.
	if err := s.quizzes.DeleteQuizzesByCourse(ctx, courseID); err != nil {
		return fmt.Errorf("delete quizzes: %w", err)
	}

	lessons, err := s.lessons.ListByCourse(ctx, courseID)
	if err != nil {
		return fmt.Errorf("list lessons: %w", err)
	}
	if len(lessons) == 0 {
		return fmt.Errorf("course must have at least one lesson to generate content")
	}

	// The master syllabus is the preferred topic source. If it is ready, its
	// generated topics are split across lessons and sent to the quiz worker.
	var syllabusTopics []string
	var syllabusRows []domain.SyllabusRow
	if syllabus, err := s.syllabus.GetByCourse(ctx, courseID); err == nil &&
		syllabus != nil &&
		syllabus.Status == domain.SyllabusReady {
		syllabusTopics = syllabus.Topics
		syllabusRows = syllabus.Rows
	}

	lessonQuizzesQueued := 0

	for index, lesson := range lessons {
		lessonID := lesson.ID // capture loop var

		files, err := s.files.ListByLesson(ctx, lessonID)
		if err != nil {
			return fmt.Errorf("list files for lesson %s: %w", lessonID, err)
		}

		var fileID string
		// Legacy fallback: DOCX lesson plan topic extraction.
		for _, f := range files {
			if f.IngestStatus == domain.IngestReady && f.FileType == domain.FileTypeDOCX {
				fileID = f.ID.String()
				break
			}
		}

		lessonTopics := distributeSyllabusTopics(syllabusTopics, len(lessons), index)
		// When the master syllabus has one row per lesson, use that row's exact
		// topic. This preserves duplicate topics that the deduplicated Topics
		// list would otherwise drop and leave a lesson without a quiz topic.
		if len(syllabusRows) == len(lessons) {
			rowIndex := index
			if lesson.SyllabusRowIndex != nil {
				rowIndex = *lesson.SyllabusRowIndex
			}
			if rowIndex >= 0 && rowIndex < len(syllabusRows) {
				topic := strings.TrimSpace(syllabusRows[rowIndex].Syllabus)
				if topic == "" {
					topic = strings.TrimSpace(syllabusRows[rowIndex].SubTask)
				}
				if topic != "" {
					lessonTopics = []string{topic}
				}
			}
		}

		// Without syllabus topics or a DOCX lesson plan there is no topic source.
		if len(lessonTopics) == 0 && fileID == "" {
			continue
		}

		quiz := &domain.Quiz{
			CourseID:   courseID,
			LessonID:   &lessonID,
			Difficulty: domain.DifficultyMedium,
			Status:     domain.QuizGenerating,
		}
		if err := s.quizzes.CreateQuiz(ctx, quiz); err != nil {
			return fmt.Errorf("create lesson quiz %s: %w", lesson.ID, err)
		}

		payload, err := json.Marshal(map[string]any{
			"type":         "lesson",
			"request_id":   uuid.NewString(),
			"quiz_id":      quiz.ID.String(),
			"course_id":    courseID.String(),
			"lesson_id":    lesson.ID.String(),
			"file_id":      fileID,
			"keywords":     lessonTopics,
			"difficulty":   string(domain.DifficultyMedium),
			"limit_chunks": 20,
		})
		if err != nil {
			return fmt.Errorf("marshal lesson quiz request: %w", err)
		}

		if err := s.queue.Publish(ctx, nats.SubjectQuizRequest, payload); err != nil {
			return fmt.Errorf("publish lesson quiz request %s: %w", lesson.ID, err)
		}
		lessonQuizzesQueued++
	}

	// If no lesson quizzes were queued (because no lesson had a DOCX file),
	// trigger the course quizzes immediately.
	if lessonQuizzesQueued == 0 {
		for _, diff := range []domain.Difficulty{domain.DifficultyMedium, domain.DifficultyHard} {
			q := &domain.Quiz{
				CourseID:   courseID,
				Difficulty: diff,
				Status:     domain.QuizReady,
			}
			_ = s.quizzes.CreateQuiz(ctx, q)
		}
	}

	s.cache.Delete(ctx, "course:"+courseID.String())
	s.cache.Delete(ctx, "quizzes:course:"+courseID.String())
	s.cache.Delete(ctx, "courses:all")
	s.cache.Delete(ctx, "courses:rank:"+course.Rank)
	return nil
}

func (s *CourseService) Delete(ctx context.Context, courseID, instructorID uuid.UUID) error {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}

	// 1. Delete Qdrant vectors
	if err := s.ragClient.DeleteCourse(ctx, courseID.String()); err != nil {
		fmt.Printf("warning: failed to delete course from rag: %v\n", err)
	}

	// 2. Delete Minio files and the course thumbnail
	if course.ThumbnailKey != "" {
		if err := s.storage.Delete(ctx, course.ThumbnailKey); err != nil {
			fmt.Printf("warning: failed to delete course thumbnail from minio: %v\n", err)
		}
	}
	if syllabus, err := s.syllabus.GetByCourse(ctx, courseID); err == nil && syllabus != nil && syllabus.MinioKey != "" {
		if err := s.storage.Delete(ctx, syllabus.MinioKey); err != nil {
			fmt.Printf("warning: failed to delete master syllabus from minio: %v\n", err)
		}
	}
	if lessons, err := s.lessons.ListAllByCourse(ctx, courseID); err == nil {
		for _, lesson := range lessons {
			if lesson.LessonPlanKey != "" {
				if err := s.storage.Delete(ctx, lesson.LessonPlanKey); err != nil {
					fmt.Printf(
						"warning: failed to delete lesson plan %s from minio: %v\n",
						lesson.LessonPlanKey,
						err,
					)
				}
			}
			if files, err := s.files.ListByLesson(ctx, lesson.ID); err == nil {
				for _, f := range files {
					if err := s.storage.Delete(ctx, f.MinioKey); err != nil {
						fmt.Printf("warning: failed to delete file %s from minio: %v\n", f.MinioKey, err)
					}
				}
			}
		}
	}

	// 3. Delete from Postgres (cascades)
	if err := s.courses.Delete(ctx, courseID); err != nil {
		return err
	}
	s.cache.Delete(ctx, "course:"+courseID.String())
	s.cache.Delete(ctx, "courses:rank:"+course.Rank)
	s.cache.Delete(ctx, "courses:all")
	return nil
}

// Lesson methods

func (s *CourseService) CreateLesson(ctx context.Context, courseID, instructorID uuid.UUID, title string, orderIdx int) (*domain.Lesson, error) {
	if err := s.verifyOwnership(ctx, courseID, instructorID); err != nil {
		return nil, err
	}

	lesson := &domain.Lesson{
		CourseID: courseID,
		Title:    title,
		OrderIdx: orderIdx,
	}
	if err := s.lessons.Create(ctx, lesson); err != nil {
		return nil, err
	}
	s.cache.Delete(ctx, "lessons:course:"+courseID.String())
	return lesson, nil
}

// GetOrCreateMaterialsLesson returns the hidden lesson that stores
// subject-level reference PDFs/PPTs. Files attached there are ingested with
// the course ID, so retrieval can use them for every topic in the subject.
func (s *CourseService) GetOrCreateMaterialsLesson(
	ctx context.Context,
	courseID, instructorID uuid.UUID,
) (*domain.Lesson, error) {
	if err := s.verifyOwnership(ctx, courseID, instructorID); err != nil {
		return nil, err
	}

	lesson, err := s.lessons.GetMaterialsLesson(ctx, courseID)
	if err == nil {
		return lesson, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	lesson = &domain.Lesson{
		CourseID: courseID,
		Title:    "Subject Reference Material",
		OrderIdx: -1,
	}
	if err := s.lessons.Create(ctx, lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

func (s *CourseService) ListLessons(ctx context.Context, courseID uuid.UUID) ([]domain.Lesson, error) {
	key := "lessons:course:" + courseID.String()

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var lessons []domain.Lesson
		if json.Unmarshal([]byte(cached), &lessons) == nil {
			return lessons, nil
		}
	}

	lessons, err := s.lessons.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(lessons); err == nil {
		s.cache.Set(ctx, key, string(data), 5*time.Minute)
	}
	return lessons, nil
}

func (s *CourseService) ListLessonTopics(ctx context.Context, lessonID uuid.UUID) ([]domain.LessonTopic, error) {
	return s.lessons.ListTopicsByLesson(ctx, lessonID)
}

func (s *CourseService) UpdateLesson(ctx context.Context, lessonID, instructorID uuid.UUID, title string, orderIdx int) (*domain.Lesson, error) {
	lesson, err := s.lessons.GetByID(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.verifyOwnership(ctx, lesson.CourseID, instructorID); err != nil {
		return nil, err
	}

	lesson.Title = title
	lesson.OrderIdx = orderIdx
	if err := s.lessons.Update(ctx, lesson); err != nil {
		return nil, err
	}
	s.cache.Delete(ctx, "lessons:course:"+lesson.CourseID.String())
	return lesson, nil
}

func (s *CourseService) DeleteLesson(ctx context.Context, lessonID, instructorID uuid.UUID) error {
	lesson, err := s.lessons.GetByID(ctx, lessonID)
	if err != nil {
		return err
	}
	if err := s.verifyOwnership(ctx, lesson.CourseID, instructorID); err != nil {
		return err
	}
	if err := s.lessons.Delete(ctx, lessonID); err != nil {
		return err
	}
	s.cache.Delete(ctx, "lessons:course:"+lesson.CourseID.String())
	return nil
}

func (s *CourseService) verifyOwnership(ctx context.Context, courseID, instructorID uuid.UUID) error {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}
	return nil
}

func (s *CourseService) enrichThumbnailURL(ctx context.Context, course *domain.Course) {
	if course.ThumbnailKey == "" || course.ThumbnailURL != "" {
		return
	}
	url, err := s.storage.PresignView(ctx, course.ThumbnailKey, "thumbnail", 1*time.Hour)
	if err == nil {
		course.ThumbnailURL = url
	}
}

func (s *CourseService) enrichCourseThumbnails(ctx context.Context, courses []domain.Course) {
	for i := range courses {
		s.enrichThumbnailURL(ctx, &courses[i])
	}
}

func (s *CourseService) invalidateCourseCaches(ctx context.Context, course *domain.Course) {
	s.cache.Delete(ctx, "course:"+course.ID.String())
	s.cache.Delete(ctx, "courses:all")
	s.cache.Delete(ctx, "courses:rank:"+course.Rank)
}

// distributeSyllabusTopics splits the generated lesson-plan topics across the
// course lessons in order. It is intentionally deterministic so repeated
// finalize calls produce the same lesson grouping.
func distributeSyllabusTopics(topics []string, lessonCount, index int) []string {
	if len(topics) == 0 || lessonCount <= 0 || index < 0 || index >= lessonCount {
		return nil
	}
	perLesson := (len(topics) + lessonCount - 1) / lessonCount
	start := index * perLesson
	if start >= len(topics) {
		return nil
	}
	end := start + perLesson
	if end > len(topics) {
		end = len(topics)
	}
	return topics[start:end]
}
