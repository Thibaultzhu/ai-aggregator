package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ai-aggregator/internal/auth"
	"ai-aggregator/internal/config"
	"ai-aggregator/internal/filestore"
	"ai-aggregator/internal/metrics"
	"ai-aggregator/internal/provider"
	"ai-aggregator/internal/ratelimit"
	"ai-aggregator/internal/router"
	"ai-aggregator/internal/storage"
	"ai-aggregator/internal/stream"
	"ai-aggregator/internal/task"

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
	Config    *config.Config
	Store     *storage.Store
	Router    *router.ModelRouter
	Tasks     *task.Engine
	Metrics   *metrics.Collector
	FileStore filestore.Store
	Logger    *slog.Logger
}

type Gateway struct {
	app     *fiber.App
	deps    Deps
	auth    *auth.Middleware
	rateLmt *ratelimit.Limiter
	// billing  *billing.Engine  // TODO: re-enable after MVP (billing done inline)
	streamer *stream.Proxy
}

// ===== Constructor =====

func New(d Deps) *Gateway {
	if d.FileStore == nil {
		d.FileStore = filestore.NewLocalStore(d.Config.FileStorageDir)
	}
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

	// ===== Public Marketplace API =====
	marketplace := gw.app.Group("/api/marketplace")
	marketplace.Get("/models", gw.marketplaceListModels)
	marketplace.Get("/models/compare", gw.marketplaceCompareModels)
	marketplace.Get("/models/:id", gw.marketplaceGetModel)

	// ===== OpenAI-compatible API (requires API Key or JWT) =====
	v1 := gw.app.Group("/v1")
	v1.Use(gw.auth.RequireAuth)
	v1.Use(gw.requireWorkspaceLimits)
	v1.Use(gw.requireAPIKeyModelAccess)
	v1.Use(gw.rateLmt.Middleware)
	// NOTE: billing PreCheck removed for MVP; balance checked inline in handlers.

	// Chat
	v1.Post("/chat/completions", gw.handleChatCompletions)

	// Models
	v1.Get("/models", gw.handleListModels)

	// Images (async)
	v1.Post("/images/generations", gw.handleCreateImage)
	v1.Get("/images/generations/:id", gw.handleGetImageTask)

	// Video (async)
	v1.Post("/video/generations", gw.handleCreateVideo)
	v1.Get("/video/generations/:id", gw.handleGetVideoTask)

	// Audio
	v1.Post("/audio/transcriptions", gw.handleTranscribe)
	v1.Post("/audio/speech", gw.handleSpeech)

	// Embeddings
	v1.Post("/embeddings", gw.handleEmbeddings)

	// Files
	v1.Post("/files", gw.handleUploadFile)
	v1.Get("/files", gw.handleListFiles)
	v1.Get("/files/:id/content", gw.handleDownloadFile)
	v1.Get("/files/:id", gw.handleGetFile)
	v1.Delete("/files/:id", gw.handleDeleteFile)

	// ===== Admin API (requires admin JWT) =====
	admin := gw.app.Group("/api/admin")
	admin.Use(gw.auth.RequireAdmin)

	admin.Get("/models", gw.adminListModels)
	admin.Post("/models", gw.adminCreateModel)
	admin.Get("/models/:id/pricing-history", gw.adminListModelPricingHistory)
	admin.Get("/models/:id", gw.adminGetModel)
	admin.Put("/models/:id", gw.adminUpdateModel)
	admin.Delete("/models/:id", gw.adminDeleteModel)
	admin.Post("/models/:id/providers", gw.adminBindProvider)
	admin.Put("/models/:id/providers/:pid", gw.adminUpdateBinding)
	admin.Delete("/models/:id/providers/:pid", gw.adminUnbindProvider)

	admin.Get("/providers", gw.adminListProviders)
	admin.Post("/providers", gw.adminCreateProvider)
	admin.Put("/providers/:id", gw.adminUpdateProvider)
	admin.Get("/providers/:id/keys", gw.adminListProviderKeys)
	admin.Post("/providers/:id/keys", gw.adminCreateProviderKey)
	admin.Delete("/providers/:id/keys/:key_id", gw.adminRevokeProviderKey)
	admin.Post("/providers/:id/keys/:key_id/validate", gw.adminValidateProviderKey)
	admin.Get("/provider-health", gw.adminProviderHealth)
	admin.Get("/providers/:id/health", gw.adminProviderHealth)
	admin.Get("/providers/:id/health/history", gw.adminProviderHealthHistory)
	admin.Post("/providers/:id/health-check", gw.adminProviderHealthCheck)
	admin.Get("/provider-templates", gw.adminListProviderTemplates)
	admin.Post("/provider-templates/:id/install", gw.adminInstallProviderTemplate)
	admin.Get("/routing-policies", gw.adminListRoutingPolicies)
	admin.Post("/routing-policies", gw.adminCreateRoutingPolicy)

	admin.Get("/organizations", gw.adminListOrganizations)
	admin.Post("/organizations", gw.adminCreateOrganization)
	admin.Get("/invoices", gw.adminListInvoices)
	admin.Get("/invoices/export", gw.adminExportInvoices)
	admin.Get("/invoices/:id/pdf", gw.adminExportInvoicePDF)
	admin.Post("/invoices", gw.adminCreateInvoice)
	admin.Put("/invoices/:id/status", gw.adminUpdateInvoiceStatus)
	admin.Get("/workspaces", gw.adminListWorkspaces)
	admin.Post("/workspaces", gw.adminCreateWorkspace)
	admin.Get("/workspaces/:id/members", gw.adminListWorkspaceMembers)
	admin.Post("/workspaces/:id/members", gw.adminAddWorkspaceMember)
	admin.Get("/workspaces/:id/projects", gw.adminListWorkspaceProjects)
	admin.Post("/workspaces/:id/projects", gw.adminCreateWorkspaceProject)
	admin.Get("/workspaces/:id/usage", gw.adminWorkspaceUsage)
	admin.Get("/workspaces/:id/usage/export", gw.adminWorkspaceUsageExport)
	admin.Get("/workspaces/:id/budgets", gw.adminListWorkspaceBudgets)
	admin.Post("/workspaces/:id/budgets", gw.adminCreateWorkspaceBudget)
	admin.Get("/workspaces/:id/quotas", gw.adminListWorkspaceQuotas)
	admin.Post("/workspaces/:id/quotas", gw.adminCreateWorkspaceQuota)

	admin.Get("/users", gw.adminListUsers)
	admin.Post("/users", gw.adminCreateUser)
	admin.Get("/users/:id", gw.adminGetUser)
	admin.Put("/users/:id", gw.adminUpdateUser)
	admin.Post("/users/:id/balance", gw.adminTopUpBalance)
	admin.Get("/users/:id/usage", gw.adminUserUsage)
	admin.Get("/keys", gw.adminListAPIKeys)
	admin.Put("/keys/:id", gw.adminUpdateAPIKey)
	admin.Delete("/keys/:id", gw.adminRevokeAPIKey)

	admin.Get("/analytics/overview", gw.adminAnalyticsOverview)
	admin.Get("/analytics/usage", gw.adminAnalyticsUsage)
	admin.Get("/analytics/cost", gw.adminAnalyticsCost)
	admin.Get("/analytics/latency", gw.adminAnalyticsLatency)
	admin.Get("/analytics/errors", gw.adminAnalyticsErrors)

	admin.Get("/settings", gw.adminGetSettings)
	admin.Put("/settings/:key", gw.adminUpdateSetting)
	admin.Post("/files/retention/run", gw.adminRunFileRetention)

	admin.Get("/alerts/rules", gw.adminListAlertRules)
	admin.Post("/alerts/rules", gw.adminCreateAlertRule)
	admin.Put("/alerts/rules/:id", gw.adminUpdateAlertRule)
	admin.Get("/alerts/history", gw.adminAlertHistory)
	admin.Post("/alerts/history/:id/ack", gw.adminAcknowledgeAlert)
	admin.Post("/alerts/history/:id/resolve", gw.adminResolveAlert)

	admin.Get("/audit-logs", gw.adminAuditLogs)
	admin.Get("/audit-logs/export", gw.adminAuditLogsExport)
	admin.Post("/audit-logs/retention/run", gw.adminRunAuditLogRetention)
	admin.Get("/guardrails/policies", gw.adminListGuardrailPolicies)
	admin.Post("/guardrails/policies", gw.adminCreateGuardrailPolicy)
	admin.Get("/guardrails/results", gw.adminListGuardrailResults)
	admin.Get("/benchmarks/tasks", gw.adminListBenchmarkTasks)
	admin.Post("/benchmarks/tasks", gw.adminCreateBenchmarkTask)
	admin.Get("/benchmarks/runs", gw.adminListBenchmarkRuns)
	admin.Post("/benchmarks/tasks/:id/runs", gw.adminRunBenchmark)
	admin.Get("/benchmarks/runs/:id", gw.adminGetBenchmarkRun)
	admin.Get("/inference/clusters", gw.adminListInferenceClusters)
	admin.Post("/inference/clusters", gw.adminCreateInferenceCluster)
	admin.Get("/inference/nodes", gw.adminListInferenceNodes)
	admin.Post("/inference/nodes", gw.adminCreateInferenceNode)
	admin.Get("/inference/deployments", gw.adminListModelDeployments)
	admin.Post("/inference/deployments", gw.adminCreateModelDeployment)

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
	userProtected.Get("/request-logs", gw.userRequestLogs)
	userProtected.Get("/request-logs/:request_id", gw.userRequestLogDetail)
	userProtected.Get("/keys", gw.userListKeys)
	userProtected.Post("/keys", gw.userCreateKey)
	userProtected.Delete("/keys/:id", gw.userDeleteKey)
	userProtected.Get("/providers", gw.userListProviders)
	userProtected.Get("/provider-keys", gw.userListProviderKeys)
	userProtected.Post("/provider-keys", gw.userCreateProviderKey)
	userProtected.Delete("/provider-keys/:id", gw.userRevokeProviderKey)
	userProtected.Get("/billing/balance", gw.userBalance)
	userProtected.Get("/billing/transactions", gw.userTransactions)
	userProtected.Get("/tools", gw.userListTools)
	userProtected.Get("/tool-credentials", gw.userListToolCredentials)
	userProtected.Post("/tool-credentials", gw.userCreateToolCredential)
	userProtected.Delete("/tool-credentials/:id", gw.userRevokeToolCredential)
	userProtected.Get("/prompt-templates", gw.userListPromptTemplates)
	userProtected.Post("/prompt-templates", gw.userCreatePromptTemplate)
	userProtected.Get("/prompt-templates/:id", gw.userGetPromptTemplate)
	userProtected.Delete("/prompt-templates/:id", gw.userArchivePromptTemplate)
	userProtected.Get("/workspaces/:id/budgets", gw.userListWorkspaceBudgets)
	userProtected.Post("/workspaces/:id/budgets", gw.userCreateWorkspaceBudget)
	userProtected.Get("/workspaces/:id/quotas", gw.userListWorkspaceQuotas)
	userProtected.Post("/workspaces/:id/quotas", gw.userCreateWorkspaceQuota)
	userProtected.Get("/workflows", gw.userListWorkflows)
	userProtected.Post("/workflows", gw.userCreateWorkflow)
	userProtected.Get("/workflows/:id", gw.userGetWorkflow)
	userProtected.Get("/workflows/:id/runs", gw.userListWorkflowRuns)
	userProtected.Post("/workflows/:id/runs", gw.userRunWorkflow)
	userProtected.Get("/workflow-runs/:id", gw.userGetWorkflowRun)
	userProtected.Get("/agent-sessions", gw.userListAgentSessions)
	userProtected.Post("/agent-sessions", gw.userCreateAgentSession)
	userProtected.Get("/agent-sessions/:id", gw.userGetAgentSession)
	userProtected.Delete("/agent-sessions/:id", gw.userCloseAgentSession)
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

func (gw *Gateway) requireWorkspaceLimits(c *fiber.Ctx) error {
	workspaceID := localString(c, "workspace_id")
	if workspaceID == "" {
		return c.Next()
	}
	allowed, reason, err := gw.deps.Store.CheckWorkspaceLimits(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Warn("workspace limit check failed, failing open", "workspace_id", workspaceID, "error", err, "request_id", getRequestID(c))
		return c.Next()
	}
	if !allowed {
		return c.Status(429).JSON(errorResponse("quota_exceeded", reason, getRequestID(c)))
	}
	return c.Next()
}

func (gw *Gateway) requireWorkspacePermission(c *fiber.Ctx, workspaceID, permission, deniedMessage string) bool {
	requestID := getRequestID(c)
	if workspaceID == "" {
		_ = c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
		return false
	}
	userID := auth.GetUserID(c)
	if userID == "" {
		_ = c.Status(401).JSON(errorResponse("authentication_error", "user is not authenticated", requestID))
		return false
	}
	allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, permission)
	if err != nil {
		gw.deps.Logger.Error("failed to check workspace permission", "workspace_id", workspaceID, "user_id", userID, "permission", permission, "error", err, "request_id", requestID)
		_ = c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		return false
	}
	if !allowed {
		_ = c.Status(403).JSON(errorResponse("permission_denied", deniedMessage, requestID))
		return false
	}
	return true
}

func (gw *Gateway) requireAPIKeyModelAccess(c *fiber.Ctx) error {
	if localString(c, "auth_type") != "api_key" {
		return c.Next()
	}
	modelID := requestedModelID(c)
	if modelID == "" {
		return c.Next()
	}
	raw, ok := c.Locals("permissions").(json.RawMessage)
	if !ok || len(raw) == 0 || apiKeyAllowsModel(raw, modelID) {
		return c.Next()
	}
	return c.Status(403).JSON(errorResponse("permission_denied", "API key is not allowed to access model: "+modelID, getRequestID(c)))
}

func requestedModelID(c *fiber.Ctx) string {
	if strings.Contains(c.Path(), "/audio/transcriptions") {
		return strings.TrimSpace(c.FormValue("model"))
	}
	var body struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Model)
}

func apiKeyAllowsModel(raw json.RawMessage, modelID string) bool {
	var perms struct {
		Models interface{} `json:"models"`
	}
	if err := json.Unmarshal(raw, &perms); err != nil || perms.Models == nil {
		return true
	}
	switch models := perms.Models.(type) {
	case string:
		return models == "*" || models == modelID
	case []interface{}:
		for _, item := range models {
			if s, ok := item.(string); ok && (s == "*" || s == modelID) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func estimateChatRequestTokens(req *provider.ChatRequest) int {
	if req == nil {
		return 0
	}
	chars := 0
	for _, msg := range req.Messages {
		chars += len([]rune(chatContentText(msg.Content)))
	}
	promptTokens := chars / 4
	if promptTokens < len(req.Messages) {
		promptTokens = len(req.Messages)
	}
	outputBudget := 256
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		outputBudget = *req.MaxTokens
	}
	return promptTokens + outputBudget
}

func chatContentText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []provider.ContentPart:
		var b strings.Builder
		for _, part := range v {
			b.WriteString(part.Text)
			if part.ImageURL != nil {
				b.WriteString(" image_url ")
			}
		}
		return b.String()
	case []interface{}:
		var b strings.Builder
		for _, item := range v {
			switch typed := item.(type) {
			case map[string]interface{}:
				if s, ok := typed["text"].(string); ok {
					b.WriteString(s)
				}
				if _, ok := typed["image_url"]; ok {
					b.WriteString(" image_url ")
				}
			case string:
				b.WriteString(typed)
			}
		}
		return b.String()
	default:
		encoded, _ := json.Marshal(v)
		return string(encoded)
	}
}

func (gw *Gateway) enforceAPIKeyTokenLimit(c *fiber.Ctx, tokens int) bool {
	apiKeyID := localString(c, "api_key_id")
	if apiKeyID == "" || tokens <= 0 {
		return false
	}
	limit := localInt(c, "rate_limit_tpm")
	allowed, err := gw.rateLmt.CheckTokenLimit(c.UserContext(), apiKeyID, tokens, limit)
	if err != nil {
		gw.deps.Logger.Warn("token rate limit check failed, allowing request", "error", err, "key_id", apiKeyID, "request_id", getRequestID(c))
		return false
	}
	if allowed {
		return false
	}
	_ = c.Status(429).JSON(fiber.Map{
		"error": fiber.Map{
			"message": "rate limit exceeded (tokens per minute)",
			"type":    "rate_limit_exceeded",
			"code":    "tpm_limit",
		},
	})
	return true
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
		return gw.respondChatError(c, start, requestID, userID, "", "", 400, "invalid_request", "validation_error", "invalid request body: "+err.Error(), 0)
	}
	if req.Model == "" {
		return gw.respondChatError(c, start, requestID, userID, "", "", 400, "invalid_request", "validation_error", "model field is required", 0)
	}
	if len(req.Messages) == 0 {
		return gw.respondChatError(c, start, requestID, userID, req.Model, "", 400, "invalid_request", "validation_error", "messages array is required and must not be empty", 0)
	}
	if result, blocked, err := gw.applyGuardrails(c, req, requestID, userID); err != nil {
		gw.deps.Logger.Warn("guardrail evaluation failed, failing open", "error", err, "request_id", requestID)
	} else if blocked {
		gw.recordAudit(c, "guardrail.block", "guardrail_result", result.ID, localString(c, "workspace_id"), fiber.Map{"model_id": req.Model, "categories": result.Categories})
		return gw.respondChatError(c, start, requestID, userID, req.Model, "", 400, "policy_violation", "policy_error", "request blocked by guardrail policy", 0)
	}
	if req.Stream {
		if gw.enforceAPIKeyTokenLimit(c, estimateChatRequestTokens(req)) {
			return nil
		}
	}

	// 2. Check balance (inline billing pre-check)
	balance, err := gw.deps.Store.GetUserBalance(ctx, userID)
	if err != nil {
		gw.deps.Logger.Warn("balance check failed, failing open", "user_id", userID, "error", err, "request_id", requestID)
		// Fail-open on DB error: allow the request through
	} else if balance <= 0 {
		return gw.respondChatError(c, start, requestID, userID, req.Model, "", 402, "insufficient_balance", "billing_error", "insufficient balance, please top up", 0)
	}

	// 3. Get all available providers (sorted by priority)
	routes, err := gw.deps.Router.RouteAllForRequest(ctx, req.Model, userID, localString(c, "workspace_id"))
	if err != nil || len(routes) == 0 {
		return gw.respondChatError(c, start, requestID, userID, req.Model, "", 404, "model_not_found", "routing_error", fmt.Sprintf("no available provider for model: %s", req.Model), 0)
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
				gw.recordFallbackLog(ctx, requestID, requestedModel, route.ProviderID, routes[i+1].ProviderID, "provider_error", "provider_unavailable", err.Error())
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
			return gw.respondChatError(c, start, requestID, userID, requestedModel, route.ProviderID, 502, "provider_unavailable", "provider_error", "all providers failed: "+lastErr.Error(), i)
		}

		// Success with this provider
		latency := time.Since(start)
		providerName := route.Adapter.Name()
		resp.Model = requestedModel

		// Extract token usage from provider response
		var inputTokens, outputTokens int
		if resp.Usage != nil {
			inputTokens = resp.Usage.PromptTokens
			outputTokens = resp.Usage.CompletionTokens
		}
		if gw.enforceAPIKeyTokenLimit(c, inputTokens+outputTokens) {
			return nil
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
		workspaceID, _ := c.Locals("workspace_id").(string)
		projectID, _ := c.Locals("project_id").(string)
		usageRecord := &storage.UsageRecord{
			RequestID:    requestID,
			UserID:       userID,
			APIKeyID:     apiKeyID,
			WorkspaceID:  workspaceID,
			ProjectID:    projectID,
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
		gw.recordRequestLog(ctx, &storage.RequestLog{
			RequestID:       requestID,
			UserID:          userID,
			APIKeyID:        apiKeyID,
			WorkspaceID:     workspaceID,
			ProjectID:       projectID,
			ModelID:         requestedModel,
			ProviderID:      routes[0].ProviderID,
			FinalProviderID: route.ProviderID,
			CredentialScope: route.CredentialScope,
			CredentialKeyID: route.CredentialKeyID,
			Method:          c.Method(),
			Path:            c.Path(),
			StatusCode:      200,
			LatencyMs:       int(latency.Milliseconds()),
			InputTokens:     inputTokens,
			OutputTokens:    outputTokens,
			ChargedCost:     chargedCost,
			UpstreamCost:    upstreamCost,
			FallbackCount:   i,
			RequestPreview:  truncatePreview(string(c.Body())),
			ResponsePreview: responsePreview(resp),
		})

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
	return gw.respondChatError(c, start, requestID, userID, requestedModel, "", 502, "no_available_provider", "routing_error", "no providers available", 0)
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
				gw.recordFallbackLog(ctx, requestID, requestedModel, route.ProviderID, routes[i+1].ProviderID, "provider_error", "provider_unavailable", err.Error())
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
			return gw.respondChatError(c, start, requestID, userID, requestedModel, route.ProviderID, 502, "provider_unavailable", "provider_error", "all providers failed: "+lastErr.Error(), i)
		}

		// Stream initiated successfully with this provider
		// Capture context data before the handler returns
		apiKeyID, _ := c.Locals("api_key_id").(string)
		workspaceID, _ := c.Locals("workspace_id").(string)
		projectID, _ := c.Locals("project_id").(string)
		providerName := route.Adapter.Name()
		method := c.Method()
		path := c.Path()
		reqPreview := truncatePreview(string(c.Body()))

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
				WorkspaceID:  workspaceID,
				ProjectID:    projectID,
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
			gw.recordRequestLog(context.Background(), &storage.RequestLog{
				RequestID:       requestID,
				UserID:          userID,
				APIKeyID:        apiKeyID,
				WorkspaceID:     workspaceID,
				ProjectID:       projectID,
				ModelID:         requestedModel,
				ProviderID:      routes[0].ProviderID,
				FinalProviderID: route.ProviderID,
				CredentialScope: route.CredentialScope,
				CredentialKeyID: route.CredentialKeyID,
				Method:          method,
				Path:            path,
				StatusCode:      200,
				LatencyMs:       int(latency.Milliseconds()),
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				ChargedCost:     chargedCost,
				UpstreamCost:    upstreamCost,
				FallbackCount:   i,
				RequestPreview:  reqPreview,
				ResponsePreview: truncatePreview("[stream]"),
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
	return gw.respondChatError(c, start, requestID, userID, requestedModel, "", 502, "no_available_provider", "routing_error", "no providers available", 0)
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

func (gw *Gateway) marketplaceListModels(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	models, err := gw.deps.Store.GetModels(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list marketplace models", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list marketplace models", requestID))
	}

	modality := c.Query("modality", "")
	query := c.Query("q", "")
	capability := c.Query("capability", "")

	items := make([]fiber.Map, 0, len(models))
	for _, m := range models {
		if modality != "" && m.Modality != modality {
			continue
		}
		if query != "" && !containsFold(m.ModelID+" "+m.DisplayName+" "+m.Modality, query) {
			continue
		}
		if capability != "" && !stringSliceContains(m.Capabilities, capability) && !stringSliceContains(m.Tags, capability) {
			continue
		}
		bindings, _ := gw.deps.Store.ListModelProvidersAdmin(c.UserContext(), m.ModelID)
		benchmarkScore, _ := gw.deps.Store.GetLatestBenchmarkScore(c.UserContext(), m.ModelID)
		items = append(items, marketplaceModelPayload(m, bindings, benchmarkScore))
	}

	return c.JSON(fiber.Map{"data": items, "count": len(items)})
}

func (gw *Gateway) marketplaceGetModel(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	model, err := gw.deps.Store.GetModelAdmin(c.UserContext(), modelID)
	if err != nil || model.Status != "active" {
		return c.Status(404).JSON(errorResponse("not_found", "model not found", requestID))
	}
	bindings, err := gw.deps.Store.ListModelProvidersAdmin(c.UserContext(), modelID)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list model providers", requestID))
	}
	benchmarkScore, _ := gw.deps.Store.GetLatestBenchmarkScore(c.UserContext(), model.ModelID)
	return c.JSON(fiber.Map{"model": marketplaceModelPayload(model, bindings, benchmarkScore), "providers": bindings})
}

func (gw *Gateway) marketplaceCompareModels(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	rawIDs := c.Query("ids", "")
	if rawIDs == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "ids query parameter is required", requestID))
	}
	items := make([]fiber.Map, 0)
	for _, modelID := range splitCSV(rawIDs) {
		model, err := gw.deps.Store.GetModelAdmin(c.UserContext(), modelID)
		if err != nil || model.Status != "active" {
			continue
		}
		bindings, _ := gw.deps.Store.ListModelProvidersAdmin(c.UserContext(), modelID)
		benchmarkScore, _ := gw.deps.Store.GetLatestBenchmarkScore(c.UserContext(), modelID)
		items = append(items, marketplaceModelPayload(model, bindings, benchmarkScore))
	}
	return c.JSON(fiber.Map{"data": items, "count": len(items)})
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
		if errors.Is(err, storage.ErrUserAlreadyExists) {
			return c.Status(409).JSON(errorResponse("conflict", "email or username is already registered", requestID))
		}
		gw.deps.Logger.Error("user creation failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to create user", requestID))
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
	requestID := getRequestID(c)
	authHeader := c.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return c.Status(401).JSON(errorResponse("authentication_error", "bearer token is required", requestID))
	}
	claims, err := auth.ValidateJWT([]byte(gw.deps.Config.JWTSecret), strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		return c.Status(401).JSON(errorResponse("authentication_error", "invalid token", requestID))
	}
	token, err := auth.GenerateJWT([]byte(gw.deps.Config.JWTSecret), claims.UserID, claims.Role, jwtExpiryHours)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_error", "failed to generate token", requestID))
	}
	email, username, role, balance, err := gw.deps.Store.GetUserByID(c.UserContext(), claims.UserID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "user not found", requestID))
	}
	return c.JSON(fiber.Map{"token": token, "user": fiber.Map{"id": claims.UserID, "email": email, "username": username, "role": role, "balance_usd": balance}})
}

// =============================================================================
// USER PROFILE
// =============================================================================

func (gw *Gateway) userGetProfile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	email, username, role, balance, err := gw.deps.Store.GetUserByID(c.UserContext(), userID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "user not found", requestID))
	}
	return c.JSON(fiber.Map{"id": userID, "email": email, "username": username, "role": role, "balance_usd": balance})
}

func (gw *Gateway) userUpdateProfile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	var req struct {
		Username string                 `json:"username"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if strings.TrimSpace(req.Username) == "" && req.Metadata == nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "username or metadata is required", requestID))
	}
	out, err := gw.deps.Store.UpdateUserProfile(c.UserContext(), userID, strings.TrimSpace(req.Username), req.Metadata)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update profile", requestID))
	}
	return c.JSON(out)
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
		Name        string `json:"name"`
		WorkspaceID string `json:"workspace_id"`
		ProjectID   string `json:"project_id"`
	}
	// BodyParser may fail for empty body; that's fine, we use defaults
	_ = c.BodyParser(&req)
	if req.Name == "" {
		req.Name = "default"
	}
	if strings.TrimSpace(req.WorkspaceID) != "" {
		req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), req.WorkspaceID, userID, "api_keys:create")
		if err != nil {
			gw.deps.Logger.Error("workspace permission check failed", "user_id", userID, "workspace_id", req.WorkspaceID, "permission", "api_keys:create", "error", err, "request_id", requestID)
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have api_keys:create permission for the workspace", requestID))
		}
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	if req.ProjectID != "" {
		if req.WorkspaceID == "" {
			return c.Status(400).JSON(errorResponse("invalid_request", "workspace_id is required when project_id is provided", requestID))
		}
		ok, err := gw.deps.Store.ProjectBelongsToWorkspace(c.UserContext(), req.ProjectID, req.WorkspaceID)
		if err != nil {
			gw.deps.Logger.Error("project ownership check failed", "workspace_id", req.WorkspaceID, "project_id", req.ProjectID, "error", err, "request_id", requestID)
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid project_id", requestID))
		}
		if !ok {
			return c.Status(400).JSON(errorResponse("invalid_request", "project_id does not belong to workspace_id", requestID))
		}
	}

	// Generate the API key
	plainKey, keyHash, err := auth.GenerateAPIKey(gw.deps.Config.APIKeyPrefix)
	if err != nil {
		gw.deps.Logger.Error("API key generation failed", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to generate API key", requestID))
	}

	keyID, err := gw.deps.Store.CreateAPIKeyWithProject(c.UserContext(), userID, req.Name, keyHash, gw.deps.Config.APIKeyPrefix, req.WorkspaceID, req.ProjectID, nil)
	if err != nil {
		gw.deps.Logger.Error("API key creation failed", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to store API key", requestID))
	}

	gw.deps.Logger.Info("API key created", "user_id", userID, "key_id", keyID, "name", req.Name, "request_id", requestID)

	return c.Status(201).JSON(fiber.Map{
		"id":           keyID,
		"name":         req.Name,
		"key":          plainKey, // Shown ONCE; never returned again
		"prefix":       gw.deps.Config.APIKeyPrefix,
		"workspace_id": req.WorkspaceID,
		"project_id":   req.ProjectID,
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

func (gw *Gateway) userListProviders(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providers, err := gw.deps.Store.GetProviders(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list user providers", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list providers", requestID))
	}
	return c.JSON(fiber.Map{"data": providers})
}

func (gw *Gateway) userListProviderKeys(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListUserProviderKeys(c.UserContext(), userID)
	if err != nil {
		gw.deps.Logger.Error("failed to list user provider keys", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list provider keys", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userCreateProviderKey(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	var req struct {
		ProviderID string `json:"provider_id"`
		KeyName    string `json:"key_name"`
		Secret     string `json:"secret"`
		Region     string `json:"region"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	req.ProviderID = strings.TrimSpace(req.ProviderID)
	req.KeyName = strings.TrimSpace(req.KeyName)
	req.Secret = strings.TrimSpace(req.Secret)
	req.Region = strings.TrimSpace(req.Region)
	if req.ProviderID == "" || req.KeyName == "" || req.Secret == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider_id, key_name, and secret are required", requestID))
	}
	providerRow, err := gw.deps.Store.GetProviderAdmin(c.UserContext(), req.ProviderID)
	if err != nil || providerRow == nil || !providerRow.IsEnabled {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider is not available", requestID))
	}
	out, err := gw.deps.Store.CreateProviderKeyScoped(c.UserContext(), storage.ProviderKeyInput{
		ProviderID: req.ProviderID,
		KeyName:    req.KeyName,
		KeyRef:     sealCredentialSecret(req.Secret),
		KeyMask:    maskCredentialSecret(req.Secret),
		Region:     req.Region,
		Scope:      "user",
		UserID:     userID,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create user provider key", "user_id", userID, "provider_id", req.ProviderID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create provider key", requestID))
	}
	gw.recordAudit(c, "provider_key.user_create", "provider", req.ProviderID, "", fiber.Map{"provider_id": req.ProviderID, "key_id": out.ID, "key_name": req.KeyName, "scope": "user"})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) userRevokeProviderKey(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	keyID := strings.TrimSpace(c.Params("id"))
	if keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider key id is required", requestID))
	}
	out, err := gw.deps.Store.RevokeUserProviderKey(c.UserContext(), keyID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "provider key not found", requestID))
		}
		gw.deps.Logger.Error("failed to revoke user provider key", "user_id", userID, "key_id", keyID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to revoke provider key", requestID))
	}
	gw.recordAudit(c, "provider_key.user_revoke", "provider", out.ProviderID, "", fiber.Map{"provider_id": out.ProviderID, "key_id": out.ID, "scope": "user"})
	return c.JSON(fiber.Map{"revoked": true, "provider_id": out.ProviderID, "key_id": out.ID})
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

func (gw *Gateway) userRequestLogs(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)

	filter, err := parseRequestLogFilter(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	result, err := gw.deps.Store.ListRequestLogsFiltered(c.UserContext(), userID, filter)
	if err != nil {
		gw.deps.Logger.Error("failed to get request logs", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get request logs", requestID))
	}
	if strings.EqualFold(c.Query("format"), "csv") {
		return writeRequestLogsCSV(c, result.Items, "request-logs.csv")
	}

	return c.JSON(fiber.Map{
		"items":  result.Items,
		"total":  result.Total,
		"limit":  result.Limit,
		"offset": result.Offset,
	})
}

func (gw *Gateway) userRequestLogDetail(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	targetRequestID := c.Params("request_id")
	if targetRequestID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "request_id is required", requestID))
	}

	log, err := gw.deps.Store.GetRequestLogByRequestID(c.UserContext(), userID, targetRequestID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "request log not found", requestID))
	}

	return c.JSON(log)
}

func (gw *Gateway) userRecentUsage(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	requestID := getRequestID(c)
	limit := c.QueryInt("limit", 10)
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	logs, err := gw.deps.Store.GetUsageLogs(c.UserContext(), userID, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to get recent usage logs", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get recent usage logs", requestID))
	}

	return c.JSON(fiber.Map{"data": logs})
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
	avgLatency, p95Latency, p99Latency, err := gw.deps.Store.GetUsageLatencySummary(ctx, userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get latency summary", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get latency summary", requestID))
	}
	errorRequests, errorRate, err := gw.deps.Store.GetUsageErrorSummary(ctx, userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get error summary", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get error summary", requestID))
	}

	balance, err := gw.deps.Store.GetUserBalance(ctx, userID)
	if err != nil {
		gw.deps.Logger.Error("failed to get balance for dashboard", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_error", "failed to get balance", requestID))
	}

	return c.JSON(fiber.Map{
		"total_requests":     totalRequests,
		"total_cost":         totalCost,
		"total_tokens":       totalTokens,
		"average_latency_ms": avgLatency,
		"p95_latency_ms":     p95Latency,
		"p99_latency_ms":     p99Latency,
		"error_requests":     errorRequests,
		"error_rate":         errorRate,
		"balance":            balance,
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
// ASYNC TASKS (Image / Video)
// =============================================================================

func (gw *Gateway) handleCreateImage(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	if gw.deps.Tasks == nil {
		return c.Status(503).JSON(errorResponse("service_unavailable", "task engine is not available", requestID))
	}

	var req struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		N              int    `json:"n"`
		Size           string `json:"size"`
		ResponseFormat string `json:"response_format"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.Model == "" || req.Prompt == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "model and prompt are required", requestID))
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.Size == "" {
		req.Size = "1024*1024"
	}

	params := map[string]interface{}{
		"prompt":          req.Prompt,
		"n":               req.N,
		"size":            req.Size,
		"response_format": req.ResponseFormat,
	}
	taskID, err := gw.deps.Tasks.Submit(c.UserContext(), userID, req.Model, params)
	if err != nil {
		gw.deps.Logger.Warn("failed to submit image task", "request_id", requestID, "model", req.Model, "error", err)
		return c.Status(502).JSON(errorResponse("provider_error", "failed to submit image generation: "+err.Error(), requestID))
	}

	return c.Status(202).JSON(fiber.Map{
		"id":         taskID,
		"object":     "async_task",
		"status":     "submitted",
		"model":      req.Model,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (gw *Gateway) handleGetImageTask(c *fiber.Ctx) error {
	return gw.handleGetAsyncTask(c)
}

func (gw *Gateway) handleCreateVideo(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	if gw.deps.Tasks == nil {
		return c.Status(503).JSON(errorResponse("service_unavailable", "task engine is not available", requestID))
	}

	var req struct {
		Model       string `json:"model"`
		Prompt      string `json:"prompt"`
		ImageURL    string `json:"image_url"`
		Duration    int    `json:"duration"`
		Resolution  string `json:"resolution"`
		CallbackURL string `json:"callback_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.Model == "" || req.Prompt == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "model and prompt are required", requestID))
	}

	params := map[string]interface{}{
		"prompt":       req.Prompt,
		"image_url":    req.ImageURL,
		"duration":     req.Duration,
		"resolution":   req.Resolution,
		"callback_url": req.CallbackURL,
	}
	taskID, err := gw.deps.Tasks.Submit(c.UserContext(), userID, req.Model, params)
	if err != nil {
		gw.deps.Logger.Warn("failed to submit video task", "request_id", requestID, "model", req.Model, "error", err)
		return c.Status(502).JSON(errorResponse("provider_error", "failed to submit video generation: "+err.Error(), requestID))
	}

	return c.Status(202).JSON(fiber.Map{
		"id":         taskID,
		"object":     "async_task",
		"status":     "submitted",
		"model":      req.Model,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (gw *Gateway) handleGetVideoTask(c *fiber.Ctx) error {
	return gw.handleGetAsyncTask(c)
}

func (gw *Gateway) handleGetAsyncTask(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	if gw.deps.Tasks == nil {
		return c.Status(503).JSON(errorResponse("service_unavailable", "task engine is not available", requestID))
	}

	taskID := c.Params("id")
	if taskID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "task id is required", requestID))
	}

	record, err := gw.deps.Tasks.GetResult(c.UserContext(), taskID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "task not found", requestID))
	}
	if record.UserID != userID {
		return c.Status(404).JSON(errorResponse("not_found", "task not found", requestID))
	}

	return c.JSON(fiber.Map{
		"id":           record.ExternalID,
		"object":       "async_task",
		"status":       record.Status,
		"model":        record.ModelID,
		"provider":     record.ProviderID,
		"created_at":   record.CreatedAt.Format(time.RFC3339),
		"completed_at": optionalTime(record.CompletedAt),
		"result":       record.ResultData,
		"cost_usd":     record.CostUSD,
	})
}

func (gw *Gateway) handleTranscribe(c *fiber.Ctx) error {
	start := time.Now()
	ctx := c.UserContext()
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)

	model := c.FormValue("model")
	if model == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "model is required", requestID))
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "multipart file field is required", requestID))
	}
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "failed to open uploaded file", requestID))
	}
	defer file.Close()

	routes, err := gw.deps.Router.RouteAll(ctx, model)
	if err != nil || len(routes) == 0 {
		return c.Status(404).JSON(errorResponse("model_not_found", "no available provider for model: "+model, requestID))
	}

	var lastErr error
	for i, route := range routes {
		req := &provider.TranscribeRequest{
			File:     file,
			Filename: fileHeader.Filename,
			Model:    route.UpstreamModel,
			Language: c.FormValue("language"),
		}
		text, err := route.Adapter.TranscribeAudio(ctx, req)
		if err != nil {
			lastErr = err
			if i < len(routes)-1 {
				gw.recordFallbackLog(ctx, requestID, model, route.ProviderID, routes[i+1].ProviderID, "provider_error", "provider_unavailable", err.Error())
				continue
			}
			break
		}
		latency := time.Since(start)
		apiKeyID, _ := c.Locals("api_key_id").(string)
		workspaceID, _ := c.Locals("workspace_id").(string)
		projectID, _ := c.Locals("project_id").(string)
		outputTokens := len([]rune(text))
		if gw.enforceAPIKeyTokenLimit(c, outputTokens) {
			return nil
		}
		gw.recordUsageAndBilling(ctx, &storage.UsageRecord{
			RequestID:    requestID,
			UserID:       userID,
			APIKeyID:     apiKeyID,
			WorkspaceID:  workspaceID,
			ProjectID:    projectID,
			ModelID:      model,
			ProviderID:   route.ProviderID,
			Modality:     "audio",
			OutputTokens: outputTokens,
			LatencyMs:    int(latency.Milliseconds()),
			StatusCode:   200,
		})
		gw.recordRequestLog(ctx, &storage.RequestLog{
			RequestID:       requestID,
			UserID:          userID,
			APIKeyID:        apiKeyID,
			WorkspaceID:     workspaceID,
			ProjectID:       projectID,
			ModelID:         model,
			ProviderID:      routes[0].ProviderID,
			FinalProviderID: route.ProviderID,
			CredentialScope: route.CredentialScope,
			CredentialKeyID: route.CredentialKeyID,
			Method:          c.Method(),
			Path:            c.Path(),
			StatusCode:      200,
			LatencyMs:       int(latency.Milliseconds()),
			OutputTokens:    outputTokens,
			FallbackCount:   i,
			RequestPreview:  truncatePreview(fileHeader.Filename),
			ResponsePreview: truncatePreview(text),
		})
		return c.JSON(fiber.Map{"text": text})
	}

	message := "all providers failed"
	if lastErr != nil {
		message += ": " + lastErr.Error()
	}
	return c.Status(502).JSON(errorResponse("provider_unavailable", message, requestID))
}

func (gw *Gateway) handleSpeech(c *fiber.Ctx) error {
	start := time.Now()
	ctx := c.UserContext()
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)

	var req provider.SpeechRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.Model == "" || strings.TrimSpace(req.Input) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "model and input are required", requestID))
	}
	if req.Voice == "" {
		req.Voice = "default"
	}

	routes, err := gw.deps.Router.RouteAll(ctx, req.Model)
	if err != nil || len(routes) == 0 {
		return c.Status(404).JSON(errorResponse("model_not_found", "no available provider for model: "+req.Model, requestID))
	}

	requestedModel := req.Model
	var lastErr error
	for i, route := range routes {
		upstreamReq := req
		upstreamReq.Model = route.UpstreamModel
		audioBytes, contentType, err := route.Adapter.SynthesizeSpeech(ctx, &upstreamReq)
		if err != nil {
			lastErr = err
			if i < len(routes)-1 {
				gw.recordFallbackLog(ctx, requestID, requestedModel, route.ProviderID, routes[i+1].ProviderID, "provider_error", "provider_unavailable", err.Error())
				continue
			}
			break
		}
		if contentType == "" {
			contentType = "audio/mpeg"
		}
		latency := time.Since(start)
		apiKeyID, _ := c.Locals("api_key_id").(string)
		workspaceID, _ := c.Locals("workspace_id").(string)
		projectID, _ := c.Locals("project_id").(string)
		inputChars := len([]rune(req.Input))
		if gw.enforceAPIKeyTokenLimit(c, inputChars) {
			return nil
		}
		upstreamCost, chargedCost := gw.calculateCost(requestedModel, 0, inputChars)
		gw.recordUsageAndBilling(ctx, &storage.UsageRecord{
			RequestID:    requestID,
			UserID:       userID,
			APIKeyID:     apiKeyID,
			WorkspaceID:  workspaceID,
			ProjectID:    projectID,
			ModelID:      requestedModel,
			ProviderID:   route.ProviderID,
			Modality:     "audio",
			OutputTokens: inputChars,
			LatencyMs:    int(latency.Milliseconds()),
			UpstreamCost: upstreamCost,
			ChargedCost:  chargedCost,
			StatusCode:   200,
		})
		gw.recordRequestLog(ctx, &storage.RequestLog{
			RequestID:       requestID,
			UserID:          userID,
			APIKeyID:        apiKeyID,
			WorkspaceID:     workspaceID,
			ProjectID:       projectID,
			ModelID:         requestedModel,
			ProviderID:      routes[0].ProviderID,
			FinalProviderID: route.ProviderID,
			CredentialScope: route.CredentialScope,
			CredentialKeyID: route.CredentialKeyID,
			Method:          c.Method(),
			Path:            c.Path(),
			StatusCode:      200,
			LatencyMs:       int(latency.Milliseconds()),
			OutputTokens:    inputChars,
			ChargedCost:     chargedCost,
			UpstreamCost:    upstreamCost,
			FallbackCount:   i,
			RequestPreview:  truncatePreview(req.Input),
			ResponsePreview: fmt.Sprintf("audio bytes: %d", len(audioBytes)),
		})
		c.Set("Content-Type", contentType)
		return c.Send(audioBytes)
	}

	message := "all providers failed"
	if lastErr != nil {
		message += ": " + lastErr.Error()
	}
	return c.Status(502).JSON(errorResponse("provider_unavailable", message, requestID))
}

// =============================================================================
// EMBEDDINGS
// =============================================================================

func (gw *Gateway) handleEmbeddings(c *fiber.Ctx) error {
	start := time.Now()
	ctx := c.UserContext()
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)

	var req provider.EmbeddingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.Model == "" || req.Input == nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "model and input are required", requestID))
	}

	routes, err := gw.deps.Router.RouteAll(ctx, req.Model)
	if err != nil || len(routes) == 0 {
		return c.Status(404).JSON(errorResponse("model_not_found", "no available provider for model: "+req.Model, requestID))
	}

	var lastErr error
	for i, route := range routes {
		upstreamReq := req
		upstreamReq.Model = route.UpstreamModel
		resp, err := route.Adapter.CreateEmbedding(ctx, &upstreamReq)
		if err != nil {
			lastErr = err
			if i < len(routes)-1 {
				gw.recordFallbackLog(ctx, requestID, req.Model, route.ProviderID, routes[i+1].ProviderID, "provider_error", "provider_unavailable", err.Error())
				continue
			}
			break
		}

		latency := time.Since(start)
		apiKeyID, _ := c.Locals("api_key_id").(string)
		workspaceID, _ := c.Locals("workspace_id").(string)
		projectID, _ := c.Locals("project_id").(string)
		inputTokens := 0
		if resp.Usage != nil {
			inputTokens = resp.Usage.PromptTokens
		}
		if gw.enforceAPIKeyTokenLimit(c, inputTokens) {
			return nil
		}
		upstreamCost, chargedCost := gw.calculateCost(req.Model, inputTokens, 0)
		gw.recordUsageAndBilling(ctx, &storage.UsageRecord{
			RequestID:    requestID,
			UserID:       userID,
			APIKeyID:     apiKeyID,
			WorkspaceID:  workspaceID,
			ProjectID:    projectID,
			ModelID:      req.Model,
			ProviderID:   route.ProviderID,
			Modality:     "embedding",
			InputTokens:  inputTokens,
			LatencyMs:    int(latency.Milliseconds()),
			UpstreamCost: upstreamCost,
			ChargedCost:  chargedCost,
			StatusCode:   200,
		})
		gw.recordRequestLog(ctx, &storage.RequestLog{
			RequestID:       requestID,
			UserID:          userID,
			APIKeyID:        apiKeyID,
			WorkspaceID:     workspaceID,
			ProjectID:       projectID,
			ModelID:         req.Model,
			ProviderID:      routes[0].ProviderID,
			FinalProviderID: route.ProviderID,
			CredentialScope: route.CredentialScope,
			CredentialKeyID: route.CredentialKeyID,
			Method:          c.Method(),
			Path:            c.Path(),
			StatusCode:      200,
			LatencyMs:       int(latency.Milliseconds()),
			InputTokens:     inputTokens,
			ChargedCost:     chargedCost,
			UpstreamCost:    upstreamCost,
			FallbackCount:   i,
			RequestPreview:  truncatePreview(string(c.Body())),
			ResponsePreview: truncatePreview(fmt.Sprintf("embedding vectors: %d", len(resp.Data))),
		})
		return c.JSON(resp)
	}

	message := "all providers failed"
	if lastErr != nil {
		message = message + ": " + lastErr.Error()
	}
	return c.Status(502).JSON(errorResponse("provider_unavailable", message, requestID))
}

func (gw *Gateway) handleUploadFile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	apiKeyID, _ := c.Locals("api_key_id").(string)
	workspaceID, _ := c.Locals("workspace_id").(string)
	if workspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, "files:write")
		if err != nil {
			gw.deps.Logger.Error("workspace permission check failed", "user_id", userID, "workspace_id", workspaceID, "permission", "files:write", "error", err, "request_id", requestID)
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have files:write permission for the workspace", requestID))
		}
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "multipart file field is required", requestID))
	}
	if fileHeader.Size <= 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "file must not be empty", requestID))
	}
	maxSizeMB, err := gw.deps.Store.GetSetting(c.UserContext(), "max_file_size_mb")
	if err == nil {
		if maxBytes := parsePositiveInt64(maxSizeMB) * 1024 * 1024; maxBytes > 0 && fileHeader.Size > maxBytes {
			return c.Status(413).JSON(errorResponse("invalid_request", "file exceeds max_file_size_mb", requestID))
		}
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "failed to open uploaded file", requestID))
	}
	defer src.Close()
	firstChunk := make([]byte, 512)
	n, readErr := io.ReadFull(src, firstChunk)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return c.Status(400).JSON(errorResponse("invalid_request", "failed to read uploaded file", requestID))
	}
	firstChunk = firstChunk[:n]
	declaredMime := fileHeader.Header.Get("Content-Type")
	detectedMime := http.DetectContentType(firstChunk)
	if !fileMimeAllowed(detectedMime, declaredMime, gw.uploadMimeAllowList(c.UserContext())) {
		return c.Status(415).JSON(errorResponse("invalid_request", "file mime type is not allowed: "+detectedMime, requestID))
	}
	remaining, err := io.ReadAll(src)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "failed to read uploaded file", requestID))
	}
	payload := append(append([]byte{}, firstChunk...), remaining...)
	scan := scanUploadedFile(payload)
	if scan.Status == "blocked" {
		gw.recordAudit(c, "file.upload_blocked", "file", "", workspaceID, fiber.Map{
			"filename":       fileHeader.Filename,
			"scanner":        scan.Scanner,
			"threat":         scan.Threat,
			"signature":      scan.Signature,
			"detected_mime":  detectedMime,
			"declared_mime":  declaredMime,
			"content_length": fileHeader.Size,
		})
		return c.Status(400).JSON(errorResponse("policy_violation", "file rejected by malware scan: "+scan.Threat, requestID))
	}

	fileID := "file-" + generateID()
	object, err := gw.deps.FileStore.Put(c.UserContext(), fileID, bytes.NewReader(payload))
	if err != nil {
		gw.deps.Logger.Error("failed to store uploaded file", "file_id", fileID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to store file", requestID))
	}

	purpose := c.FormValue("purpose")
	if purpose == "" {
		purpose = "assistants"
	}
	record, err := gw.deps.Store.CreateFileRecord(c.UserContext(), &storage.FileRecord{
		ID:          fileID,
		UserID:      userID,
		APIKeyID:    apiKeyID,
		WorkspaceID: workspaceID,
		Filename:    fileHeader.Filename,
		Purpose:     purpose,
		Bytes:       object.Bytes,
		MimeType:    detectedMime,
		StoragePath: object.Path,
		Metadata: fiber.Map{
			"source":        gw.deps.FileStore.Backend(),
			"declared_mime": declaredMime,
			"detected_mime": detectedMime,
			"sha256":        object.SHA256,
			"extension":     strings.ToLower(filepath.Ext(fileHeader.Filename)),
			"scan_status":   scan.Status,
			"scan_scanner":  scan.Scanner,
			"scan_threat":   scan.Threat,
		},
	})
	if err != nil {
		_ = gw.deps.FileStore.Delete(c.UserContext(), object.Path)
		gw.deps.Logger.Error("failed to create file record", "file_id", fileID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create file record", requestID))
	}
	gw.recordAudit(c, "file.upload", "file", record.ID, workspaceID, fiber.Map{"filename": record.Filename, "bytes": record.Bytes, "purpose": record.Purpose})
	return c.Status(201).JSON(fileResponse(record))
}

func (gw *Gateway) handleListFiles(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	limit := c.QueryInt("limit", 100)
	purpose := strings.TrimSpace(c.Query("purpose"))
	files, err := gw.deps.Store.ListFileRecords(c.UserContext(), auth.GetUserID(c), purpose, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list files", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list files", requestID))
	}
	data := make([]fiber.Map, 0, len(files))
	for i := range files {
		data = append(data, fileResponse(&files[i]))
	}
	return c.JSON(fiber.Map{"object": "list", "data": data})
}

func (gw *Gateway) handleGetFile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	record, err := gw.deps.Store.GetFileRecord(c.UserContext(), auth.GetUserID(c), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "file not found", requestID))
	}
	return c.JSON(fileResponse(record))
}

func (gw *Gateway) handleDownloadFile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	record, err := gw.deps.Store.GetFileRecord(c.UserContext(), auth.GetUserID(c), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "file not found", requestID))
	}
	if record.StoragePath == "" {
		return c.Status(404).JSON(errorResponse("not_found", "file content not found", requestID))
	}
	bytes, err := gw.deps.FileStore.Get(c.UserContext(), record.StoragePath)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "file content not found", requestID))
	}
	contentType := record.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", record.Filename))
	return c.Send(bytes)
}

func (gw *Gateway) handleDeleteFile(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := auth.GetUserID(c)
	record, err := gw.deps.Store.DeleteFileRecord(c.UserContext(), userID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "file not found", requestID))
	}
	if record.StoragePath != "" {
		_ = gw.deps.FileStore.Delete(c.UserContext(), record.StoragePath)
	}
	gw.recordAudit(c, "file.delete", "file", record.ID, record.WorkspaceID, fiber.Map{"filename": record.Filename})
	return c.JSON(fiber.Map{"id": record.ID, "object": "file", "deleted": true})
}

// =============================================================================
// ADMIN HANDLERS -- NOT YET IMPLEMENTED IN MVP
// =============================================================================

func (gw *Gateway) adminListModels(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	models, err := gw.deps.Store.ListModelsAdmin(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list admin models", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list models", requestID))
	}
	return c.JSON(fiber.Map{"data": models})
}
func (gw *Gateway) adminCreateModel(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	model, err := parseAdminModel(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	out, err := gw.deps.Store.UpsertModelAdmin(c.UserContext(), model)
	if err != nil {
		gw.deps.Logger.Error("failed to create admin model", "model_id", model.ModelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create model", requestID))
	}
	gw.recordModelPricingHistory(c, nil, out, "create")
	gw.refreshRouterRegistry(c, requestID)
	return c.Status(201).JSON(out)
}
func (gw *Gateway) adminGetModel(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	model, err := gw.deps.Store.GetModelAdmin(c.UserContext(), modelID)
	if err != nil {
		if err.Error() == "model not found" {
			return c.Status(404).JSON(errorResponse("not_found", "model not found", requestID))
		}
		gw.deps.Logger.Error("failed to get admin model", "model_id", modelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get model", requestID))
	}
	bindings, err := gw.deps.Store.ListModelProvidersAdmin(c.UserContext(), modelID)
	if err != nil {
		gw.deps.Logger.Error("failed to list admin model bindings", "model_id", modelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list model provider bindings", requestID))
	}
	history, _ := gw.deps.Store.ListModelPricingHistory(c.UserContext(), modelID, 20)
	return c.JSON(fiber.Map{"model": model, "providers": bindings, "pricing_history": history})
}
func (gw *Gateway) adminUpdateModel(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	model, err := parseAdminModel(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	model.ModelID = c.Params("id")
	if model.ModelID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "model id is required", requestID))
	}
	before, _ := gw.deps.Store.GetModelAdmin(c.UserContext(), model.ModelID)
	out, err := gw.deps.Store.UpsertModelAdmin(c.UserContext(), model)
	if err != nil {
		gw.deps.Logger.Error("failed to update admin model", "model_id", model.ModelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update model", requestID))
	}
	gw.recordModelPricingHistory(c, before, out, "update")
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(out)
}

func (gw *Gateway) adminListModelPricingHistory(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	if _, err := gw.deps.Store.GetModelAdmin(c.UserContext(), modelID); err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "model not found", requestID))
	}
	items, err := gw.deps.Store.ListModelPricingHistory(c.UserContext(), modelID, c.QueryInt("limit", 50))
	if err != nil {
		gw.deps.Logger.Error("failed to list model pricing history", "model_id", modelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list model pricing history", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) recordModelPricingHistory(c *fiber.Ctx, before, after *storage.Model, changeType string) {
	if after == nil {
		return
	}
	if changeType == "update" && before != nil && sameFloatPtr(before.InputPrice, after.InputPrice) && sameFloatPtr(before.OutputPrice, after.OutputPrice) && before.PriceUnit == after.PriceUnit {
		return
	}
	history := &storage.ModelPricingHistory{
		ModelID:        after.ModelID,
		NewInputPrice:  after.InputPrice,
		NewOutputPrice: after.OutputPrice,
		NewPriceUnit:   after.PriceUnit,
		ChangeType:     changeType,
		ChangedBy:      auth.GetUserID(c),
		Metadata: fiber.Map{
			"source":     "admin_api",
			"request_id": getRequestID(c),
		},
	}
	if before != nil {
		history.OldInputPrice = before.InputPrice
		history.OldOutputPrice = before.OutputPrice
		history.OldPriceUnit = before.PriceUnit
	}
	if history.OldPriceUnit == "" {
		history.OldPriceUnit = ""
	}
	if history.NewPriceUnit == "" {
		history.NewPriceUnit = after.PriceUnit
	}
	if err := gw.deps.Store.RecordModelPricingHistory(c.UserContext(), history); err != nil {
		gw.deps.Logger.Error("failed to record model pricing history", "model_id", after.ModelID, "error", err, "request_id", getRequestID(c))
	}
}

func sameFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (gw *Gateway) adminDeleteModel(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	if err := gw.deps.Store.DeleteModelAdmin(c.UserContext(), modelID); err != nil {
		if err.Error() == "model not found" {
			return c.Status(404).JSON(errorResponse("not_found", "model not found", requestID))
		}
		gw.deps.Logger.Error("failed to delete admin model", "model_id", modelID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to delete model", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(fiber.Map{"deleted": true, "id": modelID})
}
func (gw *Gateway) adminBindProvider(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	binding, err := parseAdminProviderBinding(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	out, err := gw.deps.Store.UpsertModelProviderAdmin(c.UserContext(), modelID, binding)
	if err != nil {
		gw.deps.Logger.Error("failed to bind provider", "model_id", modelID, "provider_id", binding.ProviderID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to bind provider", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.Status(201).JSON(out)
}
func (gw *Gateway) adminUpdateBinding(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	providerID := c.Params("pid")
	binding, err := parseAdminProviderBinding(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	binding.ProviderID = providerID
	out, err := gw.deps.Store.UpsertModelProviderAdmin(c.UserContext(), modelID, binding)
	if err != nil {
		gw.deps.Logger.Error("failed to update provider binding", "model_id", modelID, "provider_id", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update provider binding", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(out)
}
func (gw *Gateway) adminUnbindProvider(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	modelID := c.Params("id")
	providerID := c.Params("pid")
	if err := gw.deps.Store.DeleteModelProviderAdmin(c.UserContext(), modelID, providerID); err != nil {
		if err.Error() == "model provider binding not found" {
			return c.Status(404).JSON(errorResponse("not_found", "model provider binding not found", requestID))
		}
		gw.deps.Logger.Error("failed to unbind provider", "model_id", modelID, "provider_id", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to unbind provider", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(fiber.Map{"deleted": true, "model_id": modelID, "provider_id": providerID})
}

func (gw *Gateway) adminListProviders(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providers, err := gw.deps.Store.ListProvidersAdmin(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list admin providers", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list providers", requestID))
	}
	return c.JSON(fiber.Map{"data": providers})
}
func (gw *Gateway) adminCreateProvider(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerReq, err := parseAdminProvider(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	out, err := gw.deps.Store.UpsertProviderAdmin(c.UserContext(), providerReq)
	if err != nil {
		gw.deps.Logger.Error("failed to create admin provider", "provider_id", providerReq.ID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create provider", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.Status(201).JSON(out)
}
func (gw *Gateway) adminUpdateProvider(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerReq, err := parseAdminProvider(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	providerReq.ID = c.Params("id")
	if providerReq.ID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id is required", requestID))
	}
	out, err := gw.deps.Store.UpsertProviderAdmin(c.UserContext(), providerReq)
	if err != nil {
		gw.deps.Logger.Error("failed to update admin provider", "provider_id", providerReq.ID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update provider", requestID))
	}
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(out)
}

func (gw *Gateway) adminListProviderKeys(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := strings.TrimSpace(c.Params("id"))
	if providerID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id is required", requestID))
	}
	items, err := gw.deps.Store.ListProviderKeys(c.UserContext(), providerID)
	if err != nil {
		gw.deps.Logger.Error("failed to list provider keys", "provider_id", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list provider keys", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateProviderKey(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := strings.TrimSpace(c.Params("id"))
	var req struct {
		KeyName     string `json:"key_name"`
		Secret      string `json:"secret"`
		Region      string `json:"region"`
		Scope       string `json:"scope"`
		UserID      string `json:"user_id"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	req.KeyName = strings.TrimSpace(req.KeyName)
	req.Secret = strings.TrimSpace(req.Secret)
	req.Region = strings.TrimSpace(req.Region)
	req.Scope = strings.TrimSpace(req.Scope)
	req.UserID = strings.TrimSpace(req.UserID)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	if req.Scope == "" {
		req.Scope = "platform"
	}
	if providerID == "" || req.KeyName == "" || req.Secret == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id, key_name and secret are required", requestID))
	}
	if req.Scope == "user" && req.UserID == "" {
		req.UserID = localString(c, "user_id")
	}
	if req.Scope == "workspace" && req.WorkspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace_id is required for workspace scoped provider key", requestID))
	}
	out, err := gw.deps.Store.CreateProviderKeyScoped(c.UserContext(), storage.ProviderKeyInput{
		ProviderID:  providerID,
		KeyName:     req.KeyName,
		KeyRef:      sealCredentialSecret(req.Secret),
		KeyMask:     maskCredentialSecret(req.Secret),
		Region:      req.Region,
		Scope:       req.Scope,
		UserID:      req.UserID,
		WorkspaceID: req.WorkspaceID,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create provider key", "provider_id", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create provider key", requestID))
	}
	gw.recordAudit(c, "provider_key.create", "provider", providerID, req.WorkspaceID, fiber.Map{"provider_id": providerID, "key_name": req.KeyName, "region": req.Region, "scope": req.Scope, "target_user_id": req.UserID})
	gw.refreshRouterRegistry(c, requestID)
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminRevokeProviderKey(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := strings.TrimSpace(c.Params("id"))
	keyID := strings.TrimSpace(c.Params("key_id"))
	if providerID == "" || keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id and key id are required", requestID))
	}
	if err := gw.deps.Store.RevokeProviderKey(c.UserContext(), providerID, keyID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "provider key not found", requestID))
		}
		gw.deps.Logger.Error("failed to revoke provider key", "provider_id", providerID, "key_id", keyID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to revoke provider key", requestID))
	}
	gw.recordAudit(c, "provider_key.revoke", "provider", providerID, "", fiber.Map{"provider_id": providerID, "key_id": keyID})
	gw.refreshRouterRegistry(c, requestID)
	return c.JSON(fiber.Map{"revoked": true, "provider_id": providerID, "key_id": keyID})
}

func (gw *Gateway) adminValidateProviderKey(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := strings.TrimSpace(c.Params("id"))
	keyID := strings.TrimSpace(c.Params("key_id"))
	if providerID == "" || keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id and key id are required", requestID))
	}
	providerConfig, err := gw.deps.Store.GetProviderAdmin(c.UserContext(), providerID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "provider not found", requestID))
		}
		gw.deps.Logger.Error("failed to load provider for credential validation", "provider_id", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to load provider", requestID))
	}
	secret, key, err := gw.deps.Store.GetProviderKeySecretByID(c.UserContext(), providerID, keyID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "provider key not found", requestID))
		}
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}

	adapter, err := providerAdapterForValidation(providerConfig, secret)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	start := time.Now()
	status := "healthy"
	errorMessage := ""
	if err := adapter.HealthCheck(c.UserContext()); err != nil {
		status = "unhealthy"
		errorMessage = err.Error()
	}
	latencyMs := int(time.Since(start).Milliseconds())
	if recordErr := gw.deps.Store.RecordProviderHealthCheck(c.UserContext(), providerID, status, latencyMs, "", errorMessage); recordErr != nil {
		gw.deps.Logger.Warn("failed to record provider credential validation health", "provider_id", providerID, "error", recordErr, "request_id", requestID)
	}
	gw.recordAudit(c, "provider_key.validate", "provider", providerID, key.WorkspaceID, fiber.Map{
		"provider_id": providerID,
		"key_id":      keyID,
		"scope":       key.Scope,
		"status":      status,
	})
	return c.JSON(fiber.Map{
		"provider_id":   providerID,
		"key_id":        keyID,
		"key_mask":      key.KeyMask,
		"scope":         key.Scope,
		"status":        status,
		"latency_ms":    latencyMs,
		"error_message": errorMessage,
		"validated_at":  time.Now().UTC(),
	})
}

func providerAdapterForValidation(p *storage.Provider, apiKey string) (provider.Adapter, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is required")
	}
	switch p.AdapterType {
	case "mock":
		return provider.NewMockAdapterWithName(p.ID), nil
	case "dashscope", "openai_compatible", "self_hosted":
		return provider.NewDashScopeAdapter(provider.DashScopeConfig{Name: p.ID, BaseURL: p.BaseURL, APIKey: apiKey}), nil
	case "anthropic":
		return provider.NewAnthropicAdapter(provider.AnthropicConfig{Name: p.ID, BaseURL: p.BaseURL, APIKey: apiKey}), nil
	default:
		return nil, fmt.Errorf("unsupported provider adapter_type for validation: %s", p.AdapterType)
	}
}

type providerTemplateModel struct {
	ModelID       string   `json:"model_id"`
	DisplayName   string   `json:"display_name"`
	UpstreamModel string   `json:"upstream_model"`
	Capabilities  []string `json:"capabilities"`
	InputPrice    float64  `json:"input_price"`
	OutputPrice   float64  `json:"output_price"`
	MaxContext    int      `json:"max_context"`
	MaxOutput     int      `json:"max_output"`
}

type providerTemplate struct {
	ID          string                  `json:"id"`
	DisplayName string                  `json:"display_name"`
	AdapterType string                  `json:"adapter_type"`
	BaseURL     string                  `json:"base_url"`
	Models      []providerTemplateModel `json:"models"`
	Notes       string                  `json:"notes"`
}

func providerOnboardingTemplates() []providerTemplate {
	return []providerTemplate{
		{
			ID:          "openai",
			DisplayName: "OpenAI",
			AdapterType: "openai_compatible",
			BaseURL:     "https://api.openai.com/v1",
			Notes:       "OpenAI-compatible chat completions provider template.",
			Models: []providerTemplateModel{
				{ModelID: "openai-gpt-4.1", DisplayName: "OpenAI GPT-4.1", UpstreamModel: "gpt-4.1", Capabilities: []string{"chat", "completion", "function_call", "vision"}, InputPrice: 0.0020, OutputPrice: 0.0080, MaxContext: 1047576, MaxOutput: 32768},
				{ModelID: "openai-gpt-4.1-mini", DisplayName: "OpenAI GPT-4.1 Mini", UpstreamModel: "gpt-4.1-mini", Capabilities: []string{"chat", "completion", "function_call", "vision"}, InputPrice: 0.0004, OutputPrice: 0.0016, MaxContext: 1047576, MaxOutput: 32768},
			},
		},
		{
			ID:          "grok",
			DisplayName: "xAI Grok",
			AdapterType: "openai_compatible",
			BaseURL:     "https://api.x.ai/v1",
			Notes:       "xAI Grok OpenAI-compatible API template.",
			Models: []providerTemplateModel{
				{ModelID: "grok-4", DisplayName: "Grok 4", UpstreamModel: "grok-4", Capabilities: []string{"chat", "completion", "reasoning"}, InputPrice: 0.0030, OutputPrice: 0.0150, MaxContext: 256000, MaxOutput: 32768},
				{ModelID: "grok-3-mini", DisplayName: "Grok 3 Mini", UpstreamModel: "grok-3-mini", Capabilities: []string{"chat", "completion", "reasoning"}, InputPrice: 0.0003, OutputPrice: 0.0005, MaxContext: 131072, MaxOutput: 16384},
			},
		},
		{
			ID:          "anthropic",
			DisplayName: "Anthropic Claude",
			AdapterType: "anthropic",
			BaseURL:     "https://api.anthropic.com/v1",
			Notes:       "Native Anthropic Messages API adapter template.",
			Models: []providerTemplateModel{
				{ModelID: "anthropic-claude-opus-4.1", DisplayName: "Claude Opus 4.1", UpstreamModel: "claude-opus-4-1", Capabilities: []string{"chat", "completion", "vision", "reasoning"}, InputPrice: 0.0150, OutputPrice: 0.0750, MaxContext: 200000, MaxOutput: 32768},
				{ModelID: "anthropic-claude-sonnet-4", DisplayName: "Claude Sonnet 4", UpstreamModel: "claude-sonnet-4", Capabilities: []string{"chat", "completion", "vision", "reasoning"}, InputPrice: 0.0030, OutputPrice: 0.0150, MaxContext: 200000, MaxOutput: 32768},
			},
		},
	}
}

func (gw *Gateway) adminListProviderTemplates(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": providerOnboardingTemplates()})
}

func (gw *Gateway) adminInstallProviderTemplate(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	templateID := strings.TrimSpace(c.Params("id"))
	var selected *providerTemplate
	for _, item := range providerOnboardingTemplates() {
		if item.ID == templateID {
			copyItem := item
			selected = &copyItem
			break
		}
	}
	if selected == nil {
		return c.Status(404).JSON(errorResponse("not_found", "provider template not found", requestID))
	}

	providerID := selected.ID
	providerOut, err := gw.deps.Store.UpsertProviderAdmin(c.UserContext(), &storage.Provider{
		ID:          providerID,
		DisplayName: selected.DisplayName,
		AdapterType: selected.AdapterType,
		BaseURL:     selected.BaseURL,
		Config: map[string]interface{}{
			"template_id": selected.ID,
			"notes":       selected.Notes,
		},
		IsEnabled: true,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to install provider template", "template_id", selected.ID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to install provider template", requestID))
	}

	installedModels := make([]*storage.Model, 0, len(selected.Models))
	installedBindings := make([]storage.ProviderBinding, 0, len(selected.Models))
	for i, tmplModel := range selected.Models {
		inputPrice := tmplModel.InputPrice
		outputPrice := tmplModel.OutputPrice
		maxContext := tmplModel.MaxContext
		maxOutput := tmplModel.MaxOutput
		modelOut, err := gw.deps.Store.UpsertModelAdmin(c.UserContext(), &storage.Model{
			ModelID:        tmplModel.ModelID,
			DisplayName:    tmplModel.DisplayName,
			Modality:       "text",
			Capabilities:   tmplModel.Capabilities,
			InputPrice:     &inputPrice,
			OutputPrice:    &outputPrice,
			PriceUnit:      "per_1k_tokens",
			MaxContext:     &maxContext,
			MaxOutput:      &maxOutput,
			SupportsStream: true,
			Status:         "active",
			Tags:           []string{"provider-template", selected.ID},
			Metadata: map[string]interface{}{
				"provider_template": selected.ID,
				"upstream_model":    tmplModel.UpstreamModel,
			},
		})
		if err != nil {
			gw.deps.Logger.Error("failed to install provider template model", "template_id", selected.ID, "model_id", tmplModel.ModelID, "error", err, "request_id", requestID)
			return c.Status(500).JSON(errorResponse("internal_server_error", "failed to install provider template model", requestID))
		}
		bindingOut, err := gw.deps.Store.UpsertModelProviderAdmin(c.UserContext(), tmplModel.ModelID, storage.ProviderBinding{
			ProviderID:     providerID,
			Priority:       i + 1,
			UpstreamModel:  tmplModel.UpstreamModel,
			CostMultiplier: 1,
			TimeoutMs:      60000,
			MaxRetries:     1,
			IsEnabled:      true,
		})
		if err != nil {
			gw.deps.Logger.Error("failed to install provider template binding", "template_id", selected.ID, "model_id", tmplModel.ModelID, "error", err, "request_id", requestID)
			return c.Status(500).JSON(errorResponse("internal_server_error", "failed to install provider template binding", requestID))
		}
		installedModels = append(installedModels, modelOut)
		installedBindings = append(installedBindings, bindingOut)
	}

	gw.recordAudit(c, "provider_template.install", "provider", providerID, "", fiber.Map{"template_id": selected.ID, "model_count": len(installedModels)})
	gw.refreshRouterRegistry(c, requestID)
	return c.Status(201).JSON(fiber.Map{
		"provider": providerOut,
		"models":   installedModels,
		"bindings": installedBindings,
	})
}

func (gw *Gateway) refreshRouterRegistry(c *fiber.Ctx, requestID string) {
	if err := gw.deps.Router.Refresh(c.UserContext()); err != nil {
		gw.deps.Logger.Warn("router registry refresh failed after admin write", "error", err, "request_id", requestID)
	}
}

func parseAdminModel(c *fiber.Ctx) (*storage.Model, error) {
	var req struct {
		ModelID        string                 `json:"model_id"`
		DisplayName    string                 `json:"display_name"`
		Modality       string                 `json:"modality"`
		Capabilities   []string               `json:"capabilities"`
		InputPrice     *float64               `json:"input_price"`
		OutputPrice    *float64               `json:"output_price"`
		PriceUnit      string                 `json:"price_unit"`
		MaxContext     *int                   `json:"max_context"`
		MaxOutput      *int                   `json:"max_output"`
		SupportsStream *bool                  `json:"supports_stream"`
		IsAsync        *bool                  `json:"is_async"`
		Status         string                 `json:"status"`
		Tags           []string               `json:"tags"`
		Metadata       map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return nil, fmt.Errorf("invalid JSON body")
	}
	if req.ModelID == "" && c.Method() == fiber.MethodPost {
		return nil, fmt.Errorf("model_id is required")
	}
	if req.Modality == "" {
		req.Modality = "text"
	}
	if req.PriceUnit == "" {
		req.PriceUnit = "per_1k_tokens"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	supportsStream := true
	if req.SupportsStream != nil {
		supportsStream = *req.SupportsStream
	}
	isAsync := false
	if req.IsAsync != nil {
		isAsync = *req.IsAsync
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	if req.Capabilities == nil {
		req.Capabilities = []string{}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	return &storage.Model{
		ModelID:        req.ModelID,
		DisplayName:    req.DisplayName,
		Modality:       req.Modality,
		Capabilities:   req.Capabilities,
		InputPrice:     req.InputPrice,
		OutputPrice:    req.OutputPrice,
		PriceUnit:      req.PriceUnit,
		MaxContext:     req.MaxContext,
		MaxOutput:      req.MaxOutput,
		SupportsStream: supportsStream,
		IsAsync:        isAsync,
		Status:         req.Status,
		Tags:           req.Tags,
		Metadata:       req.Metadata,
	}, nil
}

func parseAdminProvider(c *fiber.Ctx) (*storage.Provider, error) {
	var req struct {
		ID          string                 `json:"id"`
		DisplayName string                 `json:"display_name"`
		AdapterType string                 `json:"adapter_type"`
		BaseURL     string                 `json:"base_url"`
		Config      map[string]interface{} `json:"config"`
		IsEnabled   *bool                  `json:"is_enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return nil, fmt.Errorf("invalid JSON body")
	}
	if req.ID == "" && c.Method() == fiber.MethodPost {
		return nil, fmt.Errorf("id is required")
	}
	if req.DisplayName == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	if req.AdapterType == "" {
		req.AdapterType = "dashscope"
	}
	if req.Config == nil {
		req.Config = map[string]interface{}{}
	}
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}
	return &storage.Provider{
		ID:          req.ID,
		DisplayName: req.DisplayName,
		AdapterType: req.AdapterType,
		BaseURL:     req.BaseURL,
		Config:      req.Config,
		IsEnabled:   isEnabled,
	}, nil
}

func parseAdminProviderBinding(c *fiber.Ctx) (storage.ProviderBinding, error) {
	var req struct {
		ProviderID     string   `json:"provider_id"`
		Priority       *int     `json:"priority"`
		UpstreamModel  string   `json:"upstream_model"`
		CostMultiplier *float64 `json:"cost_multiplier"`
		TimeoutMs      *int     `json:"timeout_ms"`
		MaxRetries     *int     `json:"max_retries"`
		IsEnabled      *bool    `json:"is_enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return storage.ProviderBinding{}, fmt.Errorf("invalid JSON body")
	}
	if req.ProviderID == "" && c.Method() == fiber.MethodPost {
		return storage.ProviderBinding{}, fmt.Errorf("provider_id is required")
	}
	priority := 1
	if req.Priority != nil {
		priority = *req.Priority
	}
	costMultiplier := 1.0
	if req.CostMultiplier != nil {
		costMultiplier = *req.CostMultiplier
	}
	timeoutMs := 30000
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}
	maxRetries := 2
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}
	return storage.ProviderBinding{
		ProviderID:     req.ProviderID,
		Priority:       priority,
		UpstreamModel:  req.UpstreamModel,
		CostMultiplier: costMultiplier,
		TimeoutMs:      timeoutMs,
		MaxRetries:     maxRetries,
		IsEnabled:      isEnabled,
	}, nil
}
func (gw *Gateway) adminProviderHealth(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListLatestProviderHealth(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list provider health", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list provider health", requestID))
	}
	return c.JSON(fiber.Map{"items": items})
}

func (gw *Gateway) adminProviderHealthHistory(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := strings.TrimSpace(c.Params("id"))
	if providerID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id is required", requestID))
	}
	limit := c.QueryInt("limit", 20)
	items, err := gw.deps.Store.ListProviderHealthHistory(c.UserContext(), providerID, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list provider health history", "provider", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list provider health history", requestID))
	}
	return c.JSON(fiber.Map{
		"provider_id": providerID,
		"items":       items,
	})
}

func (gw *Gateway) adminProviderHealthCheck(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	providerID := c.Params("id")
	if providerID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "provider id is required", requestID))
	}

	adapter, ok := gw.deps.Router.GetAdapter(providerID)
	if !ok {
		return c.Status(404).JSON(errorResponse("not_found", "provider adapter not found", requestID))
	}

	start := time.Now()
	err := adapter.HealthCheck(c.UserContext())
	status := "healthy"
	errorMessage := ""
	if err != nil {
		status = "unhealthy"
		errorMessage = err.Error()
	}
	latencyMs := int(time.Since(start).Milliseconds())

	if err := gw.deps.Store.RecordProviderHealthCheck(c.UserContext(), providerID, status, latencyMs, "", errorMessage); err != nil {
		gw.deps.Logger.Error("failed to record manual provider health check", "provider", providerID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to record provider health check", requestID))
	}

	return c.JSON(fiber.Map{
		"provider_id":   providerID,
		"status":        status,
		"latency_ms":    latencyMs,
		"error_message": errorMessage,
	})
}

func (gw *Gateway) adminListRoutingPolicies(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListRoutingPolicies(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list routing policies", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list routing policies", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateRoutingPolicy(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name          string                 `json:"name"`
		Scope         string                 `json:"scope"`
		ScopeID       string                 `json:"scope_id"`
		Strategy      string                 `json:"strategy"`
		LatencyWeight *float64               `json:"latency_weight"`
		CostWeight    *float64               `json:"cost_weight"`
		ErrorWeight   *float64               `json:"error_weight"`
		IsEnabled     *bool                  `json:"is_enabled"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	scope := defaultGuardrailValue(strings.TrimSpace(req.Scope), "global")
	if !oneOf(scope, "global", "model", "workspace") {
		return c.Status(400).JSON(errorResponse("invalid_request", "scope must be one of: global, model, workspace", requestID))
	}
	strategy := defaultGuardrailValue(strings.TrimSpace(req.Strategy), "priority")
	if !oneOf(strategy, "priority", "cost", "latency", "balanced") {
		return c.Status(400).JSON(errorResponse("invalid_request", "strategy must be one of: priority, cost, latency, balanced", requestID))
	}
	if scope != "global" && strings.TrimSpace(req.ScopeID) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "scope_id is required for model and workspace policies", requestID))
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}
	policy := &storage.RoutingPolicy{
		Name:      strings.TrimSpace(req.Name),
		Scope:     scope,
		ScopeID:   strings.TrimSpace(req.ScopeID),
		Strategy:  strategy,
		IsEnabled: enabled,
		Metadata:  req.Metadata,
	}
	if req.LatencyWeight != nil {
		policy.LatencyWeight = *req.LatencyWeight
	}
	if req.CostWeight != nil {
		policy.CostWeight = *req.CostWeight
	}
	if req.ErrorWeight != nil {
		policy.ErrorWeight = *req.ErrorWeight
	}
	out, err := gw.deps.Store.UpsertRoutingPolicy(c.UserContext(), policy)
	if err != nil {
		gw.deps.Logger.Error("failed to create routing policy", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create routing policy", requestID))
	}
	gw.deps.Router.NotifyRefresh()
	gw.recordAudit(c, "routing_policy.create", "routing_policy", out.ID, "", fiber.Map{"scope": out.Scope, "scope_id": out.ScopeID, "strategy": out.Strategy})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListOrganizations(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListOrganizations(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list organizations", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list organizations", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateOrganization(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	adminUserID := auth.GetUserID(c)

	var req struct {
		Name             string                 `json:"name"`
		Slug             string                 `json:"slug"`
		OwnerUserID      string                 `json:"owner_user_id"`
		Status           string                 `json:"status"`
		BillingMode      string                 `json:"billing_mode"`
		PaymentTermsDays int                    `json:"payment_terms_days"`
		DefaultPONumber  string                 `json:"default_po_number"`
		Metadata         map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if req.Name == "" || req.Slug == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name and slug are required", requestID))
	}
	if req.OwnerUserID == "" {
		req.OwnerUserID = adminUserID
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.BillingMode == "" {
		req.BillingMode = "prepaid"
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	out, err := gw.deps.Store.CreateOrganization(c.UserContext(), &storage.Organization{
		Name:             req.Name,
		Slug:             req.Slug,
		OwnerUserID:      req.OwnerUserID,
		Status:           req.Status,
		BillingMode:      req.BillingMode,
		PaymentTermsDays: req.PaymentTermsDays,
		DefaultPONumber:  strings.TrimSpace(req.DefaultPONumber),
		Metadata:         req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create organization", "slug", req.Slug, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create organization", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListInvoices(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	organizationID := strings.TrimSpace(c.Query("organization_id"))
	items, err := gw.deps.Store.ListInvoices(c.UserContext(), organizationID)
	if err != nil {
		gw.deps.Logger.Error("failed to list invoices", "organization_id", organizationID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list invoices", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminExportInvoices(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	organizationID := strings.TrimSpace(c.Query("organization_id"))
	items, err := gw.deps.Store.ListInvoices(c.UserContext(), organizationID)
	if err != nil {
		gw.deps.Logger.Error("failed to export invoices", "organization_id", organizationID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to export invoices", requestID))
	}
	return writeInvoicesCSV(c, items, "invoices.csv")
}

func (gw *Gateway) adminExportInvoicePDF(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	invoiceID := strings.TrimSpace(c.Params("id"))
	invoice, err := gw.deps.Store.GetInvoice(c.UserContext(), invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "invoice not found", requestID))
		}
		gw.deps.Logger.Error("failed to export invoice pdf", "invoice_id", invoiceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to export invoice pdf", requestID))
	}
	pdfBytes := buildInvoicePDF(invoice)
	filename := fmt.Sprintf("%s.pdf", sanitizeDownloadFilename(invoice.InvoiceNumber))
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	gw.recordAudit(c, "invoice.pdf_export", "invoice", invoice.ID, invoice.WorkspaceID, fiber.Map{"organization_id": invoice.OrganizationID, "invoice_number": invoice.InvoiceNumber})
	return c.Send(pdfBytes)
}

func (gw *Gateway) adminCreateInvoice(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		OrganizationID string                 `json:"organization_id"`
		WorkspaceID    string                 `json:"workspace_id"`
		PeriodStart    string                 `json:"period_start"`
		PeriodEnd      string                 `json:"period_end"`
		Status         string                 `json:"status"`
		PONumber       string                 `json:"po_number"`
		TaxUSD         float64                `json:"tax_usd"`
		Notes          string                 `json:"notes"`
		Metadata       map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if strings.TrimSpace(req.OrganizationID) == "" || strings.TrimSpace(req.PeriodStart) == "" || strings.TrimSpace(req.PeriodEnd) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "organization_id, period_start and period_end are required", requestID))
	}
	start, err := parseDateOnly(req.PeriodStart)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "period_start must be YYYY-MM-DD", requestID))
	}
	end, err := parseDateOnly(req.PeriodEnd)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "period_end must be YYYY-MM-DD", requestID))
	}
	if end.Before(start) {
		return c.Status(400).JSON(errorResponse("invalid_request", "period_end must be on or after period_start", requestID))
	}
	status := defaultGuardrailValue(strings.TrimSpace(req.Status), "draft")
	if !oneOf(status, "draft", "issued", "paid", "void") {
		return c.Status(400).JSON(errorResponse("invalid_request", "status must be one of: draft, issued, paid, void", requestID))
	}
	out, err := gw.deps.Store.CreateInvoice(c.UserContext(), &storage.Invoice{
		OrganizationID: strings.TrimSpace(req.OrganizationID),
		WorkspaceID:    strings.TrimSpace(req.WorkspaceID),
		PeriodStart:    start,
		PeriodEnd:      end,
		Status:         status,
		PONumber:       strings.TrimSpace(req.PONumber),
		TaxUSD:         req.TaxUSD,
		Notes:          strings.TrimSpace(req.Notes),
		Metadata:       req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create invoice", "organization_id", req.OrganizationID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create invoice", requestID))
	}
	gw.recordAudit(c, "invoice.create", "invoice", out.ID, out.WorkspaceID, fiber.Map{"organization_id": out.OrganizationID, "invoice_number": out.InvoiceNumber, "total_usd": out.TotalUSD})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminUpdateInvoiceStatus(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	invoiceID := strings.TrimSpace(c.Params("id"))
	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	status := strings.TrimSpace(req.Status)
	if !oneOf(status, "draft", "issued", "paid", "void") {
		return c.Status(400).JSON(errorResponse("invalid_request", "status must be one of: draft, issued, paid, void", requestID))
	}
	out, err := gw.deps.Store.UpdateInvoiceStatus(c.UserContext(), invoiceID, status, strings.TrimSpace(req.Notes))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(errorResponse("not_found", "invoice not found", requestID))
		}
		gw.deps.Logger.Error("failed to update invoice status", "invoice_id", invoiceID, "status", status, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update invoice status", requestID))
	}
	gw.recordAudit(c, "invoice.status_update", "invoice", out.ID, out.WorkspaceID, fiber.Map{"organization_id": out.OrganizationID, "invoice_number": out.InvoiceNumber, "status": out.Status})
	return c.JSON(out)
}

func (gw *Gateway) adminListWorkspaces(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	organizationID := c.Query("organization_id", "")
	items, err := gw.deps.Store.ListWorkspaces(c.UserContext(), organizationID)
	if err != nil {
		gw.deps.Logger.Error("failed to list workspaces", "organization_id", organizationID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workspaces", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateWorkspace(c *fiber.Ctx) error {
	requestID := getRequestID(c)

	var req struct {
		OrganizationID   string                 `json:"organization_id"`
		Name             string                 `json:"name"`
		Slug             string                 `json:"slug"`
		Status           string                 `json:"status"`
		MonthlyBudgetUSD *float64               `json:"monthly_budget_usd"`
		Metadata         map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if req.OrganizationID == "" || req.Name == "" || req.Slug == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "organization_id, name and slug are required", requestID))
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	out, err := gw.deps.Store.CreateWorkspace(c.UserContext(), &storage.Workspace{
		OrganizationID:   req.OrganizationID,
		Name:             req.Name,
		Slug:             req.Slug,
		Status:           req.Status,
		MonthlyBudgetUSD: req.MonthlyBudgetUSD,
		Metadata:         req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create workspace", "organization_id", req.OrganizationID, "slug", req.Slug, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workspace", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListWorkspaceProjects(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	items, err := gw.deps.Store.ListProjects(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Error("failed to list workspace projects", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workspace projects", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateWorkspaceProject(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	var req struct {
		Name     string                 `json:"name"`
		Slug     string                 `json:"slug"`
		Status   string                 `json:"status"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if workspaceID == "" || req.Name == "" || req.Slug == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id, name and slug are required", requestID))
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	out, err := gw.deps.Store.CreateProject(c.UserContext(), &storage.Project{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Slug:        req.Slug,
		Status:      req.Status,
		Metadata:    req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create workspace project", "workspace_id", workspaceID, "slug", req.Slug, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workspace project", requestID))
	}
	gw.recordAudit(c, "project.create", "project", out.ID, workspaceID, fiber.Map{"slug": out.Slug, "name": out.Name})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListWorkspaceMembers(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	items, err := gw.deps.Store.ListWorkspaceMembers(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Error("failed to list workspace members", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workspace members", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminAddWorkspaceMember(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	adminUserID := auth.GetUserID(c)

	var req struct {
		UserID   string `json:"user_id"`
		RoleName string `json:"role_name"`
		Status   string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if workspaceID == "" || req.UserID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id and user_id are required", requestID))
	}
	if req.RoleName == "" {
		req.RoleName = "member"
	}
	if req.Status == "" {
		req.Status = "active"
	}

	out, err := gw.deps.Store.UpsertWorkspaceMember(c.UserContext(), &storage.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      req.UserID,
		RoleName:    req.RoleName,
		Status:      req.Status,
		InvitedBy:   adminUserID,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to add workspace member", "workspace_id", workspaceID, "user_id", req.UserID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to add workspace member", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminWorkspaceUsage(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	out, err := gw.deps.Store.GetWorkspaceUsageSummary(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Error("failed to get workspace usage", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get workspace usage", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) adminWorkspaceUsageExport(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	filter, err := parseRequestLogFilter(c)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	if filter.Limit <= 0 || filter.Limit == 50 {
		filter.Limit = 1000
	}
	result, err := gw.deps.Store.ListWorkspaceRequestLogsFiltered(c.UserContext(), workspaceID, filter)
	if err != nil {
		gw.deps.Logger.Error("failed to export workspace usage", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to export workspace usage", requestID))
	}
	return writeRequestLogsCSV(c, result.Items, fmt.Sprintf("workspace-%s-usage.csv", workspaceID))
}

func (gw *Gateway) adminListWorkspaceBudgets(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	return gw.listWorkspaceBudgets(c, requestID, workspaceID)
}

func (gw *Gateway) userListWorkspaceBudgets(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if !gw.requireWorkspacePermission(c, workspaceID, "usage:read", "user does not have usage:read permission for the workspace") {
		return nil
	}
	return gw.listWorkspaceBudgets(c, requestID, workspaceID)
}

func (gw *Gateway) listWorkspaceBudgets(c *fiber.Ctx, requestID, workspaceID string) error {
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	items, err := gw.deps.Store.ListWorkspaceBudgets(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Error("failed to list workspace budgets", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workspace budgets", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateWorkspaceBudget(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	return gw.createWorkspaceBudget(c, requestID, workspaceID)
}

func (gw *Gateway) userCreateWorkspaceBudget(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if !gw.requireWorkspacePermission(c, workspaceID, "workspace_budgets:write", "user does not have workspace_budgets:write permission for the workspace") {
		return nil
	}
	return gw.createWorkspaceBudget(c, requestID, workspaceID)
}

func (gw *Gateway) createWorkspaceBudget(c *fiber.Ctx, requestID, workspaceID string) error {
	if requiresWorkspaceWritePermission(c) {
		if !gw.requireWorkspacePermission(c, workspaceID, "workspace_budgets:write", "user does not have workspace_budgets:write permission for the workspace") {
			return nil
		}
	}
	var req struct {
		Period       string  `json:"period"`
		AmountUSD    float64 `json:"amount_usd"`
		SoftLimitPct int     `json:"soft_limit_pct"`
		HardLimitPct int     `json:"hard_limit_pct"`
		IsActive     *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if workspaceID == "" || req.AmountUSD <= 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id and positive amount_usd are required", requestID))
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	out, err := gw.deps.Store.UpsertWorkspaceBudget(c.UserContext(), &storage.WorkspaceBudget{
		WorkspaceID:  workspaceID,
		Period:       req.Period,
		AmountUSD:    req.AmountUSD,
		SoftLimitPct: req.SoftLimitPct,
		HardLimitPct: req.HardLimitPct,
		IsActive:     isActive,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create workspace budget", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workspace budget", requestID))
	}
	gw.recordAudit(c, "workspace_budget.create", "workspace", workspaceID, workspaceID, fiber.Map{"amount_usd": req.AmountUSD, "period": out.Period})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListWorkspaceQuotas(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	return gw.listWorkspaceQuotas(c, requestID, workspaceID)
}

func (gw *Gateway) userListWorkspaceQuotas(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if !gw.requireWorkspacePermission(c, workspaceID, "usage:read", "user does not have usage:read permission for the workspace") {
		return nil
	}
	return gw.listWorkspaceQuotas(c, requestID, workspaceID)
}

func (gw *Gateway) listWorkspaceQuotas(c *fiber.Ctx, requestID, workspaceID string) error {
	if workspaceID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id is required", requestID))
	}
	items, err := gw.deps.Store.ListWorkspaceQuotas(c.UserContext(), workspaceID)
	if err != nil {
		gw.deps.Logger.Error("failed to list workspace quotas", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workspace quotas", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateWorkspaceQuota(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	return gw.createWorkspaceQuota(c, requestID, workspaceID)
}

func (gw *Gateway) userCreateWorkspaceQuota(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	workspaceID := c.Params("id")
	if !gw.requireWorkspacePermission(c, workspaceID, "workspace_quotas:write", "user does not have workspace_quotas:write permission for the workspace") {
		return nil
	}
	return gw.createWorkspaceQuota(c, requestID, workspaceID)
}

func (gw *Gateway) createWorkspaceQuota(c *fiber.Ctx, requestID, workspaceID string) error {
	if requiresWorkspaceWritePermission(c) {
		if !gw.requireWorkspacePermission(c, workspaceID, "workspace_quotas:write", "user does not have workspace_quotas:write permission for the workspace") {
			return nil
		}
	}
	var req struct {
		QuotaType  string                 `json:"quota_type"`
		LimitValue float64                `json:"limit_value"`
		IsActive   *bool                  `json:"is_active"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if workspaceID == "" || req.QuotaType == "" || req.LimitValue <= 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "workspace id, quota_type and positive limit_value are required", requestID))
	}
	switch req.QuotaType {
	case "requests_per_minute", "tokens_per_minute", "tokens_per_month", "spend_per_month":
	default:
		return c.Status(400).JSON(errorResponse("invalid_request", "quota_type must be one of: requests_per_minute, tokens_per_minute, tokens_per_month, spend_per_month", requestID))
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	out, err := gw.deps.Store.UpsertWorkspaceQuota(c.UserContext(), &storage.WorkspaceQuota{
		WorkspaceID: workspaceID,
		QuotaType:   req.QuotaType,
		LimitValue:  req.LimitValue,
		IsActive:    isActive,
		Metadata:    req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create workspace quota", "workspace_id", workspaceID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workspace quota", requestID))
	}
	gw.recordAudit(c, "workspace_quota.create", "workspace", workspaceID, workspaceID, fiber.Map{"quota_type": req.QuotaType, "limit_value": req.LimitValue})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListUsers(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	limit := c.QueryInt("limit", 100)
	items, err := gw.deps.Store.ListAdminUsers(c.UserContext(), limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list users", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list users", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}
func (gw *Gateway) adminCreateUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if req.Email == "" || req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "email, username and password are required", requestID))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_error", "failed to hash password", requestID))
	}
	userID, err := gw.deps.Store.CreateUser(c.UserContext(), req.Email, req.Username, string(hash))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create user", requestID))
	}
	if req.Role != "" && req.Role != "user" {
		_, _ = gw.deps.Store.UpdateAdminUser(c.UserContext(), userID, "", req.Role, "", "", nil, nil)
	}
	out, err := gw.deps.Store.GetAdminUser(c.UserContext(), userID)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to load user", requestID))
	}
	gw.recordAudit(c, "user.create", "user", userID, "", fiber.Map{"email": req.Email, "role": out.Role})
	return c.Status(201).JSON(out)
}
func (gw *Gateway) adminGetUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.GetAdminUser(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "user not found", requestID))
	}
	return c.JSON(out)
}
func (gw *Gateway) adminUpdateUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := c.Params("id")
	var req struct {
		Username     string                 `json:"username"`
		Role         string                 `json:"role"`
		Status       string                 `json:"status"`
		BillingMode  string                 `json:"billing_mode"`
		MonthlyQuota *float64               `json:"monthly_quota"`
		Metadata     map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	out, err := gw.deps.Store.UpdateAdminUser(c.UserContext(), userID, req.Username, req.Role, req.Status, req.BillingMode, req.MonthlyQuota, req.Metadata)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update user", requestID))
	}
	gw.recordAudit(c, "user.update", "user", userID, "", fiber.Map{"role": req.Role, "status": req.Status})
	return c.JSON(out)
}
func (gw *Gateway) adminTopUpBalance(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := c.Params("id")
	var req struct {
		AmountUSD   float64 `json:"amount_usd"`
		Description string  `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if userID == "" || req.AmountUSD <= 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "user id and positive amount_usd are required", requestID))
	}
	if req.Description == "" {
		req.Description = "Admin credit grant"
	}
	if err := gw.deps.Store.AddBalance(c.UserContext(), userID, req.AmountUSD); err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to add balance", requestID))
	}
	if err := gw.deps.Store.CreateBillingTransaction(c.UserContext(), userID, req.AmountUSD, "credit_grant", req.Description); err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to record billing transaction", requestID))
	}
	gw.recordAudit(c, "user.balance_grant", "user", userID, "", fiber.Map{"amount_usd": req.AmountUSD, "description": req.Description})
	return c.JSON(fiber.Map{"ok": true, "user_id": userID, "amount_usd": req.AmountUSD})
}
func (gw *Gateway) adminUserUsage(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	limit := c.QueryInt("limit", 50)
	logs, err := gw.deps.Store.GetAdminUserUsage(c.UserContext(), c.Params("id"), limit)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get user usage", requestID))
	}
	return c.JSON(fiber.Map{"data": logs})
}

func (gw *Gateway) adminListAPIKeys(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	limit := c.QueryInt("limit", 200)
	userID := strings.TrimSpace(c.Query("user_id"))
	includeInactive := strings.EqualFold(c.Query("include_inactive"), "true") || c.Query("include_inactive") == "1"
	keys, err := gw.deps.Store.ListAPIKeysAdmin(c.UserContext(), userID, includeInactive, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list admin api keys", "user_id", userID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list API keys", requestID))
	}
	return c.JSON(fiber.Map{"data": keys})
}

func (gw *Gateway) adminUpdateAPIKey(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "key id is required", requestID))
	}
	var req struct {
		Name         string          `json:"name"`
		WorkspaceID  string          `json:"workspace_id"`
		Permissions  json.RawMessage `json:"permissions"`
		RateLimitRPM int             `json:"rate_limit_rpm"`
		RateLimitTPM int             `json:"rate_limit_tpm"`
		ExpiresAt    *time.Time      `json:"expires_at"`
		IsActive     *bool           `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	current, err := gw.deps.Store.GetAPIKeyAdmin(c.UserContext(), keyID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "API key not found", requestID))
	}
	current.Name = strings.TrimSpace(req.Name)
	current.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	if len(req.Permissions) > 0 {
		if !json.Valid(req.Permissions) {
			return c.Status(400).JSON(errorResponse("invalid_request", "permissions must be valid JSON", requestID))
		}
		current.Permissions = req.Permissions
	}
	current.RateLimitRPM = req.RateLimitRPM
	current.RateLimitTPM = req.RateLimitTPM
	current.ExpiresAt = req.ExpiresAt
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	out, err := gw.deps.Store.UpdateAPIKeyAdmin(c.UserContext(), current)
	if err != nil {
		gw.deps.Logger.Error("failed to update api key", "key_id", keyID, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update API key", requestID))
	}
	gw.recordAudit(c, "api_key.update", "api_key", keyID, out.WorkspaceID, fiber.Map{
		"target_user_id":  out.UserID,
		"name":            out.Name,
		"rate_limit_rpm":  out.RateLimitRPM,
		"rate_limit_tpm":  out.RateLimitTPM,
		"permissions_set": len(out.Permissions) > 0,
		"is_active":       out.IsActive,
	})
	return c.JSON(out)
}

func (gw *Gateway) adminRevokeAPIKey(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "key id is required", requestID))
	}
	key, err := gw.deps.Store.RevokeAPIKeyAdmin(c.UserContext(), keyID)
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "API key not found", requestID))
	}
	gw.recordAudit(c, "api_key.revoke", "api_key", keyID, key.WorkspaceID, fiber.Map{"target_user_id": key.UserID, "name": key.Name})
	return c.JSON(fiber.Map{"ok": true, "key": key})
}

func (gw *Gateway) adminAnalyticsOverview(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.AdminAnalyticsOverview(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get analytics overview", requestID))
	}
	return c.JSON(out)
}
func (gw *Gateway) adminAnalyticsUsage(c *fiber.Ctx) error {
	return gw.adminAnalyticsSeries(c, "usage")
}
func (gw *Gateway) adminAnalyticsCost(c *fiber.Ctx) error {
	return gw.adminAnalyticsSeries(c, "cost")
}
func (gw *Gateway) adminAnalyticsLatency(c *fiber.Ctx) error {
	return gw.adminAnalyticsSeries(c, "latency")
}
func (gw *Gateway) adminAnalyticsErrors(c *fiber.Ctx) error {
	return gw.adminAnalyticsSeries(c, "errors")
}

func (gw *Gateway) adminGetSettings(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListSettings(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list settings", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}
func (gw *Gateway) adminUpdateSetting(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Value       string `json:"value"`
		ValueType   string `json:"value_type"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	out, err := gw.deps.Store.SetSettingAdmin(c.UserContext(), c.Params("key"), req.Value, req.ValueType, req.Description, auth.GetUserID(c))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to update setting", requestID))
	}
	gw.recordAudit(c, "setting.update", "system_setting", out.Key, "", fiber.Map{"value_type": out.ValueType})
	return c.JSON(out)
}

func (gw *Gateway) adminRunFileRetention(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Limit  int  `json:"limit"`
		DryRun bool `json:"dry_run"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = c.QueryInt("limit", 100)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	retentionRaw, err := gw.deps.Store.GetSetting(c.UserContext(), "file_retention_days")
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "file_retention_days setting is not configured", requestID))
	}
	retentionDays := int(parsePositiveInt64(retentionRaw))
	if retentionDays <= 0 {
		gw.recordAudit(c, "file.retention_run", "file", "", "", fiber.Map{"retention_days": 0, "dry_run": req.DryRun, "deleted_count": 0})
		return c.JSON(fiber.Map{
			"retention_days":        0,
			"dry_run":               req.DryRun,
			"deleted_count":         0,
			"storage_deleted_count": 0,
			"files":                 []fiber.Map{},
		})
	}

	files, err := gw.deps.Store.ListExpiredFileRecords(c.UserContext(), retentionDays, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list expired files", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list expired files", requestID))
	}

	deletedCount := 0
	storageDeletedCount := 0
	data := make([]fiber.Map, 0, len(files))
	for i := range files {
		file := files[i]
		data = append(data, fiber.Map{
			"id":           file.ID,
			"filename":     file.Filename,
			"bytes":        file.Bytes,
			"purpose":      file.Purpose,
			"user_id":      file.UserID,
			"workspace_id": file.WorkspaceID,
			"created_at":   file.CreatedAt.Unix(),
		})
		if req.DryRun {
			continue
		}
		if err := gw.deps.Store.MarkFileRecordDeletedByID(c.UserContext(), file.ID); err != nil {
			gw.deps.Logger.Warn("failed to mark expired file deleted", "file_id", file.ID, "error", err, "request_id", requestID)
			continue
		}
		deletedCount++
		if file.StoragePath != "" {
			if err := gw.deps.FileStore.Delete(c.UserContext(), file.StoragePath); err == nil {
				storageDeletedCount++
			} else {
				gw.deps.Logger.Warn("failed to remove expired file content", "file_id", file.ID, "path", file.StoragePath, "error", err, "request_id", requestID)
			}
		}
	}

	gw.recordAudit(c, "file.retention_run", "file", "", "", fiber.Map{
		"retention_days":        retentionDays,
		"dry_run":               req.DryRun,
		"matched_count":         len(files),
		"deleted_count":         deletedCount,
		"storage_deleted_count": storageDeletedCount,
		"limit":                 limit,
	})
	return c.JSON(fiber.Map{
		"retention_days":        retentionDays,
		"dry_run":               req.DryRun,
		"matched_count":         len(files),
		"deleted_count":         deletedCount,
		"storage_deleted_count": storageDeletedCount,
		"files":                 data,
	})
}

func (gw *Gateway) adminListAlertRules(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	includeDisabled := strings.EqualFold(c.Query("include_disabled"), "true") || c.Query("include_disabled") == "1"
	rules, err := gw.deps.Store.ListAlertRules(c.UserContext(), includeDisabled)
	if err != nil {
		gw.deps.Logger.Error("failed to list alert rules", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list alert rules", requestID))
	}
	return c.JSON(fiber.Map{"data": rules})
}
func (gw *Gateway) adminCreateAlertRule(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req storage.AlertRule
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Metric) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name and metric are required", requestID))
	}
	req.CreatedBy = auth.GetUserID(c)
	out, err := gw.deps.Store.CreateAlertRule(c.UserContext(), &req)
	if err != nil {
		gw.deps.Logger.Error("failed to create alert rule", "name", req.Name, "metric", req.Metric, "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create alert rule", requestID))
	}
	gw.recordAudit(c, "alert_rule.create", "alert_rule", out.ID, "", fiber.Map{"name": out.Name, "metric": out.Metric, "enabled": out.Enabled})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminUpdateAlertRule(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req storage.AlertRule
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
	}
	req.ID = c.Params("id")
	if req.ID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "alert rule id is required", requestID))
	}
	out, err := gw.deps.Store.UpdateAlertRule(c.UserContext(), &req)
	if err != nil {
		gw.deps.Logger.Error("failed to update alert rule", "id", req.ID, "error", err, "request_id", requestID)
		return c.Status(404).JSON(errorResponse("not_found", "alert rule not found", requestID))
	}
	gw.recordAudit(c, "alert_rule.update", "alert_rule", out.ID, "", fiber.Map{"name": out.Name, "metric": out.Metric, "enabled": out.Enabled})
	return c.JSON(out)
}
func (gw *Gateway) adminAlertHistory(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	alerts, err := gw.deps.Store.ListAlertSummaries(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list alerts", requestID))
	}
	return c.JSON(fiber.Map{"data": alerts})
}

func (gw *Gateway) adminAcknowledgeAlert(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	alertID := c.Params("id")
	if alertID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "alert id is required", requestID))
	}
	out, err := gw.deps.Store.AcknowledgeAlertEvent(c.UserContext(), alertID, auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "alert event not found", requestID))
	}
	gw.recordAudit(c, "alert_event.acknowledge", "alert_event", alertID, "", fiber.Map{"status": out.Status})
	return c.JSON(out)
}

func (gw *Gateway) adminResolveAlert(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	alertID := c.Params("id")
	if alertID == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "alert id is required", requestID))
	}
	out, err := gw.deps.Store.ResolveAlertEvent(c.UserContext(), alertID, auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "alert event not found", requestID))
	}
	gw.recordAudit(c, "alert_event.resolve", "alert_event", alertID, "", fiber.Map{"status": out.Status})
	return c.JSON(out)
}

func (gw *Gateway) adminAnalyticsSeries(c *fiber.Ctx, metric string) error {
	requestID := getRequestID(c)
	days := c.QueryInt("days", 14)
	points, err := gw.deps.Store.AdminAnalyticsSeries(c.UserContext(), metric, days)
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to get analytics series", requestID))
	}
	return c.JSON(fiber.Map{"data": points})
}

func (gw *Gateway) adminAuditLogs(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	filter, err := parseAuditLogFilter(c, 100)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	items, err := gw.deps.Store.ListAuditLogsFiltered(c.UserContext(), filter)
	if err != nil {
		gw.deps.Logger.Error("failed to list audit logs", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list audit logs", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminAuditLogsExport(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	filter, err := parseAuditLogFilter(c, 10000)
	if err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", err.Error(), requestID))
	}
	items, err := gw.deps.Store.ListAuditLogsFiltered(c.UserContext(), filter)
	if err != nil {
		gw.deps.Logger.Error("failed to export audit logs", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to export audit logs", requestID))
	}
	if err := writeAuditLogsCSV(c, items, "audit-logs.csv"); err != nil {
		gw.deps.Logger.Error("failed to write audit log csv", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to write audit log csv", requestID))
	}
	return nil
}

func (gw *Gateway) adminRunAuditLogRetention(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Limit  int  `json:"limit"`
		DryRun bool `json:"dry_run"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid JSON body", requestID))
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = c.QueryInt("limit", 100)
	}
	if limit <= 0 || limit > 10000 {
		limit = 100
	}

	retentionRaw, err := gw.deps.Store.GetSetting(c.UserContext(), "audit_log_retention_days")
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "audit_log_retention_days setting is not configured", requestID))
	}
	retentionDays := int(parsePositiveInt64(retentionRaw))
	if retentionDays <= 0 {
		gw.recordAudit(c, "audit_log.retention_run", "audit_log", "", "", fiber.Map{"retention_days": 0, "dry_run": req.DryRun, "deleted_count": 0})
		return c.JSON(fiber.Map{
			"retention_days": 0,
			"dry_run":        req.DryRun,
			"matched_count":  0,
			"deleted_count":  0,
			"items":          []fiber.Map{},
		})
	}

	items, err := gw.deps.Store.ListExpiredAuditLogs(c.UserContext(), retentionDays, limit)
	if err != nil {
		gw.deps.Logger.Error("failed to list expired audit logs", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list expired audit logs", requestID))
	}

	data := make([]fiber.Map, 0, len(items))
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
		data = append(data, fiber.Map{
			"id":            item.ID,
			"created_at":    item.CreatedAt,
			"user_id":       item.UserID,
			"workspace_id":  item.WorkspaceID,
			"action":        item.Action,
			"resource_type": item.ResourceType,
			"resource_id":   item.ResourceID,
		})
	}

	deletedCount := int64(0)
	if !req.DryRun {
		deletedCount, err = gw.deps.Store.DeleteAuditLogsByIDs(c.UserContext(), ids)
		if err != nil {
			gw.deps.Logger.Error("failed to delete expired audit logs", "error", err, "request_id", requestID)
			return c.Status(500).JSON(errorResponse("internal_server_error", "failed to delete expired audit logs", requestID))
		}
	}

	gw.recordAudit(c, "audit_log.retention_run", "audit_log", "", "", fiber.Map{
		"retention_days": retentionDays,
		"dry_run":        req.DryRun,
		"matched_count":  len(items),
		"deleted_count":  deletedCount,
		"limit":          limit,
	})
	return c.JSON(fiber.Map{
		"retention_days": retentionDays,
		"dry_run":        req.DryRun,
		"matched_count":  len(items),
		"deleted_count":  deletedCount,
		"items":          data,
	})
}

func (gw *Gateway) adminListGuardrailPolicies(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListGuardrailPolicies(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list guardrail policies", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list guardrail policies", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateGuardrailPolicy(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name             string                 `json:"name"`
		Scope            string                 `json:"scope"`
		ScopeID          string                 `json:"scope_id"`
		IsEnabled        *bool                  `json:"is_enabled"`
		PIIAction        string                 `json:"pii_action"`
		InjectionAction  string                 `json:"injection_action"`
		ModerationAction string                 `json:"moderation_action"`
		Config           map[string]interface{} `json:"config"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name is required", requestID))
	}
	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}
	policy := &storage.GuardrailPolicy{
		Name:             strings.TrimSpace(req.Name),
		Scope:            defaultGuardrailValue(req.Scope, "global"),
		ScopeID:          strings.TrimSpace(req.ScopeID),
		IsEnabled:        enabled,
		PIIAction:        defaultGuardrailValue(req.PIIAction, "mask"),
		InjectionAction:  defaultGuardrailValue(req.InjectionAction, "block"),
		ModerationAction: defaultGuardrailValue(req.ModerationAction, "block"),
		Config:           req.Config,
		CreatedBy:        auth.GetUserID(c),
	}
	if policy.Config == nil {
		policy.Config = map[string]interface{}{}
	}
	out, err := gw.deps.Store.CreateGuardrailPolicy(c.UserContext(), policy)
	if err != nil {
		gw.deps.Logger.Error("failed to create guardrail policy", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create guardrail policy", requestID))
	}
	gw.recordAudit(c, "guardrail_policy.create", "guardrail_policy", out.ID, "", fiber.Map{"name": out.Name, "scope": out.Scope})
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListGuardrailResults(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListGuardrailResults(c.UserContext(), c.QueryInt("limit", 100))
	if err != nil {
		gw.deps.Logger.Error("failed to list guardrail results", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list guardrail results", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminListBenchmarkTasks(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListBenchmarkTasks(c.UserContext())
	if err != nil {
		gw.deps.Logger.Error("failed to list benchmark tasks", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list benchmark tasks", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateBenchmarkTask(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Dataset     []map[string]interface{} `json:"dataset"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name is required", requestID))
	}
	if req.Dataset == nil {
		req.Dataset = []map[string]interface{}{}
	}
	task, err := gw.deps.Store.CreateBenchmarkTask(c.UserContext(), &storage.BenchmarkTask{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Dataset:     req.Dataset,
		CreatedBy:   auth.GetUserID(c),
		Status:      "active",
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create benchmark task", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create benchmark task", requestID))
	}
	gw.recordAudit(c, "benchmark_task.create", "benchmark_task", task.ID, "", fiber.Map{"name": task.Name})
	return c.Status(201).JSON(task)
}

func (gw *Gateway) adminListBenchmarkRuns(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListBenchmarkRuns(c.UserContext(), c.QueryInt("limit", 100))
	if err != nil {
		gw.deps.Logger.Error("failed to list benchmark runs", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list benchmark runs", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminGetBenchmarkRun(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	run, err := gw.deps.Store.GetBenchmarkRun(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "benchmark run not found", requestID))
	}
	return c.JSON(run)
}

func (gw *Gateway) adminRunBenchmark(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	task, err := gw.deps.Store.GetBenchmarkTask(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "benchmark task not found", requestID))
	}
	var req struct {
		ModelIDs []string `json:"model_ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if len(req.ModelIDs) == 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "model_ids is required", requestID))
	}

	models := make([]*storage.Model, 0, len(req.ModelIDs))
	normalizedIDs := make([]string, 0, len(req.ModelIDs))
	seen := map[string]bool{}
	for _, rawID := range req.ModelIDs {
		modelID := strings.TrimSpace(rawID)
		if modelID == "" || seen[modelID] {
			continue
		}
		model, err := gw.deps.Store.GetModelAdmin(c.UserContext(), modelID)
		if err != nil || model.Status != "active" {
			return c.Status(400).JSON(errorResponse("invalid_request", fmt.Sprintf("model %s is not active", modelID), requestID))
		}
		models = append(models, model)
		normalizedIDs = append(normalizedIDs, modelID)
		seen[modelID] = true
	}
	if len(models) == 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "no active models selected", requestID))
	}

	now := time.Now().UTC()
	results := make([]storage.BenchmarkResult, 0, len(models))
	for _, model := range models {
		results = append(results, buildBenchmarkResult(task, model))
	}
	run, err := gw.deps.Store.CreateBenchmarkRun(c.UserContext(), &storage.BenchmarkRun{
		TaskID:      task.ID,
		ModelIDs:    normalizedIDs,
		Status:      "completed",
		StartedAt:   &now,
		CompletedAt: &now,
		Metadata: fiber.Map{
			"mode":          "deterministic_local",
			"dataset_count": len(task.Dataset),
		},
	}, results)
	if err != nil {
		gw.deps.Logger.Error("failed to create benchmark run", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create benchmark run", requestID))
	}
	gw.recordAudit(c, "benchmark_run.create", "benchmark_run", run.ID, "", fiber.Map{"task_id": task.ID, "model_ids": normalizedIDs})
	return c.Status(201).JSON(run)
}

func (gw *Gateway) userListTools(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListTools(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list tools", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userListToolCredentials(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListToolCredentials(c.UserContext(), auth.GetUserID(c))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list tool credentials", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userCreateToolCredential(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		ToolID      string                 `json:"tool_id"`
		Name        string                 `json:"name"`
		Secret      string                 `json:"secret"`
		WorkspaceID string                 `json:"workspace_id"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	toolID := strings.TrimSpace(req.ToolID)
	name := strings.TrimSpace(req.Name)
	secret := strings.TrimSpace(req.Secret)
	if toolID == "" || name == "" || secret == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "tool_id, name, and secret are required", requestID))
	}
	tools, err := gw.deps.Store.ListTools(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to validate tool", requestID))
	}
	toolOK := false
	for _, tool := range tools {
		if tool.ID == toolID && tool.IsEnabled {
			toolOK = true
			break
		}
	}
	if !toolOK {
		return c.Status(400).JSON(errorResponse("invalid_request", "tool_id is not enabled", requestID))
	}
	userID := auth.GetUserID(c)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, "workflows:write")
		if err != nil {
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have workflows:write permission for the workspace", requestID))
		}
	}
	out, err := gw.deps.Store.CreateToolCredential(c.UserContext(), &storage.ToolCredential{
		UserID:          userID,
		WorkspaceID:     workspaceID,
		ToolID:          toolID,
		Name:            name,
		SecretEncrypted: sealCredentialSecret(secret),
		SecretMask:      maskCredentialSecret(secret),
		Metadata:        req.Metadata,
		Status:          "active",
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create tool credential", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create tool credential", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) userRevokeToolCredential(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.RevokeToolCredential(c.UserContext(), c.Params("id"), auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "tool credential not found", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) userListAgentSessions(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListAgentSessions(c.UserContext(), auth.GetUserID(c), c.QueryInt("limit", 100))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list agent sessions", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userCreateAgentSession(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name        string                 `json:"name"`
		WorkflowID  string                 `json:"workflow_id"`
		WorkspaceID string                 `json:"workspace_id"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name is required", requestID))
	}
	userID := auth.GetUserID(c)
	workflowID := strings.TrimSpace(req.WorkflowID)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workflowID != "" {
		wf, err := gw.deps.Store.GetWorkflow(c.UserContext(), workflowID)
		if err != nil || (wf.UserID != "" && wf.UserID != userID) {
			return c.Status(404).JSON(errorResponse("not_found", "workflow not found", requestID))
		}
		if workspaceID == "" {
			workspaceID = wf.WorkspaceID
		}
	}
	if workspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, "workflows:write")
		if err != nil {
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have workflows:write permission for the workspace", requestID))
		}
	}
	out, err := gw.deps.Store.CreateAgentSession(c.UserContext(), &storage.AgentSession{
		UserID:      userID,
		WorkspaceID: workspaceID,
		WorkflowID:  workflowID,
		Name:        name,
		Status:      "active",
		Metadata:    req.Metadata,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create agent session", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create agent session", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) userGetAgentSession(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.GetAgentSession(c.UserContext(), c.Params("id"), auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "agent session not found", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) userCloseAgentSession(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.CloseAgentSession(c.UserContext(), c.Params("id"), auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "agent session not found", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) userListPromptTemplates(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListPromptTemplates(c.UserContext(), auth.GetUserID(c), c.QueryInt("limit", 100))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list prompt templates", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userCreatePromptTemplate(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Template    string                 `json:"template"`
		Variables   []string               `json:"variables"`
		WorkspaceID string                 `json:"workspace_id"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	name := strings.TrimSpace(req.Name)
	template := strings.TrimSpace(req.Template)
	if name == "" || template == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name and template are required", requestID))
	}
	userID := auth.GetUserID(c)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, "workflows:write")
		if err != nil {
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have workflows:write permission for the workspace", requestID))
		}
	}
	variables := make([]string, 0, len(req.Variables))
	seen := map[string]bool{}
	for _, raw := range req.Variables {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		variables = append(variables, name)
		seen[name] = true
	}
	out, err := gw.deps.Store.CreatePromptTemplate(c.UserContext(), &storage.PromptTemplate{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Template:    template,
		Variables:   variables,
		Metadata:    req.Metadata,
		Status:      "active",
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create prompt template", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create prompt template", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) userGetPromptTemplate(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.GetPromptTemplate(c.UserContext(), c.Params("id"), auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "prompt template not found", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) userArchivePromptTemplate(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	out, err := gw.deps.Store.ArchivePromptTemplate(c.UserContext(), c.Params("id"), auth.GetUserID(c))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "prompt template not found", requestID))
	}
	return c.JSON(out)
}

func (gw *Gateway) userListWorkflows(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListWorkflows(c.UserContext(), auth.GetUserID(c))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workflows", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userCreateWorkflow(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		WorkspaceID string                 `json:"workspace_id"`
		Metadata    map[string]interface{} `json:"metadata"`
		Steps       []struct {
			Name           string                 `json:"name"`
			StepOrder      int                    `json:"step_order"`
			StepType       string                 `json:"step_type"`
			ModelID        string                 `json:"model_id"`
			ToolID         string                 `json:"tool_id"`
			PromptTemplate string                 `json:"prompt_template"`
			Config         map[string]interface{} `json:"config"`
		} `json:"steps"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Steps) == 0 {
		return c.Status(400).JSON(errorResponse("invalid_request", "name and steps are required", requestID))
	}
	userID := auth.GetUserID(c)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		workspaceID = localString(c, "workspace_id")
	}
	if workspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), workspaceID, userID, "workflows:write")
		if err != nil {
			gw.deps.Logger.Error("workspace permission check failed", "user_id", userID, "workspace_id", workspaceID, "permission", "workflows:write", "error", err, "request_id", requestID)
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workspace_id", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have workflows:write permission for the workspace", requestID))
		}
	}
	steps := make([]storage.WorkflowStep, 0, len(req.Steps))
	for i, step := range req.Steps {
		stepType := defaultGuardrailValue(step.StepType, "prompt")
		name := defaultGuardrailValue(step.Name, fmt.Sprintf("Step %d", i+1))
		steps = append(steps, storage.WorkflowStep{
			StepOrder:      firstNonZero(step.StepOrder, i+1),
			Name:           name,
			StepType:       stepType,
			ModelID:        step.ModelID,
			ToolID:         step.ToolID,
			PromptTemplate: step.PromptTemplate,
			Config:         step.Config,
		})
	}
	out, err := gw.deps.Store.CreateWorkflow(c.UserContext(), &storage.Workflow{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Status:      "active",
		Metadata:    req.Metadata,
		Steps:       steps,
	})
	if err != nil {
		gw.deps.Logger.Error("failed to create workflow", "error", err, "request_id", requestID)
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workflow", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) userGetWorkflow(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	wf, err := gw.deps.Store.GetWorkflow(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "workflow not found", requestID))
	}
	return c.JSON(wf)
}

func (gw *Gateway) userListWorkflowRuns(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	items, err := gw.deps.Store.ListWorkflowRuns(c.UserContext(), c.Params("id"), c.QueryInt("limit", 100))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list workflow runs", requestID))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) userGetWorkflowRun(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	run, err := gw.deps.Store.GetWorkflowRun(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "workflow run not found", requestID))
	}
	return c.JSON(run)
}

func (gw *Gateway) userRunWorkflow(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Input          map[string]interface{} `json:"input"`
		CallbackURL    string                 `json:"callback_url"`
		AgentSessionID string                 `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.Input == nil {
		req.Input = map[string]interface{}{}
	}
	callbackURL := strings.TrimSpace(req.CallbackURL)
	if callbackURL != "" && !validWebhookCallbackURL(callbackURL) {
		return c.Status(400).JSON(errorResponse("invalid_request", "callback_url must be an http or https URL", requestID))
	}
	wf, err := gw.deps.Store.GetWorkflow(c.UserContext(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(errorResponse("not_found", "workflow not found", requestID))
	}
	userID := auth.GetUserID(c)
	if wf.UserID != "" && wf.UserID != userID {
		return c.Status(404).JSON(errorResponse("not_found", "workflow not found", requestID))
	}
	if wf.WorkspaceID != "" {
		allowed, err := gw.deps.Store.WorkspaceMemberHasPermission(c.UserContext(), wf.WorkspaceID, userID, "workflows:write")
		if err != nil {
			gw.deps.Logger.Error("workspace permission check failed", "user_id", userID, "workspace_id", wf.WorkspaceID, "permission", "workflows:write", "error", err, "request_id", requestID)
			return c.Status(400).JSON(errorResponse("invalid_request", "invalid workflow workspace", requestID))
		}
		if !allowed {
			return c.Status(403).JSON(errorResponse("permission_denied", "user does not have workflows:write permission for the workspace", requestID))
		}
	}
	agentSessionID := strings.TrimSpace(req.AgentSessionID)
	if agentSessionID != "" {
		session, err := gw.deps.Store.GetAgentSession(c.UserContext(), agentSessionID, userID)
		if err != nil {
			return c.Status(404).JSON(errorResponse("not_found", "agent session not found", requestID))
		}
		if session.Status != "active" {
			return c.Status(400).JSON(errorResponse("invalid_request", "agent session is not active", requestID))
		}
		if session.WorkflowID != "" && session.WorkflowID != wf.ID {
			return c.Status(400).JSON(errorResponse("invalid_request", "agent session is bound to a different workflow", requestID))
		}
	}
	run, err := gw.deps.Store.CreateWorkflowRun(c.UserContext(), &storage.WorkflowRun{
		WorkflowID:     wf.ID,
		UserID:         userID,
		WorkspaceID:    wf.WorkspaceID,
		AgentSessionID: agentSessionID,
		Input:          req.Input,
	})
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create workflow run", requestID))
	}
	if err := gw.executeWorkflow(c.UserContext(), wf, run, req.Input); err != nil {
		_ = gw.deps.Store.CompleteWorkflowRun(c.UserContext(), run.ID, "failed", fiber.Map{"error": err.Error()}, run.TotalCostUSD)
		gw.recordWorkflowWebhook(c.UserContext(), run.ID, wf.ID, callbackURL, "failed", fiber.Map{"run_id": run.ID, "workflow_id": wf.ID, "status": "failed", "error": err.Error()})
		return c.Status(500).JSON(errorResponse("workflow_failed", err.Error(), requestID))
	}
	_ = gw.deps.Store.TouchAgentSessionRun(c.UserContext(), agentSessionID, run.ID)
	gw.recordWorkflowWebhook(c.UserContext(), run.ID, wf.ID, callbackURL, "completed", fiber.Map{"run_id": run.ID, "workflow_id": wf.ID, "status": "completed"})
	out, _ := gw.deps.Store.GetWorkflowRun(c.UserContext(), run.ID)
	return c.Status(201).JSON(out)
}

func (gw *Gateway) recordWorkflowWebhook(ctx context.Context, runID, workflowID, callbackURL, event string, payload map[string]interface{}) {
	if callbackURL == "" {
		return
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["event"] = "workflow.run." + event
	delivery := &storage.WebhookDelivery{
		WorkflowID:     workflowID,
		RunID:          runID,
		CallbackURL:    callbackURL,
		EventType:      "workflow.run." + event,
		Status:         "recorded",
		ResponseStatus: 0,
		MaxAttempts:    3,
		Payload:        payload,
	}
	if err := gw.deps.Store.RecordWebhookDelivery(ctx, delivery); err != nil {
		gw.deps.Logger.Error("failed to record workflow webhook delivery", "run_id", runID, "callback_url", callbackURL, "error", err)
		return
	}
	go gw.deliverWorkflowWebhook(context.Background(), delivery)
}

func (gw *Gateway) deliverWorkflowWebhook(ctx context.Context, delivery *storage.WebhookDelivery) {
	if delivery == nil || delivery.ID == "" || delivery.CallbackURL == "" {
		return
	}
	payloadBytes, err := json.Marshal(delivery.Payload)
	if err != nil {
		_ = gw.deps.Store.UpdateWebhookDeliveryResult(ctx, delivery.ID, "failed", 0, 1, "", "marshal webhook payload: "+err.Error(), "")
		return
	}
	signature := webhookPayloadSignature(gw.deps.Config.JWTSecret, payloadBytes)
	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	var lastStatus int
	var lastBody string
	var lastErr string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		httpReq, reqErr := http.NewRequestWithContext(reqCtx, http.MethodPost, delivery.CallbackURL, bytes.NewReader(payloadBytes))
		if reqErr != nil {
			cancel()
			lastErr = reqErr.Error()
			break
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", "ai-aggregator-webhook/1.0")
		httpReq.Header.Set("X-AAG-Event", delivery.EventType)
		httpReq.Header.Set("X-AAG-Delivery", delivery.ID)
		httpReq.Header.Set("X-AAG-Signature", signature)

		resp, reqErr := http.DefaultClient.Do(httpReq)
		if reqErr != nil {
			cancel()
			lastErr = reqErr.Error()
		} else {
			lastStatus = resp.StatusCode
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			cancel()
			lastBody = string(bodyBytes)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := gw.deps.Store.UpdateWebhookDeliveryResult(ctx, delivery.ID, "delivered", resp.StatusCode, attempt, lastBody, "", signature); err != nil {
					gw.deps.Logger.Error("failed to update delivered webhook", "delivery_id", delivery.ID, "error", err)
				}
				return
			}
			lastErr = fmt.Sprintf("webhook returned HTTP %d", resp.StatusCode)
		}
		if attempt < maxAttempts {
			_ = gw.deps.Store.UpdateWebhookDeliveryResult(ctx, delivery.ID, "retrying", lastStatus, attempt, lastBody, lastErr, signature)
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}
	if err := gw.deps.Store.UpdateWebhookDeliveryResult(ctx, delivery.ID, "failed", lastStatus, maxAttempts, lastBody, lastErr, signature); err != nil {
		gw.deps.Logger.Error("failed to update failed webhook", "delivery_id", delivery.ID, "error", err)
	}
}

func webhookPayloadSignature(secret string, payload []byte) string {
	if secret == "" {
		secret = "ai-aggregator-webhook-local"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (gw *Gateway) executeWorkflow(ctx context.Context, wf *storage.Workflow, run *storage.WorkflowRun, input map[string]interface{}) error {
	state := map[string]interface{}{"input": input}
	totalCost := 0.0
	for _, step := range wf.Steps {
		start := time.Now()
		stepInput := map[string]interface{}{"state": state}
		stepOutput := map[string]interface{}{}
		status := "completed"
		errMsg := ""
		cost := 0.0
		switch step.StepType {
		case "prompt":
			output, stepCost, err := gw.executePromptWorkflowStep(ctx, step, state)
			if err != nil {
				status = "failed"
				errMsg = err.Error()
			} else {
				stepOutput = output
				cost = stepCost
				state[step.Name] = output
				state["last"] = output
			}
		case "tool":
			output, err := executeBuiltinTool(step.ToolID, state)
			if err != nil {
				status = "failed"
				errMsg = err.Error()
			} else {
				stepOutput = output
				state[step.Name] = output
				state["last"] = output
			}
		default:
			stepOutput = map[string]interface{}{"state": state}
			state[step.Name] = stepOutput
		}
		totalCost += cost
		saved, recordErr := gw.deps.Store.RecordWorkflowRunStep(ctx, &storage.WorkflowRunStep{
			RunID:          run.ID,
			WorkflowStepID: step.ID,
			StepOrder:      step.StepOrder,
			Name:           step.Name,
			StepType:       step.StepType,
			Status:         status,
			Input:          stepInput,
			Output:         stepOutput,
			LatencyMs:      int(time.Since(start).Milliseconds()),
			CostUSD:        cost,
			ErrorMessage:   errMsg,
		})
		if recordErr != nil {
			return recordErr
		}
		_ = gw.deps.Store.RecordAgentTrace(ctx, &storage.AgentTrace{
			RunID:     run.ID,
			StepID:    saved.ID,
			TraceType: "step." + status,
			Message:   step.Name,
			Data:      fiber.Map{"step_type": step.StepType, "error": errMsg},
		})
		if status == "failed" {
			return fmt.Errorf("step %s failed: %s", step.Name, errMsg)
		}
	}
	return gw.deps.Store.CompleteWorkflowRun(ctx, run.ID, "completed", state, totalCost)
}

func (gw *Gateway) executePromptWorkflowStep(ctx context.Context, step storage.WorkflowStep, state map[string]interface{}) (map[string]interface{}, float64, error) {
	modelID := defaultGuardrailValue(step.ModelID, "qwen-turbo")
	adapter, upstreamModel, err := gw.deps.Router.Route(ctx, modelID)
	if err != nil {
		return nil, 0, err
	}
	prompt := renderWorkflowTemplate(step.PromptTemplate, state)
	resp, err := adapter.ChatCompletion(ctx, &provider.ChatRequest{
		Model: upstreamModel,
		Messages: []provider.ChatMessage{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, 0, err
	}
	content := ""
	if len(resp.Choices) > 0 {
		content, _ = resp.Choices[0].Message.Content.(string)
	}
	inputTokens, outputTokens := 0, 0
	if resp.Usage != nil {
		inputTokens = resp.Usage.PromptTokens
		outputTokens = resp.Usage.CompletionTokens
	}
	_, chargedCost := gw.calculateCost(modelID, inputTokens, outputTokens)
	return fiber.Map{"model": modelID, "content": content, "usage": resp.Usage}, chargedCost, nil
}

func executeBuiltinTool(toolID string, state map[string]interface{}) (map[string]interface{}, error) {
	if toolID == "" || toolID == "echo" {
		keys := make([]string, 0, len(state))
		for key := range state {
			keys = append(keys, key)
		}
		return fiber.Map{"echo": state["last"], "state_keys": keys}, nil
	}
	return nil, fmt.Errorf("unsupported tool: %s", toolID)
}

func renderWorkflowTemplate(template string, state map[string]interface{}) string {
	if strings.TrimSpace(template) == "" {
		template = "{{input}}"
	}
	inputBytes, _ := json.Marshal(state["input"])
	lastBytes, _ := json.Marshal(state["last"])
	out := strings.ReplaceAll(template, "{{input}}", string(inputBytes))
	out = strings.ReplaceAll(out, "{{last}}", string(lastBytes))
	return out
}

func firstNonZero(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func (gw *Gateway) adminListInferenceClusters(c *fiber.Ctx) error {
	items, err := gw.deps.Store.ListInferenceClusters(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list inference clusters", getRequestID(c)))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateInferenceCluster(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		Name        string                 `json:"name"`
		Region      string                 `json:"region"`
		NetworkMode string                 `json:"network_mode"`
		Status      string                 `json:"status"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "name is required", requestID))
	}
	networkMode := defaultGuardrailValue(req.NetworkMode, "private")
	if !oneOf(networkMode, "public", "private", "vpc") {
		return c.Status(400).JSON(errorResponse("invalid_request", "network_mode must be one of: public, private, vpc", requestID))
	}
	status := defaultGuardrailValue(req.Status, "active")
	if !oneOf(status, "active", "maintenance", "disabled") {
		return c.Status(400).JSON(errorResponse("invalid_request", "status must be one of: active, maintenance, disabled", requestID))
	}
	out, err := gw.deps.Store.CreateInferenceCluster(c.UserContext(), &storage.InferenceCluster{
		Name:        strings.TrimSpace(req.Name),
		Region:      defaultGuardrailValue(req.Region, "local"),
		NetworkMode: networkMode,
		Status:      status,
		Metadata:    req.Metadata,
	})
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create inference cluster", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListInferenceNodes(c *fiber.Ctx) error {
	items, err := gw.deps.Store.ListInferenceNodes(c.UserContext(), c.Query("cluster_id"))
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list inference nodes", getRequestID(c)))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateInferenceNode(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		ClusterID   string                 `json:"cluster_id"`
		Name        string                 `json:"name"`
		EndpointURL string                 `json:"endpoint_url"`
		GPUType     string                 `json:"gpu_type"`
		GPUCount    int                    `json:"gpu_count"`
		Status      string                 `json:"status"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.ClusterID == "" || strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "cluster_id and name are required", requestID))
	}
	status := defaultGuardrailValue(req.Status, "healthy")
	if !oneOf(status, "healthy", "degraded", "down", "maintenance") {
		return c.Status(400).JSON(errorResponse("invalid_request", "status must be one of: healthy, degraded, down, maintenance", requestID))
	}
	out, err := gw.deps.Store.CreateInferenceNode(c.UserContext(), &storage.InferenceNode{
		ClusterID:   req.ClusterID,
		Name:        strings.TrimSpace(req.Name),
		EndpointURL: req.EndpointURL,
		GPUType:     req.GPUType,
		GPUCount:    req.GPUCount,
		Status:      status,
		Metadata:    req.Metadata,
	})
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create inference node", requestID))
	}
	return c.Status(201).JSON(out)
}

func (gw *Gateway) adminListModelDeployments(c *fiber.Ctx) error {
	items, err := gw.deps.Store.ListModelDeployments(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to list model deployments", getRequestID(c)))
	}
	return c.JSON(fiber.Map{"data": items})
}

func (gw *Gateway) adminCreateModelDeployment(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	var req struct {
		ClusterID     string                 `json:"cluster_id"`
		ProviderID    string                 `json:"provider_id"`
		ModelID       string                 `json:"model_id"`
		UpstreamModel string                 `json:"upstream_model"`
		Runtime       string                 `json:"runtime"`
		EndpointURL   string                 `json:"endpoint_url"`
		Replicas      int                    `json:"replicas"`
		Status        string                 `json:"status"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(errorResponse("invalid_request", "invalid request body", requestID))
	}
	if req.ClusterID == "" || req.ProviderID == "" || req.ModelID == "" || req.EndpointURL == "" {
		return c.Status(400).JSON(errorResponse("invalid_request", "cluster_id, provider_id, model_id and endpoint_url are required", requestID))
	}
	runtime := defaultGuardrailValue(req.Runtime, "vllm")
	if !oneOf(runtime, "vllm", "sglang", "openai_compatible") {
		return c.Status(400).JSON(errorResponse("invalid_request", "runtime must be one of: vllm, sglang, openai_compatible", requestID))
	}
	status := defaultGuardrailValue(req.Status, "active")
	if !oneOf(status, "active", "deploying", "maintenance", "disabled") {
		return c.Status(400).JSON(errorResponse("invalid_request", "status must be one of: active, deploying, maintenance, disabled", requestID))
	}
	out, err := gw.deps.Store.CreateModelDeployment(c.UserContext(), &storage.ModelDeployment{
		ClusterID:     req.ClusterID,
		ProviderID:    req.ProviderID,
		ModelID:       req.ModelID,
		UpstreamModel: defaultGuardrailValue(req.UpstreamModel, req.ModelID),
		Runtime:       runtime,
		EndpointURL:   strings.TrimRight(req.EndpointURL, "/"),
		Replicas:      firstNonZero(req.Replicas, 1),
		Status:        status,
		Metadata:      req.Metadata,
	})
	if err != nil {
		return c.Status(500).JSON(errorResponse("internal_server_error", "failed to create model deployment", requestID))
	}
	if err := gw.deps.Router.Refresh(c.UserContext()); err != nil {
		gw.deps.Logger.Warn("failed to refresh router after model deployment", "error", err, "request_id", requestID)
	}
	return c.Status(201).JSON(out)
}

var (
	emailPattern      = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern      = regexp.MustCompile(`\b(?:\+?\d[\d\s().-]{7,}\d)\b`)
	cardPattern       = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
	injectionPatterns = []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"developer message",
		"system prompt",
		"reveal your instructions",
		"jailbreak",
		"dan mode",
	}
)

func (gw *Gateway) applyGuardrails(c *fiber.Ctx, req *provider.ChatRequest, requestID, userID string) (*storage.GuardrailResult, bool, error) {
	workspaceID := localString(c, "workspace_id")
	apiKeyID := localString(c, "api_key_id")
	policy, err := gw.deps.Store.GetActiveGuardrailPolicy(c.UserContext(), workspaceID)
	if err != nil || policy == nil {
		return nil, false, err
	}

	combinedText := strings.Join(chatRequestTexts(req), "\n")
	findings := detectGuardrailFindings(combinedText, policy)
	if len(findings) == 0 {
		result, err := gw.deps.Store.RecordGuardrailResult(c.UserContext(), &storage.GuardrailResult{
			RequestID:   requestID,
			UserID:      userID,
			WorkspaceID: workspaceID,
			APIKeyID:    apiKeyID,
			ModelID:     req.Model,
			PolicyID:    policy.ID,
			Action:      "allow",
			Status:      "passed",
			RiskScore:   0,
			Categories:  []string{},
			Findings:    []storage.GuardrailFinding{},
		})
		return result, false, err
	}

	action := "allow"
	status := "passed"
	riskScore := 0.0
	categories := map[string]bool{}
	for _, finding := range findings {
		categories[finding.Category] = true
		if finding.Action == "block" {
			action = "block"
			status = "blocked"
		} else if finding.Action == "mask" && action != "block" {
			action = "mask"
			status = "masked"
		}
		switch finding.Severity {
		case "critical":
			riskScore += 50
		case "high":
			riskScore += 35
		default:
			riskScore += 20
		}
	}
	if riskScore > 100 {
		riskScore = 100
	}
	if action == "mask" {
		maskChatRequest(req)
	}
	categoryList := make([]string, 0, len(categories))
	for category := range categories {
		categoryList = append(categoryList, category)
	}
	result, err := gw.deps.Store.RecordGuardrailResult(c.UserContext(), &storage.GuardrailResult{
		RequestID:   requestID,
		UserID:      userID,
		WorkspaceID: workspaceID,
		APIKeyID:    apiKeyID,
		ModelID:     req.Model,
		PolicyID:    policy.ID,
		Action:      action,
		Status:      status,
		RiskScore:   riskScore,
		Categories:  categoryList,
		Findings:    findings,
	})
	return result, action == "block", err
}

func detectGuardrailFindings(text string, policy *storage.GuardrailPolicy) []storage.GuardrailFinding {
	findings := make([]storage.GuardrailFinding, 0)
	addPII := func(kind string, count int) {
		if count > 0 && policy.PIIAction != "allow" {
			findings = append(findings, storage.GuardrailFinding{Type: kind, Category: "pii", Count: count, Action: policy.PIIAction, Severity: "medium"})
		}
	}
	addPII("email", len(emailPattern.FindAllString(text, -1)))
	addPII("phone_or_numeric_identifier", len(phonePattern.FindAllString(text, -1)))
	addPII("payment_card", len(cardPattern.FindAllString(text, -1)))

	lower := strings.ToLower(text)
	for _, pattern := range injectionPatterns {
		if strings.Contains(lower, pattern) && policy.InjectionAction != "allow" {
			findings = append(findings, storage.GuardrailFinding{Type: "prompt_injection", Category: "security", Count: 1, Action: policy.InjectionAction, Severity: "high"})
			break
		}
	}
	return findings
}

func chatRequestTexts(req *provider.ChatRequest) []string {
	texts := make([]string, 0, len(req.Messages))
	for _, message := range req.Messages {
		switch content := message.Content.(type) {
		case string:
			texts = append(texts, content)
		case []interface{}:
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		case []provider.ContentPart:
			for _, part := range content {
				if part.Text != "" {
					texts = append(texts, part.Text)
				}
			}
		}
	}
	return texts
}

func maskChatRequest(req *provider.ChatRequest) {
	for i := range req.Messages {
		switch content := req.Messages[i].Content.(type) {
		case string:
			req.Messages[i].Content = maskSensitiveText(content)
		case []interface{}:
			for j, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						partMap["text"] = maskSensitiveText(text)
						content[j] = partMap
					}
				}
			}
			req.Messages[i].Content = content
		case []provider.ContentPart:
			for j := range content {
				content[j].Text = maskSensitiveText(content[j].Text)
			}
			req.Messages[i].Content = content
		}
	}
}

func maskSensitiveText(text string) string {
	text = emailPattern.ReplaceAllString(text, "[EMAIL_MASKED]")
	text = cardPattern.ReplaceAllString(text, "[CARD_MASKED]")
	text = phonePattern.ReplaceAllString(text, "[NUMBER_MASKED]")
	return text
}

func defaultGuardrailValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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
		if err := gw.deps.Store.CreateBillingTransactionForWorkspaceProject(ctx, record.UserID, record.WorkspaceID, record.ProjectID, -record.ChargedCost, "usage_charge", desc); err != nil {
			gw.deps.Logger.Error("failed to write billing transaction",
				"error", err,
				"user_id", record.UserID,
				"amount", -record.ChargedCost,
				"request_id", record.RequestID,
			)
		}
	}
}

// recordRequestLog writes request-level observability without affecting the API response.
func (gw *Gateway) recordRequestLog(ctx context.Context, log *storage.RequestLog) {
	if log == nil || log.RequestID == "" {
		return
	}
	if err := gw.deps.Store.RecordRequestLog(ctx, log); err != nil {
		gw.deps.Logger.Error("failed to record request log",
			"error", err,
			"request_id", log.RequestID,
			"user_id", log.UserID,
		)
	}
}

func (gw *Gateway) recordAudit(c *fiber.Ctx, action, resourceType, resourceID, workspaceID string, details map[string]interface{}) {
	if details == nil {
		details = map[string]interface{}{}
	}
	if err := gw.deps.Store.RecordAuditLog(c.UserContext(), &storage.AuditLog{
		UserID:       auth.GetUserID(c),
		WorkspaceID:  workspaceID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    c.IP(),
		UserAgent:    c.Get("User-Agent"),
	}); err != nil {
		gw.deps.Logger.Warn("failed to record audit log", "action", action, "resource_id", resourceID, "error", err, "request_id", getRequestID(c))
	}
}

func (gw *Gateway) recordFallbackLog(ctx context.Context, requestID, modelID, fromProviderID, toProviderID, reason, errorCode, errorMessage string) {
	if requestID == "" {
		return
	}
	if err := gw.deps.Store.RecordFallbackLog(ctx, &storage.FallbackLog{
		RequestID:      requestID,
		ModelID:        modelID,
		FromProviderID: fromProviderID,
		ToProviderID:   toProviderID,
		Reason:         reason,
		ErrorCode:      errorCode,
		ErrorMessage:   truncatePreview(errorMessage),
	}); err != nil {
		gw.deps.Logger.Error("failed to record fallback log",
			"error", err,
			"request_id", requestID,
			"from_provider", fromProviderID,
			"to_provider", toProviderID,
		)
	}
}

// respondChatError returns a normalized error and records a request_log when
// the request has already passed auth and therefore has a user context.
func (gw *Gateway) respondChatError(c *fiber.Ctx, start time.Time, requestID, userID, modelID, providerID string, statusCode int, code, errType, message string, fallbackCount int) error {
	if errType == "" {
		errType = errorTypeForCode(code)
	}
	log := &storage.RequestLog{
		RequestID:       requestID,
		UserID:          userID,
		APIKeyID:        localString(c, "api_key_id"),
		WorkspaceID:     localString(c, "workspace_id"),
		ProjectID:       localString(c, "project_id"),
		ModelID:         modelID,
		ProviderID:      providerID,
		Method:          c.Method(),
		Path:            c.Path(),
		StatusCode:      statusCode,
		ErrorCode:       code,
		ErrorMessage:    message,
		LatencyMs:       int(time.Since(start).Milliseconds()),
		FallbackCount:   fallbackCount,
		RequestPreview:  truncatePreview(string(c.Body())),
		ResponsePreview: responsePreview(errorResponseWithType(code, errType, message, requestID)),
	}
	gw.recordRequestLog(c.UserContext(), log)
	return c.Status(statusCode).JSON(errorResponseWithType(code, errType, message, requestID))
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

// errorResponse builds a normalized error envelope.
func errorResponse(code, message, requestID string) fiber.Map {
	return errorResponseWithType(code, errorTypeForCode(code), message, requestID)
}

func errorResponseWithType(code, errType, message, requestID string) fiber.Map {
	resp := fiber.Map{
		"error": fiber.Map{
			"message": message,
			"type":    errType,
			"code":    code,
		},
	}
	if requestID != "" {
		resp["error"].(fiber.Map)["request_id"] = requestID
	}
	return resp
}

func errorTypeForCode(code string) string {
	switch code {
	case "missing_auth", "invalid_auth_format", "authentication_error", "invalid_api_key", "expired_token":
		return "auth_error"
	case "insufficient_balance", "quota_exceeded":
		return "billing_error"
	case "rate_limit_exceeded":
		return "rate_limit_error"
	case "permission_denied":
		return "permission_error"
	case "provider_timeout", "provider_unavailable", "upstream_error":
		return "provider_error"
	case "no_available_provider", "model_not_found":
		return "routing_error"
	case "invalid_request", "invalid_request_error", "unsupported_parameter":
		return "validation_error"
	case "conflict", "not_found":
		return "client_error"
	default:
		return "internal_error"
	}
}

func responsePreview(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return truncatePreview(string(b))
}

func truncatePreview(s string) string {
	const maxPreviewChars = 2000
	if len(s) <= maxPreviewChars {
		return s
	}
	return s[:maxPreviewChars]
}

func localString(c *fiber.Ctx, key string) string {
	if v, ok := c.Locals(key).(string); ok {
		return v
	}
	return ""
}

func requiresWorkspaceWritePermission(c *fiber.Ctx) bool {
	switch localString(c, "user_role") {
	case "admin", "super_admin":
		return false
	default:
		return true
	}
}

func localInt(c *fiber.Ctx, key string) int {
	if v, ok := c.Locals(key).(int); ok {
		return v
	}
	return 0
}

func optionalTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func marketplaceModelPayload(m *storage.Model, bindings []storage.ProviderBinding, benchmarkScore *float64) fiber.Map {
	enabledProviders := 0
	healthyProviders := 0
	providers := make([]fiber.Map, 0, len(bindings))
	for _, b := range bindings {
		if b.IsEnabled {
			enabledProviders++
		}
		if b.IsEnabled && b.HealthStatus != "down" && b.HealthStatus != "unhealthy" {
			healthyProviders++
		}
		providers = append(providers, fiber.Map{
			"provider_id":   b.ProviderID,
			"priority":      b.Priority,
			"health_status": b.HealthStatus,
			"is_enabled":    b.IsEnabled,
		})
	}
	return fiber.Map{
		"id":                 m.ModelID,
		"model_id":           m.ModelID,
		"display_name":       m.DisplayName,
		"modality":           m.Modality,
		"capabilities":       m.Capabilities,
		"tags":               m.Tags,
		"input_price":        m.InputPrice,
		"output_price":       m.OutputPrice,
		"price_unit":         m.PriceUnit,
		"max_context":        m.MaxContext,
		"max_output":         m.MaxOutput,
		"supports_stream":    m.SupportsStream,
		"is_async":           m.IsAsync,
		"provider_count":     enabledProviders,
		"healthy_providers":  healthyProviders,
		"providers":          providers,
		"marketplace_score":  marketplaceScore(m, enabledProviders, healthyProviders),
		"recommended_use":    recommendedUseCase(m),
		"benchmark_score":    benchmarkScore,
		"availability_label": availabilityLabel(enabledProviders, healthyProviders),
	}
}

func buildBenchmarkResult(task *storage.BenchmarkTask, model *storage.Model) storage.BenchmarkResult {
	datasetCount := len(task.Dataset)
	if datasetCount == 0 {
		datasetCount = 1
	}
	quality := 62.0 + float64(len(model.Capabilities))*3.5
	if model.MaxContext != nil {
		switch {
		case *model.MaxContext >= 100000:
			quality += 10
		case *model.MaxContext >= 32000:
			quality += 6
		case *model.MaxContext >= 8000:
			quality += 3
		}
	}
	if model.SupportsStream {
		quality += 2
	}
	if quality > 98 {
		quality = 98
	}

	latencyMs := 1400 + len(model.ModelID)*17 + datasetCount*25
	if model.Modality == "embedding" {
		latencyMs = 360 + len(model.ModelID)*9
	}
	if model.Modality == "image" || model.Modality == "video" {
		latencyMs = 4500 + datasetCount*400
	}

	inputPrice := 0.0
	outputPrice := 0.0
	if model.InputPrice != nil {
		inputPrice = *model.InputPrice
	}
	if model.OutputPrice != nil {
		outputPrice = *model.OutputPrice
	}
	costUSD := float64(datasetCount) * (inputPrice + outputPrice) / 1000.0
	if costUSD == 0 {
		costUSD = float64(datasetCount) * 0.0001
	}

	latencyScore := 100.0 - float64(latencyMs)/100.0
	if latencyScore < 20 {
		latencyScore = 20
	}
	costScore := 100.0 - costUSD*1000.0
	if costScore < 20 {
		costScore = 20
	}
	total := quality*0.65 + latencyScore*0.2 + costScore*0.15
	if total > 100 {
		total = 100
	}

	return storage.BenchmarkResult{
		ModelID:      model.ModelID,
		QualityScore: round2(quality),
		LatencyMs:    latencyMs,
		CostUSD:      costUSD,
		TotalScore:   round2(total),
		Details: fiber.Map{
			"mode":          "deterministic_local",
			"dataset_count": datasetCount,
			"modality":      model.Modality,
		},
	}
}

func marketplaceScore(m *storage.Model, providerCount, healthyProviders int) float64 {
	score := 50.0
	if m.SupportsStream {
		score += 8
	}
	if m.MaxContext != nil {
		switch {
		case *m.MaxContext >= 100000:
			score += 15
		case *m.MaxContext >= 32000:
			score += 8
		}
	}
	if m.InputPrice != nil && m.OutputPrice != nil {
		total := *m.InputPrice + *m.OutputPrice
		switch {
		case total <= 0.1:
			score += 12
		case total <= 1:
			score += 8
		case total <= 5:
			score += 4
		}
	}
	score += float64(providerCount * 4)
	score += float64(healthyProviders * 3)
	if score > 100 {
		return 100
	}
	return score
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func recommendedUseCase(m *storage.Model) string {
	if len(m.Capabilities) > 0 {
		return strings.Join(m.Capabilities, ", ")
	}
	switch m.Modality {
	case "text":
		return "chat, reasoning, content generation"
	case "embedding":
		return "search, RAG, semantic similarity"
	case "image":
		return "image generation and creative assets"
	case "video":
		return "video generation"
	case "audio":
		return "speech processing"
	default:
		return "general AI workload"
	}
}

func availabilityLabel(providerCount, healthyProviders int) string {
	if healthyProviders > 1 {
		return "high"
	}
	if healthyProviders == 1 || providerCount > 0 {
		return "standard"
	}
	return "unavailable"
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func stringSliceContains(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseRequestLogFilter(c *fiber.Ctx) (storage.RequestLogFilter, error) {
	filter := storage.RequestLogFilter{
		Limit:      c.QueryInt("limit", 50),
		Offset:     c.QueryInt("offset", 0),
		ModelID:    strings.TrimSpace(c.Query("model")),
		ProviderID: strings.TrimSpace(c.Query("provider")),
		ProjectID:  strings.TrimSpace(c.Query("project_id")),
		Status:     strings.ToLower(strings.TrimSpace(c.Query("status"))),
	}
	if filter.Status != "" && filter.Status != "all" && filter.Status != "success" && filter.Status != "error" {
		return filter, fmt.Errorf("status must be one of all, success, error")
	}
	if filter.Status == "all" {
		filter.Status = ""
	}
	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := parseRequestLogTime(fromRaw, false)
		if err != nil {
			return filter, fmt.Errorf("invalid from timestamp")
		}
		filter.From = &from
	}
	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := parseRequestLogTime(toRaw, true)
		if err != nil {
			return filter, fmt.Errorf("invalid to timestamp")
		}
		filter.To = &to
	}
	return filter, nil
}

func parseAuditLogFilter(c *fiber.Ctx, defaultLimit int) (storage.AuditLogFilter, error) {
	filter := storage.AuditLogFilter{
		Limit:        c.QueryInt("limit", defaultLimit),
		Action:       strings.TrimSpace(c.Query("action")),
		WorkspaceID:  strings.TrimSpace(c.Query("workspace_id")),
		ResourceType: strings.TrimSpace(c.Query("resource_type")),
		ResourceID:   strings.TrimSpace(c.Query("resource_id")),
	}
	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := parseRequestLogTime(fromRaw, false)
		if err != nil {
			return filter, fmt.Errorf("invalid from timestamp")
		}
		filter.From = &from
	}
	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := parseRequestLogTime(toRaw, true)
		if err != nil {
			return filter, fmt.Errorf("invalid to timestamp")
		}
		filter.To = &to
	}
	return filter, nil
}

func parseRequestLogTime(raw string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, nil
}

func parseDateOnly(raw string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(raw))
}

func writeRequestLogsCSV(c *fiber.Ctx, logs []storage.RequestLog, filename string) error {
	if filename == "" {
		filename = "request-logs.csv"
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{
		"request_id",
		"created_at",
		"method",
		"path",
		"model_id",
		"provider_id",
		"final_provider_id",
		"credential_scope",
		"credential_key_id",
		"workspace_id",
		"project_id",
		"status_code",
		"error_code",
		"latency_ms",
		"input_tokens",
		"output_tokens",
		"total_tokens",
		"charged_cost_usd",
		"upstream_cost_usd",
		"gross_margin_usd",
		"fallback_count",
	})
	for _, log := range logs {
		_ = writer.Write([]string{
			log.RequestID,
			log.CreatedAt.Format(time.RFC3339),
			log.Method,
			log.Path,
			log.ModelID,
			log.ProviderID,
			log.FinalProviderID,
			log.CredentialScope,
			log.CredentialKeyID,
			log.WorkspaceID,
			log.ProjectID,
			strconv.Itoa(log.StatusCode),
			log.ErrorCode,
			strconv.Itoa(log.LatencyMs),
			strconv.Itoa(log.InputTokens),
			strconv.Itoa(log.OutputTokens),
			strconv.Itoa(log.TotalTokens),
			fmt.Sprintf("%.8f", log.ChargedCost),
			fmt.Sprintf("%.8f", log.UpstreamCost),
			fmt.Sprintf("%.8f", log.GrossMargin),
			strconv.Itoa(log.FallbackCount),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func writeInvoicesCSV(c *fiber.Ctx, invoices []storage.Invoice, filename string) error {
	if filename == "" {
		filename = "invoices.csv"
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{
		"id",
		"invoice_number",
		"organization_id",
		"workspace_id",
		"period_start",
		"period_end",
		"status",
		"po_number",
		"subtotal_usd",
		"tax_usd",
		"total_usd",
		"due_date",
		"notes",
		"created_at",
		"updated_at",
	})
	for _, invoice := range invoices {
		dueDate := ""
		if invoice.DueDate != nil {
			dueDate = invoice.DueDate.Format("2006-01-02")
		}
		_ = writer.Write([]string{
			invoice.ID,
			invoice.InvoiceNumber,
			invoice.OrganizationID,
			invoice.WorkspaceID,
			invoice.PeriodStart.Format("2006-01-02"),
			invoice.PeriodEnd.Format("2006-01-02"),
			invoice.Status,
			invoice.PONumber,
			fmt.Sprintf("%.8f", invoice.SubtotalUSD),
			fmt.Sprintf("%.8f", invoice.TaxUSD),
			fmt.Sprintf("%.8f", invoice.TotalUSD),
			dueDate,
			invoice.Notes,
			invoice.CreatedAt.Format(time.RFC3339),
			invoice.UpdatedAt.Format(time.RFC3339),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func buildInvoicePDF(invoice *storage.Invoice) []byte {
	if invoice == nil {
		return []byte("%PDF-1.4\n%%EOF\n")
	}
	dueDate := "-"
	if invoice.DueDate != nil {
		dueDate = invoice.DueDate.Format("2006-01-02")
	}
	lines := []string{
		"AI Aggregator Invoice",
		"",
		"Invoice Number: " + invoice.InvoiceNumber,
		"Status: " + strings.ToUpper(invoice.Status),
		"Organization ID: " + invoice.OrganizationID,
		"Workspace ID: " + defaultGuardrailValue(invoice.WorkspaceID, "-"),
		"Period: " + invoice.PeriodStart.Format("2006-01-02") + " to " + invoice.PeriodEnd.Format("2006-01-02"),
		"PO Number: " + defaultGuardrailValue(invoice.PONumber, "-"),
		"Due Date: " + dueDate,
		"",
		fmt.Sprintf("Subtotal USD: %.2f", invoice.SubtotalUSD),
		fmt.Sprintf("Tax USD: %.2f", invoice.TaxUSD),
		fmt.Sprintf("Total USD: %.2f", invoice.TotalUSD),
		"",
		"Notes: " + defaultGuardrailValue(invoice.Notes, "-"),
		"",
		"Generated by AI Aggregator.",
	}

	var content strings.Builder
	content.WriteString("BT\n/F1 18 Tf\n72 760 Td\n")
	for i, line := range lines {
		if i == 1 {
			content.WriteString("/F1 11 Tf\n")
		}
		content.WriteString("(")
		content.WriteString(escapePDFText(line))
		content.WriteString(") Tj\n0 -22 Td\n")
	}
	content.WriteString("ET\n")
	stream := content.String()

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	for i, obj := range objects {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i < len(offsets); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return buf.Bytes()
}

func escapePDFText(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, "\r", " ", "\n", " ")
	return replacer.Replace(value)
}

func sanitizeDownloadFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "invoice"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func writeAuditLogsCSV(c *fiber.Ctx, logs []storage.AuditLog, filename string) error {
	if filename == "" {
		filename = "audit-logs.csv"
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{
		"id",
		"created_at",
		"user_id",
		"organization_id",
		"workspace_id",
		"action",
		"resource_type",
		"resource_id",
		"ip_address",
		"user_agent",
		"details_json",
	})
	for _, log := range logs {
		detailsBytes, err := json.Marshal(log.Details)
		if err != nil {
			return err
		}
		_ = writer.Write([]string{
			strconv.FormatInt(log.ID, 10),
			log.CreatedAt.Format(time.RFC3339),
			log.UserID,
			log.OrganizationID,
			log.WorkspaceID,
			log.Action,
			log.ResourceType,
			log.ResourceID,
			log.IPAddress,
			log.UserAgent,
			string(detailsBytes),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func fileResponse(record *storage.FileRecord) fiber.Map {
	created := record.CreatedAt.Unix()
	return fiber.Map{
		"id":         record.ID,
		"object":     "file",
		"bytes":      record.Bytes,
		"created_at": created,
		"filename":   record.Filename,
		"purpose":    record.Purpose,
		"status":     record.Status,
		"mime_type":  record.MimeType,
		"metadata":   record.Metadata,
	}
}

func (gw *Gateway) uploadMimeAllowList(ctx context.Context) []string {
	raw, err := gw.deps.Store.GetSetting(ctx, "allowed_upload_mime_types")
	if err != nil {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.ToLower(strings.TrimSpace(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func fileMimeAllowed(detected, declared string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	detected = strings.ToLower(strings.TrimSpace(strings.Split(detected, ";")[0]))
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	for _, item := range allowed {
		switch {
		case item == "*" || item == "*/*":
			return true
		case strings.HasSuffix(item, "/*"):
			prefix := strings.TrimSuffix(item, "/*") + "/"
			if strings.HasPrefix(detected, prefix) || strings.HasPrefix(declared, prefix) {
				return true
			}
		case item == detected || item == declared:
			return true
		}
	}
	return false
}

type fileScanResult struct {
	Status    string `json:"status"`
	Scanner   string `json:"scanner"`
	Threat    string `json:"threat,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func scanUploadedFile(payload []byte) fileScanResult {
	const scanner = "local-signature-v1"
	if bytes.Contains(payload, []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")) {
		return fileScanResult{
			Status:    "blocked",
			Scanner:   scanner,
			Threat:    "eicar_test_file",
			Signature: "EICAR-STANDARD-ANTIVIRUS-TEST-FILE",
		}
	}
	return fileScanResult{Status: "clean", Scanner: scanner}
}

func parsePositiveInt64(raw string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func validWebhookCallbackURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func sealCredentialSecret(secret string) string {
	return "local:v1:" + hex.EncodeToString([]byte(secret))
}

func maskCredentialSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}
	return secret[:4] + strings.Repeat("*", 8) + secret[len(secret)-4:]
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
