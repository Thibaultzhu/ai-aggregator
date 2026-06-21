package billing

import (
	"context"
	"log/slog"

	"ai-aggregator/internal/storage"

	"github.com/gofiber/fiber/v2"
)

// Engine handles usage tracking and cost calculation.
type Engine struct {
	store          *storage.Store
	defaultQuota   float64
}

func NewEngine(store *storage.Store, defaultQuota float64) *Engine {
	return &Engine{
		store:        store,
		defaultQuota: defaultQuota,
	}
}

// PreCheck is middleware that verifies the user has sufficient balance/quota
// before allowing the request to proceed.
func (e *Engine) PreCheck(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Next()
	}

	uid := userID.(string)
	ctx := c.UserContext()

	// Check balance (for prepaid users)
	balance, err := e.store.GetUserBalance(ctx, uid)
	if err != nil {
		slog.Warn("failed to check balance", "user_id", uid, "error", err)
		return c.Next() // Fail-open on DB error
	}

	if balance <= 0 {
		return c.Status(402).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "insufficient balance, please top up",
				"type":    "insufficient_balance",
				"code":    "no_balance",
			},
		})
	}

	// Check daily quota
	dailyUsage, err := e.store.GetDailyUsage(ctx, uid)
	if err != nil {
		slog.Warn("failed to check daily usage", "user_id", uid, "error", err)
		return c.Next()
	}

	if dailyUsage >= e.defaultQuota {
		return c.Status(429).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "daily spending quota exceeded",
				"type":    "quota_exceeded",
				"code":    "daily_limit",
			},
		})
	}

	return c.Next()
}

// CalculateCost computes the cost for a given usage based on model pricing.
func (e *Engine) CalculateCost(ctx context.Context, modelID string, modality string, inputTokens, outputTokens, imageCount int, durationSec float64) (upstreamCost, chargedCost float64) {
	// TODO: Look up model pricing from DB/cache
	// Calculate based on:
	//   text:     (input_tokens * input_price + output_tokens * output_price) / 1000
	//   image:    image_count * price_per_image
	//   video:    duration_sec * price_per_second
	//   audio:    duration_sec * price_per_second (ASR) or char_count * price_per_char (TTS)
	//   embedding: input_tokens * price / 1000
	//
	// chargedCost = upstreamCost * markup_multiplier

	return 0, 0
}

// RecordUsage asynchronously records a usage event and deducts balance.
func (e *Engine) RecordUsage(ctx context.Context, record *storage.UsageRecord) {
	// 1. Calculate cost
	upstream, charged := e.CalculateCost(ctx, record.ModelID, record.Modality,
		record.InputTokens, record.OutputTokens, 0, 0)
	record.UpstreamCost = upstream
	record.ChargedCost = charged

	// 2. Write to DB (non-blocking)
	go func() {
		if err := e.store.RecordUsage(ctx, record); err != nil {
			slog.Error("failed to record usage", "error", err, "request_id", record.RequestID)
		}

		// 3. Deduct balance
		if charged > 0 {
			if err := e.store.DeductBalance(ctx, record.UserID, charged); err != nil {
				slog.Error("failed to deduct balance", "error", err, "user_id", record.UserID)
			}
		}
	}()
}
