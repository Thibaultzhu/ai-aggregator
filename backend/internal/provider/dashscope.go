package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// DashScopeAdapter implements the Adapter interface for Alibaba Cloud
// DashScope (Bailian) API using the OpenAI-compatible endpoint.
//
// Supports both CN (dashscope.aliyuncs.com) and INTL (dashscope-intl.aliyuncs.com).
type DashScopeAdapter struct {
	name       string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type DashScopeConfig struct {
	Name    string // "bailian_cn" or "bailian_intl"
	BaseURL string // e.g., "https://dashscope.aliyuncs.com/compatible-mode/v1"
	APIKey  string
	Timeout time.Duration
}

func NewDashScopeAdapter(cfg DashScopeConfig) *DashScopeAdapter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &DashScopeAdapter{
		name:    cfg.Name,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (d *DashScopeAdapter) Name() string {
	return d.name
}

// ===== Chat Completion (Non-streaming) =====

func (d *DashScopeAdapter) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, d.parseError(resp)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &chatResp, nil
}

// ===== Chat Completion (Streaming) =====

func (d *DashScopeAdapter) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	// Ensure stream is enabled
	req.Stream = true

	// Inject stream_options for DashScope compatible-mode
	// (enables usage info in the final chunk)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, d.parseError(resp)
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer size for large chunks
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			// Parse SSE data
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// End of stream
			if data == "[DONE]" {
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				slog.Warn("failed to parse stream chunk", "error", err, "data", data)
				continue
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Error("stream scanner error", "error", err)
		}
	}()

	return ch, nil
}

// ===== Image Generation =====

func (d *DashScopeAdapter) CreateImage(ctx context.Context, req *ImageRequest) (*AsyncTaskResponse, error) {
	// DashScope uses a different endpoint for image generation
	// POST /api/v1/services/aigc/text2image/image-synthesis
	// This is NOT OpenAI-compatible, requires custom mapping
	body := map[string]interface{}{
		"model": req.Model,
		"input": map[string]interface{}{
			"prompt": req.Prompt,
		},
		"parameters": map[string]interface{}{
			"n":    req.N,
			"size": req.Size,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Use the native DashScope API for image generation
	nativeURL := strings.Replace(d.baseURL, "/compatible-mode/v1", "/api/v1/services/aigc/text2image/image-synthesis", 1)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, nativeURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)
	httpReq.Header.Set("X-DashScope-Async", "enable")

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, d.parseError(resp)
	}

	var result struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &AsyncTaskResponse{
		ID:        result.Output.TaskID,
		Object:    "async_task",
		Status:    mapTaskStatus(result.Output.TaskStatus),
		Model:     req.Model,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ===== Video Generation =====

func (d *DashScopeAdapter) CreateVideo(ctx context.Context, req *VideoRequest) (*AsyncTaskResponse, error) {
	input := map[string]interface{}{
		"prompt": req.Prompt,
	}
	if req.ImageURL != "" {
		input["image_url"] = req.ImageURL
	}

	parameters := map[string]interface{}{}
	if req.Duration > 0 {
		parameters["duration"] = req.Duration
	}
	if req.Resolution != "" {
		parameters["resolution"] = req.Resolution
	}

	body := map[string]interface{}{
		"model":      req.Model,
		"input":      input,
		"parameters": parameters,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	nativeURL := strings.Replace(d.baseURL, "/compatible-mode/v1", "/api/v1/services/aigc/video-generation/generation", 1)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, nativeURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)
	httpReq.Header.Set("X-DashScope-Async", "enable")

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, d.parseError(resp)
	}

	var result struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &AsyncTaskResponse{
		ID:        result.Output.TaskID,
		Object:    "async_task",
		Status:    mapTaskStatus(result.Output.TaskStatus),
		Model:     req.Model,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ===== Poll Async Task =====

func (d *DashScopeAdapter) PollTask(ctx context.Context, upstreamTaskID string) (*AsyncTaskResult, error) {
	nativeURL := strings.Replace(d.baseURL, "/compatible-mode/v1", "/api/v1/tasks/"+upstreamTaskID, 1)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, nativeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, d.parseError(resp)
	}

	var result struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
			Results    []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"output"`
		Usage map[string]interface{} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	taskResult := &AsyncTaskResult{
		ID:     result.Output.TaskID,
		Object: "async_task",
		Status: mapTaskStatus(result.Output.TaskStatus),
	}

	if taskResult.Status == "completed" {
		taskResult.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		if len(result.Output.Results) > 0 {
			taskResult.Data = result.Output.Results
		}
		taskResult.Usage = result.Usage
	}

	return taskResult, nil
}

// ===== Embedding =====

func (d *DashScopeAdapter) CreateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, d.parseError(resp)
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &embResp, nil
}

// ===== Audio (TODO: implement) =====

func (d *DashScopeAdapter) TranscribeAudio(ctx context.Context, req *TranscribeRequest) (string, error) {
	return "", &ErrUnsupportedModality{Provider: d.name, Op: "transcribe_audio"}
}

func (d *DashScopeAdapter) SynthesizeSpeech(ctx context.Context, req *SpeechRequest) ([]byte, string, error) {
	return nil, "", &ErrUnsupportedModality{Provider: d.name, Op: "synthesize_speech"}
}

// ===== Health Check =====

func (d *DashScopeAdapter) HealthCheck(ctx context.Context) error {
	// Simple health check: list models
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}
	d.setHeaders(httpReq)

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx2)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check: upstream returned %d", resp.StatusCode)
	}
	return nil
}

// ===== Helpers =====

func (d *DashScopeAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
}

func (d *DashScopeAdapter) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error *APIError `json:"error"`
		// DashScope native error format
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("upstream error (status %d): %s", resp.StatusCode, string(body))
	}

	if errResp.Error != nil {
		return fmt.Errorf("upstream error: [%s] %s", errResp.Error.Code, errResp.Error.Message)
	}
	if errResp.Code != "" {
		return fmt.Errorf("upstream error: [%s] %s", errResp.Code, errResp.Message)
	}

	return fmt.Errorf("upstream error (status %d): %s", resp.StatusCode, string(body))
}

func mapTaskStatus(dashscopeStatus string) string {
	switch strings.ToUpper(dashscopeStatus) {
	case "PENDING":
		return "pending"
	case "RUNNING":
		return "processing"
	case "SUCCEEDED":
		return "completed"
	case "FAILED":
		return "failed"
	default:
		return strings.ToLower(dashscopeStatus)
	}
}
