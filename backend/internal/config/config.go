package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server
	Env     string // "development" | "production"
	Host    string
	Port    int
	BaseURL string

	// Database
	DatabaseURL   string
	RedisURL      string
	ClickHouseURL string

	// Auth
	JWTSecret    string
	APIKeyPrefix string

	// Mock Mode
	MockProviderMode bool // when true, all providers route through a mock adapter

	// Provider Keys
	DashScopeKey     string // universal key (DASHSCOPE_API_KEY)
	DashScopeKeyCN   string
	DashScopeKeyINTL string
	DashScopeEP_CN   string
	DashScopeEP_INTL string

	// OSS
	OSSAccessKeyID     string
	OSSAccessKeySecret string
	OSSBucket          string
	OSSEndpoint        string
	FileStorageBackend string
	FileStorageDir     string

	// Rate Limits
	DefaultRPM        int
	DefaultTPM        int
	DefaultDailyQuota float64

	// Task Engine
	TaskWorkerCount int
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("APP_PORT", "8080"))
	rpm, _ := strconv.Atoi(getEnv("DEFAULT_RPM", "60"))
	tpm, _ := strconv.Atoi(getEnv("DEFAULT_TPM", "100000"))
	quota, _ := strconv.ParseFloat(getEnv("DEFAULT_DAILY_QUOTA_USD", "100.00"), 64)
	workers, _ := strconv.Atoi(getEnv("TASK_WORKER_COUNT", "10"))

	cfg := &Config{
		Env:     getEnv("APP_ENV", "development"),
		Host:    getEnv("APP_HOST", "0.0.0.0"),
		Port:    port,
		BaseURL: getEnv("APP_BASE_URL", "http://localhost:8080"),

		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),
		ClickHouseURL: getEnv("CLICKHOUSE_URL", ""),

		JWTSecret:    getEnv("JWT_SECRET", ""),
		APIKeyPrefix: getEnv("API_KEY_PREFIX", "sk-aggr-"),

		DashScopeKey:     getEnv("DASHSCOPE_API_KEY", ""),
		DashScopeKeyCN:   getEnv("DASHSCOPE_API_KEY_CN", ""),
		DashScopeKeyINTL: getEnv("DASHSCOPE_API_KEY_INTL", ""),
		DashScopeEP_CN:   getEnv("DASHSCOPE_ENDPOINT_CN", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		DashScopeEP_INTL: getEnv("DASHSCOPE_ENDPOINT_INTL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"),

		OSSAccessKeyID:     getEnv("OSS_ACCESS_KEY_ID", ""),
		OSSAccessKeySecret: getEnv("OSS_ACCESS_KEY_SECRET", ""),
		OSSBucket:          getEnv("OSS_BUCKET", "aggr-dev-artifacts"),
		OSSEndpoint:        getEnv("OSS_ENDPOINT", "oss-ap-southeast-5.aliyuncs.com"),
		FileStorageBackend: strings.ToLower(getEnv("FILE_STORAGE_BACKEND", "local")),
		FileStorageDir:     getEnv("FILE_STORAGE_DIR", "/tmp/ai-aggregator-files"),

		DefaultRPM:        rpm,
		DefaultTPM:        tpm,
		DefaultDailyQuota: quota,
		TaskWorkerCount:   workers,
	}

	// Parse mock provider mode from env (accepts "true", "1", "yes")
	cfg.MockProviderMode = parseBoolEnv(getEnv("MOCK_PROVIDER_MODE", "false"))

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" && cfg.Env == "production" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBoolEnv(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "true" || v == "1" || v == "yes"
}
