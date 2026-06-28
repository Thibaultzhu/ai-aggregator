package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.nativeURL("/api/v1/services/aigc/text2image/image-synthesis"), bytes.NewReader(jsonBody))
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.nativeURL("/api/v1/services/aigc/video-generation/generation"), bytes.NewReader(jsonBody))
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, d.nativeURL("/api/v1/tasks/"+upstreamTaskID), nil)
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

// ===== Audio =====

func (d *DashScopeAdapter) TranscribeAudio(ctx context.Context, req *TranscribeRequest) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", req.Model); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return "", fmt.Errorf("write language field: %w", err)
		}
	}

	filename := req.Filename
	if filename == "" {
		filename = "audio"
	}
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return "", fmt.Errorf("create file field: %w", err)
	}
	if _, err := io.Copy(part, req.File); err != nil {
		return "", fmt.Errorf("copy audio file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.nativeURL("/api/v1/services/audio/asr/transcription"), &body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", d.parseError(resp)
	}

	var result struct {
		Text   string `json:"text"`
		Output struct {
			Text       string `json:"text"`
			Sentence   string `json:"sentence"`
			Transcript string `json:"transcript"`
		} `json:"output"`
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	switch {
	case result.Text != "":
		return result.Text, nil
	case result.Output.Text != "":
		return result.Output.Text, nil
	case result.Output.Transcript != "":
		return result.Output.Transcript, nil
	case result.Output.Sentence != "":
		return result.Output.Sentence, nil
	case result.Result.Text != "":
		return result.Result.Text, nil
	default:
		return "", fmt.Errorf("decode response: missing transcription text")
	}
}

func (d *DashScopeAdapter) SynthesizeSpeech(ctx context.Context, req *SpeechRequest) ([]byte, string, error) {
	body := map[string]interface{}{
		"model": req.Model,
		"input": map[string]interface{}{
			"text": req.Input,
		},
		"parameters": map[string]interface{}{
			"voice":           req.Voice,
			"response_format": req.ResponseFormat,
		},
	}
	if req.Speed > 0 {
		body["parameters"].(map[string]interface{})["speed"] = req.Speed
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.nativeURL("/api/v1/services/audio/tts/speech-synthesizer"), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	d.setHeaders(httpReq)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", d.parseError(resp)
	}

	contentType := resp.Header.Get("Content-Type")
	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	if strings.HasPrefix(contentType, "application/json") {
		var result struct {
			Output struct {
				Audio string `json:"audio"`
				URL   string `json:"url"`
			} `json:"output"`
		}
		if err := json.Unmarshal(audioBytes, &result); err == nil && result.Output.Audio != "" {
			return []byte(result.Output.Audio), "text/plain", nil
		}
	}
	if contentType == "" {
		contentType = contentTypeForAudioFormat(req.ResponseFormat)
	}
	return audioBytes, contentType, nil
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

func (d *DashScopeAdapter) nativeURL(path string) string {
	if strings.Contains(d.baseURL, "/compatible-mode/v1") {
		return strings.Replace(d.baseURL, "/compatible-mode/v1", path, 1)
	}
	return strings.TrimRight(d.baseURL, "/") + path
}

func contentTypeForAudioFormat(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
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
