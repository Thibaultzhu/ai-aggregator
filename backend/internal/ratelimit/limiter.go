package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// Limiter implements sliding-window rate limiting using Redis sorted sets.
type Limiter struct {
	redis      *redis.Client
	defaultRPM int
	defaultTPM int
}

func NewLimiter(rdb *redis.Client, defaultRPM, defaultTPM int) *Limiter {
	return &Limiter{
		redis:      rdb,
		defaultRPM: defaultRPM,
		defaultTPM: defaultTPM,
	}
}

// Middleware applies per-key RPM rate limiting via Redis sorted set sliding window.
func (l *Limiter) Middleware(c *fiber.Ctx) error {
	keyID := c.Locals("api_key_id")
	if keyID == nil {
		keyID = c.Locals("user_id")
	}
	if keyID == nil {
		return c.Next()
	}

	id := fmt.Sprintf("%v", keyID)
	ctx := c.UserContext()

	allowed, remaining, retryAfter, err := l.checkRPM(ctx, id, l.defaultRPM)
	if err != nil {
		// Fail-open on Redis errors
		slog.Warn("rate limit check failed, allowing request", "error", err, "key", id)
		return c.Next()
	}

	c.Set("X-RateLimit-Limit", strconv.Itoa(l.defaultRPM))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

	if !allowed {
		c.Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		return c.Status(429).JSON(fiber.Map{
			"error": fiber.Map{
				"message":     "rate limit exceeded (requests per minute)",
				"type":        "rate_limit_exceeded",
				"code":        "rpm_limit",
				"retry_after": int(retryAfter.Seconds()),
			},
		})
	}

	return c.Next()
}

// checkRPM implements a sliding-window rate limiter using Redis sorted sets.
//
// Algorithm:
//   - Key: "ratelimit:{id}:rpm"
//   - Each request adds a member with score = current unix timestamp (ms)
//   - Remove all members outside the 1-minute window
//   - Count remaining members; reject if >= limit
//   - Set TTL on the key to auto-cleanup
func (l *Limiter) checkRPM(ctx context.Context, id string, limit int) (allowed bool, remaining int, retryAfter time.Duration, err error) {
	key := fmt.Sprintf("ratelimit:%s:rpm", id)
	now := time.Now().UnixMilli()
	windowStart := now - int64(time.Minute/time.Millisecond)

	pipe := l.redis.Pipeline()

	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart))

	// Count current entries
	countCmd := pipe.ZCard(ctx, key)

	// Add current request
	member := fmt.Sprintf("%d:%d", now, now%1000000) // unique member
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: member})

	// Set TTL (2 minutes to be safe)
	pipe.Expire(ctx, key, 2*time.Minute)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	count := int(countCmd.Val())

	if count >= limit {
		// Find the oldest entry to calculate retry-after
		oldest, oerr := l.redis.ZRangeWithScores(ctx, key, 0, 0).Result()
		retry := time.Second * 5 // default retry
		if oerr == nil && len(oldest) > 0 {
			oldestTime := int64(oldest[0].Score)
			retry = time.Duration(now-oldestTime)*time.Millisecond + time.Minute - time.Duration(now-windowStart)*time.Millisecond
			if retry < time.Second {
				retry = time.Second
			}
			if retry > time.Minute {
				retry = time.Minute
			}
		}
		return false, 0, retry, nil
	}

	return true, limit - count, 0, nil
}

// CheckTokenLimit checks TPM (tokens per minute) using a simple counter.
func (l *Limiter) CheckTokenLimit(ctx context.Context, keyID string, tokens int) (bool, error) {
	if l.defaultTPM <= 0 {
		return true, nil
	}
	key := fmt.Sprintf("ratelimit:%s:tpm", keyID)

	pipe := l.redis.Pipeline()
	incrCmd := pipe.IncrBy(ctx, key, int64(tokens))
	pipe.Expire(ctx, key, 2*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, err // fail-open
	}

	return incrCmd.Val() <= int64(l.defaultTPM), nil
}

// IncrConcurrency increments the concurrent request counter.
func (l *Limiter) IncrConcurrency(ctx context.Context, keyID string) (int64, error) {
	key := fmt.Sprintf("ratelimit:%s:concurrent", keyID)
	pipe := l.redis.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 5*time.Minute) // safety TTL
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// DecrConcurrency decrements the concurrent request counter.
func (l *Limiter) DecrConcurrency(ctx context.Context, keyID string) {
	key := fmt.Sprintf("ratelimit:%s:concurrent", keyID)
	l.redis.Decr(ctx, key)
}
