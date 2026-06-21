package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// ===== Store =====

// Store is the unified data access layer combining PostgreSQL and Redis.
type Store struct {
	pg    *pgxpool.Pool
	redis *goredis.Client
}

// NewStore creates a new Store backed by the given pgx pool and Redis client.
func NewStore(ctx context.Context, pgPool *pgxpool.Pool, rdb *goredis.Client) *Store {
	slog.Info("storage layer initialized")
	return &Store{pg: pgPool, redis: rdb}
}

// Close shuts down both the PostgreSQL pool and Redis client.
func (s *Store) Close() {
	if s.pg != nil {
		s.pg.Close()
		slog.Info("postgres pool closed")
	}
	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			slog.Error("failed to close redis", "error", err)
		} else {
			slog.Info("redis client closed")
		}
	}
}

// Redis returns the underlying Redis client for direct use (e.g., rate limiting).
func (s *Store) Redis() *goredis.Client {
	return s.redis
}

// GetEnvVar retrieves an environment variable. Used by the model router to resolve
// provider API keys from the environment.
func (s *Store) GetEnvVar(key string) string {
	return os.Getenv(key)
}

// ===== Connection Factories =====

// NewPgPool creates a connection pool for PostgreSQL using pgx/v5.
func NewPgPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pg: %w", err)
	}

	slog.Info("postgres pool connected", "max_conns", cfg.MaxConns)
	return pool, nil
}

// NewRedisClient parses a Redis URL and establishes a connection using go-redis/v9.
func NewRedisClient(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	opts.PoolSize = 20
	opts.MinIdleConns = 5
	opts.ConnMaxIdleTime = 5 * time.Minute

	client := goredis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	slog.Info("redis connected", "addr", opts.Addr, "db", opts.DB)
	return client, nil
}

// ===== Provider Types =====

// Provider represents an upstream AI provider (e.g., DashScope, OpenAI-compatible endpoint).
type Provider struct {
	ID          string
	DisplayName string
	AdapterType string
	BaseURL     string
	Config      map[string]interface{}
	IsEnabled   bool
}

// GetProviders returns all enabled providers from the database.
func (s *Store) GetProviders(ctx context.Context) ([]*Provider, error) {
	query := `
		SELECT id, display_name, adapter_type, base_url, config, is_enabled
		FROM providers
		WHERE is_enabled = true
		ORDER BY id`

	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		var configBytes []byte
		if err := rows.Scan(
			&p.ID,
			&p.DisplayName,
			&p.AdapterType,
			&p.BaseURL,
			&configBytes,
			&p.IsEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &p.Config); err != nil {
				slog.Warn("failed to unmarshal provider config", "provider_id", p.ID, "error", err)
				p.Config = make(map[string]interface{})
			}
		} else {
			p.Config = make(map[string]interface{})
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}

	return providers, nil
}

// UpdateProviderHealth updates the health status and last check timestamp for all
// model-provider bindings associated with the given provider.
func (s *Store) UpdateProviderHealth(ctx context.Context, providerID, status string) error {
	query := `
		UPDATE model_providers
		SET health_status = $1, last_health_chk = now()
		WHERE provider_id = $2`

	tag, err := s.pg.Exec(ctx, query, status, providerID)
	if err != nil {
		return fmt.Errorf("update provider health: %w", err)
	}

	slog.Debug("provider health updated", "provider_id", providerID, "status", status, "rows", tag.RowsAffected())
	return nil
}

// ===== Model Types =====

// Model represents an AI model available for routing.
type Model struct {
	ModelID        string    `json:"model_id"`
	DisplayName    string    `json:"display_name"`
	Modality       string    `json:"modality"`
	IsAsync        bool      `json:"is_async"`
	InputPrice     *float64  `json:"input_price"`
	OutputPrice    *float64  `json:"output_price"`
	PriceUnit      string    `json:"price_unit"`
	MaxContext     *int      `json:"max_context"`
	MaxOutput      *int      `json:"max_output"`
	SupportsStream bool      `json:"supports_stream"`
	Status         string    `json:"status"`
}

// GetModels returns all active models from the database with full pricing information.
func (s *Store) GetModels(ctx context.Context) ([]*Model, error) {
	query := `
		SELECT model_id, display_name, modality, is_async,
		       input_price, output_price,
		       price_unit, max_context, max_output, supports_stream, status
		FROM models
		WHERE status = 'active'
		ORDER BY model_id`

	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query models: %w", err)
	}
	defer rows.Close()

	var models []*Model
	for rows.Next() {
		m := &Model{}
		if err := rows.Scan(
			&m.ModelID,
			&m.DisplayName,
			&m.Modality,
			&m.IsAsync,
			&m.InputPrice,
			&m.OutputPrice,
			&m.PriceUnit,
			&m.MaxContext,
			&m.MaxOutput,
			&m.SupportsStream,
			&m.Status,
		); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate models: %w", err)
	}

	return models, nil
}

// GetModelPricing returns the input_price, output_price, and price_unit for a given model.
// Returns nil values if the model doesn't exist or pricing is not set.
func (s *Store) GetModelPricing(ctx context.Context, modelID string) (inputPrice, outputPrice *float64, priceUnit string, err error) {
	query := `
		SELECT input_price, output_price, price_unit
		FROM models
		WHERE model_id = $1 AND status = 'active'`

	err = s.pg.QueryRow(ctx, query, modelID).Scan(&inputPrice, &outputPrice, &priceUnit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("query model pricing: %w", err)
	}

	return inputPrice, outputPrice, priceUnit, nil
}

// ProviderBinding represents a model-to-provider routing binding.
type ProviderBinding struct {
	ProviderID     string
	Priority       int
	UpstreamModel  string
	CostMultiplier float64
	TimeoutMs      int
	MaxRetries     int
	IsEnabled      bool
	HealthStatus   string
	LastHealthChk  time.Time
}

// GetModelProviders returns all enabled provider bindings for a given model,
// joining with the providers table to ensure the provider itself is also enabled.
func (s *Store) GetModelProviders(ctx context.Context, modelID string) ([]ProviderBinding, error) {
	query := `
		SELECT mp.provider_id, mp.priority, mp.upstream_model, mp.cost_multiplier,
		       mp.timeout_ms, mp.max_retries, mp.is_enabled,
		       COALESCE(mp.health_status, 'unknown'),
		       COALESCE(mp.last_health_chk, '1970-01-01'::timestamptz)
		FROM model_providers mp
		JOIN providers p ON p.id = mp.provider_id
		WHERE mp.model_id = $1
		  AND mp.is_enabled = true
		  AND p.is_enabled = true
		ORDER BY mp.priority ASC`

	rows, err := s.pg.Query(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("query model providers: %w", err)
	}
	defer rows.Close()

	var bindings []ProviderBinding
	for rows.Next() {
		var b ProviderBinding
		if err := rows.Scan(
			&b.ProviderID,
			&b.Priority,
			&b.UpstreamModel,
			&b.CostMultiplier,
			&b.TimeoutMs,
			&b.MaxRetries,
			&b.IsEnabled,
			&b.HealthStatus,
			&b.LastHealthChk,
		); err != nil {
			return nil, fmt.Errorf("scan model provider binding: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model providers: %w", err)
	}

	return bindings, nil
}

// ===== Usage Recording =====

// UsageRecord holds all fields for recording an API call's usage data.
type UsageRecord struct {
	RequestID    string    `json:"request_id"`
	UserID       string    `json:"user_id"`
	APIKeyID     string    `json:"api_key_id"`
	ModelID      string    `json:"model_id"`
	ProviderID   string    `json:"provider_id"`
	Modality     string    `json:"modality"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	LatencyMs    int       `json:"latency_ms"`
	TTFTMs       int       `json:"ttft_ms"`
	IsStream     bool      `json:"is_stream"`
	UpstreamCost float64   `json:"upstream_cost_usd"`
	ChargedCost  float64   `json:"charged_cost_usd"`
	StatusCode   int       `json:"status_code"`
	ErrorType    string    `json:"error_type,omitempty"`
	IsCached     bool      `json:"is_cached"`
	Region       string    `json:"region,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RecordUsage inserts a usage log entry into the usage_logs table.
func (s *Store) RecordUsage(ctx context.Context, record *UsageRecord) error {
	totalTokens := record.InputTokens + record.OutputTokens

	query := `
		INSERT INTO usage_logs (
			request_id, user_id, api_key_id, model_id, provider_id, modality,
			input_tokens, output_tokens, total_tokens, latency_ms, ttft_ms, is_stream,
			upstream_cost_usd, charged_cost_usd, status_code, error_type,
			is_cached, region
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18
		)`

	_, err := s.pg.Exec(ctx, query,
		record.RequestID,
		record.UserID,
		record.APIKeyID,
		record.ModelID,
		record.ProviderID,
		record.Modality,
		record.InputTokens,
		record.OutputTokens,
		totalTokens,
		record.LatencyMs,
		record.TTFTMs,
		record.IsStream,
		record.UpstreamCost,
		record.ChargedCost,
		record.StatusCode,
		record.ErrorType,
		record.IsCached,
		record.Region,
	)
	if err != nil {
		return fmt.Errorf("insert usage log: %w", err)
	}

	slog.Debug("usage recorded",
		"request_id", record.RequestID,
		"user_id", record.UserID,
		"model", record.ModelID,
		"charged", record.ChargedCost,
	)
	return nil
}

// GetUsageLogs returns the most recent usage records for a given user.
func (s *Store) GetUsageLogs(ctx context.Context, userID string, limit int) ([]UsageRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT request_id, user_id, api_key_id, model_id, provider_id, modality,
		       input_tokens, output_tokens, latency_ms, ttft_ms, is_stream,
		       upstream_cost_usd, charged_cost_usd, status_code, error_type,
		       is_cached, region, created_at
		FROM usage_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := s.pg.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query usage logs: %w", err)
	}
	defer rows.Close()

	records := make([]UsageRecord, 0)
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(
			&r.RequestID,
			&r.UserID,
			&r.APIKeyID,
			&r.ModelID,
			&r.ProviderID,
			&r.Modality,
			&r.InputTokens,
			&r.OutputTokens,
			&r.LatencyMs,
			&r.TTFTMs,
			&r.IsStream,
			&r.UpstreamCost,
			&r.ChargedCost,
			&r.StatusCode,
			&r.ErrorType,
			&r.IsCached,
			&r.Region,
			&r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage log: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage logs: %w", err)
	}

	return records, nil
}

// GetUsageSummary returns aggregate usage statistics for a given user.
func (s *Store) GetUsageSummary(ctx context.Context, userID string) (totalRequests int64, totalCost float64, totalTokens int64, err error) {
	query := `
		SELECT
			COALESCE(COUNT(*), 0),
			COALESCE(SUM(charged_cost_usd), 0),
			COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM usage_logs
		WHERE user_id = $1`

	err = s.pg.QueryRow(ctx, query, userID).Scan(&totalRequests, &totalCost, &totalTokens)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("query usage summary: %w", err)
	}

	return totalRequests, totalCost, totalTokens, nil
}

// ===== User CRUD =====

// CreateUser inserts a new user record and returns the generated UUID.
func (s *Store) CreateUser(ctx context.Context, email, username, passwordHash string) (string, error) {
	query := `
		INSERT INTO users (email, username, password_hash, role, balance_usd, created_at)
		VALUES ($1, $2, $3, 'user', 0, now())
		RETURNING id`

	var userID string
	err := s.pg.QueryRow(ctx, query, email, username, passwordHash).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("insert user: %w", err)
	}

	slog.Info("user created", "user_id", userID, "email", email)
	return userID, nil
}

// GetUserByEmail looks up a user by email address, returning credentials and role for login.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (userID, username, passwordHash, role string, err error) {
	query := `SELECT id, username, password_hash, role FROM users WHERE email = $1`

	err = s.pg.QueryRow(ctx, query, email).Scan(&userID, &username, &passwordHash, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", "", fmt.Errorf("user not found: %s", email)
		}
		return "", "", "", "", fmt.Errorf("query user by email: %w", err)
	}

	return userID, username, passwordHash, role, nil
}

// GetUserByID retrieves a user's profile information and balance by their ID.
func (s *Store) GetUserByID(ctx context.Context, userID string) (email, username, role string, balance float64, err error) {
	query := `SELECT email, username, role, balance_usd FROM users WHERE id = $1`

	err = s.pg.QueryRow(ctx, query, userID).Scan(&email, &username, &role, &balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", 0, fmt.Errorf("user not found: %s", userID)
		}
		return "", "", "", 0, fmt.Errorf("query user by id: %w", err)
	}

	return email, username, role, balance, nil
}

// GetUserBalance returns the current USD balance for a user.
func (s *Store) GetUserBalance(ctx context.Context, userID string) (float64, error) {
	query := `SELECT balance_usd FROM users WHERE id = $1`

	var balance float64
	err := s.pg.QueryRow(ctx, query, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("user not found: %s", userID)
		}
		return 0, fmt.Errorf("query user balance: %w", err)
	}

	return balance, nil
}

// DeductBalance subtracts the given amount from a user's balance. The update is
// conditional: it only succeeds if the user has sufficient funds (balance >= amount).
// Returns an error if the balance is insufficient or the user does not exist.
func (s *Store) DeductBalance(ctx context.Context, userID string, amount float64) error {
	query := `
		UPDATE users
		SET balance_usd = balance_usd - $1
		WHERE id = $2 AND balance_usd >= $1`

	tag, err := s.pg.Exec(ctx, query, amount, userID)
	if err != nil {
		return fmt.Errorf("deduct balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insufficient balance for user %s (attempted deduction: %.6f)", userID, amount)
	}

	slog.Debug("balance deducted", "user_id", userID, "amount", amount)
	return nil
}

// AddBalance increases a user's balance by the given amount. Used for top-ups
// and initial credit grants.
func (s *Store) AddBalance(ctx context.Context, userID string, amount float64) error {
	query := `
		UPDATE users
		SET balance_usd = balance_usd + $1
		WHERE id = $2`

	tag, err := s.pg.Exec(ctx, query, amount, userID)
	if err != nil {
		return fmt.Errorf("add balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	slog.Info("balance added", "user_id", userID, "amount", amount)
	return nil
}

// ===== API Key CRUD =====

// APIKeyInfo holds metadata about an API key (never includes the hash).
type APIKeyInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	KeyPrefix   string          `json:"key_prefix"`
	Permissions json.RawMessage `json:"permissions"`
	IsActive    bool            `json:"is_active"`
	LastUsedAt  *time.Time      `json:"last_used_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// CreateAPIKey inserts a new API key record and returns its generated ID.
// If permissions is nil, defaults to {"models":"*"} (access all models).
func (s *Store) CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix string, permissions json.RawMessage) (string, error) {
	if permissions == nil {
		permissions = json.RawMessage(`{"models":"*"}`)
	}

	query := `
		INSERT INTO api_keys (user_id, name, key_hash, key_prefix, permissions, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, true, now())
		RETURNING id`

	var keyID string
	err := s.pg.QueryRow(ctx, query, userID, name, keyHash, keyPrefix, permissions).Scan(&keyID)
	if err != nil {
		return "", fmt.Errorf("insert api key: %w", err)
	}

	slog.Info("api key created", "key_id", keyID, "user_id", userID, "name", name)
	return keyID, nil
}

// ValidateAPIKey looks up an API key by its SHA-256 hash and checks that it is active.
// Returns the owning user's ID, the key ID, and its permissions JSON.
func (s *Store) ValidateAPIKey(ctx context.Context, keyHash string) (userID, keyID string, perms json.RawMessage, err error) {
	query := `
		SELECT user_id, id, permissions
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true`

	err = s.pg.QueryRow(ctx, query, keyHash).Scan(&userID, &keyID, &perms)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, fmt.Errorf("api key not found or inactive")
		}
		return "", "", nil, fmt.Errorf("validate api key: %w", err)
	}

	// Update last_used_at asynchronously (non-blocking, best-effort).
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.pg.Exec(updateCtx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, keyID)
	}()

	return userID, keyID, perms, nil
}

// ListAPIKeys returns all API keys belonging to a user, without exposing the key hash.
func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]APIKeyInfo, error) {
	query := `
		SELECT id, name, key_prefix, permissions, is_active, last_used_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := s.pg.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	keys := make([]APIKeyInfo, 0)
	for rows.Next() {
		var k APIKeyInfo
		if err := rows.Scan(
			&k.ID,
			&k.Name,
			&k.KeyPrefix,
			&k.Permissions,
			&k.IsActive,
			&k.LastUsedAt,
			&k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}

	return keys, nil
}

// RevokeAPIKey deactivates an API key by setting is_active = false.
// The key is scoped to the user to prevent cross-user revocation.
func (s *Store) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	query := `
		UPDATE api_keys
		SET is_active = false
		WHERE id = $1 AND user_id = $2`

	tag, err := s.pg.Exec(ctx, query, keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api key not found or already revoked: %s", keyID)
	}

	slog.Info("api key revoked", "key_id", keyID, "user_id", userID)
	return nil
}

// ===== Billing =====

// CreateBillingTransaction records a billing event (top-up, deduction, refund, etc.)
// in the billing_transactions table. It uses a database transaction to atomically
// insert the transaction record and capture the user's balance after the transaction.
func (s *Store) CreateBillingTransaction(ctx context.Context, userID string, amountUSD float64, txType string, description string) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for billing transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert the billing transaction and get its ID back atomically.
	var txID string
	err = tx.QueryRow(ctx, `
		INSERT INTO billing_transactions (user_id, amount_usd, tx_type, description, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id`,
		userID, amountUSD, txType, description,
	).Scan(&txID)
	if err != nil {
		return fmt.Errorf("insert billing transaction: %w", err)
	}

	// Query the user's current balance after the transaction to record it.
	var balanceAfter float64
	err = tx.QueryRow(ctx, `SELECT balance_usd FROM users WHERE id = $1`, userID).Scan(&balanceAfter)
	if err != nil {
		return fmt.Errorf("query balance after transaction: %w", err)
	}

	// Update this specific transaction (by id) with the balance snapshot — safe under concurrency.
	_, err = tx.Exec(ctx, `
		UPDATE billing_transactions SET balance_after_usd = $1 WHERE id = $2`,
		balanceAfter, txID,
	)
	if err != nil {
		return fmt.Errorf("update balance_after_usd: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit billing transaction: %w", err)
	}

	slog.Info("billing transaction recorded", "user_id", userID, "amount_usd", amountUSD, "type", txType, "balance_after_usd", balanceAfter)
	return nil
}

// BillingTransaction represents a billing event record from the billing_transactions table.
type BillingTransaction struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	AmountUSD       float64   `json:"amount_usd"`
	BalanceAfterUSD *float64  `json:"balance_after_usd"`
	TxType          string    `json:"tx_type"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

// GetBillingTransactions returns the most recent billing transactions for a given user,
// ordered by creation time descending.
func (s *Store) GetBillingTransactions(ctx context.Context, userID string, limit int) ([]BillingTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT id, user_id, amount_usd, balance_after_usd, tx_type, description, created_at
		FROM billing_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := s.pg.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query billing transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]BillingTransaction, 0)
	for rows.Next() {
		var bt BillingTransaction
		if err := rows.Scan(
			&bt.ID,
			&bt.UserID,
			&bt.AmountUSD,
			&bt.BalanceAfterUSD,
			&bt.TxType,
			&bt.Description,
			&bt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan billing transaction: %w", err)
		}
		transactions = append(transactions, bt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate billing transactions: %w", err)
	}

	return transactions, nil
}

// ===== System Settings =====

const settingCachePrefix = "setting:"
const settingCacheTTL = 30 * time.Second

// GetSetting retrieves a system setting by key, using Redis as a short-lived cache
// (30-second TTL) to avoid repeated database lookups for hot-path reads.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	cacheKey := settingCachePrefix + key

	// Try Redis cache first.
	if s.redis != nil {
		cached, err := s.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			return cached, nil
		}
		// On cache miss or Redis error, fall through to DB.
	}

	// Fetch from PostgreSQL.
	query := `SELECT value FROM system_settings WHERE key = $1`

	var value string
	err := s.pg.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("setting not found: %s", key)
		}
		return "", fmt.Errorf("query setting: %w", err)
	}

	// Populate cache (best-effort).
	if s.redis != nil {
		if setErr := s.redis.Set(ctx, cacheKey, value, settingCacheTTL).Err(); setErr != nil {
			slog.Warn("failed to cache setting", "key", key, "error", setErr)
		}
	}

	return value, nil
}

// SetSetting updates a system setting in PostgreSQL and invalidates the Redis cache entry.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	query := `
		INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()`

	_, err := s.pg.Exec(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("upsert setting: %w", err)
	}

	// Invalidate cache.
	if s.redis != nil {
		cacheKey := settingCachePrefix + key
		if delErr := s.redis.Del(ctx, cacheKey).Err(); delErr != nil {
			slog.Warn("failed to invalidate setting cache", "key", key, "error", delErr)
		}
	}

	slog.Info("setting updated", "key", key)
	return nil
}

// ===== Rate Limit / Quota Helpers =====

// GetDailyUsage returns the total USD charged to a user since the start of the
// current day (UTC). Used by the billing engine for daily quota enforcement.
func (s *Store) GetDailyUsage(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(charged_cost_usd), 0)
		FROM usage_logs
		WHERE user_id = $1
		  AND created_at >= CURRENT_DATE`

	var total float64
	err := s.pg.QueryRow(ctx, query, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("query daily usage: %w", err)
	}

	return total, nil
}

// ===== Async Tasks =====

// TaskRecord represents an asynchronous task (e.g., image/video generation).
type TaskRecord struct {
	ExternalID     string
	UserID         string
	ModelID        string
	ProviderID     string
	UpstreamTaskID string
	Status         string
	RequestParams  map[string]interface{}
	ResultData     map[string]interface{}
	CostUSD        float64
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// CreateTask inserts a new async task record.
func (s *Store) CreateTask(ctx context.Context, task *TaskRecord) error {
	paramsJSON, err := json.Marshal(task.RequestParams)
	if err != nil {
		return fmt.Errorf("marshal request params: %w", err)
	}

	query := `
		INSERT INTO async_tasks (
			external_id, user_id, model_id, provider_id, upstream_task_id,
			status, request_params, cost_usd, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())`

	_, err = s.pg.Exec(ctx, query,
		task.ExternalID,
		task.UserID,
		task.ModelID,
		task.ProviderID,
		task.UpstreamTaskID,
		task.Status,
		paramsJSON,
		task.CostUSD,
	)
	if err != nil {
		return fmt.Errorf("insert async task: %w", err)
	}

	slog.Info("async task created", "external_id", task.ExternalID, "model", task.ModelID)
	return nil
}

// UpdateTaskStatus updates the status and result of an async task.
// If result is non-nil, it is serialized as JSON and stored in result_data.
// Sets completed_at when the status is terminal (completed or failed).
func (s *Store) UpdateTaskStatus(ctx context.Context, externalID, status string, result map[string]interface{}) error {
	var resultJSON []byte
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result data: %w", err)
		}
	}

	query := `
		UPDATE async_tasks
		SET status = $1,
		    result_data = COALESCE($2::jsonb, result_data),
		    completed_at = CASE
		        WHEN $1 IN ('completed', 'failed') THEN now()
		        ELSE completed_at
		    END
		WHERE external_id = $3`

	tag, err := s.pg.Exec(ctx, query, status, resultJSON, externalID)
	if err != nil {
		return fmt.Errorf("update async task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("async task not found: %s", externalID)
	}

	slog.Debug("async task updated", "external_id", externalID, "status", status)
	return nil
}

// GetTask retrieves an async task by its external ID.
func (s *Store) GetTask(ctx context.Context, externalID string) (*TaskRecord, error) {
	query := `
		SELECT external_id, user_id, model_id, provider_id, upstream_task_id,
		       status, request_params, result_data, cost_usd, created_at, completed_at
		FROM async_tasks
		WHERE external_id = $1`

	task := &TaskRecord{}
	var paramsBytes, resultBytes []byte

	err := s.pg.QueryRow(ctx, query, externalID).Scan(
		&task.ExternalID,
		&task.UserID,
		&task.ModelID,
		&task.ProviderID,
		&task.UpstreamTaskID,
		&task.Status,
		&paramsBytes,
		&resultBytes,
		&task.CostUSD,
		&task.CreatedAt,
		&task.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("task not found: %s", externalID)
		}
		return nil, fmt.Errorf("query async task: %w", err)
	}

	if len(paramsBytes) > 0 {
		if err := json.Unmarshal(paramsBytes, &task.RequestParams); err != nil {
			slog.Warn("failed to unmarshal task request_params", "external_id", externalID, "error", err)
		}
	}
	if len(resultBytes) > 0 {
		if err := json.Unmarshal(resultBytes, &task.ResultData); err != nil {
			slog.Warn("failed to unmarshal task result_data", "external_id", externalID, "error", err)
		}
	}

	return task, nil
}

// GetPendingTasks returns async tasks that are still in progress, ordered by
// creation time. Used by the task engine's polling workers.
func (s *Store) GetPendingTasks(ctx context.Context, limit int) ([]*TaskRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT external_id, user_id, model_id, provider_id, upstream_task_id,
		       status, request_params, result_data, cost_usd, created_at, completed_at
		FROM async_tasks
		WHERE status IN ('pending', 'submitted', 'processing')
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := s.pg.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*TaskRecord
	for rows.Next() {
		task := &TaskRecord{}
		var paramsBytes, resultBytes []byte

		if err := rows.Scan(
			&task.ExternalID,
			&task.UserID,
			&task.ModelID,
			&task.ProviderID,
			&task.UpstreamTaskID,
			&task.Status,
			&paramsBytes,
			&resultBytes,
			&task.CostUSD,
			&task.CreatedAt,
			&task.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending task: %w", err)
		}

		if len(paramsBytes) > 0 {
			if err := json.Unmarshal(paramsBytes, &task.RequestParams); err != nil {
				slog.Warn("failed to unmarshal task request_params", "external_id", task.ExternalID, "error", err)
			}
		}
		if len(resultBytes) > 0 {
			if err := json.Unmarshal(resultBytes, &task.ResultData); err != nil {
				slog.Warn("failed to unmarshal task result_data", "external_id", task.ExternalID, "error", err)
			}
		}

		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending tasks: %w", err)
	}

	return tasks, nil
}
