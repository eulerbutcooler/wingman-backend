package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/Amanyd/backend/internal/service"
	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/Amanyd/backend/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ChatHandler struct {
	svc *service.ChatService
}

type createSessionRequest struct {
	CourseID *string `json:"course_id"`
}

func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid json"))
		return
	}

	claims := GetClaims(r)

	var courseID *uuid.UUID
	if req.CourseID != nil {
		id, err := uuid.Parse(*req.CourseID)
		if err != nil {
			apierr.WriteJSON(w, apierr.BadRequest("invalid course id"))
			return
		}
		courseID = &id
	}

	session, err := h.svc.CreateSession(r.Context(), claims.UserID, courseID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusCreated, session)
}

func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r)
	sessions, err := h.svc.ListSessions(r.Context(), claims.UserID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, sessions)
}

type sendMessageRequest struct {
	Query string `json:"query" validate:"required"`
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid session id"))
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid json"))
		return
	}
	if err := validator.ValidateStruct(req); err != nil {
		apierr.WriteJSON(w, err)
		return
	}

	claims := GetClaims(r)
	stream, err := h.svc.SendMessage(r.Context(), sessionID, claims.UserID, req.Query)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		apierr.WriteJSON(w, apierr.Internal("streaming not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Flush headers immediately so the client knows the connection is alive.
	// Without this, the upstream proxy (Next.js) will time out waiting for
	// the first SSE token while the LLM is still generating.
	flusher.Flush()

	// Tee the stream: forward every SSE frame to the client while accumulating
	// tokens + citations so the assistant's answer can be persisted for reload.
	var contentBuilder strings.Builder
	var citations []domain.Citation

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "[DONE]" {
			continue
		}

		var frame map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			continue
		}
		if tk, ok := frame["token"]; ok {
			var token string
			if json.Unmarshal(tk, &token) == nil {
				contentBuilder.WriteString(token)
			}
		}
		if c, ok := frame["citations"]; ok {
			var items []port.CitationItem
			if json.Unmarshal(c, &items) == nil {
				citations = toDomainCitations(items)
			}
		}
	}

	// Persist the assistant message after the stream closes. Use a detached
	// context so saving still succeeds even if the client disconnects mid-stream.
	_ = h.svc.SaveAssistantMessage(
		context.Background(),
		sessionID,
		contentBuilder.String(),
		citations,
	)
}

func toDomainCitations(items []port.CitationItem) []domain.Citation {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.Citation, len(items))
	for i, it := range items {
		out[i] = domain.Citation{
			FileName: it.FileName,
			FileID:   it.FileID,
			Score:    it.Score,
		}
	}
	return out
}

func (h *ChatHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	if err != nil {
		apierr.WriteJSON(w, apierr.BadRequest("invalid session id"))
		return
	}

	messages, err := h.svc.GetHistory(r.Context(), sessionID)
	if err != nil {
		apierr.WriteJSON(w, mapDomainError(err))
		return
	}
	apierr.WriteData(w, http.StatusOK, messages)
}
