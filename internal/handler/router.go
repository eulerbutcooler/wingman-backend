package handler

import (
	"time"

	"github.com/Amanyd/backend/internal/config"
	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func NewRouter(
	userH *UserHandler,
	courseH *CourseHandler,
	lessonH *LessonHandler,
	fileH *FileHandler,
	quizH *QuizHandler,
	chatH *ChatHandler,
	analytH *AnalyticsHandler,
	healthH *HealthHandler,
	cfg *config.Config,
	log *zap.Logger,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", healthH.Health)
	r.Post("/api/v1/auth/register", userH.Register)
	r.Post("/api/v1/auth/login", userH.Login)
	r.Post("/api/v1/auth/refresh", userH.RefreshToken)

	r.Group(func(r chi.Router) {
		r.Use(JWTAuthMiddleware(cfg.JWT.JWTAccessSecret))

		r.Get("/api/v1/users/me", userH.Me)

		r.Get("/api/v1/courses", courseH.List)
		r.Get("/api/v1/courses/{courseId}", courseH.Get)
		r.Get("/api/v1/courses/{courseId}/lessons", lessonH.List)
		r.Get("/api/v1/lessons/{lessonId}/files", fileH.ListByLesson)
		r.Get("/api/v1/files/{fileId}/status", fileH.IngestStatus)
		r.Get("/api/v1/files/{fileId}/view", fileH.ViewURL)

		r.Get("/api/v1/courses/{courseId}/quizzes", quizH.ListByCourse)
		r.Get("/api/v1/quizzes/{quizId}", quizH.Get)
		r.Post("/api/v1/quizzes/{quizId}/attempt", quizH.StartAttempt)
		r.Post("/api/v1/attempts/{attemptId}/answer", quizH.SubmitAnswer)
		r.Post("/api/v1/attempts/{attemptId}/finish", quizH.FinishAttempt)
		r.Get("/api/v1/attempts/{attemptId}/results", quizH.Results)

		r.Get("/api/v1/chat/sessions", chatH.ListSessions)
		r.Post("/api/v1/chat/sessions", chatH.CreateSession)
		r.Post("/api/v1/chat/sessions/{sessionId}/message", chatH.SendMessage)
		r.Get("/api/v1/chat/sessions/{sessionId}/history", chatH.GetHistory)

		r.Group(func(r chi.Router) {
			r.Use(RBACMiddleware(domain.RoleInstructor))

			r.Post("/api/v1/courses", courseH.Create)
			r.Put("/api/v1/courses/{courseId}", courseH.Update)
			r.Delete("/api/v1/courses/{courseId}", courseH.Delete)

			r.Post("/api/v1/courses/{courseId}/lessons", lessonH.Create)
			r.Put("/api/v1/lessons/{lessonId}", lessonH.Update)
			r.Delete("/api/v1/lessons/{lessonId}", lessonH.Delete)

			r.Post("/api/v1/lessons/{lessonId}/files/upload", fileH.InitUpload)
			r.Post("/api/v1/files/{fileId}/confirm", fileH.ConfirmUpload)
			r.Post("/api/v1/quizzes/{quizId}/reset", quizH.Reset)

			r.Get("/api/v1/analytics", analytH.Overview)
			r.Get("/api/v1/analytics/{courseId}", analytH.CourseMetrics)
		})
	})

	return r
}

func NewUserHandler(svc *service.UserService) *UserHandler       { return &UserHandler{svc: svc} }
func NewCourseHandler(svc *service.CourseService) *CourseHandler  { return &CourseHandler{svc: svc} }
func NewLessonHandler(svc *service.CourseService) *LessonHandler  { return &LessonHandler{svc: svc} }
func NewFileHandler(svc *service.FileService) *FileHandler        { return &FileHandler{svc: svc} }
func NewQuizHandler(svc *service.QuizService) *QuizHandler        { return &QuizHandler{svc: svc} }
func NewChatHandler(svc *service.ChatService) *ChatHandler        { return &ChatHandler{svc: svc} }
func NewAnalyticsHandler(svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}
func NewHealthHandler() *HealthHandler { return &HealthHandler{} }
