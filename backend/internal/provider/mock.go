package provider

import (
	"context"
	"fmt"
	"time"
)

// MockAdapter implements the Adapter interface with predictable, hardcoded
// responses. It is used for local development and testing when real upstream
// provider credentials are not available (MOCK_PROVIDER_MODE=true).
type MockAdapter struct{}

// NewMockAdapter creates a new mock provider adapter.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

// Compile-time interface check.
var _ Adapter = (*MockAdapter)(nil)

func (m *MockAdapter) Name() string {
	return "mock"
}

func (m *MockAdapter) ChatCompletion(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
	finishReason := "stop"
	return &ChatResponse{
		ID:      "chatcmpl-mock-001",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: &ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Mock response from model %s. This is a simulated reply for development purposes.", req.Model),
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *MockAdapter) ChatCompletionStream(_ context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 10)

	go func() {
		defer close(ch)

		chunks := []string{
			"Mock ",
			"response ",
			"from ",
			"model ",
			req.Model,
			". ",
			"This ",
			"is ",
			"simulated.",
		}

		for i, text := range chunks {
			finishReason := (*string)(nil)
			if i == len(chunks)-1 {
				reason := "stop"
				finishReason = &reason
			}

			chunk := StreamChunk{
				ID:      "chatcmpl-mock-stream-001",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []Choice{
					{
						Index: 0,
						Delta: &ChatMessage{
							Role:    "assistant",
							Content: text,
						},
						FinishReason: finishReason,
					},
				},
			}

			// Attach usage info to the final chunk.
			if i == len(chunks)-1 {
				chunk.Usage = &Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				}
			}

			ch <- chunk
		}
	}()

	return ch, nil
}

func (m *MockAdapter) CreateImage(_ context.Context, req *ImageRequest) (*AsyncTaskResponse, error) {
	return &AsyncTaskResponse{
		ID:        "mock-img-task-001",
		Object:    "async_task",
		Status:    "PENDING",
		Model:     req.Model,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (m *MockAdapter) CreateVideo(_ context.Context, req *VideoRequest) (*AsyncTaskResponse, error) {
	return &AsyncTaskResponse{
		ID:        "mock-vid-task-001",
		Object:    "async_task",
		Status:    "PENDING",
		Model:     req.Model,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (m *MockAdapter) PollTask(_ context.Context, upstreamTaskID string) (*AsyncTaskResult, error) {
	return &AsyncTaskResult{
		ID:          upstreamTaskID,
		Object:      "async_task",
		Status:      "SUCCEEDED",
		Model:       "mock-model",
		CreatedAt:   time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339),
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"url": "https://example.com/mock-output.png",
		},
		Usage: map[string]interface{}{
			"image_count": 1,
		},
	}, nil
}

func (m *MockAdapter) CreateEmbedding(_ context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	// Return a small deterministic embedding vector.
	embedding := make([]float64, 8)
	for i := range embedding {
		embedding[i] = float64(i) * 0.1
	}

	return &EmbeddingResponse{
		Object: "list",
		Data: []EmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: embedding,
			},
		},
		Model: req.Model,
		Usage: &Usage{
			PromptTokens:     5,
			CompletionTokens: 0,
			TotalTokens:      5,
		},
	}, nil
}

func (m *MockAdapter) TranscribeAudio(_ context.Context, _ *TranscribeRequest) (string, error) {
	return "This is a mock transcription result for development purposes.", nil
}

func (m *MockAdapter) SynthesizeSpeech(_ context.Context, _ *SpeechRequest) ([]byte, string, error) {
	// Return a tiny valid-looking byte slice and the MIME type.
	return []byte("MOCK_AUDIO_DATA"), "audio/mpeg", nil
}

func (m *MockAdapter) HealthCheck(_ context.Context) error {
	return nil
}
