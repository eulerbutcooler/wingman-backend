package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/infra/nats"
	"github.com/Amanyd/backend/internal/port"
	"github.com/Amanyd/backend/internal/service"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type IngestDoneWorkerDeps struct {
	Files   port.FileRepository
	Lessons port.LessonRepository
	Cache   port.Cache
}

func StartIngestDoneWorker(ctx context.Context, js jetstream.JetStream, deps IngestDoneWorkerDeps, log *zap.Logger) error {
	cons, err := nats.CreateOrUpdateConsumer(ctx, js, nats.StreamIngestDone, nats.DurableIngestDone, nats.SubjectIngestDone)
	if err != nil {
		return err
	}

	log.Info("ingest_done_worker started")

	return nats.ConsumeLoop(ctx, cons, func(msg jetstream.Msg) {
		// If the parent context is already cancelled (shutdown), ack the message
		// so it doesn't get redelivered in an infinite error loop.
		select {
		case <-ctx.Done():
			msg.Ack()
			return
		default:
		}

		if err := handleIngestDone(context.Background(), msg, deps, log); err != nil {
			log.Error("ingest_done_worker handle failed", zap.Error(err))
			msg.Nak()
			return
		}
		msg.Ack()
	})
}

type ingestDonePayload struct {
	Status string `json:"status"`
	FileID string `json:"file_id"`
}

func handleIngestDone(ctx context.Context, msg jetstream.Msg, deps IngestDoneWorkerDeps, log *zap.Logger) error {
	var payload ingestDonePayload
	if err := json.Unmarshal(msg.Data(), &payload); err != nil {
		log.Warn("ingest_done bad json, dropping", zap.Error(err))
		msg.Ack()
		return nil
	}

	fileID, err := uuid.Parse(payload.FileID)
	if err != nil {
		log.Warn("ingest_done bad file_id, dropping", zap.String("file_id", payload.FileID))
		msg.Ack()
		return nil
	}

	status := domain.IngestReady
	if payload.Status != "success" {
		log.Info("ingest_done failed", zap.String("file_id", payload.FileID))
		status = domain.IngestFailed
	}

	if err := deps.Files.UpdateIngestStatus(ctx, fileID, status); err != nil {
		return fmt.Errorf("update file status: %w", err)
	}

	// Fetch the file to resolve its lesson id for cache invalidation (and the
	// all-ready check below). A missing file is non-fatal: the DB status is
	// already updated, and the per-file status cache can still be invalidated.
	file, err := deps.Files.GetByID(ctx, fileID)
	if err != nil {
		log.Warn("ingest_done get file for cache invalidation", zap.Error(err))
		invalidateFileCaches(ctx, deps.Cache, fileID, uuid.Nil)
		return nil
	}

	// Invalidate the caches FileService serves so the status and list endpoints
	// reflect the new state immediately instead of returning stale entries
	// until their TTLs expire (30s for status, 2m for the lesson list).
	invalidateFileCaches(ctx, deps.Cache, fileID, file.LessonID)

	if status != domain.IngestReady {
		return nil
	}

	lesson, err := deps.Lessons.GetByID(ctx, file.LessonID)
	if err != nil {
		return fmt.Errorf("get lesson: %w", err)
	}

	allReady, err := deps.Files.AllReadyForCourse(ctx, lesson.CourseID)
	if err != nil {
		return fmt.Errorf("check all ready: %w", err)
	}
	if allReady {
		log.Info("all files ready for course", zap.String("course_id", lesson.CourseID.String()))
	} else {
		log.Info("ingest_done not all files ready yet", zap.String("course_id", lesson.CourseID.String()))
	}

	return nil
}

// invalidateFileCaches drops the per-file status and per-lesson list cache
// entries so readers see the freshly written ingest status. Failures are
// ignored: a stale entry will still expire by its TTL.
func invalidateFileCaches(ctx context.Context, c port.Cache, fileID, lessonID uuid.UUID) {
	if c == nil {
		return
	}
	_ = c.Delete(ctx, service.CacheKeyFileStatus+fileID.String())
	if lessonID != uuid.Nil {
		_ = c.Delete(ctx, service.CacheKeyFilesLesson+lessonID.String())
	}
}
