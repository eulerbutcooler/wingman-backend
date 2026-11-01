package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Amanyd/backend/internal/port"
)

type ragClient struct {
	baseURL        string
	token          string
	http           *http.Client // for non-streaming (short timeout)
	gradeHTTP      *http.Client // for open-ended grading (longer timeout)
	syllabusHTTP   *http.Client // for syllabus parsing + topic extraction
	lessonPlanHTTP *http.Client // for lesson-plan PDF generation (can take minutes)
	httpStream     *http.Client // for SSE streaming (no timeout; context drives cancellation)
}

func NewRAGClient(baseURL, token string) port.RagClient {
	return &ragClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
		// Grading may wait behind quiz generation on the local GPU. The route
		// allows 120s; give the client a little extra so it does not race it.
		gradeHTTP: &http.Client{
			Timeout: 150 * time.Second,
		},
		syllabusHTTP: &http.Client{
			Timeout: 180 * time.Second,
		},
		// A 33-row subject can take 10+ minutes with the local model.
		lessonPlanHTTP: &http.Client{
			Timeout: 30 * time.Minute,
		},
		// No Timeout on the streaming client: http.Client.Timeout covers the
		// entire response including body reads, which would cut off long LLM
		// generations. Cancellation is handled via the request context instead.
		httpStream: &http.Client{},
	}
}

func (c *ragClient) ChatStream(ctx context.Context, req port.ChatRequest) (io.ReadCloser, error) {
	req.Stream = true
	return c.doRequest(ctx, req, true)
}

func (c *ragClient) Chat(ctx context.Context, req port.ChatRequest) (*port.ChatResponse, error) {
	req.Stream = false
	body, err := c.doRequest(ctx, req, false)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp port.ChatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("rag decode response: %w", err)
	}
	return &resp, nil
}

func (c *ragClient) GradeAnswer(ctx context.Context, req port.GradeRequest) (*port.GradeResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("rag marshal grade request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/quiz/grade"
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("rag new grade request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.gradeHTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag grade http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rag grade unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var gradeResp port.GradeResponse
	if err := json.NewDecoder(resp.Body).Decode(&gradeResp); err != nil {
		return nil, fmt.Errorf("rag decode grade response: %w", err)
	}
	return &gradeResp, nil
}

func (c *ragClient) ExtractSyllabus(
	ctx context.Context,
	req port.SyllabusExtractRequest,
) (*port.SyllabusExtractResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("rag marshal syllabus request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/syllabus/extract"
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("rag new syllabus request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.syllabusHTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag syllabus http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rag syllabus unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result port.SyllabusExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("rag decode syllabus response: %w", err)
	}
	return &result, nil
}

func (c *ragClient) ListSyllabusSubjects(
	ctx context.Context,
	req port.SyllabusSubjectsRequest,
) (*port.SyllabusSubjectsResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("rag marshal subjects request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/syllabus/subjects"
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("rag new subjects request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.syllabusHTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag subjects http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rag subjects unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result port.SyllabusSubjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("rag decode subjects response: %w", err)
	}
	return &result, nil
}

func (c *ragClient) GenerateLessonPlans(
	ctx context.Context,
	req port.LessonPlanGenerateRequest,
) (*port.LessonPlanGenerateResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("rag marshal lesson plan request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/lesson-plans/generate"
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("rag new lesson plan request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.lessonPlanHTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag lesson plan http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"rag lesson plan unexpected status %d: %s",
			resp.StatusCode,
			string(respBody),
		)
	}

	var result port.LessonPlanGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("rag decode lesson plan response: %w", err)
	}
	return &result, nil
}

func (c *ragClient) DeleteCourse(ctx context.Context, courseID string) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/course/" + courseID
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("rag new delete request: %w", err)
	}

	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("rag delete http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rag delete unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *ragClient) DeleteFileVectors(ctx context.Context, fileID string) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/file/" + fileID
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("rag new delete file request: %w", err)
	}

	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("rag delete file http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rag delete file unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *ragClient) doRequest(ctx context.Context, req port.ChatRequest, stream bool) (io.ReadCloser, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("rag marshal request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/v1/chat/"
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("rag new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	client := c.http
	if stream {
		client = c.httpStream
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag http do: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("rag unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return resp.Body, nil
}
