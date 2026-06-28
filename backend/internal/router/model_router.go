package router

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"ai-aggregator/internal/provider"
	"ai-aggregator/internal/storage"
)

// ModelRouter is the central routing component that maps incoming requests
// to the appropriate upstream provider based on model_id, priority, and health.
type ModelRouter struct {
	store     *storage.Store
	adapters  map[string]provider.Adapter // provider_id -> Adapter
	providers map[string]*storage.Provider
	models    map[string]*ModelEntry // model_id -> routing info
	mu        sync.RWMutex
	refreshCh chan struct{}
	mockMode  bool
}

// ModelEntry holds the routing configuration for a single model.
type ModelEntry struct {
	ModelID     string
	Modality    string
	DisplayName string
	Providers   []storage.ProviderBinding
	IsAsync     bool
}

func NewModelRouter(ctx context.Context, store *storage.Store, mockMode bool) (*ModelRouter, error) {
	r := &ModelRouter{
		store:     store,
		adapters:  make(map[string]provider.Adapter),
		providers: make(map[string]*storage.Provider),
		models:    make(map[string]*ModelEntry),
		refreshCh: make(chan struct{}, 1),
		mockMode:  mockMode,
	}

	// Initialize provider adapters
	if err := r.initAdapters(ctx); err != nil {
		return nil, fmt.Errorf("init adapters: %w", err)
	}

	// Load model registry from DB
	if err := r.refreshRegistry(ctx); err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	// Start background health checker
	go r.healthCheckLoop(ctx)

	// Start registry refresh listener
	go r.refreshLoop(ctx)

	slog.Info("model router initialized",
		"models", len(r.models),
		"providers", len(r.adapters),
	)

	return r, nil
}

// Route selects the best provider for a given model and returns the adapter
// along with the upstream model name.
func (r *ModelRouter) Route(ctx context.Context, modelID string) (provider.Adapter, string, error) {
	if err := r.refreshRegistry(ctx); err != nil {
		slog.Warn("failed to refresh registry before route", "model", modelID, "error", err)
	}

	r.mu.RLock()
	entry, ok := r.models[modelID]
	r.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("model not found: %s", modelID)
	}

	// Sort providers by active policy.
	candidates := make([]storage.ProviderBinding, len(entry.Providers))
	copy(candidates, entry.Providers)
	r.sortCandidates(ctx, modelID, "", candidates)

	// Try each provider in priority order
	var lastErr error
	for _, binding := range candidates {
		if !binding.IsEnabled {
			continue
		}
		if binding.HealthStatus == "down" || binding.HealthStatus == "unhealthy" {
			continue
		}

		adapter, ok := r.adapters[binding.ProviderID]
		if !ok {
			continue
		}

		upstreamModel := binding.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = modelID
		}

		return adapter, upstreamModel, nil
	}

	if lastErr != nil {
		return nil, "", fmt.Errorf("all providers failed for model %s: %w", modelID, lastErr)
	}
	return nil, "", fmt.Errorf("no available provider for model %s", modelID)
}

// RouteEntry represents a single provider routing entry with its adapter and upstream model.
type RouteEntry struct {
	Adapter         provider.Adapter
	UpstreamModel   string
	ProviderID      string
	CredentialKeyID string
	CredentialScope string
}

// RouteAll returns all available adapters for a model, sorted by the active routing policy.
// Only includes providers that are enabled and not marked as "down".
func (r *ModelRouter) RouteAll(ctx context.Context, modelID string) ([]RouteEntry, error) {
	return r.RouteAllForRequest(ctx, modelID, "", "")
}

func (r *ModelRouter) RouteAllForRequest(ctx context.Context, modelID, userID, workspaceID string) ([]RouteEntry, error) {
	if err := r.refreshRegistry(ctx); err != nil {
		slog.Warn("failed to refresh registry before route_all", "model", modelID, "error", err)
	}

	r.mu.RLock()
	entry, ok := r.models[modelID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	// Sort providers by priority first; smart policies may refine this order below.
	candidates := make([]storage.ProviderBinding, len(entry.Providers))
	copy(candidates, entry.Providers)
	r.sortCandidates(ctx, modelID, "", candidates)

	var routes []RouteEntry
	for _, binding := range candidates {
		if !binding.IsEnabled {
			continue
		}
		if binding.HealthStatus == "down" || binding.HealthStatus == "unhealthy" {
			continue
		}

		adapter, credentialKeyID, credentialScope, ok := r.adapterForRequest(ctx, binding.ProviderID, userID, workspaceID)
		if !ok {
			continue
		}

		upstreamModel := binding.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = modelID
		}

		routes = append(routes, RouteEntry{
			Adapter:         adapter,
			UpstreamModel:   upstreamModel,
			ProviderID:      binding.ProviderID,
			CredentialKeyID: credentialKeyID,
			CredentialScope: credentialScope,
		})
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("no available provider for model %s", modelID)
	}

	return routes, nil
}

func (r *ModelRouter) sortCandidates(ctx context.Context, modelID, workspaceID string, candidates []storage.ProviderBinding) {
	policy, err := r.store.GetActiveRoutingPolicy(ctx, modelID, workspaceID)
	if err != nil {
		slog.Warn("failed to load routing policy, falling back to priority", "model", modelID, "error", err)
	}
	if policy == nil || policy.Strategy == "" || policy.Strategy == "priority" {
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Priority < candidates[j].Priority
		})
		return
	}
	stats, err := r.store.GetProviderRoutingStats(ctx, modelID)
	if err != nil {
		slog.Warn("failed to load provider routing stats, falling back to priority", "model", modelID, "error", err)
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Priority < candidates[j].Priority
		})
		return
	}
	score := func(binding storage.ProviderBinding) float64 {
		stat, hasStats := stats[binding.ProviderID]
		latency := stat.AvgLatencyMs
		if !hasStats || latency <= 0 {
			latency = float64(1000 + binding.Priority*100)
		}
		cost := binding.CostMultiplier
		if cost <= 0 {
			cost = 1
		}
		errorRate := stat.ErrorRate
		switch policy.Strategy {
		case "cost":
			return cost*1000 + float64(binding.Priority)
		case "latency":
			return latency + float64(binding.Priority)
		case "balanced":
			return latency*policy.LatencyWeight + cost*1000*policy.CostWeight + errorRate*1000*policy.ErrorWeight + float64(binding.Priority)
		default:
			return float64(binding.Priority)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := score(candidates[i])
		right := score(candidates[j])
		if left == right {
			return candidates[i].Priority < candidates[j].Priority
		}
		return left < right
	})
}

// GetAdapter returns a specific provider adapter by ID.
func (r *ModelRouter) GetAdapter(providerID string) (provider.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[providerID]
	return adapter, ok
}

// GetModel returns model entry by ID.
func (r *ModelRouter) GetModel(modelID string) (*ModelEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.models[modelID]
	return entry, ok
}

// ListModels returns all registered models.
func (r *ModelRouter) ListModels() []*ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ModelEntry, 0, len(r.models))
	for _, entry := range r.models {
		result = append(result, entry)
	}
	return result
}

// NotifyRefresh triggers a registry reload.
func (r *ModelRouter) NotifyRefresh() {
	select {
	case r.refreshCh <- struct{}{}:
	default:
		// Already pending
	}
}

// Refresh reloads the model/provider binding registry synchronously.
func (r *ModelRouter) Refresh(ctx context.Context) error {
	if err := r.initAdapters(ctx); err != nil {
		return err
	}
	return r.refreshRegistry(ctx)
}

// ===== Internal Methods =====

func (r *ModelRouter) initAdapters(ctx context.Context) error {
	// Load provider configs from DB and create adapters
	providers, err := r.store.GetProviders(ctx)
	if err != nil {
		return fmt.Errorf("load providers: %w", err)
	}

	// Mock mode: register a single mock adapter mapped to every provider in the DB.
	if r.mockMode {
		slog.Info("mock provider mode enabled", "provider_count", len(providers))
		for _, p := range providers {
			r.providers[p.ID] = p
			r.adapters[p.ID] = provider.NewMockAdapterWithName(p.ID)
			slog.Info("mock adapter registered", "provider", p.ID)
		}
		// Also register under the synthetic "mock" key so lookups always work.
		r.adapters["mock"] = provider.NewMockAdapter()
		return nil
	}

	for _, p := range providers {
		if !p.IsEnabled {
			continue
		}
		r.providers[p.ID] = p

		switch p.AdapterType {
		case "mock":
			r.adapters[p.ID] = provider.NewMockAdapterWithName(p.ID)
			slog.Info("mock adapter initialized", "provider", p.ID)

		case "dashscope":
			apiKey := r.resolveProviderKey(p)
			if apiKey == "" {
				slog.Warn("no API key for provider, skipping", "provider", p.ID)
				continue
			}
			adapter := provider.NewDashScopeAdapter(provider.DashScopeConfig{
				Name:    p.ID,
				BaseURL: p.BaseURL,
				APIKey:  apiKey,
			})
			r.adapters[p.ID] = adapter
			slog.Info("adapter initialized", "provider", p.ID, "url", p.BaseURL)

		case "openai_compatible", "self_hosted":
			adapter := provider.NewDashScopeAdapter(provider.DashScopeConfig{
				Name:    p.ID,
				BaseURL: p.BaseURL,
				APIKey:  r.resolveProviderKey(p),
			})
			r.adapters[p.ID] = adapter
			slog.Info("openai-compatible adapter initialized", "provider", p.ID, "url", p.BaseURL, "type", p.AdapterType)

		case "anthropic":
			apiKey := r.resolveProviderKey(p)
			if apiKey == "" {
				slog.Warn("no API key for provider, skipping", "provider", p.ID)
				continue
			}
			adapter := provider.NewAnthropicAdapter(provider.AnthropicConfig{
				Name:    p.ID,
				BaseURL: p.BaseURL,
				APIKey:  apiKey,
			})
			r.adapters[p.ID] = adapter
			slog.Info("anthropic adapter initialized", "provider", p.ID, "url", p.BaseURL)

		default:
			slog.Warn("unknown adapter type, skipping", "provider", p.ID, "type", p.AdapterType)
		}
	}

	return nil
}

func (r *ModelRouter) adapterForRequest(ctx context.Context, providerID, userID, workspaceID string) (provider.Adapter, string, string, bool) {
	r.mu.RLock()
	p := r.providers[providerID]
	adapter := r.adapters[providerID]
	mockMode := r.mockMode
	r.mu.RUnlock()

	if p == nil {
		return adapter, "", "", adapter != nil
	}
	if mockMode || p.AdapterType == "mock" {
		_, keyID, scope, _ := r.store.GetScopedProviderKeySecretWithMetadata(ctx, p.ID, userID, workspaceID)
		if adapter != nil {
			return adapter, keyID, scope, true
		}
		return provider.NewMockAdapterWithName(providerID), keyID, scope, true
	}
	if p.AdapterType != "dashscope" && p.AdapterType != "openai_compatible" && p.AdapterType != "self_hosted" && p.AdapterType != "anthropic" {
		return adapter, "", "", adapter != nil
	}

	apiKey, keyID, scope := r.resolveProviderKeyForRequestWithMetadata(ctx, p, userID, workspaceID)
	if apiKey == "" {
		return adapter, keyID, scope, adapter != nil
	}
	if p.AdapterType == "anthropic" {
		return provider.NewAnthropicAdapter(provider.AnthropicConfig{
			Name:    p.ID,
			BaseURL: p.BaseURL,
			APIKey:  apiKey,
		}), keyID, scope, true
	}
	return provider.NewDashScopeAdapter(provider.DashScopeConfig{
		Name:    p.ID,
		BaseURL: p.BaseURL,
		APIKey:  apiKey,
	}), keyID, scope, true
}

func (r *ModelRouter) refreshRegistry(ctx context.Context) error {
	models, err := r.store.GetModels(ctx)
	if err != nil {
		return fmt.Errorf("load models: %w", err)
	}

	newModels := make(map[string]*ModelEntry)
	for _, m := range models {
		bindings, err := r.store.GetModelProviders(ctx, m.ModelID)
		if err != nil {
			slog.Warn("failed to load model bindings", "model", m.ModelID, "error", err)
			continue
		}

		entry := &ModelEntry{
			ModelID:     m.ModelID,
			Modality:    m.Modality,
			DisplayName: m.DisplayName,
			IsAsync:     m.IsAsync,
			Providers:   bindings,
		}
		newModels[m.ModelID] = entry
	}

	r.mu.Lock()
	r.models = newModels
	r.mu.Unlock()

	slog.Info("model registry refreshed", "count", len(newModels))
	return nil
}

func (r *ModelRouter) healthCheckLoop(ctx context.Context) {
	r.runHealthChecks(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runHealthChecks(ctx)
		}
	}
}

func (r *ModelRouter) runHealthChecks(ctx context.Context) {
	for providerID, adapter := range r.adapters {
		if providerID == "mock" {
			continue
		}
		start := time.Now()
		err := adapter.HealthCheck(ctx)
		status := "healthy"
		errorMessage := ""
		if err != nil {
			status = "unhealthy"
			errorMessage = err.Error()
			slog.Warn("provider health check failed", "provider", providerID, "error", err)
		}
		latencyMs := int(time.Since(start).Milliseconds())

		// Update health status in memory
		r.mu.Lock()
		for _, entry := range r.models {
			for i := range entry.Providers {
				if entry.Providers[i].ProviderID == providerID {
					entry.Providers[i].HealthStatus = status
					entry.Providers[i].LastHealthChk = time.Now()
				}
			}
		}
		r.mu.Unlock()

		// Update in DB (async, non-blocking)
		go func(pid, s, msg string, ms int) {
			if err := r.store.RecordProviderHealthCheck(ctx, pid, s, ms, "", msg); err != nil {
				slog.Warn("failed to record provider health check", "provider", pid, "error", err)
			}
		}(providerID, status, errorMessage, latencyMs)
	}
}

func (r *ModelRouter) refreshLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.refreshCh:
			if err := r.refreshRegistry(ctx); err != nil {
				slog.Error("registry refresh failed", "error", err)
			}
		}
	}
}

func (r *ModelRouter) resolveProviderKey(p *storage.Provider) string {
	return r.resolveProviderKeyForRequest(context.Background(), p, "", "")
}

func (r *ModelRouter) resolveProviderKeyForRequest(ctx context.Context, p *storage.Provider, userID, workspaceID string) string {
	secret, _, _ := r.resolveProviderKeyForRequestWithMetadata(ctx, p, userID, workspaceID)
	return secret
}

func (r *ModelRouter) resolveProviderKeyForRequestWithMetadata(ctx context.Context, p *storage.Provider, userID, workspaceID string) (string, string, string) {
	if secret, keyID, scope, err := r.store.GetScopedProviderKeySecretWithMetadata(ctx, p.ID, userID, workspaceID); err == nil && secret != "" {
		return secret, keyID, scope
	} else if err != nil {
		slog.Warn("failed to load provider credential from database", "provider", p.ID, "error", err)
	}
	secret := r.resolveProviderKeyFromEnv(p)
	if secret == "" {
		return "", "", ""
	}
	return secret, "", "env"
}

func (r *ModelRouter) resolveProviderKeyFromEnv(p *storage.Provider) string {
	// Resolution order (2-tier fallback):
	// 1. Region-specific key (DASHSCOPE_API_KEY_CN / DASHSCOPE_API_KEY_INTL)
	// 2. Universal key (DASHSCOPE_API_KEY)
	switch p.ID {
	case "bailian_cn":
		if v := os.Getenv("DASHSCOPE_API_KEY_CN"); v != "" {
			return v
		}
		return os.Getenv("DASHSCOPE_API_KEY")
	case "bailian_intl":
		if v := os.Getenv("DASHSCOPE_API_KEY_INTL"); v != "" {
			return v
		}
		return os.Getenv("DASHSCOPE_API_KEY")
	default:
		// For unknown providers, try the universal key as a fallback.
		return os.Getenv("DASHSCOPE_API_KEY")
	}
}
