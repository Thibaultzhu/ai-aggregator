package task

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ai-aggregator/internal/provider"
	"ai-aggregator/internal/router"
	"ai-aggregator/internal/storage"
)

// Config holds task engine configuration.
type Config struct {
	WorkerCount    int
	PollInterval   time.Duration
	DefaultTimeout time.Duration
}

// Engine manages async tasks (image/video generation).
type Engine struct {
	store   *storage.Store
	router  *router.ModelRouter
	config  Config
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

func NewEngine(store *storage.Store, router *router.ModelRouter, cfg Config) *Engine {
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = 10
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 5 * time.Minute
	}
	return &Engine{
		store:  store,
		router: router,
		config: cfg,
	}
}

// Start launches the worker pool.
func (e *Engine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)

	for i := 0; i < e.config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}

	slog.Info("task engine started", "workers", e.config.WorkerCount)
}

// Stop gracefully shuts down the worker pool.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	slog.Info("task engine stopped")
}

// Submit creates a new async task and enqueues it for processing.
func (e *Engine) Submit(ctx context.Context, userID, modelID string, params map[string]interface{}) (string, error) {
	// 1. Generate external task ID
	externalID := generateTaskID()

	// 2. Route to provider
	adapter, upstreamModel, err := e.router.Route(ctx, modelID)
	if err != nil {
		return "", err
	}

	// 3. Submit to upstream (image or video)
	var upstreamTaskID string
	modality := e.getModelModality(modelID)

	switch modality {
	case "image":
		// Parse params into ImageRequest
		result, err := adapter.CreateImage(ctx, paramsToImageRequest(params, upstreamModel))
		if err != nil {
			return "", err
		}
		upstreamTaskID = result.ID

	case "video":
		result, err := adapter.CreateVideo(ctx, paramsToVideoRequest(params, upstreamModel))
		if err != nil {
			return "", err
		}
		upstreamTaskID = result.ID

	default:
		return "", &provider.ErrUnsupportedModality{Provider: adapter.Name(), Op: modality}
	}

	// 4. Store task in DB
	record := &storage.TaskRecord{
		ExternalID:     externalID,
		UserID:         userID,
		ModelID:        modelID,
		ProviderID:     adapter.Name(),
		UpstreamTaskID: upstreamTaskID,
		Status:         "submitted",
		RequestParams:  params,
	}
	if err := e.store.CreateTask(ctx, record); err != nil {
		return "", err
	}

	return externalID, nil
}

// GetResult returns the current status of a task.
func (e *Engine) GetResult(ctx context.Context, externalID string) (*storage.TaskRecord, error) {
	return e.store.GetTask(ctx, externalID)
}

// ===== Worker =====

func (e *Engine) worker(ctx context.Context, id int) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.pollTasks(ctx, id)
		}
	}
}

func (e *Engine) pollTasks(ctx context.Context, workerID int) {
	tasks, err := e.store.GetPendingTasks(ctx, 5)
	if err != nil {
		slog.Error("failed to get pending tasks", "worker", workerID, "error", err)
		return
	}

	for _, task := range tasks {
		adapter, ok := e.router.GetAdapter(task.ProviderID)
		if !ok {
			slog.Warn("adapter not found for task", "task_id", task.ExternalID, "provider", task.ProviderID)
			continue
		}

		result, err := adapter.PollTask(ctx, task.UpstreamTaskID)
		if err != nil {
			slog.Warn("poll task failed", "task_id", task.ExternalID, "error", err)
			continue
		}

		if result.Status == "completed" || result.Status == "failed" {
			var resultData map[string]interface{}
			if result.Data != nil {
				// TODO: convert result.Data to map
			}
			if err := e.store.UpdateTaskStatus(ctx, task.ExternalID, result.Status, resultData); err != nil {
				slog.Error("failed to update task", "task_id", task.ExternalID, "error", err)
			} else {
				slog.Info("task completed", "task_id", task.ExternalID, "status", result.Status)
			}
		}
	}
}

// ===== Helpers =====

func generateTaskID() string {
	// TODO: "task_" + random 16 hex chars
	return "task_" + time.Now().Format("20060102150405")
}

func (e *Engine) getModelModality(modelID string) string {
	if m, ok := e.router.GetModel(modelID); ok {
		return m.Modality
	}
	return "unknown"
}

func paramsToImageRequest(params map[string]interface{}, upstreamModel string) *provider.ImageRequest {
	req := &provider.ImageRequest{Model: upstreamModel}
	if v, ok := params["prompt"].(string); ok {
		req.Prompt = v
	}
	if v, ok := params["n"].(float64); ok {
		req.N = int(v)
	}
	if v, ok := params["size"].(string); ok {
		req.Size = v
	}
	return req
}

func paramsToVideoRequest(params map[string]interface{}, upstreamModel string) *provider.VideoRequest {
	req := &provider.VideoRequest{Model: upstreamModel}
	if v, ok := params["prompt"].(string); ok {
		req.Prompt = v
	}
	if v, ok := params["image_url"].(string); ok {
		req.ImageURL = v
	}
	if v, ok := params["duration"].(float64); ok {
		req.Duration = int(v)
	}
	if v, ok := params["resolution"].(string); ok {
		req.Resolution = v
	}
	return req
}
