package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AnthropicAdapter struct {
	name       string
	baseURL    string
	apiKey     string
	version    string
	httpClient *http.Client
}

type AnthropicConfig struct {
	Name    string
	BaseURL string
	APIKey  string
	Version string
	Timeout time.Duration
}

func NewAnthropicAdapter(cfg AnthropicConfig) *AnthropicAdapter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Version == "" {
		cfg.Version = "2023-06-01"
	}
	return &AnthropicAdapter{
		name:    cfg.Name,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		version: cfg.Version,
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

var _ Adapter = (*AnthropicAdapter)(nil)

func (a *AnthropicAdapter) Name() string {
	return a.name
}

func (a *AnthropicAdapter) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := a.toAnthropicRequest(req, false)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}
	a.setHeaders(httpReq)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do anthropic request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, a.parseError(resp)
	}

	var out struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Role       string `json:"role"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	textParts := make([]string, 0, len(out.Content))
	for _, part := range out.Content {
		if part.Type == "text" && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	finishReason := mapAnthropicStopReason(out.StopReason)
	return &ChatResponse{
		ID:      out.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{{
			Index: 0,
			Message: &ChatMessage{
				Role:    "assistant",
				Content: strings.Join(textParts, ""),
			},
			FinishReason: &finishReason,
		}},
		Usage: &Usage{
			PromptTokens:     out.Usage.InputTokens,
			CompletionTokens: out.Usage.OutputTokens,
			TotalTokens:      out.Usage.InputTokens + out.Usage.OutputTokens,
		},
	}, nil
}

func (a *AnthropicAdapter) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	copyReq := *req
	copyReq.Stream = false
	resp, err := a.ChatCompletion(ctx, &copyReq)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		content := ""
		if len(resp.Choices) > 0 && resp.Choices[0].Message != nil {
			content, _ = resp.Choices[0].Message.Content.(string)
		}
		ch <- StreamChunk{
			ID:      resp.ID,
			Object:  "chat.completion.chunk",
			Created: resp.Created,
			Model:   resp.Model,
			Choices: []Choice{{
				Index: 0,
				Delta: &ChatMessage{Role: "assistant", Content: content},
			}},
		}
		finish := "stop"
		ch <- StreamChunk{
			ID:      resp.ID,
			Object:  "chat.completion.chunk",
			Created: resp.Created,
			Model:   resp.Model,
			Choices: []Choice{{Index: 0, FinishReason: &finish}},
			Usage:   resp.Usage,
		}
	}()
	return ch, nil
}

func (a *AnthropicAdapter) CreateImage(context.Context, *ImageRequest) (*AsyncTaskResponse, error) {
	return nil, &ErrUnsupportedModality{Provider: a.name, Op: "create_image"}
}

func (a *AnthropicAdapter) CreateVideo(context.Context, *VideoRequest) (*AsyncTaskResponse, error) {
	return nil, &ErrUnsupportedModality{Provider: a.name, Op: "create_video"}
}

func (a *AnthropicAdapter) PollTask(context.Context, string) (*AsyncTaskResult, error) {
	return nil, &ErrUnsupportedModality{Provider: a.name, Op: "poll_task"}
}

func (a *AnthropicAdapter) CreateEmbedding(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, &ErrUnsupportedModality{Provider: a.name, Op: "create_embedding"}
}

func (a *AnthropicAdapter) TranscribeAudio(context.Context, *TranscribeRequest) (string, error) {
	return "", &ErrUnsupportedModality{Provider: a.name, Op: "transcribe_audio"}
}

func (a *AnthropicAdapter) SynthesizeSpeech(context.Context, *SpeechRequest) ([]byte, string, error) {
	return nil, "", &ErrUnsupportedModality{Provider: a.name, Op: "synthesize_speech"}
}

func (a *AnthropicAdapter) HealthCheck(ctx context.Context) error {
	maxTokens := 1
	_, err := a.ChatCompletion(ctx, &ChatRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: &maxTokens,
		Messages:  []ChatMessage{{Role: "user", Content: "ping"}},
	})
	return err
}

func (a *AnthropicAdapter) toAnthropicRequest(req *ChatRequest, stream bool) ([]byte, error) {
	maxTokens := 1024
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	messages := make([]map[string]interface{}, 0, len(req.Messages))
	var systemParts []string
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "system" {
			systemParts = append(systemParts, chatContentToText(msg.Content))
			continue
		}
		if role == "assistant" || role == "user" {
			messages = append(messages, map[string]interface{}{
				"role":    role,
				"content": anthropicContent(msg.Content),
			})
		}
	}
	body := map[string]interface{}{
		"model":      req.Model,
		"max_tokens": maxTokens,
		"messages":   messages,
		"stream":     stream,
	}
	if len(systemParts) > 0 {
		body["system"] = strings.Join(systemParts, "\n")
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		body["stop_sequences"] = req.Stop
	}
	return json.Marshal(body)
}

func (a *AnthropicAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", a.version)
}

func (a *AnthropicAdapter) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic upstream error: [%s] %s", errResp.Error.Type, errResp.Error.Message)
	}
	return fmt.Errorf("anthropic upstream error (status %d): %s", resp.StatusCode, string(body))
}

func anthropicContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []ContentPart:
		parts := make([]map[string]interface{}, 0, len(v))
		for _, part := range v {
			if part.Type == "text" {
				parts = append(parts, map[string]interface{}{"type": "text", "text": part.Text})
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return chatContentToText(content)
}

func chatContentToText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []ContentPart:
		var b strings.Builder
		for _, part := range v {
			if part.Text != "" {
				b.WriteString(part.Text)
			}
		}
		return b.String()
	default:
		encoded, _ := json.Marshal(v)
		return string(encoded)
	}
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		if reason == "" {
			return "stop"
		}
		return reason
	}
}
