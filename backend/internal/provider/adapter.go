package provider

import (
	"context"
	"io"
)

// ===== Common Types (OpenAI-compatible) =====

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentPart
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ===== Request Types =====

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        interface{}   `json:"stop,omitempty"`
	Tools       []Tool        `json:"tools,omitempty"`
	ToolChoice  interface{}   `json:"tool_choice,omitempty"`

	// Bailian-specific (pass-through)
	EnableSearch *bool   `json:"enable_search,omitempty"`
	Seed         *int64  `json:"seed,omitempty"`
	ResultFormat string  `json:"result_format,omitempty"`
}

type ImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	ResponseFormat string `json:"response_format"` // "url" | "b64_json"
}

type VideoRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	ImageURL    string  `json:"image_url,omitempty"`
	Duration    int     `json:"duration,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
	CallbackURL string  `json:"callback_url,omitempty"`
}

type EmbeddingRequest struct {
	Model           string      `json:"model"`
	Input           interface{} `json:"input"` // string or []string
	EncodingFormat  string      `json:"encoding_format,omitempty"`
	Dimensions      int         `json:"dimensions,omitempty"`
}

type TranscribeRequest struct {
	File     io.Reader `json:"-"`
	Filename string    `json:"-"`
	Model    string    `json:"model"`
	Language string    `json:"language,omitempty"`
}

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// ===== Response Types =====

type ChatResponse struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"`
	Created           int64        `json:"created"`
	Model             string       `json:"model"`
	Choices           []Choice     `json:"choices"`
	Usage             *Usage       `json:"usage,omitempty"`
	SystemFingerprint string       `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type AsyncTaskResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Status    string `json:"status"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

type AsyncTaskResult struct {
	ID          string      `json:"id"`
	Object      string      `json:"object"`
	Status      string      `json:"status"`
	Model       string      `json:"model"`
	CreatedAt   string      `json:"created_at"`
	CompletedAt string      `json:"completed_at,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	Usage       interface{} `json:"usage,omitempty"`
	Error       *APIError   `json:"error,omitempty"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *Usage          `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type ImageResponse struct {
	Created int64        `json:"created"`
	Data    []ImageData  `json:"data"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type APIError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      string `json:"code"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ===== Provider Adapter Interface =====

// Adapter is the core interface that all upstream providers must implement.
// Each method corresponds to a modality. Providers that don't support a
// modality should return ErrUnsupportedModality.
type Adapter interface {
	// Name returns the provider identifier (e.g., "bailian_cn")
	Name() string

	// ChatCompletion sends a non-streaming chat completion request.
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// ChatCompletionStream sends a streaming chat completion request.
	// Returns a channel that emits StreamChunks.
	ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)

	// CreateImage submits an image generation task (async).
	CreateImage(ctx context.Context, req *ImageRequest) (*AsyncTaskResponse, error)

	// CreateVideo submits a video generation task (async).
	CreateVideo(ctx context.Context, req *VideoRequest) (*AsyncTaskResponse, error)

	// PollTask checks the status of an async task.
	PollTask(ctx context.Context, upstreamTaskID string) (*AsyncTaskResult, error)

	// CreateEmbedding generates text embeddings.
	CreateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)

	// TranscribeAudio converts audio to text.
	TranscribeAudio(ctx context.Context, req *TranscribeRequest) (string, error)

	// SynthesizeSpeech converts text to audio.
	SynthesizeSpeech(ctx context.Context, req *SpeechRequest) ([]byte, string, error)

	// HealthCheck verifies the provider is reachable and functional.
	HealthCheck(ctx context.Context) error
}

// ErrUnsupportedModality is returned when a provider doesn't support the requested operation.
type ErrUnsupportedModality struct {
	Provider string
	Op       string
}

func (e *ErrUnsupportedModality) Error() string {
	return "provider " + e.Provider + " does not support " + e.Op
}
