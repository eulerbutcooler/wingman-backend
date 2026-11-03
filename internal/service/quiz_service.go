package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/infra/nats"
	"github.com/Amanyd/backend/internal/port"
	"github.com/google/uuid"
)

type QuizService struct {
	quizzes port.QuizRepository
	courses port.CourseRepository
	lessons port.LessonRepository
	queue   port.MessageQueue
	cache   port.Cache
	rag     port.RagClient
}

func NewQuizService(
	quizzes port.QuizRepository,
	courses port.CourseRepository,
	lessons port.LessonRepository,
	queue port.MessageQueue,
	cache port.Cache,
	rag port.RagClient,
) *QuizService {
	return &QuizService{
		quizzes: quizzes,
		courses: courses,
		lessons: lessons,
		queue:   queue,
		cache:   cache,
		rag:     rag,
	}
}

func (s *QuizService) ListByCourse(ctx context.Context, courseID uuid.UUID) ([]domain.Quiz, error) {
	key := "quizzes:course:" + courseID.String()

	if cached, err := s.cache.Get(ctx, key); err == nil {
		var quizzes []domain.Quiz
		if json.Unmarshal([]byte(cached), &quizzes) == nil {
			if quizzes == nil {
				quizzes = make([]domain.Quiz, 0)
			}
			return quizzes, nil
		}
	}

	quizzes, err := s.quizzes.ListQuizzesByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(quizzes); err == nil {
		s.cache.Set(ctx, key, string(data), 5*time.Minute)
	}
	return quizzes, nil
}

func (s *QuizService) ListQuestionBank(
	ctx context.Context,
	courseID uuid.UUID,
) ([]domain.Question, error) {
	return s.quizzes.ListQuestionsByCourse(ctx, courseID)
}

func (s *QuizService) ListByCourseWithUser(ctx context.Context, courseID uuid.UUID, userID uuid.UUID) ([]domain.Quiz, error) {
	if userID == uuid.Nil {
		return s.ListByCourse(ctx, courseID)
	}
	return s.quizzes.ListQuizzesByCourseWithAttempts(ctx, courseID, userID)
}

type QuizWithQuestions struct {
	Quiz      domain.Quiz       `json:"quiz"`
	Questions []domain.Question `json:"questions,omitempty"`
}

func (s *QuizService) GetQuiz(ctx context.Context, quizID uuid.UUID, includeAnswers bool) (*QuizWithQuestions, error) {
	quiz, err := s.quizzes.GetQuizByID(ctx, quizID)
	if err != nil {
		return nil, err
	}

	var questions []domain.Question
	if quiz.LessonID != nil {
		// Lesson quiz: 1 easy question per topic from the question bank
		questions, err = s.quizzes.SampleQuestionsByQuiz(ctx, quizID, 0)
	} else {
		// Course exam: sample questions matching difficulty (medium for basic, hard for advanced)
		questions, err = s.quizzes.SampleQuestionsByCourse(ctx, quiz.CourseID, quiz.Difficulty, 0)
	}
	if err != nil || len(questions) == 0 {
		questions, err = s.quizzes.ListQuestionsByQuiz(ctx, quizID)
		if err != nil {
			return nil, err
		}
	}

	if !includeAnswers {
		for i := range questions {
			// Strip answers for students so they can't inspect them during an attempt.
			questions[i].Answer = ""
		}
	}

	return &QuizWithQuestions{Quiz: *quiz, Questions: questions}, nil
}

func (s *QuizService) StartAttempt(ctx context.Context, quizID, userID uuid.UUID) (*domain.Attempt, error) {
	if _, err := s.quizzes.GetQuizByID(ctx, quizID); err != nil {
		return nil, err
	}

	attempt := &domain.Attempt{
		QuizID: quizID,
		UserID: userID,
	}
	if err := s.quizzes.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
}

func (s *QuizService) SubmitAnswer(ctx context.Context, attemptID, questionID, userID uuid.UUID, userAnswer string) (*domain.Answer, error) {
	attempt, err := s.quizzes.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, domain.ErrForbidden
	}

	question, err := s.quizzes.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, err
	}

	var isCorrect bool
	if question.Type == domain.QuestionOpenEnded {
		grade, gradeErr := s.rag.GradeAnswer(ctx, port.GradeRequest{
			Question:        question.Question,
			ReferenceAnswer: question.Answer,
			UserAnswer:      userAnswer,
		})
		if gradeErr != nil {
			isCorrect = false
		} else {
			isCorrect = grade.IsCorrect
		}
	} else {
		// Identify correct choice text from database
		var correctAnswerText string
		for _, c := range question.Choices {
			if c.Label == question.Answer {
				correctAnswerText = c.Text
				break
			}
		}
		cleanUserAnswer := strings.TrimSpace(userAnswer)
		isCorrect = (cleanUserAnswer == question.Answer) ||
			(correctAnswerText != "" && strings.EqualFold(cleanUserAnswer, strings.TrimSpace(correctAnswerText)))
	}

	answer := &domain.Answer{
		AttemptID:  attemptID,
		QuestionID: questionID,
		UserAnswer: userAnswer,
		IsCorrect:  isCorrect,
	}
	if err := s.quizzes.CreateAnswer(ctx, answer); err != nil {
		return nil, err
	}
	return answer, nil
}

func (s *QuizService) FinishAttempt(ctx context.Context, attemptID, userID uuid.UUID) (*domain.Attempt, error) {
	attempt, err := s.quizzes.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, domain.ErrForbidden
	}

	answers, err := s.quizzes.ListAnswersByAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	var correct int
	for _, a := range answers {
		if a.IsCorrect {
			correct++
		}
	}

	total := len(answers)
	var score float64
	if total > 0 {
		score = float64(correct) / float64(total) * 100
	}

	now := time.Now()
	attempt.Score = score
	attempt.Total = total
	attempt.EndedAt = &now

	if err := s.quizzes.UpdateAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
}

type AttemptResults struct {
	Attempt   domain.Attempt    `json:"attempt"`
	Answers   []domain.Answer   `json:"answers,omitempty"`
	Questions []domain.Question `json:"questions,omitempty"`
}

func (s *QuizService) GetResults(ctx context.Context, attemptID, userID uuid.UUID) (*AttemptResults, error) {
	attempt, err := s.quizzes.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, domain.ErrForbidden
	}

	answers, err := s.quizzes.ListAnswersByAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	questions := make([]domain.Question, 0, len(answers))
	for _, a := range answers {
		q, err := s.quizzes.GetQuestionByID(ctx, a.QuestionID)
		if err == nil && q != nil {
			questions = append(questions, *q)
		}
	}

	return &AttemptResults{
		Attempt:   *attempt,
		Answers:   answers,
		Questions: questions,
	}, nil
}

func (s *QuizService) ResetQuiz(ctx context.Context, quizID, instructorID uuid.UUID) error {
	quiz, err := s.quizzes.GetQuizByID(ctx, quizID)
	if err != nil {
		return err
	}

	course, err := s.courses.GetByID(ctx, quiz.CourseID)
	if err != nil {
		return err
	}
	if course.InstructorID != instructorID {
		return domain.ErrForbidden
	}

	if err := s.quizzes.DeleteQuestionsByQuiz(ctx, quizID); err != nil {
		return err
	}

	if err := s.quizzes.UpdateQuizStatus(ctx, quizID, domain.QuizGenerating); err != nil {
		return err
	}

	s.cache.Delete(ctx, "quiz:"+quizID.String())
	s.cache.Delete(ctx, "quizzes:course:"+course.ID.String())

	if quiz.LessonID == nil {
		// Course exams sample dynamically from the lesson question banks, so
		// there is nothing to regenerate.
		return s.quizzes.UpdateQuizStatus(ctx, quizID, domain.QuizReady)
	}

	keywords := []string{}
	if lesson, err := s.lessons.GetByID(ctx, *quiz.LessonID); err == nil && len(lesson.Keywords) > 0 {
		_ = json.Unmarshal(lesson.Keywords, &keywords)
	}
	if len(keywords) == 0 {
		// Fall back to the stored lesson topics so a reset still has a topic
		// source when the lesson keywords were never persisted.
		if topics, err := s.lessons.ListTopicsByLesson(ctx, *quiz.LessonID); err == nil {
			for _, topic := range topics {
				keywords = append(keywords, topic.Title)
			}
		}
	}

	payload, err := json.Marshal(map[string]any{
		"type":         "lesson",
		"request_id":   uuid.NewString(),
		"quiz_id":      quiz.ID.String(),
		"course_id":    course.ID.String(),
		"lesson_id":    quiz.LessonID.String(),
		"keywords":     keywords,
		"difficulty":   string(quiz.Difficulty),
		"limit_chunks": 20,
	})
	if err != nil {
		return fmt.Errorf("marshal quiz request: %w", err)
	}

	return s.queue.Publish(ctx, nats.SubjectQuizRequest, payload)
}
