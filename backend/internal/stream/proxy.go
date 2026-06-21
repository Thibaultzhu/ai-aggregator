package stream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"

	"ai-aggregator/internal/provider"

	"github.com/gofiber/fiber/v2"
)

// StreamResult captures the final usage stats from a completed stream.
type StreamResult struct {
	Usage provider.Usage
}

// Proxy handles SSE stream relay from upstream to client.
type Proxy struct{}

func NewProxy() *Proxy {
	return &Proxy{}
}

// RelayStream relays an upstream SSE stream to the Fiber response writer.
// It returns the accumulated usage stats via a result channel that is closed
// AFTER the stream is fully written. The caller should read from the channel
// to get the final usage data.
//
// IMPORTANT: Metrics recording is the caller's responsibility, not Proxy's.
// This avoids double-recording and data races.
func (p *Proxy) RelayStream(c *fiber.Ctx, ch <-chan provider.StreamChunk) <-chan StreamResult {
	resultCh := make(chan StreamResult, 1)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no") // Prevent Nginx from buffering

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		var totalUsage provider.Usage

		for chunk := range ch {
			// Accumulate usage from the final chunk (DashScope sends usage in last chunk)
			if chunk.Usage != nil {
				totalUsage = *chunk.Usage
			}

			// Serialize and write SSE event
			data, err := json.Marshal(chunk)
			if err != nil {
				slog.Warn("failed to marshal stream chunk", "error", err)
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			if err := w.Flush(); err != nil {
				slog.Warn("stream flush error", "error", err)
				break
			}
		}

		// Send [DONE]
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.Flush()

		// Send result back to caller AFTER stream is complete
		resultCh <- StreamResult{Usage: totalUsage}
		close(resultCh)
	})

	return resultCh
}
