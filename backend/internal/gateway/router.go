package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"ai-aggregator/internal/auth"
	"ai-aggregator/internal/config"
	"ai-aggregator/internal/metrics"
	"ai-aggregator/internal/provider"
	"ai-aggregator/internal/ratelimit"
	"ai-aggregator/internal/router"
	"ai-aggregator/internal/storage"
	"ai-aggregator/internal/stream"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlog "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"golang.org/x/crypto/bcrypt"
)

// ===== Pricing Defaults (MVP) =====
// In production these come from the DB per-model; hardcoded for MVP.

const (
	defaultInputPricePer1K  = 0.002 // USD per 1K input tokens
	defaultOutputPricePer1K = 0.006 // USD per 1K output tokens
	defaultCostMarkup       = 1.3   // 30% markup on upstream cost
	jwtExpiryHours          = 24
	initialCreditsUSD       = 10.0
)

// ===== Dependency Injection =====

type Deps struct {
	Config  *config.Config
	Store   *storage.Store
	Router  *router.ModelRouter
	// Tasks   *task.Engine   // TODO: re-enable after MVP
	Metrics *metrics.Collector
	Logger  *slog.Logger
}

type Gateway struct {
	app      *fiber.App
	deps     Deps
	auth     *auth.Middleware
	rateLmt  *ratelimit.Limiter
	// billing  *billing.Engine  // TODO: re-enable after MVP (billing done inline)
	streamer *stream.Proxy
}

// ===== Constructor =====

func New(d Deps) *Gateway {
	app := fiber.New(fiber.Config{
		AppName:               "AI Aggregator",
		DisableStartupMessage: true,
		ReadTimeout:           120 * time.Second,
		WriteTimeout:          120 * time.Second,
		IdleTimeout:           120 * time.Second,
		BodyLimit:             50 * 1024 * 1024, // 50MB
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"message":    err.Error(),
					"type":       "api_error",
					"code":       "internal_error",
					"request_id": getRequestID(c),
				},
			})
		},
	})

	// Global middleware
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Request-Id",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	if d.Config.Env == "development" {
		app.Use(fiberlog.New())
	}

	gw := &Gateway{
		app:      app,
		deps:     d,
		auth:     auth.NewMiddleware(d.Store, d.Config.JWTSecret, d.Config.APIKeyPrefix),
		rateLmt:  ratelimit.NewLimiter(d.Store.Redis(), d.Config.DefaultRPM, d.Config.DefaultTPM),
		streamer: stream.NewProxy(),
	}

	gw.registerRoutes()
	return gw
}

// ===== Server Lifecycle =====

func (gw *Gateway) Start(host string, port int) *fiber.App {
	addr := fmt.Sprintf("%s:%d", host, port)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := gw.app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	return gw.app
}

// ===== Route Registration =====

func (gw *Gateway) registerRoutes() {
	// ===== Health & Metrics =====
	gw.app.Get("/health", gw.handleHealth)
	gw.app.Get("/metrics", gw.handleMetrics)

	// ===== OpenAI-compatible API (requires API Key or JWT) =====
	v1 := gw.app.Group("/v1")
	v1.Use(gw.auth.RequireAuth)
	v1.Use(gw.rateLmt.Middleware)
	// NOTE: billing PreCheck removed for MVP; balance checked inline in handlers.

	// Chat
	v1.Post("/chat/completions", gw.handleChatCompletions)

	// Models
	v1.Get("/models", gw.handleListModels)

	// Images (async) -- not yet implemented in MVP
	v1.Post("/images/generations", gw.handleCreateImage)
	v1.Get("/images/generations/:id", gw.handleGetImageTask)

	// Video (async) -- not yet implemented in MVP
	v1.Post("/video/generations", gw.handleCreateVideo)
	v1.Get("/video/generations/:id", gw.handleGetVideoTask)

	// Audio
	v1.Post("/audio/transcriptions", gw.handleTranscribe)
	v1.Post("/audio/speech", gw.handleSpeech)

	// Embeddings
	v1.Post("/embeddings", gw.handleEmbeddings)

	// Files
	v1.Post("/files", gw.handleUploadFile)
	v1.Get("/files/:id", gw.handleGetFile)
	v1.Delete("/files/:id", gw.handleDeleteFile)

	// ===== Admin API (requires admin JWT) =====
	admin := gw.app.Group("/api/admin")
	admin.Use(gw.auth.RequireAdmin)

	admin.Get("/models", gw.adminListModels)
	admin.Post("/models", gw.adminCreateModel)
	admin.Get("/models/:id", gw.adminGetModel)
	admin.Put("/models/:id", gw.adminUpdateModel)
	admin.Delete("/models/:id", gw.adminDeleteModel)
	admin.Post("/models/:id/providers", gw.adminBindProvider)
	admin.Put("/models/:id/providers/:pid", gw.adminUpdateBinding)
	admin.Delete("/models/:id/providers/:pid", gw.adminUnbindProvider)

	admin.Get("/providers", gw.adminListProviders)
	admin.Post("/providers", gw.adminCreateProvider)
	admin.Put("/providers/:id", gw.adminUpdateProvider)
	admin.Get("/providers/:id/health", gw.adminProviderHealth)

	admin.Get("/users", gw.adminListUsers)
	admin.Post("/users", gw.adminCreateUser)
	admin.Get("/users/:id", gw.adminGetUser)
	admin.Put("/users/:id", gw.adminUpdateUser)
	admin.Post("/users/:id/balance", gw.adminTopUpBalance)
	admin.Get("/users/:id/usage", gw.adminUserUsage)

	admin.Get("/analytics/overview", gw.adminAnalyticsOverview)
	admin.Get("/analytics/usage", gw.adminAnalyticsUsage)
	admin.Get("/analytics/cost", gw.adminAnalyticsCost)
	admin.Get("/analytics/latency", gw.adminAnalyticsLatency)
	admin.Get("/analytics/errors", gw.adminAnalyticsErrors)

	admin.Get("/settings", gw.adminGetSettings)
	admin.Put("/settings/:key", gw.adminUpdateSetting)

	admin.Get("/alerts/rules", gw.adminListAlertRules)
	admin.Post("/alerts/rules", gw.adminCreateAlertRule)
	admin.Get("/alerts/history", gw.adminAlertHistory)

	admin.Get("/audit-logs", gw.adminAuditLogs)

	// ===== User API =====
	userAPI := gw.app.Group("/api/user")

	// Auth (public)
	userAPI.Post("/auth/register", gw.userRegister)
	userAPI.Post("/auth/login", gw.userLogin)
	userAPI.Post("/auth/refresh", gw.userRefreshToken)

	// Protected user routes (require JWT)
	userProtected := userAPI.Use(gw.auth.RequireJWT)
	userProtected.Get("/profile", gw.userGetProfile)
	userProtected.Put("/profile", gw.userUpdateProfile)
	userProtected.Get("/dashboard", gw.userDashboard)
	userProtected.Get("/usage", gw.userUsage)
	userProtected.Get("/usage/recent", gw.userRecentUsage)
	userProtected.Get("/keys", gw.userListKeys)
	userProtected.Post("/keys", gw.userCreateKey)
	userProtected.Delete("/keys/:id", gw.userDeleteKey)
	userProtected.Get("/billing/balance", gw.userBalance)
	userProtected.Get("/billing/transactions", gw.userTransactions)
}

// =============================================================================
// HEALTH & METRICS
// =============================================================================

func (gw *Gateway) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": "0.1.0-mvp",
	})
}

func (gw *Gateway) handleMetrics(c *fiber.Ctx) error {
	return gw.deps.Metrics.Handler(c)
}

// =============================================================================
// CHAT COMPLETIONS
// =============================================================================

func (gw *Gateway) handleChatCompletions(c *fiber.Ctx) error {
	start := time.Now()
	ctx := c.UserContext()
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)

	// 1. Parse full request body into provider.ChatRequest
	req, err := parseChatRequest(c.Body())
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "invalid request body: "+err.Error(), requestID))
	}
	if req.Model == "" {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "model field is required", requestID))
	}
	if len(req.Messages) == 0 {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "messages array is required and must not be empty", requestID))
	}

	// 2. Check balance (inline billing pre-check)
	balance, err := gw.deps.Store.GetUserBalance(ctx, userID)
	if err != nil {
		gw.deps.Logger.Warn("balance check failed, failing open", "user_id", userID, "error", err, "request_id", requestID)
		// Fail-open on DB error: allow the request through
	} else if balance <= 0 {
		return c.Status(402).JSON(errorResponse("insufficient_balance", "insufficient balance, please top up", requestID))
	}

	// 3. Get all available providers (sorted by priority)
	routes, err := gw.deps.Router.RouteAll(ctx, req.Model)
	if err != nil || len(routes) == 0 {
		return c.Status(404).JSON(errorResponse("model_not_found", fmt.Sprintf("no available provider for model: %s", req.Model), requestID))
	}

	// 4. Save the original aggregator model ID BEFORE handlers overwrite with upstream name.
	// usage_logs.model_id must record what the USER requested, not the internal upstream name.
	requestedModel := req.Model

	gw.deps.Logger.Info("chat completion request",
		"request_id", requestID,
		"user_id", userID,
		"requested_model", requestedModel,
		"provider_count", len(routes),
		"primary_provider", routes[0].ProviderID,
		"stream", req.Stream,
	)

	// 5. Dispatch to stream or non-stream handler with fallback routes
	if req.Stream {
		return gw.handleStreamChat(c, ctx, routes, req, requestedModel, userID, requestID, start)
	}
	return gw.handleNonStreamChat(c, ctx, routes, req, requestedModel, userID, requestID, start)
}

// handleNonStreamChat sends a synchronous chat completion request to upstream providers with fallback.
// It tries each provider in priority order. On failure, it logs and tries the next.
// On success, it records usage and metrics, deducts balance, and returns the JSON response.
func (gw *Gateway) handleNonStreamChat(c *fiber.Ctx, ctx context.Context, routes []router.RouteEntry, req *provider.ChatRequest, requestedModel, userID, requestID string, start time.Time) error {
	var lastErr error

	for i, route := range routes {
		req.Model = route.UpstreamModel

		resp, err := route.Adapter.ChatCompletion(ctx, req)
		if err != nil {
			lastErr = err
			gw.deps.Logger.Warn("provider failed for non-stream chat completion",
				"request_id", requestID,
				"provider", route.ProviderID,
				"upstream_model", route.UpstreamModel,
				"error", err,
				"attempt", i+1,
				"total_routes", len(routes),
			)
			if i < len(routes)-1 {
				gw.deps.Logger.Warn("trying fallback provider",
					"request_id", requestID,
					"failed_provider", route.ProviderID,
					"fallback_index", i+1,
					"error", err,
				)
				continue
			}
			// All providers failed
			latency := time.Since(start)
			gw.deps.Logger.Error("all providers failed for non-stream chat completion",
				"request_id", requestID,
				"user_id", userID,
				"model", requestedModel,
				"last_error", lastErr,
				"latency_ms", latency.Milliseconds(),
				"attempts", len(routes),
			)
			gw.deps.Metrics.RecordRequest(requestedModel, route.ProviderID, "text", false, 502, latency, 0)
			return c.Status(502).JSON(errorResponse("upstream_error", "all providers failed: "+lastErr.Error(), requestID))
		}

		// Success with this provider
		latency := time.Since(start)
		providerName := route.Adapter.Name()

		// Extract token usage from provider response
		var inputTokens, outputTokens int
		if resp.Usage != nil {
			inputTokens = resp.Usage.PromptTokens
			outputTokens = resp.Usage.CompletionTokens
		}

		// Calculate cost (DB pricing first, fallback to defaults)
		upstreamCost, chargedCost := gw.calculateCost(requestedModel, inputTokens, outputTokens)

		// Record Prometheus metrics
		gw.deps.Metrics.RecordRequest(requestedModel, providerName, "text", false, 200, latency, 0)
		gw.deps.Metrics.RecordTokens(requestedModel, "input", inputTokens)
		gw.deps.Metrics.RecordTokens(requestedModel, "output", outputTokens)
		gw.deps.Metrics.RecordCost(requestedModel, providerName, "upstream", upstreamCost)
		gw.deps.Metrics.RecordCost(requestedModel, providerName, "charged", chargedCost)

		// Record usage to DB and deduct balance SYNCHRONOUSLY (no goroutine)
		apiKeyID, _ := c.Locals("api_key_id").(string)
		usageRecord := &storage.UsageRecord{
			RequestID:    requestID,
			UserID:       userID,
			APIKeyID:     apiKeyID,
			ModelID:      requestedModel, // aggregator model, NOT upstream
			ProviderID:   providerName,
			Modality:     "text",
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			LatencyMs:    int(latency.Milliseconds()),
			IsStream:     false,
			UpstreamCost: upstreamCost,
			ChargedCost:  chargedCost,
			StatusCode:   200,
		}

		gw.recordUsageAndBilling(ctx, usageRecord)

		if i > 0 {
			gw.deps.Logger.Info("chat completion completed via fallback",
				"request_id", requestID,
				"user_id", userID,
				"model", requestedModel,
				"provider", providerName,
				"fallback_index", i,
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"upstream_cost", upstreamCost,
				"charged_cost", chargedCost,
				"latency_ms", latency.Milliseconds(),
			)
		} else {
			gw.deps.Logger.Info("chat completion completed",
				"request_id", requestID,
				"user_id", userID,
				"model", requestedModel,
				"provider", providerName,
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"upstream_cost", upstreamCost,
				"charged_cost", chargedCost,
				"latency_ms", latency.Milliseconds(),
			)
		}

		return c.JSON(resp)
	}

	// Should not reach here (routes is guaranteed non-empty from caller), but safeguard
	return c.Status(502).JSON(errorResponse("upstream_error", "no providers available", requestID))
}

// handleStreamChat initiates a streaming chat completion with provider fallback, relays SSE
// chunks to the client, captures final usage from the last chunk, and records billing/metrics
// asynchronously. If stream initiation fails on one provider, it tries the next.
func (gw *Gateway) handleStreamChat(c *fiber.Ctx, ctx context.Context, routes []router.RouteEntry, req *provider.ChatRequest, requestedModel, userID, requestID string, start time.Time) error {
	var lastErr error

	for i, route := range routes {
		req.Model = route.UpstreamModel

		ch, err := route.Adapter.ChatCompletionStream(ctx, req)
		if err != nil {
			lastErr = err
			gw.deps.Logger.Warn("provider failed for stream chat initiation",
				"request_id", requestID,
				"provider", route.ProviderID,
				"upstream_model", route.UpstreamModel,
				"error", err,
				"attempt", i+1,
				"total_routes", len(routes),
			)
			if i < len(routes)-1 {
				gw.deps.Logger.Warn("trying fallback provider for stream",
					"request_id", requestID,
					"failed_provider", route.ProviderID,
					"fallback_index", i+1,
					"error", err,
				)
				continue
			}
			// All providers failed
			gw.deps.Logger.Error("all providers failed for stream chat initiation",
				"request_id", requestID,
				"user_id", userID,
				"model", requestedModel,
				"last_error", lastErr,
				"attempts", len(routes),
			)
			return c.Status(502).JSON(errorResponse("upstream_error", "all providers failed: "+lastErr.Error(), requestID))
		}

		// Stream initiated successfully with this provider
		// Capture context data before the handler returns
		apiKeyID, _ := c.Locals("api_key_id").(string)
		providerName := route.Adapter.Name()

		if i > 0 {
			gw.deps.Logger.Info("stream initiated via fallback provider",
				"request_id", requestID,
				"provider", providerName,
				"fallback_index", i,
			)
		}

		// Wrap the upstream channel to capture final usage when the stream ends.
		wrappedCh := make(chan provider.StreamChunk)
		var finalUsage *provider.Usage

		go func() {
			defer close(wrappedCh)
			for chunk := range ch {
				if chunk.Usage != nil {
					finalUsage = chunk.Usage
				}
				wrappedCh <- chunk
			}

			// Stream complete -- record usage, cost, and metrics.
			latency := time.Since(start)

			var inputTokens, outputTokens int
			if finalUsage != nil {
				inputTokens = finalUsage.PromptTokens
				outputTokens = finalUsage.CompletionTokens
			}

			upstreamCost, chargedCost := gw.calculateCost(requestedModel, inputTokens, outputTokens)

			gw.deps.Metrics.RecordTokens(requestedModel, "input", inputTokens)
			gw.deps.Metrics.RecordTokens(requestedModel, "output", outputTokens)
			gw.deps.Metrics.RecordCost(requestedModel, providerName, "upstream", upstreamCost)
			gw.deps.Metrics.RecordCost(requestedModel, providerName, "charged", chargedCost)

			// Record usage to DB and deduct balance (sync within goroutine)
			gw.recordUsageAndBilling(context.Background(), &storage.UsageRecord{
				RequestID:    requestID,
				UserID:       userID,
				APIKeyID:     apiKeyID,
				ModelID:      requestedModel, // aggregator model, NOT upstream
				ProviderID:   providerName,
				Modality:     "text",
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				LatencyMs:    int(latency.Milliseconds()),
				IsStream:     true,
				UpstreamCost: upstreamCost,
				ChargedCost:  chargedCost,
				StatusCode:   200,
			})

			gw.deps.Logger.Info("stream chat completed",
				"request_id", requestID,
				"user_id", userID,
				"model", requestedModel,
				"provider", providerName,
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"upstream_cost", upstreamCost,
				"charged_cost", chargedCost,
				"latency_ms", latency.Milliseconds(),
			)
		}()

		// Record request-level metric
		gw.deps.Metrics.RecordRequest(requestedModel, providerName, "text", true, 200, time.Since(start), 0)
		// RelayStream handles SSE writing internally; returns a result channel (not error)
		gw.streamer.RelayStream(c, wrappedCh)
		return nil
	}

	// Should not reach here (routes is guaranteed non-empty from caller), but safeguard
	return c.Status(502).JSON(errorResponse("upstream_error", "no providers available", requestID))
}

// =============================================================================
// MODELS
// =============================================================================

func (gw *Gateway) handleListModels(c *fiber.Ctx) error {
	models := gw.deps.Router.ListModels()
	data := make([]fiber.Map, 0, len(models))
	for _, m := range models {
		data = append(data, fiber.Map{
			"id":           m.ModelID,
			"object":       "model",
			"created":      time.Now().Unix(),
			"owned_by":     "alibaba",
			"display_name": m.DisplayName,
			"modality":     m.Modality,
		})
	}
	return c.JSON(fiber.Map{
		"object": "list",
		"data":   data,
	})
}

// =============================================================================
// USER AUTH
// =============================================================================

func (gw *Gateway) userRegister(c *fiber.Ctx) error {
	requestID := getRequestID(c)

	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "invalid request body", requestID))
	}

	// Validate required fields
	if req.Email == "" {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "email is required", requestID))
	}
	if req.Password == "" {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "password is required", requestID))
	}
	if len(req.Password) < 8 {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "password must be at least 8 characters", requestID))
	}
	if req.Username == "" {
		req.Username = req.Email
	}

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		gw.deps.Logger.Error("bcrypt hash failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to process password", requestID))
	}

	// Create user (store generates the DB-side ID)
	userID, err := gw.deps.Store.CreateUser(c.UserContext(), req.Email, req.Username, string(hashedPassword))
	if err != nil {
		return c.Status(409).JSON(errorResponse("conflict", "email already registered or creation failed: "+err.Error(), requestID))
	}

	// Grant initial credits ($10) and record in billing_transactions
	if err := gw.deps.Store.AddBalance(c.UserContext(), userID, initialCreditsUSD); err != nil {
		gw.deps.Logger.Warn("failed to add initial credits", "user_id", userID, "error", err)
		// Non-fatal: user is created, just without initial credits
	} else {
		// Write audit trail for the credit grant
		if err := gw.deps.Store.CreateBillingTransaction(c.UserContext(), userID, initialCreditsUSD, "credit_grant", "Welcome bonus for new user"); err != nil {
			gw.deps.Logger.Warn("failed to record credit_grant transaction", "user_id", userID, "error", err)
		}
	}

	// Generate JWT
	jwtSecret := []byte(gw.deps.Config.JWTSecret)
	token, err := auth.GenerateJWT(jwtSecret, userID, "user", jwtExpiryHours)
	if err != nil {
		gw.deps.Logger.Error("JWT generation failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to generate token", requestID))
	}

	gw.deps.Logger.Info("user registered", "user_id", userID, "email", req.Email, "request_id", requestID)

	return c.Status(201).JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":       userID,
			"email":    req.Email,
			"username": req.Username,
			"role":     "user",
		},
	})
}

func (gw *Gateway) userLogin(c *fiber.Ctx) error {
	requestID := getRequestID(c)

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "invalid request body", requestID))
	}
	if req.Email == "" || req.Password == "" {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "email and password are required", requestID))
	}

	// Look up user by email
	userID, username, passwordHash, role, err := gw.deps.Store.GetUserByEmail(c.UserContext(), req.Email)
	if err != nil {
		return c.Status(401).JSON(errorResponse("authentication_error", "invalid email or password", requestID))
	}

	// Compare password hash
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(errorResponse("authentication_error", "invalid email or password", requestID))
	}

	// Generate JWT
	jwtSecret := []byte(gw.deps.Config.JWTSecret)
	token, err := auth.GenerateJWT(jwtSecret, userID, role, jwtExpiryHours)
	if err != nil {
		gw.deps.Logger.Error("JWT generation failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to generate token", requestID))
	}

	gw.deps.Logger.Info("user logged in", "user_id", userID, "email", req.Email, "request_id", requestID)

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":       userID,
			"email":    req.Email,
			"username": username,
			"role":     role,
		},
	})
}

func (gw *Gateway) userRefreshToken(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// USER PROFILE
// =============================================================================

func (gw *Gateway) userGetProfile(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) userUpdateProfile(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// API KEYS
// =============================================================================

func (gw *Gateway) userListKeys(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	keys, err := gw.deps.Store.ListAPIKeys(c.UserContext(), userID)
	if err != nil {
		gw.deps.Logger.Error("failed to list API keys", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to list API keys", requestID))
	}

	return c.JSON(fiber.Map{"data": keys})
}

func (gw *Gateway) userCreateKey(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	var req struct {
		Name string `json:"name"`
	}
	// BodyParser may fail for empty body; that's fine, we use defaults
	_ = c.BodyParser(&req)
	if req.Name == "" {
		req.Name = "default"
	}

	// Generate the API key
	plainKey, keyHash, err := auth.GenerateAPIKey(gw.deps.Config.APIKeyPrefix)
	if err != nil {
		gw.deps.Logger.Error("API key generation failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to generate API key", requestID))
	}

	keyID, err := gw.deps.Store.CreateAPIKey(c.UserContext(), userID, req.Name, keyHash, gw.deps.Config.APIKeyPrefix, nil)
	if err != nil {
		gw.deps.Logger.Error("API key creation failed", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to store API key", requestID))
	}

	gw.deps.Logger.Info("API key created", "user_id", userID, "key_id", keyID, "name", req.Name, "request_id", requestID)

	return c.Status(201).JSON(fiber.Map{
		"id":     keyID,
		"name":   req.Name,
		"key":    plainKey, // Shown ONCE; never returned again
		"prefix": gw.deps.Config.APIKeyPrefix,
	})
}

func (gw *Gateway) userDeleteKey(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	keyID := c.Params("id")

	if keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request_error", "key id is required", requestID))
	}

	if err := gw.deps.Store.RevokeAPIKey(c.UserContext(), userID, keyID); err != nil {
		gw.deps.Logger.Error("API key revocation failed", "user_id", userID, "key_id", keyID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to revoke API key", requestID))
	}

	gw.deps.Logger.Info("API key revoked", "user_id", userID, "key_id", keyID, "request_id", requestID)

	return c.JSON(fiber.Map{"deleted": true, "id": keyID})
}

// =============================================================================
// BILLING & USAGE
// =============================================================================

func (gw *Gateway) userBalance(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	balance, err := gw.deps.Store.GetUserBalance(c.UserContext(), userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get balance", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get balance", requestID))
	}

	return c.JSON(fiber.Map{"balance_usd": balance})
}

func (gw *Gateway) userUsage(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	logs, err := gw.deps.Store.GetUsageLogs(c.UserContext(), userID, 50)
	if err != nil {
		gw.deps.Logger.Error("failed to get usage logs", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get usage logs", requestID))
	}

	return c.JSON(fiber.Map{"data": logs})
}

func (gw *Gateway) userRecentUsage(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) userDashboard(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	ctx := c.UserContext()

	totalRequests, totalCost, totalTokens, err := gw.deps.Store.GetUsageSummary(ctx, userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get usage summary", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get usage summary", requestID))
	}

	balance, err := gw.deps.Store.GetUserBalance(ctx, userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get balance for dashboard", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get balance", requestID))
	}

	return c.JSON(fiber.Map{
		"total_requests": totalRequests,
		"total_cost":     totalCost,
		"total_tokens":   totalTokens,
		"balance":        balance,
	})
}

func (gw *Gateway) userTransactions(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	txns, err := gw.deps.Store.GetBillingTransactions(c.UserContext(), userID, 50)
	if err != nil {
		gw.deps.Logger.Error("failed to get billing transactions", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get billing transactions", requestID))
	}

	return c.JSON(fiber.Map{"data": txns})
}

// =============================================================================
// ASYNC TASKS (Image / Video) -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) handleCreateImage(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleGetImageTask(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleCreateVideo(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleGetVideoTask(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// AUDIO -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) handleTranscribe(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleSpeech(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// EMBEDDINGS -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) handleEmbeddings(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// FILES -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) handleUploadFile(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleGetFile(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

func (gw *Gateway) handleDeleteFile(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"},
	})
}

// =============================================================================
// ADMIN HANDLERS -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) adminListModels(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminCreateModel(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminGetModel(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUpdateModel(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminDeleteModel(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminBindProvider(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUpdateBinding(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUnbindProvider(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminListProviders(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminCreateProvider(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUpdateProvider(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminProviderHealth(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminListUsers(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminCreateUser(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminGetUser(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUpdateUser(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminTopUpBalance(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUserUsage(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminAnalyticsOverview(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminAnalyticsUsage(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminAnalyticsCost(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminAnalyticsLatency(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminAnalyticsErrors(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminGetSettings(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminUpdateSetting(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminListAlertRules(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminCreateAlertRule(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}
func (gw *Gateway) adminAlertHistory(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

func (gw *Gateway) adminAuditLogs(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": fiber.Map{"message": "not yet implemented in MVP", "type": "not_implemented"}})
}

// =============================================================================
// COST CALCULATION
// =============================================================================

// calculateCost computes upstream cost and charged cost (with markup) for text modality.
// Priority: DB model pricing (input_price/output_price per 1K tokens) -> hardcoded defaults.
// Formula: cost = (input_tokens * input_price + output_tokens * output_price) / 1000
// chargedCost = upstreamCost * markup
func (gw *Gateway) calculateCost(modelID string, inputTokens, outputTokens int) (upstreamCost, chargedCost float64) {
	inputPrice := defaultInputPricePer1K
	outputPrice := defaultOutputPricePer1K
	markup := defaultCostMarkup

	// Try DB pricing first
	dbInput, dbOutput, _, err := gw.deps.Store.GetModelPricing(context.Background(), modelID)
	if err != nil {
		gw.deps.Logger.Warn("failed to get model pricing from DB, using defaults", "model", modelID, "error", err)
	} else {
		if dbInput != nil {
			inputPrice = *dbInput
		}
		if dbOutput != nil {
			outputPrice = *dbOutput
		}
	}

	upstreamCost = (float64(inputTokens)*inputPrice + float64(outputTokens)*outputPrice) / 1000.0
	chargedCost = upstreamCost * markup
	return upstreamCost, chargedCost
}

// =============================================================================
// USAGE RECORDING
// =============================================================================

// recordUsageAndBilling persists usage to DB, deducts balance, and writes a billing transaction.
// This runs SYNCHRONOUSLY to ensure billing consistency. If any step fails, it logs the error
// but does not return it to the caller (the response has already been sent).
func (gw *Gateway) recordUsageAndBilling(ctx context.Context, record *storage.UsageRecord) {
	// 1. Record usage log
	if err := gw.deps.Store.RecordUsage(ctx, record); err != nil {
		gw.deps.Logger.Error("failed to record usage",
			"error", err,
			"request_id", record.RequestID,
			"user_id", record.UserID,
		)
		return // Can't deduct if we can't record usage
	}

	// 2. Deduct balance
	if record.ChargedCost > 0 {
		if err := gw.deps.Store.DeductBalance(ctx, record.UserID, record.ChargedCost); err != nil {
			gw.deps.Logger.Error("failed to deduct balance",
				"error", err,
				"user_id", record.UserID,
				"amount", record.ChargedCost,
				"request_id", record.RequestID,
			)
			return // Don't write billing tx if deduction failed
		}

		// 3. Write billing transaction record (audit trail)
		desc := fmt.Sprintf("usage_charge: %s via %s (req:%s)", record.ModelID, record.ProviderID, record.RequestID)
		if err := gw.deps.Store.CreateBillingTransaction(ctx, record.UserID, -record.ChargedCost, "usage_charge", desc); err != nil {
			gw.deps.Logger.Error("failed to write billing transaction",
				"error", err,
				"user_id", record.UserID,
				"amount", -record.ChargedCost,
				"request_id", record.RequestID,
			)
		}
	}
}

// =============================================================================
// REQUEST PARSING
// =============================================================================

// parseChatRequest deserializes the raw JSON body into a provider.ChatRequest.
func parseChatRequest(body []byte) (*provider.ChatRequest, error) {
	var req provider.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &req, nil
}

// =============================================================================
// HELPERS
// =============================================================================

// errorResponse builds a standard OpenAI-compatible error envelope.
func errorResponse(errType, message, requestID string) fiber.Map {
	resp := fiber.Map{
		"error": fiber.Map{
			"message": message,
			"type":    errType,
		},
	}
	if requestID != "" {
		resp["error"].(fiber.Map)["request_id"] = requestID
	}
	return resp
}

// getRequestID safely extracts the request ID set by the requestid middleware.
func getRequestID(c *fiber.Ctx) string {
	if rid, ok := c.Locals("requestid").(string); ok {
		return rid
	}
	return ""
}

// generateID creates a random 32-character hex identifier using crypto/rand.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely unlikely)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
