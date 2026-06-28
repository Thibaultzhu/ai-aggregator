package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-aggregator/internal/config"
	"ai-aggregator/internal/filestore"
	"ai-aggregator/internal/gateway"
	"ai-aggregator/internal/metrics"
	"ai-aggregator/internal/router"
	"ai-aggregator/internal/storage"
	"ai-aggregator/internal/task"
)

func main() {
	// ===== Load Config =====
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// ===== Init Logger =====
	logLevel := slog.LevelInfo
	if cfg.Env == "development" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// ===== Init Storage =====
	ctx := context.Background()

	pg, err := storage.NewPgPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	rdb, err := storage.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect redis", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	store := storage.NewStore(ctx, pg, rdb)

	// ===== Init Model Router =====
	modelRouter, err := router.NewModelRouter(ctx, store, cfg.MockProviderMode)
	if err != nil {
		slog.Error("failed to init model router", "error", err)
		os.Exit(1)
	}

	// ===== Init Task Engine =====
	taskEngine := task.NewEngine(store, modelRouter, task.Config{
		WorkerCount:    cfg.TaskWorkerCount,
		PollInterval:   5 * time.Second,
		DefaultTimeout: 5 * time.Minute,
	})
	taskEngine.Start(ctx)
	defer taskEngine.Stop()

	// ===== Init Metrics =====
	metricsCollector := metrics.NewCollector()
	if cfg.FileStorageBackend != "" && cfg.FileStorageBackend != "local" {
		slog.Warn("unsupported FILE_STORAGE_BACKEND; falling back to local file store", "backend", cfg.FileStorageBackend)
	}
	fileStore := filestore.NewLocalStore(cfg.FileStorageDir)

	// ===== Init Gateway =====
	gw := gateway.New(gateway.Deps{
		Config:    cfg,
		Store:     store,
		Router:    modelRouter,
		Tasks:     taskEngine,
		Metrics:   metricsCollector,
		FileStore: fileStore,
		Logger:    logger,
	})

	// ===== Start Server =====
	app := gw.Start(cfg.Host, cfg.Port)

	// ===== Graceful Shutdown =====
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	if err := app.Shutdown(); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server exited")
}
