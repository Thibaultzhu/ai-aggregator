package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// ===== Store =====

// Store is the unified data access layer combining PostgreSQL and Redis.
type Store struct {
	pg    *pgxpool.Pool
	redis *goredis.Client
}

var ErrUserAlreadyExists = errors.New("user already exists")

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
	ID          string                 `json:"id"`
	DisplayName string                 `json:"display_name"`
	AdapterType string                 `json:"adapter_type"`
	BaseURL     string                 `json:"base_url"`
	Config      map[string]interface{} `json:"config"`
	IsEnabled   bool                   `json:"is_enabled"`
}

type ProviderKey struct {
	ID            string     `json:"id"`
	ProviderID    string     `json:"provider_id"`
	KeyName       string     `json:"key_name"`
	KeyMask       string     `json:"key_mask"`
	Region        string     `json:"region,omitempty"`
	Scope         string     `json:"scope"`
	UserID        string     `json:"user_id,omitempty"`
	WorkspaceID   string     `json:"workspace_id,omitempty"`
	SealVersion   string     `json:"seal_version"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	LastUsedScope string     `json:"last_used_scope,omitempty"`
	IsActive      bool       `json:"is_active"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ProviderKeyInput struct {
	ProviderID  string
	KeyName     string
	KeyRef      string
	KeyMask     string
	Region      string
	Scope       string
	UserID      string
	WorkspaceID string
}

// GetProviders returns all enabled providers from the database.
func (s *Store) GetProviders(ctx context.Context) ([]*Provider, error) {
	query := `
		SELECT id, display_name, adapter_type, COALESCE(base_url, ''), config, is_enabled
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

// ListProvidersAdmin returns every provider, including disabled providers.
func (s *Store) ListProvidersAdmin(ctx context.Context) ([]*Provider, error) {
	query := `
		SELECT id, display_name, adapter_type, COALESCE(base_url, ''), config, is_enabled
		FROM providers
		ORDER BY id`

	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query admin providers: %w", err)
	}
	defer rows.Close()

	providers := make([]*Provider, 0)
	for rows.Next() {
		p := &Provider{}
		var configBytes []byte
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.AdapterType, &p.BaseURL, &configBytes, &p.IsEnabled); err != nil {
			return nil, fmt.Errorf("scan admin provider: %w", err)
		}
		p.Config = map[string]interface{}{}
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &p.Config); err != nil {
				return nil, fmt.Errorf("unmarshal provider config: %w", err)
			}
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin providers: %w", err)
	}
	return providers, nil
}

func (s *Store) GetProviderAdmin(ctx context.Context, providerID string) (*Provider, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, fmt.Errorf("provider id is required")
	}
	p := &Provider{}
	var configBytes []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id, display_name, adapter_type, COALESCE(base_url, ''), config, is_enabled
		FROM providers
		WHERE id = $1`, providerID).Scan(&p.ID, &p.DisplayName, &p.AdapterType, &p.BaseURL, &configBytes, &p.IsEnabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("provider not found")
		}
		return nil, fmt.Errorf("query provider: %w", err)
	}
	p.Config = map[string]interface{}{}
	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &p.Config); err != nil {
			return nil, fmt.Errorf("unmarshal provider config: %w", err)
		}
	}
	return p, nil
}

// UpsertProviderAdmin creates or updates a provider.
func (s *Store) UpsertProviderAdmin(ctx context.Context, p *Provider) (*Provider, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is required")
	}
	configBytes, err := json.Marshal(p.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal provider config: %w", err)
	}
	query := `
		INSERT INTO providers (id, display_name, adapter_type, base_url, config, is_enabled, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, now())
		ON CONFLICT (id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			adapter_type = EXCLUDED.adapter_type,
			base_url = EXCLUDED.base_url,
			config = EXCLUDED.config,
			is_enabled = EXCLUDED.is_enabled,
			updated_at = now()
		RETURNING id, display_name, adapter_type, COALESCE(base_url, ''), config, is_enabled`

	out := &Provider{}
	var outConfig []byte
	if err := s.pg.QueryRow(ctx, query, p.ID, p.DisplayName, p.AdapterType, p.BaseURL, configBytes, p.IsEnabled).
		Scan(&out.ID, &out.DisplayName, &out.AdapterType, &out.BaseURL, &outConfig, &out.IsEnabled); err != nil {
		return nil, fmt.Errorf("upsert provider: %w", err)
	}
	out.Config = map[string]interface{}{}
	if len(outConfig) > 0 {
		if err := json.Unmarshal(outConfig, &out.Config); err != nil {
			return nil, fmt.Errorf("unmarshal upserted provider config: %w", err)
		}
	}
	return out, nil
}

func (s *Store) CreateProviderKey(ctx context.Context, providerID, keyName, keyRef, keyMask, region string) (*ProviderKey, error) {
	return s.CreateProviderKeyScoped(ctx, ProviderKeyInput{
		ProviderID: providerID,
		KeyName:    keyName,
		KeyRef:     keyRef,
		KeyMask:    keyMask,
		Region:     region,
		Scope:      "platform",
	})
}

func (s *Store) CreateProviderKeyScoped(ctx context.Context, input ProviderKeyInput) (*ProviderKey, error) {
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.KeyName = strings.TrimSpace(input.KeyName)
	input.KeyRef = strings.TrimSpace(input.KeyRef)
	input.KeyMask = strings.TrimSpace(input.KeyMask)
	input.Region = strings.TrimSpace(input.Region)
	input.Scope = strings.TrimSpace(input.Scope)
	input.UserID = strings.TrimSpace(input.UserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	if input.Scope == "" {
		input.Scope = "platform"
	}
	if input.ProviderID == "" || input.KeyName == "" || input.KeyRef == "" {
		return nil, fmt.Errorf("provider_id, key_name and key_ref are required")
	}
	if input.Scope != "platform" && input.Scope != "user" && input.Scope != "workspace" {
		return nil, fmt.Errorf("invalid provider key scope")
	}
	if input.Scope == "user" && input.UserID == "" {
		return nil, fmt.Errorf("user_id is required for user scoped provider key")
	}
	if input.Scope == "workspace" && input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required for workspace scoped provider key")
	}
	out := &ProviderKey{}
	err := s.pg.QueryRow(ctx, `
		INSERT INTO provider_keys (provider_id, key_name, key_ref, key_mask, region, scope, user_id, workspace_id, seal_version, is_active)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, NULLIF($7, '')::uuid, NULLIF($8, '')::uuid, $9, true)
		RETURNING id::text, provider_id, key_name, COALESCE(key_mask, ''), COALESCE(region, ''), scope, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), seal_version, is_active, created_at`,
		input.ProviderID, input.KeyName, input.KeyRef, input.KeyMask, input.Region, input.Scope, input.UserID, input.WorkspaceID, sealVersionFromRef(input.KeyRef),
	).Scan(&out.ID, &out.ProviderID, &out.KeyName, &out.KeyMask, &out.Region, &out.Scope, &out.UserID, &out.WorkspaceID, &out.SealVersion, &out.IsActive, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert provider key: %w", err)
	}
	return out, nil
}

func (s *Store) ListProviderKeys(ctx context.Context, providerID string) ([]ProviderKey, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, provider_id, key_name, key_ref, COALESCE(key_mask, ''), COALESCE(region, ''), scope,
		       COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(seal_version, 'local:v1'),
		       last_used_at, COALESCE(last_used_scope, ''), is_active, revoked_at, created_at
		FROM provider_keys
		WHERE provider_id = $1
		ORDER BY created_at DESC`, providerID)
	if err != nil {
		return nil, fmt.Errorf("query provider keys: %w", err)
	}
	defer rows.Close()
	items := []ProviderKey{}
	for rows.Next() {
		var item ProviderKey
		var keyRef string
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.KeyName, &keyRef, &item.KeyMask, &item.Region, &item.Scope, &item.UserID, &item.WorkspaceID, &item.SealVersion, &item.LastUsedAt, &item.LastUsedScope, &item.IsActive, &item.RevokedAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan provider key: %w", err)
		}
		if item.KeyMask != "" && item.KeyMask != "stored-secret" {
			// Use persisted mask when available so listing never needs to unseal.
		} else if secret, err := unsealLocalSecret(keyRef); err == nil {
			item.KeyMask = maskSecret(secret)
		} else {
			item.KeyMask = "stored-secret"
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider keys: %w", err)
	}
	return items, nil
}

func (s *Store) ListUserProviderKeys(ctx context.Context, userID string) ([]ProviderKey, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, provider_id, key_name, key_ref, COALESCE(key_mask, ''), COALESCE(region, ''), scope,
		       COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(seal_version, 'local:v1'),
		       last_used_at, COALESCE(last_used_scope, ''), is_active, revoked_at, created_at
		FROM provider_keys
		WHERE scope = 'user'
		  AND user_id = $1::uuid
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user provider keys: %w", err)
	}
	defer rows.Close()
	items := []ProviderKey{}
	for rows.Next() {
		var item ProviderKey
		var keyRef string
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.KeyName, &keyRef, &item.KeyMask, &item.Region, &item.Scope, &item.UserID, &item.WorkspaceID, &item.SealVersion, &item.LastUsedAt, &item.LastUsedScope, &item.IsActive, &item.RevokedAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user provider key: %w", err)
		}
		if item.KeyMask == "" || item.KeyMask == "stored-secret" {
			if secret, err := unsealLocalSecret(keyRef); err == nil {
				item.KeyMask = maskSecret(secret)
			} else {
				item.KeyMask = "stored-secret"
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user provider keys: %w", err)
	}
	return items, nil
}

func (s *Store) RevokeProviderKey(ctx context.Context, providerID, keyID string) error {
	tag, err := s.pg.Exec(ctx, `
		UPDATE provider_keys
		SET is_active = false, revoked_at = now()
		WHERE provider_id = $1 AND id = $2::uuid`, providerID, keyID)
	if err != nil {
		return fmt.Errorf("revoke provider key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("provider key not found")
	}
	return nil
}

func (s *Store) RevokeUserProviderKey(ctx context.Context, keyID, userID string) (*ProviderKey, error) {
	out := &ProviderKey{}
	err := s.pg.QueryRow(ctx, `
		UPDATE provider_keys
		SET is_active = false, revoked_at = now()
		WHERE id = $1::uuid
		  AND scope = 'user'
		  AND user_id = $2::uuid
		RETURNING id::text, provider_id, key_name, COALESCE(key_mask, ''), COALESCE(region, ''), scope,
		          COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(seal_version, 'local:v1'),
		          is_active, revoked_at, created_at`,
		keyID, userID,
	).Scan(&out.ID, &out.ProviderID, &out.KeyName, &out.KeyMask, &out.Region, &out.Scope, &out.UserID, &out.WorkspaceID, &out.SealVersion, &out.IsActive, &out.RevokedAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("provider key not found")
		}
		return nil, fmt.Errorf("revoke user provider key: %w", err)
	}
	return out, nil
}

func (s *Store) GetProviderKeySecretByID(ctx context.Context, providerID, keyID string) (string, *ProviderKey, error) {
	var item ProviderKey
	var keyRef string
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, provider_id, key_name, key_ref, COALESCE(key_mask, ''), COALESCE(region, ''), scope,
		       COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(seal_version, 'local:v1'),
		       last_used_at, COALESCE(last_used_scope, ''), is_active, revoked_at, created_at
		FROM provider_keys
		WHERE provider_id = $1 AND id = $2::uuid`,
		strings.TrimSpace(providerID), strings.TrimSpace(keyID),
	).Scan(&item.ID, &item.ProviderID, &item.KeyName, &keyRef, &item.KeyMask, &item.Region, &item.Scope, &item.UserID, &item.WorkspaceID, &item.SealVersion, &item.LastUsedAt, &item.LastUsedScope, &item.IsActive, &item.RevokedAt, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, fmt.Errorf("provider key not found")
		}
		return "", nil, fmt.Errorf("query provider key: %w", err)
	}
	if !item.IsActive {
		return "", &item, fmt.Errorf("provider key is revoked")
	}
	secret, err := unsealLocalSecret(keyRef)
	if err != nil {
		return "", &item, err
	}
	if item.KeyMask == "" || item.KeyMask == "stored-secret" {
		item.KeyMask = maskSecret(secret)
	}
	return secret, &item, nil
}

func (s *Store) GetActiveProviderKeySecret(ctx context.Context, providerID string) (string, error) {
	return s.GetScopedProviderKeySecret(ctx, providerID, "", "")
}

func (s *Store) GetScopedProviderKeySecret(ctx context.Context, providerID, userID, workspaceID string) (string, error) {
	secret, _, _, err := s.GetScopedProviderKeySecretWithMetadata(ctx, providerID, userID, workspaceID)
	return secret, err
}

func (s *Store) GetScopedProviderKeySecretWithMetadata(ctx context.Context, providerID, userID, workspaceID string) (string, string, string, error) {
	var keyRef string
	var keyID string
	var selectedScope string
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, key_ref, scope
		FROM provider_keys
		WHERE provider_id = $1
		  AND is_active = true
		  AND (
		  	($3 <> '' AND scope = 'workspace' AND workspace_id = $3::uuid)
		  	OR ($2 <> '' AND scope = 'user' AND user_id = $2::uuid)
		  	OR scope = 'platform'
		  )
		ORDER BY
		  CASE
		  	WHEN $3 <> '' AND scope = 'workspace' AND workspace_id = $3::uuid THEN 1
		  	WHEN $2 <> '' AND scope = 'user' AND user_id = $2::uuid THEN 2
		  	WHEN scope = 'platform' THEN 3
		  	ELSE 9
		  END,
		  created_at DESC
		LIMIT 1`, providerID, strings.TrimSpace(userID), strings.TrimSpace(workspaceID)).Scan(&keyID, &keyRef, &selectedScope)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", nil
		}
		return "", "", "", fmt.Errorf("query active provider key: %w", err)
	}
	secret, err := unsealLocalSecret(keyRef)
	if err != nil {
		return "", "", "", err
	}
	go func(id, scope string) {
		updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = s.pg.Exec(updateCtx, `UPDATE provider_keys SET last_used_at = now(), last_used_scope = $2 WHERE id = $1::uuid`, id, scope)
	}(keyID, selectedScope)
	return secret, keyID, selectedScope, nil
}

func sealVersionFromRef(ref string) string {
	parts := strings.Split(ref, ":")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0] + ":" + parts[1]
	}
	return "local:v1"
}

func unsealLocalSecret(sealed string) (string, error) {
	const prefix = "local:v1:"
	if !strings.HasPrefix(sealed, prefix) {
		return "", fmt.Errorf("unsupported provider key seal format")
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(sealed, prefix))
	if err != nil {
		return "", fmt.Errorf("decode sealed provider key: %w", err)
	}
	return string(raw), nil
}

func maskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}
	return secret[:4] + strings.Repeat("*", 8) + secret[len(secret)-4:]
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

// ProviderHealthCheck records one provider health-check observation.
type ProviderHealthCheck struct {
	ProviderID          string    `json:"provider_id"`
	DisplayName         string    `json:"display_name"`
	AdapterType         string    `json:"adapter_type"`
	Status              string    `json:"status"`
	LatencyMs           int       `json:"latency_ms"`
	ErrorCode           string    `json:"error_code,omitempty"`
	ErrorMessage        string    `json:"error_message,omitempty"`
	CheckedAt           time.Time `json:"checked_at"`
	RequestCount        int64     `json:"request_count_24h"`
	ErrorCount          int64     `json:"error_count_24h"`
	ErrorRate           float64   `json:"error_rate_24h"`
	FallbackCount       int64     `json:"fallback_count_24h"`
	AvgRequestLatencyMs float64   `json:"avg_request_latency_ms_24h"`
}

// RecordProviderHealthCheck inserts one provider health check history row and
// updates the denormalized model_providers health status used by routing.
func (s *Store) RecordProviderHealthCheck(ctx context.Context, providerID, status string, latencyMs int, errorCode, errorMessage string) error {
	query := `
		INSERT INTO provider_health_checks (provider_id, status, latency_ms, error_code, error_message, checked_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), now())`
	if _, err := s.pg.Exec(ctx, query, providerID, status, latencyMs, errorCode, errorMessage); err != nil {
		return fmt.Errorf("insert provider health check: %w", err)
	}
	if err := s.UpdateProviderHealth(ctx, providerID, status); err != nil {
		return err
	}
	return nil
}

// ListLatestProviderHealth returns every enabled provider with its latest health row.
func (s *Store) ListLatestProviderHealth(ctx context.Context) ([]ProviderHealthCheck, error) {
	query := `
		SELECT p.id, p.display_name, p.adapter_type,
		       COALESCE(h.status, 'unknown') AS status,
		       COALESCE(h.latency_ms, 0) AS latency_ms,
		       COALESCE(h.error_code, '') AS error_code,
		       COALESCE(h.error_message, '') AS error_message,
		       COALESCE(h.checked_at, p.updated_at, p.created_at) AS checked_at,
		       COALESCE(stats.request_count, 0) AS request_count_24h,
		       COALESCE(stats.error_count, 0) AS error_count_24h,
		       CASE WHEN COALESCE(stats.request_count, 0) = 0 THEN 0
		            ELSE COALESCE(stats.error_count, 0)::float / stats.request_count::float END AS error_rate_24h,
		       COALESCE(stats.fallback_count, 0) AS fallback_count_24h,
		       COALESCE(stats.avg_latency_ms, 0) AS avg_request_latency_ms_24h
		FROM providers p
		LEFT JOIN LATERAL (
			SELECT status, latency_ms, error_code, error_message, checked_at
			FROM provider_health_checks
			WHERE provider_id = p.id
			ORDER BY checked_at DESC
			LIMIT 1
		) h ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS request_count,
			       COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
			       COALESCE(SUM(fallback_count), 0) AS fallback_count,
			       COALESCE(AVG(latency_ms), 0) AS avg_latency_ms
			FROM request_logs
			WHERE final_provider_id = p.id
			  AND created_at >= now() - interval '24 hours'
		) stats ON true
		WHERE p.is_enabled = true
		ORDER BY p.id`

	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query provider health: %w", err)
	}
	defer rows.Close()

	items := make([]ProviderHealthCheck, 0)
	for rows.Next() {
		var item ProviderHealthCheck
		if err := rows.Scan(
			&item.ProviderID,
			&item.DisplayName,
			&item.AdapterType,
			&item.Status,
			&item.LatencyMs,
			&item.ErrorCode,
			&item.ErrorMessage,
			&item.CheckedAt,
			&item.RequestCount,
			&item.ErrorCount,
			&item.ErrorRate,
			&item.FallbackCount,
			&item.AvgRequestLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("scan provider health: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider health: %w", err)
	}
	return items, nil
}

func (s *Store) ListProviderHealthHistory(ctx context.Context, providerID string, limit int) ([]ProviderHealthCheck, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.pg.Query(ctx, `
		SELECT p.id, p.display_name, p.adapter_type,
		       h.status, h.latency_ms, COALESCE(h.error_code, ''), COALESCE(h.error_message, ''), h.checked_at
		FROM provider_health_checks h
		JOIN providers p ON p.id = h.provider_id
		WHERE h.provider_id = $1
		ORDER BY h.checked_at DESC
		LIMIT $2`, providerID, limit)
	if err != nil {
		return nil, fmt.Errorf("query provider health history: %w", err)
	}
	defer rows.Close()
	items := make([]ProviderHealthCheck, 0)
	for rows.Next() {
		var item ProviderHealthCheck
		if err := rows.Scan(&item.ProviderID, &item.DisplayName, &item.AdapterType, &item.Status, &item.LatencyMs, &item.ErrorCode, &item.ErrorMessage, &item.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan provider health history: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider health history: %w", err)
	}
	return items, nil
}

// FallbackLog records a provider fallback attempt.
type FallbackLog struct {
	RequestID      string    `json:"request_id"`
	ModelID        string    `json:"model_id,omitempty"`
	FromProviderID string    `json:"from_provider_id,omitempty"`
	ToProviderID   string    `json:"to_provider_id,omitempty"`
	Reason         string    `json:"reason"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// RecordFallbackLog inserts one fallback transition.
func (s *Store) RecordFallbackLog(ctx context.Context, log *FallbackLog) error {
	if log == nil {
		return nil
	}
	query := `
		INSERT INTO fallback_logs (
			request_id, model_id, from_provider_id, to_provider_id,
			reason, error_code, error_message, created_at
		) VALUES (
			$1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''),
			$5, NULLIF($6, ''), NULLIF($7, ''), now()
		)`
	if _, err := s.pg.Exec(ctx, query,
		log.RequestID,
		log.ModelID,
		log.FromProviderID,
		log.ToProviderID,
		log.Reason,
		log.ErrorCode,
		log.ErrorMessage,
	); err != nil {
		return fmt.Errorf("insert fallback log: %w", err)
	}
	return nil
}

// ===== Model Types =====

// Model represents an AI model available for routing.
type Model struct {
	ModelID        string                 `json:"model_id"`
	DisplayName    string                 `json:"display_name"`
	Modality       string                 `json:"modality"`
	Capabilities   []string               `json:"capabilities,omitempty"`
	IsAsync        bool                   `json:"is_async"`
	InputPrice     *float64               `json:"input_price"`
	OutputPrice    *float64               `json:"output_price"`
	PriceUnit      string                 `json:"price_unit"`
	MaxContext     *int                   `json:"max_context"`
	MaxOutput      *int                   `json:"max_output"`
	SupportsStream bool                   `json:"supports_stream"`
	Status         string                 `json:"status"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ModelPricingHistory struct {
	ID             string                 `json:"id"`
	ModelID        string                 `json:"model_id"`
	OldInputPrice  *float64               `json:"old_input_price,omitempty"`
	NewInputPrice  *float64               `json:"new_input_price,omitempty"`
	OldOutputPrice *float64               `json:"old_output_price,omitempty"`
	NewOutputPrice *float64               `json:"new_output_price,omitempty"`
	OldPriceUnit   string                 `json:"old_price_unit"`
	NewPriceUnit   string                 `json:"new_price_unit"`
	ChangeType     string                 `json:"change_type"`
	ChangedBy      string                 `json:"changed_by,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

// GetModels returns all active models from the database with full pricing information.
func (s *Store) GetModels(ctx context.Context) ([]*Model, error) {
	query := `
		SELECT model_id, COALESCE(display_name, ''), modality, capabilities, is_async,
		       input_price, output_price,
		       price_unit, max_context, max_output, supports_stream, status, tags, metadata
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
		var capabilitiesBytes []byte
		var metadataBytes []byte
		if err := rows.Scan(
			&m.ModelID,
			&m.DisplayName,
			&m.Modality,
			&capabilitiesBytes,
			&m.IsAsync,
			&m.InputPrice,
			&m.OutputPrice,
			&m.PriceUnit,
			&m.MaxContext,
			&m.MaxOutput,
			&m.SupportsStream,
			&m.Status,
			&m.Tags,
			&metadataBytes,
		); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		if len(capabilitiesBytes) > 0 {
			if err := json.Unmarshal(capabilitiesBytes, &m.Capabilities); err != nil {
				return nil, fmt.Errorf("unmarshal model capabilities: %w", err)
			}
		}
		m.Metadata = map[string]interface{}{}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &m.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal model metadata: %w", err)
			}
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate models: %w", err)
	}

	return models, nil
}

// ListModelsAdmin returns every model regardless of status.
func (s *Store) ListModelsAdmin(ctx context.Context) ([]*Model, error) {
	query := `
		SELECT model_id, COALESCE(display_name, ''), modality, capabilities, is_async,
		       input_price, output_price, price_unit, max_context, max_output,
		       supports_stream, status, tags, metadata
		FROM models
		ORDER BY model_id`

	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query admin models: %w", err)
	}
	defer rows.Close()

	models := make([]*Model, 0)
	for rows.Next() {
		m := &Model{}
		var capabilitiesBytes []byte
		var metadataBytes []byte
		if err := rows.Scan(
			&m.ModelID,
			&m.DisplayName,
			&m.Modality,
			&capabilitiesBytes,
			&m.IsAsync,
			&m.InputPrice,
			&m.OutputPrice,
			&m.PriceUnit,
			&m.MaxContext,
			&m.MaxOutput,
			&m.SupportsStream,
			&m.Status,
			&m.Tags,
			&metadataBytes,
		); err != nil {
			return nil, fmt.Errorf("scan admin model: %w", err)
		}
		if len(capabilitiesBytes) > 0 {
			if err := json.Unmarshal(capabilitiesBytes, &m.Capabilities); err != nil {
				return nil, fmt.Errorf("unmarshal model capabilities: %w", err)
			}
		}
		m.Metadata = map[string]interface{}{}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &m.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal model metadata: %w", err)
			}
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin models: %w", err)
	}
	return models, nil
}

// GetModelAdmin returns one model by external model_id.
func (s *Store) GetModelAdmin(ctx context.Context, modelID string) (*Model, error) {
	query := `
		SELECT model_id, COALESCE(display_name, ''), modality, capabilities, is_async,
		       input_price, output_price, price_unit, max_context, max_output,
		       supports_stream, status, tags, metadata
		FROM models
		WHERE model_id = $1`

	m := &Model{}
	var capabilitiesBytes []byte
	var metadataBytes []byte
	err := s.pg.QueryRow(ctx, query, modelID).Scan(
		&m.ModelID,
		&m.DisplayName,
		&m.Modality,
		&capabilitiesBytes,
		&m.IsAsync,
		&m.InputPrice,
		&m.OutputPrice,
		&m.PriceUnit,
		&m.MaxContext,
		&m.MaxOutput,
		&m.SupportsStream,
		&m.Status,
		&m.Tags,
		&metadataBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("model not found")
		}
		return nil, fmt.Errorf("query admin model: %w", err)
	}
	if len(capabilitiesBytes) > 0 {
		if err := json.Unmarshal(capabilitiesBytes, &m.Capabilities); err != nil {
			return nil, fmt.Errorf("unmarshal model capabilities: %w", err)
		}
	}
	m.Metadata = map[string]interface{}{}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &m.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal model metadata: %w", err)
		}
	}
	return m, nil
}

// UpsertModelAdmin creates or updates a model catalog row.
func (s *Store) UpsertModelAdmin(ctx context.Context, m *Model) (*Model, error) {
	if m == nil {
		return nil, fmt.Errorf("model is required")
	}
	capabilitiesBytes, err := json.Marshal(m.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("marshal model capabilities: %w", err)
	}
	metadataBytes, err := json.Marshal(m.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal model metadata: %w", err)
	}
	query := `
		INSERT INTO models (
			model_id, display_name, modality, capabilities, input_price, output_price,
			price_unit, max_context, max_output, supports_stream, is_async, status,
			tags, metadata, updated_at
		) VALUES (
			$1, NULLIF($2, ''), $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, now()
		)
		ON CONFLICT (model_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			modality = EXCLUDED.modality,
			capabilities = EXCLUDED.capabilities,
			input_price = EXCLUDED.input_price,
			output_price = EXCLUDED.output_price,
			price_unit = EXCLUDED.price_unit,
			max_context = EXCLUDED.max_context,
			max_output = EXCLUDED.max_output,
			supports_stream = EXCLUDED.supports_stream,
			is_async = EXCLUDED.is_async,
			status = EXCLUDED.status,
			tags = EXCLUDED.tags,
			metadata = EXCLUDED.metadata,
			updated_at = now()`
	if _, err := s.pg.Exec(ctx, query,
		m.ModelID,
		m.DisplayName,
		m.Modality,
		capabilitiesBytes,
		m.InputPrice,
		m.OutputPrice,
		m.PriceUnit,
		m.MaxContext,
		m.MaxOutput,
		m.SupportsStream,
		m.IsAsync,
		m.Status,
		m.Tags,
		metadataBytes,
	); err != nil {
		return nil, fmt.Errorf("upsert model: %w", err)
	}
	return s.GetModelAdmin(ctx, m.ModelID)
}

func (s *Store) RecordModelPricingHistory(ctx context.Context, item *ModelPricingHistory) error {
	if item == nil {
		return nil
	}
	metadataBytes := mustJSONBytes(item.Metadata)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO model_pricing_history (
			model_id, old_input_price, new_input_price, old_output_price, new_output_price,
			old_price_unit, new_price_unit, change_type, changed_by, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::uuid, $10
		)`,
		item.ModelID,
		item.OldInputPrice,
		item.NewInputPrice,
		item.OldOutputPrice,
		item.NewOutputPrice,
		item.OldPriceUnit,
		item.NewPriceUnit,
		item.ChangeType,
		item.ChangedBy,
		metadataBytes,
	)
	if err != nil {
		return fmt.Errorf("insert model pricing history: %w", err)
	}
	return nil
}

func (s *Store) ListModelPricingHistory(ctx context.Context, modelID string, limit int) ([]ModelPricingHistory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, model_id, old_input_price, new_input_price, old_output_price, new_output_price,
		       old_price_unit, new_price_unit, change_type, COALESCE(changed_by::text, ''), metadata, created_at
		FROM model_pricing_history
		WHERE model_id=$1
		ORDER BY created_at DESC
		LIMIT $2`, modelID, limit)
	if err != nil {
		return nil, fmt.Errorf("query model pricing history: %w", err)
	}
	defer rows.Close()
	items := []ModelPricingHistory{}
	for rows.Next() {
		var item ModelPricingHistory
		var metadataRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.ModelID,
			&item.OldInputPrice,
			&item.NewInputPrice,
			&item.OldOutputPrice,
			&item.NewOutputPrice,
			&item.OldPriceUnit,
			&item.NewPriceUnit,
			&item.ChangeType,
			&item.ChangedBy,
			&metadataRaw,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model pricing history: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteModelAdmin removes a model and its provider bindings.
func (s *Store) DeleteModelAdmin(ctx context.Context, modelID string) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete model tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM model_providers WHERE model_id = $1`, modelID); err != nil {
		return fmt.Errorf("delete model bindings: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM models WHERE model_id = $1`, modelID)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("model not found")
	}
	return tx.Commit(ctx)
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
	ProviderID     string    `json:"provider_id"`
	Priority       int       `json:"priority"`
	UpstreamModel  string    `json:"upstream_model"`
	CostMultiplier float64   `json:"cost_multiplier"`
	TimeoutMs      int       `json:"timeout_ms"`
	MaxRetries     int       `json:"max_retries"`
	IsEnabled      bool      `json:"is_enabled"`
	HealthStatus   string    `json:"health_status"`
	LastHealthChk  time.Time `json:"last_health_chk"`
}

type RoutingPolicy struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Scope         string                 `json:"scope"`
	ScopeID       string                 `json:"scope_id,omitempty"`
	Strategy      string                 `json:"strategy"`
	LatencyWeight float64                `json:"latency_weight"`
	CostWeight    float64                `json:"cost_weight"`
	ErrorWeight   float64                `json:"error_weight"`
	IsEnabled     bool                   `json:"is_enabled"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type ProviderRoutingStats struct {
	ProviderID     string
	RequestCount   int64
	ErrorRate      float64
	AvgLatencyMs   float64
	AvgChargedCost float64
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

// ListModelProvidersAdmin returns all bindings for one model, including disabled rows.
func (s *Store) ListModelProvidersAdmin(ctx context.Context, modelID string) ([]ProviderBinding, error) {
	query := `
		SELECT provider_id, priority, COALESCE(upstream_model, ''), cost_multiplier,
		       timeout_ms, max_retries, is_enabled,
		       COALESCE(health_status, 'unknown'),
		       COALESCE(last_health_chk, '1970-01-01'::timestamptz)
		FROM model_providers
		WHERE model_id = $1
		ORDER BY priority ASC, provider_id ASC`

	rows, err := s.pg.Query(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("query admin model providers: %w", err)
	}
	defer rows.Close()

	bindings := make([]ProviderBinding, 0)
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
			return nil, fmt.Errorf("scan admin model provider: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin model providers: %w", err)
	}
	return bindings, nil
}

// UpsertModelProviderAdmin creates or updates one model-provider binding.
func (s *Store) UpsertModelProviderAdmin(ctx context.Context, modelID string, b ProviderBinding) (ProviderBinding, error) {
	query := `
		INSERT INTO model_providers (
			model_id, provider_id, priority, upstream_model, is_stream,
			cost_multiplier, timeout_ms, max_retries, is_enabled
		) VALUES (
			$1, $2, $3, NULLIF($4, ''), true,
			$5, $6, $7, $8
		)
		ON CONFLICT (model_id, provider_id) DO UPDATE SET
			priority = EXCLUDED.priority,
			upstream_model = EXCLUDED.upstream_model,
			cost_multiplier = EXCLUDED.cost_multiplier,
			timeout_ms = EXCLUDED.timeout_ms,
			max_retries = EXCLUDED.max_retries,
			is_enabled = EXCLUDED.is_enabled`

	if _, err := s.pg.Exec(ctx, query,
		modelID,
		b.ProviderID,
		b.Priority,
		b.UpstreamModel,
		b.CostMultiplier,
		b.TimeoutMs,
		b.MaxRetries,
		b.IsEnabled,
	); err != nil {
		return ProviderBinding{}, fmt.Errorf("upsert model provider: %w", err)
	}

	bindings, err := s.ListModelProvidersAdmin(ctx, modelID)
	if err != nil {
		return ProviderBinding{}, err
	}
	for _, item := range bindings {
		if item.ProviderID == b.ProviderID {
			return item, nil
		}
	}
	return ProviderBinding{}, fmt.Errorf("model provider binding not found")
}

// DeleteModelProviderAdmin removes one model-provider binding.
func (s *Store) DeleteModelProviderAdmin(ctx context.Context, modelID, providerID string) error {
	tag, err := s.pg.Exec(ctx, `DELETE FROM model_providers WHERE model_id = $1 AND provider_id = $2`, modelID, providerID)
	if err != nil {
		return fmt.Errorf("delete model provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("model provider binding not found")
	}
	return nil
}

func (s *Store) ListRoutingPolicies(ctx context.Context) ([]RoutingPolicy, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name, scope, scope_id, strategy, latency_weight, cost_weight, error_weight,
		       is_enabled, metadata, created_at, updated_at
		FROM routing_policies
		ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query routing policies: %w", err)
	}
	defer rows.Close()
	items := []RoutingPolicy{}
	for rows.Next() {
		var item RoutingPolicy
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Scope, &item.ScopeID, &item.Strategy, &item.LatencyWeight, &item.CostWeight, &item.ErrorWeight, &item.IsEnabled, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan routing policy: %w", err)
		}
		item.Metadata = map[string]interface{}{}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal routing policy metadata: %w", err)
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertRoutingPolicy(ctx context.Context, policy *RoutingPolicy) (*RoutingPolicy, error) {
	if policy == nil {
		return nil, fmt.Errorf("routing policy is required")
	}
	if policy.Scope == "" {
		policy.Scope = "global"
	}
	if policy.Strategy == "" {
		policy.Strategy = "priority"
	}
	if policy.Name == "" {
		policy.Name = policy.Scope + " " + policy.Strategy + " routing"
	}
	if policy.LatencyWeight == 0 {
		policy.LatencyWeight = 0.4
	}
	if policy.CostWeight == 0 {
		policy.CostWeight = 0.3
	}
	if policy.ErrorWeight == 0 {
		policy.ErrorWeight = 0.3
	}
	metadataBytes := mustJSONBytes(policy.Metadata)
	out := &RoutingPolicy{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO routing_policies (name, scope, scope_id, strategy, latency_weight, cost_weight, error_weight, is_enabled, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, name, scope, scope_id, strategy, latency_weight, cost_weight, error_weight, is_enabled, metadata, created_at, updated_at`,
		policy.Name, policy.Scope, policy.ScopeID, policy.Strategy, policy.LatencyWeight, policy.CostWeight, policy.ErrorWeight, policy.IsEnabled, metadataBytes,
	).Scan(&out.ID, &out.Name, &out.Scope, &out.ScopeID, &out.Strategy, &out.LatencyWeight, &out.CostWeight, &out.ErrorWeight, &out.IsEnabled, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert routing policy: %w", err)
	}
	out.Metadata = map[string]interface{}{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal routing policy metadata: %w", err)
		}
	}
	return out, nil
}

func (s *Store) GetActiveRoutingPolicy(ctx context.Context, modelID, workspaceID string) (*RoutingPolicy, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name, scope, scope_id, strategy, latency_weight, cost_weight, error_weight,
		       is_enabled, metadata, created_at, updated_at
		FROM routing_policies
		WHERE is_enabled = true
		  AND (
		    (scope = 'model' AND scope_id = $1)
		    OR (scope = 'workspace' AND scope_id = $2)
		    OR (scope = 'global' AND scope_id = '')
		  )
		ORDER BY CASE scope WHEN 'model' THEN 1 WHEN 'workspace' THEN 2 ELSE 3 END, updated_at DESC
		LIMIT 1`, modelID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query active routing policy: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var item RoutingPolicy
	var metadataRaw []byte
	if err := rows.Scan(&item.ID, &item.Name, &item.Scope, &item.ScopeID, &item.Strategy, &item.LatencyWeight, &item.CostWeight, &item.ErrorWeight, &item.IsEnabled, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan active routing policy: %w", err)
	}
	item.Metadata = map[string]interface{}{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal active routing policy metadata: %w", err)
		}
	}
	return &item, rows.Err()
}

func (s *Store) GetProviderRoutingStats(ctx context.Context, modelID string) (map[string]ProviderRoutingStats, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT COALESCE(final_provider_id, provider_id, '') AS provider_id,
		       COUNT(*) AS request_count,
		       COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
		       COALESCE(AVG(charged_cost_usd), 0) AS avg_charged_cost,
		       CASE WHEN COUNT(*) = 0 THEN 0 ELSE
		         COUNT(*) FILTER (WHERE status_code >= 400)::float / COUNT(*)::float
		       END AS error_rate
		FROM request_logs
		WHERE model_id = $1
		  AND created_at >= now() - interval '24 hours'
		GROUP BY COALESCE(final_provider_id, provider_id, '')`, modelID)
	if err != nil {
		return nil, fmt.Errorf("query provider routing stats: %w", err)
	}
	defer rows.Close()
	out := map[string]ProviderRoutingStats{}
	for rows.Next() {
		var item ProviderRoutingStats
		if err := rows.Scan(&item.ProviderID, &item.RequestCount, &item.AvgLatencyMs, &item.AvgChargedCost, &item.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan provider routing stats: %w", err)
		}
		if item.ProviderID != "" {
			out[item.ProviderID] = item
		}
	}
	return out, rows.Err()
}

// ===== v0.3 Enterprise Control Plane =====

type Organization struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Slug             string                 `json:"slug"`
	OwnerUserID      string                 `json:"owner_user_id,omitempty"`
	Status           string                 `json:"status"`
	BillingMode      string                 `json:"billing_mode"`
	PaymentTermsDays int                    `json:"payment_terms_days"`
	DefaultPONumber  string                 `json:"default_po_number,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type Workspace struct {
	ID               string                 `json:"id"`
	OrganizationID   string                 `json:"organization_id"`
	Name             string                 `json:"name"`
	Slug             string                 `json:"slug"`
	Status           string                 `json:"status"`
	MonthlyBudgetUSD *float64               `json:"monthly_budget_usd"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type WorkspaceMember struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	RoleName    string    `json:"role_name"`
	Status      string    `json:"status"`
	InvitedBy   string    `json:"invited_by,omitempty"`
	JoinedAt    time.Time `json:"joined_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Project struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type WorkspaceUsageSummary struct {
	WorkspaceID string `json:"workspace_id"`

	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int64   `json:"total_tokens"`

	ByModel    []WorkspaceUsageAttribution `json:"by_model"`
	ByProvider []WorkspaceUsageAttribution `json:"by_provider"`
	ByUser     []WorkspaceUsageAttribution `json:"by_user"`
	ByProject  []WorkspaceUsageAttribution `json:"by_project"`
}

type WorkspaceUsageAttribution struct {
	ID            string  `json:"id"`
	Label         string  `json:"label,omitempty"`
	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	UpstreamCost  float64 `json:"upstream_cost"`
	TotalTokens   int64   `json:"total_tokens"`
}

type WorkspaceBudget struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Period       string    `json:"period"`
	AmountUSD    float64   `json:"amount_usd"`
	SoftLimitPct int       `json:"soft_limit_pct"`
	HardLimitPct int       `json:"hard_limit_pct"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type WorkspaceQuota struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	QuotaType   string                 `json:"quota_type"`
	LimitValue  float64                `json:"limit_value"`
	IsActive    bool                   `json:"is_active"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Invoice struct {
	ID             string                 `json:"id"`
	InvoiceNumber  string                 `json:"invoice_number"`
	OrganizationID string                 `json:"organization_id"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	PeriodStart    time.Time              `json:"period_start"`
	PeriodEnd      time.Time              `json:"period_end"`
	Status         string                 `json:"status"`
	PONumber       string                 `json:"po_number,omitempty"`
	SubtotalUSD    float64                `json:"subtotal_usd"`
	TaxUSD         float64                `json:"tax_usd"`
	TotalUSD       float64                `json:"total_usd"`
	DueDate        *time.Time             `json:"due_date,omitempty"`
	Notes          string                 `json:"notes,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type AuditLog struct {
	ID             int64                  `json:"id"`
	UserID         string                 `json:"user_id,omitempty"`
	OrganizationID string                 `json:"organization_id,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	Action         string                 `json:"action"`
	ResourceType   string                 `json:"resource_type,omitempty"`
	ResourceID     string                 `json:"resource_id,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
	IPAddress      string                 `json:"ip_address,omitempty"`
	UserAgent      string                 `json:"user_agent,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

type AuditLogFilter struct {
	Limit        int
	Action       string
	WorkspaceID  string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
}

type BenchmarkTask struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Dataset     []map[string]interface{} `json:"dataset"`
	CreatedBy   string                   `json:"created_by,omitempty"`
	Status      string                   `json:"status"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type BenchmarkRun struct {
	ID          string                 `json:"id"`
	TaskID      string                 `json:"task_id"`
	ModelIDs    []string               `json:"model_ids"`
	Status      string                 `json:"status"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	Results     []BenchmarkResult      `json:"results,omitempty"`
}

type BenchmarkResult struct {
	ID           string                 `json:"id"`
	RunID        string                 `json:"run_id"`
	TaskID       string                 `json:"task_id"`
	ModelID      string                 `json:"model_id"`
	QualityScore float64                `json:"quality_score"`
	LatencyMs    int                    `json:"latency_ms"`
	CostUSD      float64                `json:"cost_usd"`
	TotalScore   float64                `json:"total_score"`
	Details      map[string]interface{} `json:"details,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

type GuardrailPolicy struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Scope            string                 `json:"scope"`
	ScopeID          string                 `json:"scope_id,omitempty"`
	IsEnabled        bool                   `json:"is_enabled"`
	PIIAction        string                 `json:"pii_action"`
	InjectionAction  string                 `json:"injection_action"`
	ModerationAction string                 `json:"moderation_action"`
	Config           map[string]interface{} `json:"config,omitempty"`
	CreatedBy        string                 `json:"created_by,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type GuardrailFinding struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Count    int    `json:"count"`
	Action   string `json:"action"`
	Severity string `json:"severity,omitempty"`
}

type GuardrailResult struct {
	ID          string                 `json:"id"`
	RequestID   string                 `json:"request_id"`
	UserID      string                 `json:"user_id,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	APIKeyID    string                 `json:"api_key_id,omitempty"`
	ModelID     string                 `json:"model_id"`
	PolicyID    string                 `json:"policy_id,omitempty"`
	Action      string                 `json:"action"`
	Status      string                 `json:"status"`
	RiskScore   float64                `json:"risk_score"`
	Categories  []string               `json:"categories"`
	Findings    []GuardrailFinding     `json:"findings"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Workflow struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Steps       []WorkflowStep         `json:"steps,omitempty"`
}

type WorkflowStep struct {
	ID             string                 `json:"id"`
	WorkflowID     string                 `json:"workflow_id"`
	StepOrder      int                    `json:"step_order"`
	Name           string                 `json:"name"`
	StepType       string                 `json:"step_type"`
	ModelID        string                 `json:"model_id,omitempty"`
	ToolID         string                 `json:"tool_id,omitempty"`
	PromptTemplate string                 `json:"prompt_template,omitempty"`
	Config         map[string]interface{} `json:"config,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type WorkflowRun struct {
	ID             string                 `json:"id"`
	WorkflowID     string                 `json:"workflow_id"`
	UserID         string                 `json:"user_id,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	AgentSessionID string                 `json:"agent_session_id,omitempty"`
	Status         string                 `json:"status"`
	Input          map[string]interface{} `json:"input,omitempty"`
	Output         map[string]interface{} `json:"output,omitempty"`
	TotalCostUSD   float64                `json:"total_cost_usd"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	Steps          []WorkflowRunStep      `json:"steps,omitempty"`
	Traces         []AgentTrace           `json:"traces,omitempty"`
	Webhooks       []WebhookDelivery      `json:"webhooks,omitempty"`
}

type WorkflowRunStep struct {
	ID             string                 `json:"id"`
	RunID          string                 `json:"run_id"`
	WorkflowStepID string                 `json:"workflow_step_id,omitempty"`
	StepOrder      int                    `json:"step_order"`
	Name           string                 `json:"name"`
	StepType       string                 `json:"step_type"`
	Status         string                 `json:"status"`
	Input          map[string]interface{} `json:"input,omitempty"`
	Output         map[string]interface{} `json:"output,omitempty"`
	LatencyMs      int                    `json:"latency_ms"`
	CostUSD        float64                `json:"cost_usd"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

type Tool struct {
	ID          string                 `json:"id"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	ToolType    string                 `json:"tool_type"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	IsEnabled   bool                   `json:"is_enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type ToolCredential struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id,omitempty"`
	WorkspaceID     string                 `json:"workspace_id,omitempty"`
	ToolID          string                 `json:"tool_id"`
	Name            string                 `json:"name"`
	SecretEncrypted string                 `json:"-"`
	SecretMask      string                 `json:"secret_mask"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Status          string                 `json:"status"`
	LastUsedAt      *time.Time             `json:"last_used_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type AgentTrace struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"run_id"`
	StepID    string                 `json:"step_id,omitempty"`
	TraceType string                 `json:"trace_type"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type AgentSession struct {
	ID             string                 `json:"id"`
	UserID         string                 `json:"user_id,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	WorkflowID     string                 `json:"workflow_id,omitempty"`
	Name           string                 `json:"name"`
	Status         string                 `json:"status"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	LastRunID      string                 `json:"last_run_id,omitempty"`
	LastActivityAt *time.Time             `json:"last_activity_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type PromptTemplate struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Template    string                 `json:"template"`
	Variables   []string               `json:"variables,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type WebhookDelivery struct {
	ID             string                 `json:"id"`
	WorkflowID     string                 `json:"workflow_id"`
	RunID          string                 `json:"run_id"`
	CallbackURL    string                 `json:"callback_url"`
	EventType      string                 `json:"event_type"`
	Status         string                 `json:"status"`
	ResponseStatus int                    `json:"response_status,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	AttemptCount   int                    `json:"attempt_count,omitempty"`
	MaxAttempts    int                    `json:"max_attempts,omitempty"`
	ResponseBody   string                 `json:"response_body,omitempty"`
	Signature      string                 `json:"signature,omitempty"`
	DeliveredAt    *time.Time             `json:"delivered_at,omitempty"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type InferenceCluster struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Region      string                 `json:"region"`
	NetworkMode string                 `json:"network_mode"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type InferenceNode struct {
	ID          string                 `json:"id"`
	ClusterID   string                 `json:"cluster_id"`
	Name        string                 `json:"name"`
	EndpointURL string                 `json:"endpoint_url"`
	GPUType     string                 `json:"gpu_type"`
	GPUCount    int                    `json:"gpu_count"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type ModelDeployment struct {
	ID            string                 `json:"id"`
	ClusterID     string                 `json:"cluster_id"`
	ProviderID    string                 `json:"provider_id"`
	ModelID       string                 `json:"model_id"`
	UpstreamModel string                 `json:"upstream_model"`
	Runtime       string                 `json:"runtime"`
	EndpointURL   string                 `json:"endpoint_url"`
	Replicas      int                    `json:"replicas"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

func (s *Store) ListOrganizations(ctx context.Context) ([]Organization, error) {
	query := `
		SELECT id::text, name, slug, COALESCE(owner_user_id::text, ''), status, billing_mode,
		       COALESCE(payment_terms_days, 30), COALESCE(default_po_number, ''),
		       metadata, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC`
	rows, err := s.pg.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query organizations: %w", err)
	}
	defer rows.Close()

	items := make([]Organization, 0)
	for rows.Next() {
		var item Organization
		var metadataBytes []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.OwnerUserID, &item.Status, &item.BillingMode, &item.PaymentTermsDays, &item.DefaultPONumber, &metadataBytes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		item.Metadata = map[string]interface{}{}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal organization metadata: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organizations: %w", err)
	}
	return items, nil
}

func (s *Store) CreateOrganization(ctx context.Context, org *Organization) (*Organization, error) {
	if org == nil {
		return nil, fmt.Errorf("organization is required")
	}
	metadataBytes, err := json.Marshal(org.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal organization metadata: %w", err)
	}
	query := `
		INSERT INTO organizations (name, slug, owner_user_id, status, billing_mode, payment_terms_days, default_po_number, metadata)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8)
		RETURNING id::text, name, slug, COALESCE(owner_user_id::text, ''), status, billing_mode,
		          COALESCE(payment_terms_days, 30), COALESCE(default_po_number, ''),
		          metadata, created_at, updated_at`

	out := &Organization{}
	var outMetadata []byte
	if org.PaymentTermsDays <= 0 {
		org.PaymentTermsDays = 30
	}
	if err := s.pg.QueryRow(ctx, query, org.Name, org.Slug, org.OwnerUserID, org.Status, org.BillingMode, org.PaymentTermsDays, org.DefaultPONumber, metadataBytes).
		Scan(&out.ID, &out.Name, &out.Slug, &out.OwnerUserID, &out.Status, &out.BillingMode, &out.PaymentTermsDays, &out.DefaultPONumber, &outMetadata, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}
	out.Metadata = map[string]interface{}{}
	if len(outMetadata) > 0 {
		if err := json.Unmarshal(outMetadata, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal created organization metadata: %w", err)
		}
	}
	return out, nil
}

func (s *Store) ListWorkspaces(ctx context.Context, organizationID string) ([]Workspace, error) {
	query := `
		SELECT id::text, organization_id::text, name, slug, status, monthly_budget_usd, metadata, created_at, updated_at
		FROM workspaces
		WHERE ($1 = '' OR organization_id = $1::uuid)
		ORDER BY created_at DESC`
	rows, err := s.pg.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("query workspaces: %w", err)
	}
	defer rows.Close()

	items := make([]Workspace, 0)
	for rows.Next() {
		var item Workspace
		var metadataBytes []byte
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Slug, &item.Status, &item.MonthlyBudgetUSD, &metadataBytes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		item.Metadata = map[string]interface{}{}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal workspace metadata: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return items, nil
}

func (s *Store) CreateWorkspace(ctx context.Context, ws *Workspace) (*Workspace, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	metadataBytes, err := json.Marshal(ws.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal workspace metadata: %w", err)
	}
	query := `
		INSERT INTO workspaces (organization_id, name, slug, status, monthly_budget_usd, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id::text, organization_id::text, name, slug, status, monthly_budget_usd, metadata, created_at, updated_at`

	out := &Workspace{}
	var outMetadata []byte
	if err := s.pg.QueryRow(ctx, query, ws.OrganizationID, ws.Name, ws.Slug, ws.Status, ws.MonthlyBudgetUSD, metadataBytes).
		Scan(&out.ID, &out.OrganizationID, &out.Name, &out.Slug, &out.Status, &out.MonthlyBudgetUSD, &outMetadata, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert workspace: %w", err)
	}
	out.Metadata = map[string]interface{}{}
	if len(outMetadata) > 0 {
		if err := json.Unmarshal(outMetadata, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal created workspace metadata: %w", err)
		}
	}
	return out, nil
}

func (s *Store) UpsertWorkspaceMember(ctx context.Context, member *WorkspaceMember) (*WorkspaceMember, error) {
	if member == nil {
		return nil, fmt.Errorf("workspace member is required")
	}
	query := `
		INSERT INTO workspace_members (workspace_id, user_id, role_name, status, invited_by, joined_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, NULLIF($5, '')::uuid, now())
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET
			role_name = EXCLUDED.role_name,
			status = EXCLUDED.status,
			updated_at = now()
		RETURNING id::text, workspace_id::text, user_id::text, role_name, status,
		          COALESCE(invited_by::text, ''), COALESCE(joined_at, '1970-01-01'::timestamptz),
		          created_at, updated_at`

	out := &WorkspaceMember{}
	if err := s.pg.QueryRow(ctx, query, member.WorkspaceID, member.UserID, member.RoleName, member.Status, member.InvitedBy).
		Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.RoleName, &out.Status, &out.InvitedBy, &out.JoinedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert workspace member: %w", err)
	}
	return out, nil
}

func (s *Store) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	query := `
		SELECT id::text, workspace_id::text, user_id::text, role_name, status,
		       COALESCE(invited_by::text, ''), COALESCE(joined_at, '1970-01-01'::timestamptz),
		       created_at, updated_at
		FROM workspace_members
		WHERE workspace_id = $1::uuid
		ORDER BY created_at DESC`
	rows, err := s.pg.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query workspace members: %w", err)
	}
	defer rows.Close()

	items := make([]WorkspaceMember, 0)
	for rows.Next() {
		var item WorkspaceMember
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.UserID, &item.RoleName, &item.Status, &item.InvitedBy, &item.JoinedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace member: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace members: %w", err)
	}
	return items, nil
}

func (s *Store) CreateProject(ctx context.Context, project *Project) (*Project, error) {
	if project == nil {
		return nil, fmt.Errorf("project is required")
	}
	if project.Status == "" {
		project.Status = "active"
	}
	metadataBytes := mustJSONBytes(project.Metadata)
	out := &Project{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO projects (workspace_id, name, slug, status, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, workspace_id::text, name, slug, status, metadata, created_at, updated_at`,
		project.WorkspaceID, project.Name, project.Slug, project.Status, metadataBytes,
	).Scan(&out.ID, &out.WorkspaceID, &out.Name, &out.Slug, &out.Status, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	out.Metadata = map[string]interface{}{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal project metadata: %w", err)
		}
	}
	return out, nil
}

func (s *Store) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workspace_id::text, name, slug, status, metadata, created_at, updated_at
		FROM projects
		WHERE workspace_id = $1::uuid
		ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		var item Project
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.Slug, &item.Status, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		item.Metadata = map[string]interface{}{}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal project metadata: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return items, nil
}

func (s *Store) ProjectBelongsToWorkspace(ctx context.Context, projectID, workspaceID string) (bool, error) {
	if projectID == "" || workspaceID == "" {
		return false, nil
	}
	var exists bool
	if err := s.pg.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM projects
			WHERE id = $1::uuid AND workspace_id = $2::uuid AND status = 'active'
		)`, projectID, workspaceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query project workspace ownership: %w", err)
	}
	return exists, nil
}

func (s *Store) IsActiveWorkspaceMember(ctx context.Context, workspaceID, userID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM workspace_members
			WHERE workspace_id = $1::uuid
			  AND user_id = $2::uuid
			  AND status = 'active'
		)`
	var exists bool
	if err := s.pg.QueryRow(ctx, query, workspaceID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query workspace membership: %w", err)
	}
	return exists, nil
}

func (s *Store) WorkspaceMemberHasPermission(ctx context.Context, workspaceID, userID, permission string) (bool, error) {
	if permission == "" {
		return false, nil
	}
	query := `
		SELECT wm.role_name, COALESCE(r.permissions, '[]'::jsonb)
		FROM workspace_members wm
		JOIN workspaces w ON w.id = wm.workspace_id
		LEFT JOIN roles r ON r.id = wm.role_id
			OR (r.organization_id = w.organization_id AND r.name = wm.role_name)
		WHERE wm.workspace_id = $1::uuid
		  AND wm.user_id = $2::uuid
		  AND wm.status = 'active'
		ORDER BY r.id IS NULL
		LIMIT 1`
	var roleName string
	var permissionsBytes []byte
	if err := s.pg.QueryRow(ctx, query, workspaceID, userID).Scan(&roleName, &permissionsBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query workspace member permission: %w", err)
	}
	if workspaceRoleNameAllows(roleName, permission) {
		return true, nil
	}
	return permissionSetAllows(permissionsBytes, permission), nil
}

func workspaceRoleNameAllows(roleName, permission string) bool {
	switch strings.ToLower(strings.TrimSpace(roleName)) {
	case "owner", "admin":
		return true
	case "member":
		switch permission {
		case "api_keys:create", "api_keys:read", "usage:read", "files:write", "files:read", "workflows:write", "workflows:read":
			return true
		}
	case "viewer", "read_only", "readonly":
		switch permission {
		case "api_keys:read", "usage:read", "files:read", "workflows:read":
			return true
		}
	}
	return false
}

func permissionSetAllows(raw json.RawMessage, permission string) bool {
	if len(raw) == 0 {
		return false
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		for _, value := range values {
			if value == "*" || value == permission {
				return true
			}
		}
		return false
	}
	var object map[string]interface{}
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	if enabled, ok := object["*"].(bool); ok && enabled {
		return true
	}
	if enabled, ok := object[permission].(bool); ok && enabled {
		return true
	}
	return false
}

func (s *Store) GetWorkspaceUsageSummary(ctx context.Context, workspaceID string) (*WorkspaceUsageSummary, error) {
	query := `
		SELECT
			COALESCE(COUNT(*), 0),
			COALESCE(SUM(charged_cost_usd), 0),
			COALESCE(SUM(total_tokens), 0)
		FROM request_logs
		WHERE workspace_id = $1::uuid`

	out := &WorkspaceUsageSummary{WorkspaceID: workspaceID}
	if err := s.pg.QueryRow(ctx, query, workspaceID).Scan(&out.TotalRequests, &out.TotalCost, &out.TotalTokens); err != nil {
		return nil, fmt.Errorf("query workspace usage summary: %w", err)
	}

	var err error
	if out.ByModel, err = s.listWorkspaceUsageAttribution(ctx, workspaceID, "model"); err != nil {
		return nil, err
	}
	if out.ByProvider, err = s.listWorkspaceUsageAttribution(ctx, workspaceID, "provider"); err != nil {
		return nil, err
	}
	if out.ByUser, err = s.listWorkspaceUsageAttribution(ctx, workspaceID, "user"); err != nil {
		return nil, err
	}
	if out.ByProject, err = s.listWorkspaceUsageAttribution(ctx, workspaceID, "project"); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) listWorkspaceUsageAttribution(ctx context.Context, workspaceID, dimension string) ([]WorkspaceUsageAttribution, error) {
	var query string
	switch dimension {
	case "model":
		query = `
			SELECT
				model_id AS id,
				model_id AS label,
				COUNT(*) AS total_requests,
				COALESCE(SUM(charged_cost_usd), 0) AS total_cost,
				COALESCE(SUM(upstream_cost_usd), 0) AS upstream_cost,
				COALESCE(SUM(total_tokens), 0) AS total_tokens
			FROM request_logs
			WHERE workspace_id = $1::uuid
			GROUP BY model_id
			ORDER BY total_cost DESC, total_requests DESC
			LIMIT 5`
	case "provider":
		query = `
			SELECT
				COALESCE(final_provider_id, provider_id, '') AS id,
				COALESCE(final_provider_id, provider_id, '') AS label,
				COUNT(*) AS total_requests,
				COALESCE(SUM(charged_cost_usd), 0) AS total_cost,
				COALESCE(SUM(upstream_cost_usd), 0) AS upstream_cost,
				COALESCE(SUM(total_tokens), 0) AS total_tokens
			FROM request_logs
			WHERE workspace_id = $1::uuid
			GROUP BY COALESCE(final_provider_id, provider_id, '')
			ORDER BY total_cost DESC, total_requests DESC
			LIMIT 5`
	case "user":
		query = `
			SELECT
				COALESCE(rl.user_id::text, 'anonymous') AS id,
				COALESCE(NULLIF(u.email, ''), NULLIF(u.username, ''), rl.user_id::text, 'anonymous') AS label,
				COUNT(*) AS total_requests,
				COALESCE(SUM(rl.charged_cost_usd), 0) AS total_cost,
				COALESCE(SUM(rl.upstream_cost_usd), 0) AS upstream_cost,
				COALESCE(SUM(rl.total_tokens), 0) AS total_tokens
			FROM request_logs rl
			LEFT JOIN users u ON u.id = rl.user_id
			WHERE rl.workspace_id = $1::uuid
			GROUP BY rl.user_id, u.email, u.username
			ORDER BY total_cost DESC, total_requests DESC
			LIMIT 5`
	case "project":
		query = `
			SELECT
				COALESCE(rl.project_id::text, 'unassigned') AS id,
				COALESCE(p.name, 'Unassigned') AS label,
				COUNT(*) AS total_requests,
				COALESCE(SUM(rl.charged_cost_usd), 0) AS total_cost,
				COALESCE(SUM(rl.upstream_cost_usd), 0) AS upstream_cost,
				COALESCE(SUM(rl.total_tokens), 0) AS total_tokens
			FROM request_logs rl
			LEFT JOIN projects p ON p.id = rl.project_id
			WHERE rl.workspace_id = $1::uuid
			GROUP BY rl.project_id, p.name
			ORDER BY total_cost DESC, total_requests DESC
			LIMIT 5`
	default:
		return nil, fmt.Errorf("unsupported workspace usage attribution dimension: %s", dimension)
	}

	rows, err := s.pg.Query(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query workspace usage attribution %s: %w", dimension, err)
	}
	defer rows.Close()

	items := []WorkspaceUsageAttribution{}
	for rows.Next() {
		var item WorkspaceUsageAttribution
		if err := rows.Scan(&item.ID, &item.Label, &item.TotalRequests, &item.TotalCost, &item.UpstreamCost, &item.TotalTokens); err != nil {
			return nil, fmt.Errorf("scan workspace usage attribution %s: %w", dimension, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace usage attribution %s: %w", dimension, err)
	}
	return items, nil
}

func (s *Store) UpsertWorkspaceBudget(ctx context.Context, budget *WorkspaceBudget) (*WorkspaceBudget, error) {
	if budget == nil {
		return nil, fmt.Errorf("workspace budget is required")
	}
	if budget.Period == "" {
		budget.Period = "monthly"
	}
	if budget.SoftLimitPct == 0 {
		budget.SoftLimitPct = 80
	}
	if budget.HardLimitPct == 0 {
		budget.HardLimitPct = 100
	}
	query := `
		INSERT INTO workspace_budgets (workspace_id, period, amount_usd, soft_limit_pct, hard_limit_pct, is_active)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id::text, workspace_id::text, period, amount_usd, soft_limit_pct, hard_limit_pct, is_active, created_at, updated_at`
	out := &WorkspaceBudget{}
	if err := s.pg.QueryRow(ctx, query, budget.WorkspaceID, budget.Period, budget.AmountUSD, budget.SoftLimitPct, budget.HardLimitPct, budget.IsActive).
		Scan(&out.ID, &out.WorkspaceID, &out.Period, &out.AmountUSD, &out.SoftLimitPct, &out.HardLimitPct, &out.IsActive, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert workspace budget: %w", err)
	}
	return out, nil
}

func (s *Store) ListWorkspaceBudgets(ctx context.Context, workspaceID string) ([]WorkspaceBudget, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workspace_id::text, period, amount_usd, soft_limit_pct, hard_limit_pct, is_active, created_at, updated_at
		FROM workspace_budgets
		WHERE workspace_id = $1::uuid
		ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query workspace budgets: %w", err)
	}
	defer rows.Close()
	items := make([]WorkspaceBudget, 0)
	for rows.Next() {
		var item WorkspaceBudget
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Period, &item.AmountUSD, &item.SoftLimitPct, &item.HardLimitPct, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace budget: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertWorkspaceQuota(ctx context.Context, quota *WorkspaceQuota) (*WorkspaceQuota, error) {
	if quota == nil {
		return nil, fmt.Errorf("workspace quota is required")
	}
	metadataBytes, err := json.Marshal(quota.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal quota metadata: %w", err)
	}
	query := `
		INSERT INTO workspace_quotas (workspace_id, quota_type, limit_value, is_active, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, workspace_id::text, quota_type, limit_value, is_active, metadata, created_at, updated_at`
	out := &WorkspaceQuota{}
	var outMetadata []byte
	if err := s.pg.QueryRow(ctx, query, quota.WorkspaceID, quota.QuotaType, quota.LimitValue, quota.IsActive, metadataBytes).
		Scan(&out.ID, &out.WorkspaceID, &out.QuotaType, &out.LimitValue, &out.IsActive, &outMetadata, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert workspace quota: %w", err)
	}
	out.Metadata = map[string]interface{}{}
	if len(outMetadata) > 0 {
		_ = json.Unmarshal(outMetadata, &out.Metadata)
	}
	return out, nil
}

func (s *Store) ListWorkspaceQuotas(ctx context.Context, workspaceID string) ([]WorkspaceQuota, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workspace_id::text, quota_type, limit_value, is_active, metadata, created_at, updated_at
		FROM workspace_quotas
		WHERE workspace_id = $1::uuid
		ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query workspace quotas: %w", err)
	}
	defer rows.Close()
	items := make([]WorkspaceQuota, 0)
	for rows.Next() {
		var item WorkspaceQuota
		var metadataBytes []byte
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.QuotaType, &item.LimitValue, &item.IsActive, &metadataBytes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace quota: %w", err)
		}
		item.Metadata = map[string]interface{}{}
		if len(metadataBytes) > 0 {
			_ = json.Unmarshal(metadataBytes, &item.Metadata)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CheckWorkspaceLimits(ctx context.Context, workspaceID string) (bool, string, error) {
	if workspaceID == "" {
		return true, "", nil
	}
	var monthlySpend float64
	if err := s.pg.QueryRow(ctx, `
		SELECT COALESCE(SUM(charged_cost_usd), 0)
		FROM usage_logs
		WHERE workspace_id = $1::uuid
		  AND created_at >= date_trunc('month', now())`, workspaceID).Scan(&monthlySpend); err != nil {
		return false, "", fmt.Errorf("query workspace monthly spend: %w", err)
	}
	var budgetLimit float64
	if err := s.pg.QueryRow(ctx, `
		SELECT COALESCE(MIN(amount_usd * hard_limit_pct / 100.0), -1)
		FROM workspace_budgets
		WHERE workspace_id = $1::uuid
		  AND is_active = true
		  AND period = 'monthly'`, workspaceID).Scan(&budgetLimit); err != nil {
		return false, "", fmt.Errorf("query workspace budget: %w", err)
	}
	if budgetLimit >= 0 && monthlySpend >= budgetLimit {
		return false, "workspace monthly budget exceeded", nil
	}

	rows, err := s.pg.Query(ctx, `
		SELECT quota_type, limit_value
		FROM workspace_quotas
		WHERE workspace_id = $1::uuid
		  AND is_active = true`, workspaceID)
	if err != nil {
		return false, "", fmt.Errorf("query workspace quotas: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var quotaType string
		var limitValue float64
		if err := rows.Scan(&quotaType, &limitValue); err != nil {
			return false, "", fmt.Errorf("scan workspace quota: %w", err)
		}
		var current float64
		var query string
		switch quotaType {
		case "requests_per_minute":
			query = `SELECT COUNT(*) FROM request_logs WHERE workspace_id = $1::uuid AND created_at >= now() - interval '1 minute'`
		case "tokens_per_month":
			query = `SELECT COALESCE(SUM(total_tokens), 0) FROM usage_logs WHERE workspace_id = $1::uuid AND created_at >= date_trunc('month', now())`
		case "spend_per_month":
			query = `SELECT COALESCE(SUM(charged_cost_usd), 0) FROM usage_logs WHERE workspace_id = $1::uuid AND created_at >= date_trunc('month', now())`
		default:
			continue
		}
		if err := s.pg.QueryRow(ctx, query, workspaceID).Scan(&current); err != nil {
			return false, "", fmt.Errorf("query workspace quota usage: %w", err)
		}
		if current >= limitValue {
			return false, fmt.Sprintf("workspace quota exceeded: %s", quotaType), nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, "", fmt.Errorf("iterate workspace quotas: %w", err)
	}
	return true, "", nil
}

func (s *Store) RecordAuditLog(ctx context.Context, log *AuditLog) error {
	if log == nil {
		return nil
	}
	detailsBytes, err := json.Marshal(log.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	_, err = s.pg.Exec(ctx, `
		INSERT INTO audit_logs (user_id, organization_id, workspace_id, action, resource_type, resource_id, details, ip_address, user_agent)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9)`,
		log.UserID, log.OrganizationID, log.WorkspaceID, log.Action, log.ResourceType, log.ResourceID, detailsBytes, log.IPAddress, log.UserAgent)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	return s.ListAuditLogsFiltered(ctx, AuditLogFilter{Limit: limit})
}

func (s *Store) ListAuditLogsFiltered(ctx context.Context, filter AuditLogFilter) ([]AuditLog, error) {
	filter = normalizeAuditLogFilter(filter)
	where, args := auditLogWhereClause(filter)
	args = append(args, filter.Limit)
	rows, err := s.pg.Query(ctx, `
		SELECT id, COALESCE(user_id::text, ''), COALESCE(organization_id::text, ''), COALESCE(workspace_id::text, ''),
		       action, COALESCE(resource_type, ''), COALESCE(resource_id, ''), COALESCE(details, '{}'::jsonb),
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		FROM audit_logs
		`+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()
	items := make([]AuditLog, 0)
	for rows.Next() {
		var item AuditLog
		var detailsBytes []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.OrganizationID, &item.WorkspaceID, &item.Action, &item.ResourceType, &item.ResourceID, &detailsBytes, &item.IPAddress, &item.UserAgent, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		item.Details = map[string]interface{}{}
		if len(detailsBytes) > 0 {
			_ = json.Unmarshal(detailsBytes, &item.Details)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeAuditLogFilter(filter AuditLogFilter) AuditLogFilter {
	if filter.Limit <= 0 {
		filter.Limit = 100
	} else if filter.Limit > 10000 {
		filter.Limit = 10000
	}
	filter.Action = strings.TrimSpace(filter.Action)
	filter.WorkspaceID = strings.TrimSpace(filter.WorkspaceID)
	filter.ResourceType = strings.TrimSpace(filter.ResourceType)
	filter.ResourceID = strings.TrimSpace(filter.ResourceID)
	return filter
}

func auditLogWhereClause(filter AuditLogFilter) (string, []interface{}) {
	args := []interface{}{}
	conditions := []string{}
	nextArg := func(value interface{}) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = "+nextArg(filter.Action))
	}
	if filter.WorkspaceID != "" {
		conditions = append(conditions, "workspace_id = "+nextArg(filter.WorkspaceID)+"::uuid")
	}
	if filter.ResourceType != "" {
		conditions = append(conditions, "resource_type = "+nextArg(filter.ResourceType))
	}
	if filter.ResourceID != "" {
		conditions = append(conditions, "resource_id = "+nextArg(filter.ResourceID))
	}
	if filter.From != nil {
		conditions = append(conditions, "created_at >= "+nextArg(*filter.From))
	}
	if filter.To != nil {
		conditions = append(conditions, "created_at <= "+nextArg(*filter.To))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (s *Store) ListExpiredAuditLogs(ctx context.Context, retentionDays, limit int) ([]AuditLog, error) {
	if retentionDays <= 0 {
		return []AuditLog{}, nil
	}
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, COALESCE(user_id::text, ''), COALESCE(organization_id::text, ''), COALESCE(workspace_id::text, ''),
		       action, COALESCE(resource_type, ''), COALESCE(resource_id, ''), COALESCE(details, '{}'::jsonb),
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		FROM audit_logs
		WHERE created_at < now() - make_interval(days => $1)
		ORDER BY created_at ASC
		LIMIT $2`, retentionDays, limit)
	if err != nil {
		return nil, fmt.Errorf("query expired audit logs: %w", err)
	}
	defer rows.Close()
	items := make([]AuditLog, 0)
	for rows.Next() {
		var item AuditLog
		var detailsBytes []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.OrganizationID, &item.WorkspaceID, &item.Action, &item.ResourceType, &item.ResourceID, &detailsBytes, &item.IPAddress, &item.UserAgent, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan expired audit log: %w", err)
		}
		item.Details = map[string]interface{}{}
		if len(detailsBytes) > 0 {
			_ = json.Unmarshal(detailsBytes, &item.Details)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteAuditLogsByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pg.Exec(ctx, `DELETE FROM audit_logs WHERE id = ANY($1)`, ids)
	if err != nil {
		return 0, fmt.Errorf("delete audit logs: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *Store) CreateBenchmarkTask(ctx context.Context, task *BenchmarkTask) (*BenchmarkTask, error) {
	if task == nil {
		return nil, fmt.Errorf("benchmark task is required")
	}
	datasetBytes, err := json.Marshal(task.Dataset)
	if err != nil {
		return nil, fmt.Errorf("marshal benchmark dataset: %w", err)
	}
	out := &BenchmarkTask{}
	var datasetRaw []byte
	err = s.pg.QueryRow(ctx, `
		INSERT INTO benchmark_tasks (name, description, dataset, created_by, status)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, COALESCE(NULLIF($5, ''), 'active'))
		RETURNING id::text, name, description, dataset, COALESCE(created_by::text, ''), status, created_at, updated_at`,
		task.Name, task.Description, datasetBytes, task.CreatedBy, task.Status,
	).Scan(&out.ID, &out.Name, &out.Description, &datasetRaw, &out.CreatedBy, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert benchmark task: %w", err)
	}
	_ = json.Unmarshal(datasetRaw, &out.Dataset)
	return out, nil
}

func (s *Store) ListBenchmarkTasks(ctx context.Context) ([]BenchmarkTask, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name, description, dataset, COALESCE(created_by::text, ''), status, created_at, updated_at
		FROM benchmark_tasks
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query benchmark tasks: %w", err)
	}
	defer rows.Close()
	items := make([]BenchmarkTask, 0)
	for rows.Next() {
		var item BenchmarkTask
		var datasetRaw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &datasetRaw, &item.CreatedBy, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan benchmark task: %w", err)
		}
		_ = json.Unmarshal(datasetRaw, &item.Dataset)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetBenchmarkTask(ctx context.Context, taskID string) (*BenchmarkTask, error) {
	item := &BenchmarkTask{}
	var datasetRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, name, description, dataset, COALESCE(created_by::text, ''), status, created_at, updated_at
		FROM benchmark_tasks
		WHERE id = $1`, taskID,
	).Scan(&item.ID, &item.Name, &item.Description, &datasetRaw, &item.CreatedBy, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("benchmark task not found")
		}
		return nil, fmt.Errorf("query benchmark task: %w", err)
	}
	_ = json.Unmarshal(datasetRaw, &item.Dataset)
	return item, nil
}

func (s *Store) CreateBenchmarkRun(ctx context.Context, run *BenchmarkRun, results []BenchmarkResult) (*BenchmarkRun, error) {
	if run == nil {
		return nil, fmt.Errorf("benchmark run is required")
	}
	metadataBytes, err := json.Marshal(run.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal benchmark run metadata: %w", err)
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin benchmark run tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := &BenchmarkRun{}
	var metadataRaw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO benchmark_runs (task_id, model_ids, status, started_at, completed_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, task_id::text, model_ids, status, started_at, completed_at, metadata, created_at`,
		run.TaskID, run.ModelIDs, run.Status, run.StartedAt, run.CompletedAt, metadataBytes,
	).Scan(&out.ID, &out.TaskID, &out.ModelIDs, &out.Status, &out.StartedAt, &out.CompletedAt, &metadataRaw, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert benchmark run: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	out.Results = make([]BenchmarkResult, 0, len(results))
	for _, result := range results {
		detailsBytes, err := json.Marshal(result.Details)
		if err != nil {
			return nil, fmt.Errorf("marshal benchmark result details: %w", err)
		}
		var saved BenchmarkResult
		var detailsRaw []byte
		err = tx.QueryRow(ctx, `
			INSERT INTO benchmark_results (run_id, task_id, model_id, quality_score, latency_ms, cost_usd, total_score, details)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id::text, run_id::text, task_id::text, model_id, quality_score, latency_ms, cost_usd, total_score, details, created_at`,
			out.ID, out.TaskID, result.ModelID, result.QualityScore, result.LatencyMs, result.CostUSD, result.TotalScore, detailsBytes,
		).Scan(&saved.ID, &saved.RunID, &saved.TaskID, &saved.ModelID, &saved.QualityScore, &saved.LatencyMs, &saved.CostUSD, &saved.TotalScore, &detailsRaw, &saved.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert benchmark result: %w", err)
		}
		_ = json.Unmarshal(detailsRaw, &saved.Details)
		out.Results = append(out.Results, saved)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit benchmark run: %w", err)
	}
	return out, nil
}

func (s *Store) ListBenchmarkRuns(ctx context.Context, limit int) ([]BenchmarkRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, task_id::text, model_ids, status, started_at, completed_at, metadata, created_at
		FROM benchmark_runs
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query benchmark runs: %w", err)
	}
	defer rows.Close()
	items := make([]BenchmarkRun, 0)
	for rows.Next() {
		var item BenchmarkRun
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.TaskID, &item.ModelIDs, &item.Status, &item.StartedAt, &item.CompletedAt, &metadataRaw, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan benchmark run: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetBenchmarkRun(ctx context.Context, runID string) (*BenchmarkRun, error) {
	item := &BenchmarkRun{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, task_id::text, model_ids, status, started_at, completed_at, metadata, created_at
		FROM benchmark_runs
		WHERE id = $1`, runID,
	).Scan(&item.ID, &item.TaskID, &item.ModelIDs, &item.Status, &item.StartedAt, &item.CompletedAt, &metadataRaw, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("benchmark run not found")
		}
		return nil, fmt.Errorf("query benchmark run: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &item.Metadata)
	results, err := s.ListBenchmarkResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	item.Results = results
	return item, nil
}

func (s *Store) ListBenchmarkResults(ctx context.Context, runID string) ([]BenchmarkResult, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, run_id::text, task_id::text, model_id, quality_score, latency_ms, cost_usd, total_score, details, created_at
		FROM benchmark_results
		WHERE run_id = $1
		ORDER BY total_score DESC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query benchmark results: %w", err)
	}
	defer rows.Close()
	items := make([]BenchmarkResult, 0)
	for rows.Next() {
		var item BenchmarkResult
		var detailsRaw []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.TaskID, &item.ModelID, &item.QualityScore, &item.LatencyMs, &item.CostUSD, &item.TotalScore, &detailsRaw, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan benchmark result: %w", err)
		}
		_ = json.Unmarshal(detailsRaw, &item.Details)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetLatestBenchmarkScore(ctx context.Context, modelID string) (*float64, error) {
	var score float64
	err := s.pg.QueryRow(ctx, `
		SELECT total_score
		FROM benchmark_results
		WHERE model_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, modelID).Scan(&score)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest benchmark score: %w", err)
	}
	return &score, nil
}

func (s *Store) ListGuardrailPolicies(ctx context.Context) ([]GuardrailPolicy, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name, scope, COALESCE(scope_id, ''), is_enabled,
		       pii_action, injection_action, moderation_action, config,
		       COALESCE(created_by::text, ''), created_at, updated_at
		FROM guardrail_policies
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query guardrail policies: %w", err)
	}
	defer rows.Close()

	items := make([]GuardrailPolicy, 0)
	for rows.Next() {
		var item GuardrailPolicy
		var configRaw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Scope, &item.ScopeID, &item.IsEnabled, &item.PIIAction, &item.InjectionAction, &item.ModerationAction, &configRaw, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan guardrail policy: %w", err)
		}
		item.Config = map[string]interface{}{}
		_ = json.Unmarshal(configRaw, &item.Config)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetActiveGuardrailPolicy(ctx context.Context, workspaceID string) (*GuardrailPolicy, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT id::text, name, scope, COALESCE(scope_id, ''), is_enabled,
		       pii_action, injection_action, moderation_action, config,
		       COALESCE(created_by::text, ''), created_at, updated_at
		FROM guardrail_policies
		WHERE is_enabled = true
		  AND (
		    (scope = 'workspace' AND scope_id = $1)
		    OR (scope = 'global' AND scope_id IS NULL)
		  )
		ORDER BY CASE WHEN scope = 'workspace' THEN 0 ELSE 1 END, created_at DESC
		LIMIT 1`, workspaceID)
	var item GuardrailPolicy
	var configRaw []byte
	err := row.Scan(&item.ID, &item.Name, &item.Scope, &item.ScopeID, &item.IsEnabled, &item.PIIAction, &item.InjectionAction, &item.ModerationAction, &configRaw, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query active guardrail policy: %w", err)
	}
	item.Config = map[string]interface{}{}
	_ = json.Unmarshal(configRaw, &item.Config)
	return &item, nil
}

func (s *Store) CreateGuardrailPolicy(ctx context.Context, policy *GuardrailPolicy) (*GuardrailPolicy, error) {
	if policy == nil {
		return nil, fmt.Errorf("guardrail policy is required")
	}
	configBytes, err := json.Marshal(policy.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal guardrail policy config: %w", err)
	}
	out := &GuardrailPolicy{}
	var configRaw []byte
	err = s.pg.QueryRow(ctx, `
		INSERT INTO guardrail_policies (name, scope, scope_id, is_enabled, pii_action, injection_action, moderation_action, config, created_by)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, NULLIF($9, '')::uuid)
		RETURNING id::text, name, scope, COALESCE(scope_id, ''), is_enabled,
		          pii_action, injection_action, moderation_action, config,
		          COALESCE(created_by::text, ''), created_at, updated_at`,
		policy.Name, policy.Scope, policy.ScopeID, policy.IsEnabled, policy.PIIAction, policy.InjectionAction, policy.ModerationAction, configBytes, policy.CreatedBy,
	).Scan(&out.ID, &out.Name, &out.Scope, &out.ScopeID, &out.IsEnabled, &out.PIIAction, &out.InjectionAction, &out.ModerationAction, &configRaw, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert guardrail policy: %w", err)
	}
	out.Config = map[string]interface{}{}
	_ = json.Unmarshal(configRaw, &out.Config)
	return out, nil
}

func (s *Store) RecordGuardrailResult(ctx context.Context, result *GuardrailResult) (*GuardrailResult, error) {
	if result == nil {
		return nil, fmt.Errorf("guardrail result is required")
	}
	findingsBytes, err := json.Marshal(result.Findings)
	if err != nil {
		return nil, fmt.Errorf("marshal guardrail findings: %w", err)
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin guardrail tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := &GuardrailResult{}
	var findingsRaw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO guardrail_results (request_id, user_id, workspace_id, api_key_id, model_id, policy_id, action, status, risk_score, categories, findings)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, NULLIF($6, '')::uuid, $7, $8, $9, $10, $11)
		RETURNING id::text, request_id, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''),
		          COALESCE(api_key_id::text, ''), model_id, COALESCE(policy_id::text, ''), action, status,
		          risk_score, categories, findings, created_at`,
		result.RequestID, result.UserID, result.WorkspaceID, result.APIKeyID, result.ModelID, result.PolicyID,
		result.Action, result.Status, result.RiskScore, result.Categories, findingsBytes,
	).Scan(&out.ID, &out.RequestID, &out.UserID, &out.WorkspaceID, &out.APIKeyID, &out.ModelID, &out.PolicyID, &out.Action, &out.Status, &out.RiskScore, &out.Categories, &findingsRaw, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert guardrail result: %w", err)
	}
	_ = json.Unmarshal(findingsRaw, &out.Findings)

	for _, finding := range result.Findings {
		if finding.Category == "pii" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO pii_detections (guardrail_result_id, pii_type, count, action)
				VALUES ($1, $2, $3, $4)`,
				out.ID, finding.Type, finding.Count, finding.Action); err != nil {
				return nil, fmt.Errorf("insert pii detection: %w", err)
			}
		}
		if finding.Action == "block" {
			detailsBytes, _ := json.Marshal(finding)
			if _, err := tx.Exec(ctx, `
				INSERT INTO policy_violations (guardrail_result_id, policy_id, request_id, violation_type, severity, action, details)
				VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7)`,
				out.ID, result.PolicyID, result.RequestID, finding.Type, defaultString(finding.Severity, "medium"), finding.Action, detailsBytes); err != nil {
				return nil, fmt.Errorf("insert policy violation: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit guardrail tx: %w", err)
	}
	return out, nil
}

func (s *Store) ListGuardrailResults(ctx context.Context, limit int) ([]GuardrailResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, request_id, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''),
		       COALESCE(api_key_id::text, ''), model_id, COALESCE(policy_id::text, ''), action, status,
		       risk_score, categories, findings, created_at
		FROM guardrail_results
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query guardrail results: %w", err)
	}
	defer rows.Close()
	items := make([]GuardrailResult, 0)
	for rows.Next() {
		var item GuardrailResult
		var findingsRaw []byte
		if err := rows.Scan(&item.ID, &item.RequestID, &item.UserID, &item.WorkspaceID, &item.APIKeyID, &item.ModelID, &item.PolicyID, &item.Action, &item.Status, &item.RiskScore, &item.Categories, &findingsRaw, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan guardrail result: %w", err)
		}
		_ = json.Unmarshal(findingsRaw, &item.Findings)
		items = append(items, item)
	}
	return items, rows.Err()
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// ===== Usage Recording =====

// UsageRecord holds all fields for recording an API call's usage data.
type UsageRecord struct {
	RequestID    string    `json:"request_id"`
	UserID       string    `json:"user_id"`
	APIKeyID     string    `json:"api_key_id"`
	WorkspaceID  string    `json:"workspace_id,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
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

type AdminUser struct {
	ID             string                 `json:"id"`
	Email          string                 `json:"email"`
	Username       string                 `json:"username"`
	Role           string                 `json:"role"`
	Status         string                 `json:"status"`
	BalanceUSD     float64                `json:"balance_usd"`
	MonthlyQuota   float64                `json:"monthly_quota"`
	BillingMode    string                 `json:"billing_mode"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	LastLoginAt    *time.Time             `json:"last_login_at,omitempty"`
	APIKeyCount    int64                  `json:"api_key_count,omitempty"`
	TotalRequests  int64                  `json:"total_requests,omitempty"`
	TotalCost      float64                `json:"total_cost,omitempty"`
	LastActivityAt *time.Time             `json:"last_activity_at,omitempty"`
}

type SystemSetting struct {
	Key         string     `json:"key"`
	Value       string     `json:"value"`
	ValueType   string     `json:"value_type"`
	Description string     `json:"description,omitempty"`
	UpdatedBy   string     `json:"updated_by,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type AnalyticsOverview struct {
	TotalRequests    int64   `json:"total_requests"`
	TotalUsers       int64   `json:"total_users"`
	ActiveUsers      int64   `json:"active_users"`
	TotalCost        float64 `json:"total_cost"`
	UpstreamCost     float64 `json:"upstream_cost"`
	GrossMargin      float64 `json:"gross_margin"`
	TotalTokens      int64   `json:"total_tokens"`
	AverageLatency   float64 `json:"average_latency_ms"`
	P95Latency       float64 `json:"p95_latency_ms"`
	P99Latency       float64 `json:"p99_latency_ms"`
	ErrorRequests    int64   `json:"error_requests"`
	ErrorRate        float64 `json:"error_rate"`
	ProviderCount    int64   `json:"provider_count"`
	HealthyProviders int64   `json:"healthy_providers"`
}

type AnalyticsPoint struct {
	Bucket       time.Time `json:"bucket"`
	Label        string    `json:"label"`
	Requests     int64     `json:"requests,omitempty"`
	Tokens       int64     `json:"tokens,omitempty"`
	ChargedCost  float64   `json:"charged_cost_usd,omitempty"`
	UpstreamCost float64   `json:"upstream_cost_usd,omitempty"`
	GrossMargin  float64   `json:"gross_margin_usd,omitempty"`
	LatencyMs    float64   `json:"latency_ms,omitempty"`
	Errors       int64     `json:"errors,omitempty"`
	ErrorRate    float64   `json:"error_rate,omitempty"`
}

type AlertSummary struct {
	ID             string                 `json:"id"`
	DedupeKey      string                 `json:"dedupe_key,omitempty"`
	RuleID         string                 `json:"rule_id,omitempty"`
	Severity       string                 `json:"severity"`
	Status         string                 `json:"status"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	FirstSeenAt    time.Time              `json:"first_seen_at,omitempty"`
	LastSeenAt     time.Time              `json:"last_seen_at,omitempty"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time             `json:"resolved_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

type AlertRule struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Metric        string                 `json:"metric"`
	Operator      string                 `json:"operator"`
	Threshold     *float64               `json:"threshold,omitempty"`
	Severity      string                 `json:"severity"`
	WindowMinutes int                    `json:"window_minutes"`
	Enabled       bool                   `json:"enabled"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy     string                 `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type FileRecord struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	UserID      string                 `json:"user_id,omitempty"`
	APIKeyID    string                 `json:"api_key_id,omitempty"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Filename    string                 `json:"filename"`
	Purpose     string                 `json:"purpose"`
	Bytes       int64                  `json:"bytes"`
	MimeType    string                 `json:"mime_type,omitempty"`
	StoragePath string                 `json:"-"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// RequestLog is the v0.2 request-level observability record.
type RequestLog struct {
	RequestID       string    `json:"request_id"`
	UserID          string    `json:"user_id,omitempty"`
	APIKeyID        string    `json:"api_key_id,omitempty"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	ProjectID       string    `json:"project_id,omitempty"`
	ModelID         string    `json:"model_id,omitempty"`
	ProviderID      string    `json:"provider_id,omitempty"`
	FinalProviderID string    `json:"final_provider_id,omitempty"`
	CredentialScope string    `json:"credential_scope,omitempty"`
	CredentialKeyID string    `json:"credential_key_id,omitempty"`
	Method          string    `json:"method"`
	Path            string    `json:"path"`
	StatusCode      int       `json:"status_code"`
	ErrorCode       string    `json:"error_code,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	LatencyMs       int       `json:"latency_ms"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	TotalTokens     int       `json:"total_tokens"`
	ChargedCost     float64   `json:"charged_cost_usd"`
	UpstreamCost    float64   `json:"upstream_cost_usd"`
	GrossMargin     float64   `json:"gross_margin_usd"`
	FallbackCount   int       `json:"fallback_count"`
	RequestPreview  string    `json:"request_preview,omitempty"`
	ResponsePreview string    `json:"response_preview,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type RequestLogFilter struct {
	Limit      int
	Offset     int
	ModelID    string
	ProviderID string
	ProjectID  string
	Status     string
	From       *time.Time
	To         *time.Time
}

type RequestLogListResult struct {
	Items  []RequestLog `json:"items"`
	Total  int64        `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

// RecordRequestLog inserts a request-level observability record. Duplicate
// request IDs are ignored so the response path never fails because logging races.
func (s *Store) RecordRequestLog(ctx context.Context, log *RequestLog) error {
	if log == nil {
		return nil
	}
	log.TotalTokens = log.InputTokens + log.OutputTokens
	log.GrossMargin = log.ChargedCost - log.UpstreamCost

	query := `
		INSERT INTO request_logs (
			request_id, user_id, api_key_id, workspace_id, project_id, model_id, provider_id, final_provider_id,
			credential_scope, credential_key_id,
			method, path, status_code, error_code, error_message, latency_ms,
			input_tokens, output_tokens, total_tokens,
			charged_cost_usd, upstream_cost_usd, gross_margin_usd,
			fallback_count, request_preview, response_preview
		) VALUES (
			$1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
			NULLIF($9, ''), NULLIF($10, '')::uuid,
			$11, $12, $13, NULLIF($14, ''), NULLIF($15, ''), $16,
			$17, $18, $19,
			$20, $21, $22,
			$23, NULLIF($24, ''), NULLIF($25, '')
		)
		ON CONFLICT (request_id) DO NOTHING`

	_, err := s.pg.Exec(ctx, query,
		log.RequestID,
		log.UserID,
		log.APIKeyID,
		log.WorkspaceID,
		log.ProjectID,
		log.ModelID,
		log.ProviderID,
		log.FinalProviderID,
		log.CredentialScope,
		log.CredentialKeyID,
		log.Method,
		log.Path,
		log.StatusCode,
		log.ErrorCode,
		log.ErrorMessage,
		log.LatencyMs,
		log.InputTokens,
		log.OutputTokens,
		log.TotalTokens,
		log.ChargedCost,
		log.UpstreamCost,
		log.GrossMargin,
		log.FallbackCount,
		log.RequestPreview,
		log.ResponsePreview,
	)
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}
	return nil
}

// ListRequestLogs returns recent request logs for one user. Ownership is enforced
// by the user_id predicate.
func (s *Store) ListRequestLogs(ctx context.Context, userID string, limit int) ([]RequestLog, error) {
	result, err := s.ListRequestLogsFiltered(ctx, userID, RequestLogFilter{Limit: limit})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *Store) ListRequestLogsFiltered(ctx context.Context, userID string, filter RequestLogFilter) (*RequestLogListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where, args := requestLogWhereClause(userID, filter)
	countQuery := `SELECT COUNT(*) FROM request_logs ` + where
	var total int64
	if err := s.pg.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count request logs: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := `
		SELECT request_id, COALESCE(user_id::text, ''), COALESCE(api_key_id::text, ''),
		       COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), COALESCE(model_id, ''), COALESCE(provider_id, ''), COALESCE(final_provider_id, ''),
		       COALESCE(credential_scope, ''), COALESCE(credential_key_id::text, ''),
		       method, path, status_code, COALESCE(error_code, ''), COALESCE(error_message, ''),
		       latency_ms, input_tokens, output_tokens, total_tokens,
		       charged_cost_usd, upstream_cost_usd, gross_margin_usd,
		       fallback_count, COALESCE(request_preview, ''), COALESCE(response_preview, ''), created_at
		FROM request_logs ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query request logs: %w", err)
	}
	defer rows.Close()

	logs := make([]RequestLog, 0)
	for rows.Next() {
		var r RequestLog
		if err := rows.Scan(
			&r.RequestID,
			&r.UserID,
			&r.APIKeyID,
			&r.WorkspaceID,
			&r.ProjectID,
			&r.ModelID,
			&r.ProviderID,
			&r.FinalProviderID,
			&r.CredentialScope,
			&r.CredentialKeyID,
			&r.Method,
			&r.Path,
			&r.StatusCode,
			&r.ErrorCode,
			&r.ErrorMessage,
			&r.LatencyMs,
			&r.InputTokens,
			&r.OutputTokens,
			&r.TotalTokens,
			&r.ChargedCost,
			&r.UpstreamCost,
			&r.GrossMargin,
			&r.FallbackCount,
			&r.RequestPreview,
			&r.ResponsePreview,
			&r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request log: %w", err)
		}
		logs = append(logs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request logs: %w", err)
	}
	return &RequestLogListResult{Items: logs, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *Store) ListWorkspaceRequestLogsFiltered(ctx context.Context, workspaceID string, filter RequestLogFilter) (*RequestLogListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = 500
	}
	if filter.Limit > 5000 {
		filter.Limit = 5000
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where, args := requestLogWorkspaceWhereClause(workspaceID, filter)
	countQuery := `SELECT COUNT(*) FROM request_logs ` + where
	var total int64
	if err := s.pg.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count workspace request logs: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := `
		SELECT request_id, COALESCE(user_id::text, ''), COALESCE(api_key_id::text, ''),
		       COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), COALESCE(model_id, ''), COALESCE(provider_id, ''), COALESCE(final_provider_id, ''),
		       COALESCE(credential_scope, ''), COALESCE(credential_key_id::text, ''),
		       method, path, status_code, COALESCE(error_code, ''), COALESCE(error_message, ''),
		       latency_ms, input_tokens, output_tokens, total_tokens,
		       charged_cost_usd, upstream_cost_usd, gross_margin_usd,
		       fallback_count, COALESCE(request_preview, ''), COALESCE(response_preview, ''), created_at
		FROM request_logs ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := s.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workspace request logs: %w", err)
	}
	defer rows.Close()

	logs := make([]RequestLog, 0)
	for rows.Next() {
		var r RequestLog
		if err := rows.Scan(
			&r.RequestID,
			&r.UserID,
			&r.APIKeyID,
			&r.WorkspaceID,
			&r.ProjectID,
			&r.ModelID,
			&r.ProviderID,
			&r.FinalProviderID,
			&r.CredentialScope,
			&r.CredentialKeyID,
			&r.Method,
			&r.Path,
			&r.StatusCode,
			&r.ErrorCode,
			&r.ErrorMessage,
			&r.LatencyMs,
			&r.InputTokens,
			&r.OutputTokens,
			&r.TotalTokens,
			&r.ChargedCost,
			&r.UpstreamCost,
			&r.GrossMargin,
			&r.FallbackCount,
			&r.RequestPreview,
			&r.ResponsePreview,
			&r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workspace request log: %w", err)
		}
		logs = append(logs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace request logs: %w", err)
	}
	return &RequestLogListResult{Items: logs, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func requestLogWhereClause(userID string, filter RequestLogFilter) (string, []interface{}) {
	args := []interface{}{userID}
	conditions := []string{"user_id = $1::uuid"}
	return requestLogScopedWhereClause(args, conditions, filter)
}

func requestLogWorkspaceWhereClause(workspaceID string, filter RequestLogFilter) (string, []interface{}) {
	args := []interface{}{workspaceID}
	conditions := []string{"workspace_id = $1::uuid"}
	return requestLogScopedWhereClause(args, conditions, filter)
}

func requestLogScopedWhereClause(args []interface{}, conditions []string, filter RequestLogFilter) (string, []interface{}) {
	nextArg := func(value interface{}) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.ModelID != "" {
		conditions = append(conditions, "model_id = "+nextArg(filter.ModelID))
	}
	if filter.ProviderID != "" {
		arg := nextArg(filter.ProviderID)
		conditions = append(conditions, "(provider_id = "+arg+" OR final_provider_id = "+arg+")")
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "project_id = "+nextArg(filter.ProjectID)+"::uuid")
	}
	switch filter.Status {
	case "success":
		conditions = append(conditions, "status_code >= 200 AND status_code < 400")
	case "error":
		conditions = append(conditions, "status_code >= 400")
	}
	if filter.From != nil {
		conditions = append(conditions, "created_at >= "+nextArg(*filter.From))
	}
	if filter.To != nil {
		conditions = append(conditions, "created_at <= "+nextArg(*filter.To))
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// GetRequestLogByRequestID returns one request log if it belongs to the user.
func (s *Store) GetRequestLogByRequestID(ctx context.Context, userID, requestID string) (*RequestLog, error) {
	query := `
		SELECT request_id, COALESCE(user_id::text, ''), COALESCE(api_key_id::text, ''),
		       COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), COALESCE(model_id, ''), COALESCE(provider_id, ''), COALESCE(final_provider_id, ''),
		       COALESCE(credential_scope, ''), COALESCE(credential_key_id::text, ''),
		       method, path, status_code, COALESCE(error_code, ''), COALESCE(error_message, ''),
		       latency_ms, input_tokens, output_tokens, total_tokens,
		       charged_cost_usd, upstream_cost_usd, gross_margin_usd,
		       fallback_count, COALESCE(request_preview, ''), COALESCE(response_preview, ''), created_at
		FROM request_logs
		WHERE user_id = $1 AND request_id = $2`

	var r RequestLog
	err := s.pg.QueryRow(ctx, query, userID, requestID).Scan(
		&r.RequestID,
		&r.UserID,
		&r.APIKeyID,
		&r.WorkspaceID,
		&r.ProjectID,
		&r.ModelID,
		&r.ProviderID,
		&r.FinalProviderID,
		&r.CredentialScope,
		&r.CredentialKeyID,
		&r.Method,
		&r.Path,
		&r.StatusCode,
		&r.ErrorCode,
		&r.ErrorMessage,
		&r.LatencyMs,
		&r.InputTokens,
		&r.OutputTokens,
		&r.TotalTokens,
		&r.ChargedCost,
		&r.UpstreamCost,
		&r.GrossMargin,
		&r.FallbackCount,
		&r.RequestPreview,
		&r.ResponsePreview,
		&r.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("request log not found")
		}
		return nil, fmt.Errorf("query request log: %w", err)
	}
	return &r, nil
}

// RecordUsage inserts a usage log entry into the usage_logs table.
func (s *Store) RecordUsage(ctx context.Context, record *UsageRecord) error {
	totalTokens := record.InputTokens + record.OutputTokens

	query := `
		INSERT INTO usage_logs (
			request_id, user_id, api_key_id, workspace_id, project_id, model_id, provider_id, modality,
			input_tokens, output_tokens, total_tokens, latency_ms, ttft_ms, is_stream,
			upstream_cost_usd, charged_cost_usd, status_code, error_type,
			is_cached, region
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18,
			$19, $20
		)`

	_, err := s.pg.Exec(ctx, query,
		record.RequestID,
		record.UserID,
		record.APIKeyID,
		record.WorkspaceID,
		record.ProjectID,
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
		SELECT request_id, user_id, api_key_id, COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), model_id, provider_id, modality,
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
			&r.WorkspaceID,
			&r.ProjectID,
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

func (s *Store) GetUsageLatencySummary(ctx context.Context, userID string) (averageLatency, p95Latency, p99Latency float64, err error) {
	query := `
		SELECT
			COALESCE(AVG(latency_ms), 0),
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0),
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0)
		FROM usage_logs
		WHERE user_id = $1`
	if err = s.pg.QueryRow(ctx, query, userID).Scan(&averageLatency, &p95Latency, &p99Latency); err != nil {
		return 0, 0, 0, fmt.Errorf("query usage latency summary: %w", err)
	}
	return averageLatency, p95Latency, p99Latency, nil
}

func (s *Store) GetUsageErrorSummary(ctx context.Context, userID string) (errorRequests int64, errorRate float64, err error) {
	var totalRequests int64
	query := `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status_code >= 400)
		FROM usage_logs
		WHERE user_id = $1`
	if err = s.pg.QueryRow(ctx, query, userID).Scan(&totalRequests, &errorRequests); err != nil {
		return 0, 0, fmt.Errorf("query usage error summary: %w", err)
	}
	if totalRequests > 0 {
		errorRate = float64(errorRequests) / float64(totalRequests)
	}
	return errorRequests, errorRate, nil
}

func (s *Store) ListAdminUsers(ctx context.Context, limit int) ([]AdminUser, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), COALESCE(u.username, ''), u.role, u.status,
		       u.balance_usd, u.monthly_quota, u.billing_mode, COALESCE(u.metadata, '{}'::jsonb),
		       u.created_at, u.updated_at, u.last_login_at,
		       COUNT(DISTINCT k.id) AS api_key_count,
		       COUNT(DISTINCT l.request_id) AS total_requests,
		       COALESCE(SUM(l.charged_cost_usd), 0) AS total_cost,
		       MAX(l.created_at) AS last_activity_at
		FROM users u
		LEFT JOIN api_keys k ON k.user_id = u.id
		LEFT JOIN usage_logs l ON l.user_id = u.id
		GROUP BY u.id
		ORDER BY u.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()
	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		var metadataBytes []byte
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Username, &u.Role, &u.Status,
			&u.BalanceUSD, &u.MonthlyQuota, &u.BillingMode, &metadataBytes,
			&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
			&u.APIKeyCount, &u.TotalRequests, &u.TotalCost, &u.LastActivityAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		u.Metadata = map[string]interface{}{}
		_ = json.Unmarshal(metadataBytes, &u.Metadata)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetAdminUser(ctx context.Context, userID string) (*AdminUser, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), COALESCE(u.username, ''), u.role, u.status,
		       u.balance_usd, u.monthly_quota, u.billing_mode, COALESCE(u.metadata, '{}'::jsonb),
		       u.created_at, u.updated_at, u.last_login_at,
		       COUNT(DISTINCT k.id) AS api_key_count,
		       COUNT(DISTINCT l.request_id) AS total_requests,
		       COALESCE(SUM(l.charged_cost_usd), 0) AS total_cost,
		       MAX(l.created_at) AS last_activity_at
		FROM users u
		LEFT JOIN api_keys k ON k.user_id = u.id
		LEFT JOIN usage_logs l ON l.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query admin user: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	var u AdminUser
	var metadataBytes []byte
	if err := rows.Scan(
		&u.ID, &u.Email, &u.Username, &u.Role, &u.Status,
		&u.BalanceUSD, &u.MonthlyQuota, &u.BillingMode, &metadataBytes,
		&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
		&u.APIKeyCount, &u.TotalRequests, &u.TotalCost, &u.LastActivityAt,
	); err != nil {
		return nil, fmt.Errorf("scan admin user: %w", err)
	}
	u.Metadata = map[string]interface{}{}
	_ = json.Unmarshal(metadataBytes, &u.Metadata)
	return &u, nil
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID, username string, metadata map[string]interface{}) (*AdminUser, error) {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal user metadata: %w", err)
	}
	if _, err := s.pg.Exec(ctx, `
		UPDATE users
		SET username = COALESCE(NULLIF($2, ''), username),
		    metadata = CASE WHEN $3::jsonb = '{}'::jsonb THEN metadata ELSE $3::jsonb END,
		    updated_at = now()
		WHERE id = $1`, userID, username, metadataBytes); err != nil {
		return nil, fmt.Errorf("update user profile: %w", err)
	}
	return s.GetAdminUser(ctx, userID)
}

func (s *Store) UpdateAdminUser(ctx context.Context, userID, username, role, status, billingMode string, monthlyQuota *float64, metadata map[string]interface{}) (*AdminUser, error) {
	current, err := s.GetAdminUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if username == "" {
		username = current.Username
	}
	if role == "" {
		role = current.Role
	}
	if status == "" {
		status = current.Status
	}
	if billingMode == "" {
		billingMode = current.BillingMode
	}
	if monthlyQuota == nil {
		monthlyQuota = &current.MonthlyQuota
	}
	if metadata == nil {
		metadata = current.Metadata
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal user metadata: %w", err)
	}
	if _, err := s.pg.Exec(ctx, `
		UPDATE users
		SET username = $2, role = $3, status = $4, billing_mode = $5,
		    monthly_quota = $6, metadata = $7, updated_at = now()
		WHERE id = $1`, userID, username, role, status, billingMode, *monthlyQuota, metadataBytes); err != nil {
		return nil, fmt.Errorf("update admin user: %w", err)
	}
	return s.GetAdminUser(ctx, userID)
}

func (s *Store) GetAdminUserUsage(ctx context.Context, userID string, limit int) ([]UsageRecord, error) {
	return s.GetUsageLogs(ctx, userID, limit)
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrUserAlreadyExists
		}
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
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	KeyPrefix    string          `json:"key_prefix"`
	WorkspaceID  string          `json:"workspace_id,omitempty"`
	ProjectID    string          `json:"project_id,omitempty"`
	Permissions  json.RawMessage `json:"permissions"`
	RateLimitRPM int             `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM int             `json:"rate_limit_tpm,omitempty"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	IsActive     bool            `json:"is_active"`
	LastUsedAt   *time.Time      `json:"last_used_at"`
	CreatedAt    time.Time       `json:"created_at"`
}

type AdminAPIKeyInfo struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	UserEmail     string          `json:"user_email"`
	Name          string          `json:"name"`
	KeyPrefix     string          `json:"key_prefix"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	WorkspaceName string          `json:"workspace_name,omitempty"`
	ProjectID     string          `json:"project_id,omitempty"`
	ProjectName   string          `json:"project_name,omitempty"`
	Permissions   json.RawMessage `json:"permissions"`
	RateLimitRPM  int             `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM  int             `json:"rate_limit_tpm,omitempty"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	IsActive      bool            `json:"is_active"`
	LastUsedAt    *time.Time      `json:"last_used_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CreateAPIKey inserts a new API key record and returns its generated ID.
// If permissions is nil, defaults to {"models":"*"} (access all models).
func (s *Store) CreateAPIKey(ctx context.Context, userID, name, keyHash, keyPrefix, workspaceID string, permissions json.RawMessage) (string, error) {
	return s.CreateAPIKeyWithProject(ctx, userID, name, keyHash, keyPrefix, workspaceID, "", permissions)
}

func (s *Store) CreateAPIKeyWithProject(ctx context.Context, userID, name, keyHash, keyPrefix, workspaceID, projectID string, permissions json.RawMessage) (string, error) {
	if permissions == nil {
		permissions = json.RawMessage(`{"models":"*"}`)
	}

	query := `
		INSERT INTO api_keys (user_id, name, key_hash, key_prefix, workspace_id, project_id, permissions, is_active, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, $7, true, now())
		RETURNING id`

	var keyID string
	err := s.pg.QueryRow(ctx, query, userID, name, keyHash, keyPrefix, workspaceID, projectID, permissions).Scan(&keyID)
	if err != nil {
		return "", fmt.Errorf("insert api key: %w", err)
	}

	slog.Info("api key created", "key_id", keyID, "user_id", userID, "name", name)
	return keyID, nil
}

// ValidateAPIKey looks up an API key by its SHA-256 hash and checks that it is active.
// Returns the owning user's ID, the key ID, and its permissions JSON.
func (s *Store) ValidateAPIKey(ctx context.Context, keyHash string) (userID, keyID, workspaceID, projectID string, perms json.RawMessage, rateLimitRPM, rateLimitTPM int, err error) {
	query := `
		SELECT user_id, id, COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), permissions,
		       COALESCE(rate_limit_rpm, 0), COALESCE(rate_limit_tpm, 0)
		FROM api_keys
		WHERE key_hash = $1
		  AND is_active = true
		  AND (expires_at IS NULL OR expires_at > now())`

	err = s.pg.QueryRow(ctx, query, keyHash).Scan(&userID, &keyID, &workspaceID, &projectID, &perms, &rateLimitRPM, &rateLimitTPM)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", "", nil, 0, 0, fmt.Errorf("api key not found, inactive, or expired")
		}
		return "", "", "", "", nil, 0, 0, fmt.Errorf("validate api key: %w", err)
	}

	// Update last_used_at asynchronously (non-blocking, best-effort).
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.pg.Exec(updateCtx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, keyID)
	}()

	return userID, keyID, workspaceID, projectID, perms, rateLimitRPM, rateLimitTPM, nil
}

// ListAPIKeys returns all API keys belonging to a user, without exposing the key hash.
func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]APIKeyInfo, error) {
	query := `
		SELECT id, name, key_prefix, COALESCE(workspace_id::text, ''), COALESCE(project_id::text, ''), permissions,
		       COALESCE(rate_limit_rpm, 0), COALESCE(rate_limit_tpm, 0), expires_at,
		       is_active, last_used_at, created_at
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
			&k.WorkspaceID,
			&k.ProjectID,
			&k.Permissions,
			&k.RateLimitRPM,
			&k.RateLimitTPM,
			&k.ExpiresAt,
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

func (s *Store) ListAPIKeysAdmin(ctx context.Context, userID string, includeInactive bool, limit int) ([]AdminAPIKeyInfo, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.pg.Query(ctx, `
		SELECT k.id::text, k.user_id::text, COALESCE(u.email, ''), COALESCE(k.name, ''),
		       k.key_prefix, COALESCE(k.workspace_id::text, ''), COALESCE(w.name, ''),
		       COALESCE(k.project_id::text, ''), COALESCE(p.name, ''),
		       k.permissions, COALESCE(k.rate_limit_rpm, 0), COALESCE(k.rate_limit_tpm, 0),
		       k.expires_at, k.is_active, k.last_used_at, k.created_at
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		LEFT JOIN workspaces w ON w.id = k.workspace_id
		LEFT JOIN projects p ON p.id = k.project_id
		WHERE ($1 = '' OR k.user_id = $1::uuid)
		  AND ($2 OR k.is_active = true)
		ORDER BY k.created_at DESC
		LIMIT $3`, userID, includeInactive, limit)
	if err != nil {
		return nil, fmt.Errorf("query admin api keys: %w", err)
	}
	defer rows.Close()
	keys := make([]AdminAPIKeyInfo, 0)
	for rows.Next() {
		var k AdminAPIKeyInfo
		if err := rows.Scan(
			&k.ID,
			&k.UserID,
			&k.UserEmail,
			&k.Name,
			&k.KeyPrefix,
			&k.WorkspaceID,
			&k.WorkspaceName,
			&k.ProjectID,
			&k.ProjectName,
			&k.Permissions,
			&k.RateLimitRPM,
			&k.RateLimitTPM,
			&k.ExpiresAt,
			&k.IsActive,
			&k.LastUsedAt,
			&k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin api keys: %w", err)
	}
	return keys, nil
}

func (s *Store) UpdateAPIKeyAdmin(ctx context.Context, key *AdminAPIKeyInfo) (*AdminAPIKeyInfo, error) {
	if key == nil || key.ID == "" {
		return nil, fmt.Errorf("api key id is required")
	}
	if len(key.Permissions) == 0 {
		key.Permissions = json.RawMessage(`{"models":"*"}`)
	}
	if !json.Valid(key.Permissions) {
		return nil, fmt.Errorf("api key permissions must be valid JSON")
	}
	var rpm interface{}
	if key.RateLimitRPM > 0 {
		rpm = key.RateLimitRPM
	}
	var tpm interface{}
	if key.RateLimitTPM > 0 {
		tpm = key.RateLimitTPM
	}
	tag, err := s.pg.Exec(ctx, `
		UPDATE api_keys
		SET name = NULLIF($2, ''),
		    workspace_id = NULLIF($3, '')::uuid,
		    project_id = NULLIF($4, '')::uuid,
		    permissions = $5,
		    rate_limit_rpm = $6,
		    rate_limit_tpm = $7,
		    expires_at = $8,
		    is_active = $9
		WHERE id = $1`,
		key.ID,
		key.Name,
		key.WorkspaceID,
		key.ProjectID,
		key.Permissions,
		rpm,
		tpm,
		key.ExpiresAt,
		key.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("admin update api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("api key not found: %s", key.ID)
	}
	return s.GetAPIKeyAdmin(ctx, key.ID)
}

func (s *Store) GetAPIKeyAdmin(ctx context.Context, keyID string) (*AdminAPIKeyInfo, error) {
	keys, err := s.ListAPIKeysAdmin(ctx, "", true, 500)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].ID == keyID {
			return &keys[i], nil
		}
	}
	return nil, fmt.Errorf("api key not found: %s", keyID)
}

func (s *Store) RevokeAPIKeyAdmin(ctx context.Context, keyID string) (*AdminAPIKeyInfo, error) {
	tag, err := s.pg.Exec(ctx, `
		UPDATE api_keys
		SET is_active = false
		WHERE id = $1`, keyID)
	if err != nil {
		return nil, fmt.Errorf("admin revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("api key not found: %s", keyID)
	}
	return s.GetAPIKeyAdmin(ctx, keyID)
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
	return s.CreateBillingTransactionForWorkspace(ctx, userID, "", amountUSD, txType, description)
}

// CreateBillingTransactionForWorkspace records a billing event with optional workspace attribution.
func (s *Store) CreateBillingTransactionForWorkspace(ctx context.Context, userID, workspaceID string, amountUSD float64, txType string, description string) error {
	return s.CreateBillingTransactionForWorkspaceProject(ctx, userID, workspaceID, "", amountUSD, txType, description)
}

func (s *Store) CreateBillingTransactionForWorkspaceProject(ctx context.Context, userID, workspaceID, projectID string, amountUSD float64, txType string, description string) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for billing transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	organizationID := ""
	if workspaceID != "" {
		err = tx.QueryRow(ctx, `SELECT organization_id::text FROM workspaces WHERE id = $1`, workspaceID).Scan(&organizationID)
		if err != nil {
			return fmt.Errorf("query billing workspace organization: %w", err)
		}
	}

	// Insert the billing transaction and get its ID back atomically.
	var txID string
	err = tx.QueryRow(ctx, `
		INSERT INTO billing_transactions (user_id, organization_id, workspace_id, project_id, amount_usd, tx_type, description, created_at)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7, now())
		RETURNING id`,
		userID, organizationID, workspaceID, projectID, amountUSD, txType, description,
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

func (s *Store) ListInvoices(ctx context.Context, organizationID string) ([]Invoice, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, invoice_number, organization_id::text, COALESCE(workspace_id::text, ''),
		       period_start, period_end, status, po_number, subtotal_usd, tax_usd, total_usd,
		       due_date, notes, metadata, created_at, updated_at
		FROM invoices
		WHERE ($1 = '' OR organization_id = $1::uuid)
		ORDER BY created_at DESC`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("query invoices: %w", err)
	}
	defer rows.Close()
	items := []Invoice{}
	for rows.Next() {
		item, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetInvoice(ctx context.Context, invoiceID string) (*Invoice, error) {
	if strings.TrimSpace(invoiceID) == "" {
		return nil, fmt.Errorf("invoice id is required")
	}
	row := s.pg.QueryRow(ctx, `
		SELECT id::text, invoice_number, organization_id::text, COALESCE(workspace_id::text, ''),
		       period_start, period_end, status, po_number, subtotal_usd, tax_usd, total_usd,
		       due_date, notes, metadata, created_at, updated_at
		FROM invoices
		WHERE id = $1::uuid`, strings.TrimSpace(invoiceID))
	out, err := scanInvoice(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, fmt.Errorf("invoice not found")
		}
		return nil, err
	}
	return &out, nil
}

func (s *Store) CreateInvoice(ctx context.Context, invoice *Invoice) (*Invoice, error) {
	if invoice == nil {
		return nil, fmt.Errorf("invoice is required")
	}
	if invoice.OrganizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if invoice.PeriodStart.IsZero() || invoice.PeriodEnd.IsZero() {
		return nil, fmt.Errorf("period_start and period_end are required")
	}
	var termsDays int
	var defaultPO string
	err := s.pg.QueryRow(ctx, `
		SELECT COALESCE(payment_terms_days, 30), COALESCE(default_po_number, '')
		FROM organizations
		WHERE id = $1::uuid`, invoice.OrganizationID).Scan(&termsDays, &defaultPO)
	if err != nil {
		return nil, fmt.Errorf("query invoice organization: %w", err)
	}
	if invoice.PONumber == "" {
		invoice.PONumber = defaultPO
	}
	if invoice.Status == "" {
		invoice.Status = "draft"
	}
	if invoice.InvoiceNumber == "" {
		invoice.InvoiceNumber = fmt.Sprintf("INV-%s-%s", invoice.PeriodStart.Format("200601"), strings.ToUpper(generateShortID()))
	}
	if invoice.DueDate == nil {
		due := invoice.PeriodEnd.AddDate(0, 0, termsDays)
		invoice.DueDate = &due
	}
	var subtotal float64
	if invoice.WorkspaceID != "" {
		err = s.pg.QueryRow(ctx, `
			SELECT COALESCE(SUM(-amount_usd), 0)
			FROM billing_transactions
			WHERE organization_id = $1::uuid
			  AND workspace_id = $2::uuid
			  AND amount_usd < 0
			  AND created_at >= $3
			  AND created_at < $4`,
			invoice.OrganizationID, invoice.WorkspaceID, invoice.PeriodStart, invoice.PeriodEnd.AddDate(0, 0, 1),
		).Scan(&subtotal)
	} else {
		err = s.pg.QueryRow(ctx, `
			SELECT COALESCE(SUM(-amount_usd), 0)
			FROM billing_transactions
			WHERE organization_id = $1::uuid
			  AND amount_usd < 0
			  AND created_at >= $2
			  AND created_at < $3`,
			invoice.OrganizationID, invoice.PeriodStart, invoice.PeriodEnd.AddDate(0, 0, 1),
		).Scan(&subtotal)
	}
	if err != nil {
		return nil, fmt.Errorf("query invoice subtotal: %w", err)
	}
	invoice.SubtotalUSD = subtotal
	invoice.TotalUSD = invoice.SubtotalUSD + invoice.TaxUSD
	metadataBytes := mustJSONBytes(invoice.Metadata)
	row := s.pg.QueryRow(ctx, `
		INSERT INTO invoices (
			invoice_number, organization_id, workspace_id, period_start, period_end, status,
			po_number, subtotal_usd, tax_usd, total_usd, due_date, notes, metadata
		) VALUES (
			$1, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13
		)
		RETURNING id::text, invoice_number, organization_id::text, COALESCE(workspace_id::text, ''),
		          period_start, period_end, status, po_number, subtotal_usd, tax_usd, total_usd,
		          due_date, notes, metadata, created_at, updated_at`,
		invoice.InvoiceNumber, invoice.OrganizationID, invoice.WorkspaceID, invoice.PeriodStart, invoice.PeriodEnd, invoice.Status,
		invoice.PONumber, invoice.SubtotalUSD, invoice.TaxUSD, invoice.TotalUSD, invoice.DueDate, invoice.Notes, metadataBytes,
	)
	out, err := scanInvoice(row)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) UpdateInvoiceStatus(ctx context.Context, invoiceID, status, notes string) (*Invoice, error) {
	if invoiceID == "" {
		return nil, fmt.Errorf("invoice id is required")
	}
	if status == "" {
		return nil, fmt.Errorf("status is required")
	}
	row := s.pg.QueryRow(ctx, `
		UPDATE invoices
		SET status = $2,
		    notes = CASE WHEN $3 = '' THEN notes ELSE $3 END,
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING id::text, invoice_number, organization_id::text, COALESCE(workspace_id::text, ''),
		          period_start, period_end, status, po_number, subtotal_usd, tax_usd, total_usd,
		          due_date, notes, metadata, created_at, updated_at`,
		invoiceID, status, notes,
	)
	out, err := scanInvoice(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, fmt.Errorf("invoice not found")
		}
		return nil, err
	}
	return &out, nil
}

type invoiceScanner interface {
	Scan(dest ...interface{}) error
}

func scanInvoice(scanner invoiceScanner) (Invoice, error) {
	var item Invoice
	var metadataRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.InvoiceNumber,
		&item.OrganizationID,
		&item.WorkspaceID,
		&item.PeriodStart,
		&item.PeriodEnd,
		&item.Status,
		&item.PONumber,
		&item.SubtotalUSD,
		&item.TaxUSD,
		&item.TotalUSD,
		&item.DueDate,
		&item.Notes,
		&metadataRaw,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return Invoice{}, fmt.Errorf("scan invoice: %w", err)
	}
	item.Metadata = map[string]interface{}{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
			return Invoice{}, fmt.Errorf("unmarshal invoice metadata: %w", err)
		}
	}
	return item, nil
}

func generateShortID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
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

func (s *Store) ListSettings(ctx context.Context) ([]SystemSetting, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT key, value, COALESCE(value_type, 'string'), COALESCE(description, ''),
		       COALESCE(updated_by::text, ''), updated_at
		FROM system_settings
		ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()
	settings := make([]SystemSetting, 0)
	for rows.Next() {
		var item SystemSetting
		if err := rows.Scan(&item.Key, &item.Value, &item.ValueType, &item.Description, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings = append(settings, item)
	}
	return settings, rows.Err()
}

func (s *Store) SetSettingAdmin(ctx context.Context, key, value, valueType, description, updatedBy string) (*SystemSetting, error) {
	if valueType == "" {
		valueType = "string"
	}
	if _, err := s.pg.Exec(ctx, `
		INSERT INTO system_settings (key, value, value_type, description, updated_by, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::uuid, now())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			value_type = EXCLUDED.value_type,
			description = COALESCE(EXCLUDED.description, system_settings.description),
			updated_by = EXCLUDED.updated_by,
			updated_at = now()`, key, value, valueType, description, updatedBy); err != nil {
		return nil, fmt.Errorf("upsert admin setting: %w", err)
	}
	if s.redis != nil {
		_ = s.redis.Del(ctx, settingCachePrefix+key).Err()
	}
	out := &SystemSetting{}
	if err := s.pg.QueryRow(ctx, `
		SELECT key, value, COALESCE(value_type, 'string'), COALESCE(description, ''),
		       COALESCE(updated_by::text, ''), updated_at
		FROM system_settings WHERE key = $1`, key).Scan(&out.Key, &out.Value, &out.ValueType, &out.Description, &out.UpdatedBy, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("query updated setting: %w", err)
	}
	return out, nil
}

func (s *Store) AdminAnalyticsOverview(ctx context.Context) (*AnalyticsOverview, error) {
	out := &AnalyticsOverview{}
	if err := s.pg.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM request_logs),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(DISTINCT user_id) FROM request_logs WHERE created_at >= now() - interval '30 days'),
			(SELECT COALESCE(SUM(charged_cost_usd), 0) FROM request_logs),
			(SELECT COALESCE(SUM(upstream_cost_usd), 0) FROM request_logs),
			(SELECT COALESCE(SUM(gross_margin_usd), 0) FROM request_logs),
			(SELECT COALESCE(SUM(total_tokens), 0) FROM request_logs),
			(SELECT COALESCE(AVG(latency_ms), 0) FROM request_logs),
			(SELECT COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) FROM request_logs),
			(SELECT COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms), 0) FROM request_logs),
			(SELECT COUNT(*) FROM request_logs WHERE status_code >= 400),
			(SELECT COUNT(*) FROM providers),
			(SELECT COUNT(DISTINCT provider_id) FROM provider_health_checks WHERE status = 'healthy' AND checked_at >= now() - interval '24 hours')`).
		Scan(&out.TotalRequests, &out.TotalUsers, &out.ActiveUsers, &out.TotalCost, &out.UpstreamCost, &out.GrossMargin, &out.TotalTokens, &out.AverageLatency, &out.P95Latency, &out.P99Latency, &out.ErrorRequests, &out.ProviderCount, &out.HealthyProviders); err != nil {
		return nil, fmt.Errorf("query analytics overview: %w", err)
	}
	if out.TotalRequests > 0 {
		out.ErrorRate = float64(out.ErrorRequests) / float64(out.TotalRequests)
	}
	return out, nil
}

func (s *Store) AdminAnalyticsSeries(ctx context.Context, metric string, days int) ([]AnalyticsPoint, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	selectExpr := "COUNT(*) AS requests, COALESCE(SUM(total_tokens), 0) AS tokens, 0::numeric AS charged_cost, 0::numeric AS upstream_cost, 0::numeric AS gross_margin, COALESCE(AVG(latency_ms), 0) AS latency_ms, COUNT(*) FILTER (WHERE status_code >= 400) AS errors"
	switch metric {
	case "cost":
		selectExpr = "COUNT(*) AS requests, 0::bigint AS tokens, COALESCE(SUM(charged_cost_usd), 0) AS charged_cost, COALESCE(SUM(upstream_cost_usd), 0) AS upstream_cost, COALESCE(SUM(gross_margin_usd), 0) AS gross_margin, 0::numeric AS latency_ms, 0::bigint AS errors"
	case "latency":
		selectExpr = "COUNT(*) AS requests, 0::bigint AS tokens, 0::numeric AS charged_cost, 0::numeric AS upstream_cost, 0::numeric AS gross_margin, COALESCE(AVG(latency_ms), 0) AS latency_ms, 0::bigint AS errors"
	case "errors":
		selectExpr = "COUNT(*) AS requests, 0::bigint AS tokens, 0::numeric AS charged_cost, 0::numeric AS upstream_cost, 0::numeric AS gross_margin, 0::numeric AS latency_ms, COUNT(*) FILTER (WHERE status_code >= 400) AS errors"
	}
	query := fmt.Sprintf(`
		SELECT date_trunc('day', created_at) AS bucket, %s
		FROM request_logs
		WHERE created_at >= now() - ($1::int * interval '1 day')
		GROUP BY bucket
		ORDER BY bucket ASC`, selectExpr)
	rows, err := s.pg.Query(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("query analytics series: %w", err)
	}
	defer rows.Close()
	points := make([]AnalyticsPoint, 0)
	for rows.Next() {
		var p AnalyticsPoint
		if err := rows.Scan(&p.Bucket, &p.Requests, &p.Tokens, &p.ChargedCost, &p.UpstreamCost, &p.GrossMargin, &p.LatencyMs, &p.Errors); err != nil {
			return nil, fmt.Errorf("scan analytics point: %w", err)
		}
		p.Label = p.Bucket.Format("2006-01-02")
		if p.Requests > 0 {
			p.ErrorRate = float64(p.Errors) / float64(p.Requests)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (s *Store) ListAlertRules(ctx context.Context, includeDisabled bool) ([]AlertRule, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name, metric, operator, threshold, severity, window_minutes, enabled,
		       COALESCE(metadata, '{}'::jsonb), COALESCE(created_by::text, ''), created_at, updated_at
		FROM alert_rules
		WHERE ($1 OR enabled = true)
		ORDER BY enabled DESC, severity DESC, created_at DESC`, includeDisabled)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	defer rows.Close()
	rules := make([]AlertRule, 0)
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return rules, nil
}

func (s *Store) CreateAlertRule(ctx context.Context, rule *AlertRule) (*AlertRule, error) {
	if rule == nil {
		return nil, fmt.Errorf("alert rule is required")
	}
	if rule.Operator == "" {
		rule.Operator = ">="
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.WindowMinutes <= 0 {
		rule.WindowMinutes = 60
	}
	if rule.Metadata == nil {
		rule.Metadata = map[string]interface{}{}
	}
	metadataBytes, err := json.Marshal(rule.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal alert rule metadata: %w", err)
	}
	var id string
	if err := s.pg.QueryRow(ctx, `
		INSERT INTO alert_rules (name, metric, operator, threshold, severity, window_minutes, enabled, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::uuid)
		RETURNING id::text`,
		rule.Name, rule.Metric, rule.Operator, rule.Threshold, rule.Severity, rule.WindowMinutes, rule.Enabled, metadataBytes, rule.CreatedBy,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("insert alert rule: %w", err)
	}
	return s.GetAlertRule(ctx, id)
}

func (s *Store) GetAlertRule(ctx context.Context, id string) (*AlertRule, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT id::text, name, metric, operator, threshold, severity, window_minutes, enabled,
		       COALESCE(metadata, '{}'::jsonb), COALESCE(created_by::text, ''), created_at, updated_at
		FROM alert_rules
		WHERE id = $1`, id)
	return scanAlertRule(row)
}

func (s *Store) UpdateAlertRule(ctx context.Context, rule *AlertRule) (*AlertRule, error) {
	if rule == nil || rule.ID == "" {
		return nil, fmt.Errorf("alert rule id is required")
	}
	current, err := s.GetAlertRule(ctx, rule.ID)
	if err != nil {
		return nil, err
	}
	if rule.Name == "" {
		rule.Name = current.Name
	}
	if rule.Metric == "" {
		rule.Metric = current.Metric
	}
	if rule.Operator == "" {
		rule.Operator = current.Operator
	}
	if rule.Severity == "" {
		rule.Severity = current.Severity
	}
	if rule.WindowMinutes <= 0 {
		rule.WindowMinutes = current.WindowMinutes
	}
	if rule.Metadata == nil {
		rule.Metadata = current.Metadata
	}
	metadataBytes, err := json.Marshal(rule.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal alert rule metadata: %w", err)
	}
	if _, err := s.pg.Exec(ctx, `
		UPDATE alert_rules
		SET name = $2, metric = $3, operator = $4, threshold = $5,
		    severity = $6, window_minutes = $7, enabled = $8,
		    metadata = $9, updated_at = now()
		WHERE id = $1`,
		rule.ID, rule.Name, rule.Metric, rule.Operator, rule.Threshold,
		rule.Severity, rule.WindowMinutes, rule.Enabled, metadataBytes,
	); err != nil {
		return nil, fmt.Errorf("update alert rule: %w", err)
	}
	return s.GetAlertRule(ctx, rule.ID)
}

func (s *Store) ListAlertSummaries(ctx context.Context) ([]AlertSummary, error) {
	now := time.Now().UTC()
	health, err := s.ListLatestProviderHealth(ctx)
	if err == nil {
		for _, item := range health {
			if item.Status == "healthy" || item.Status == "unknown" {
				continue
			}
			_ = s.UpsertAlertEvent(ctx, &AlertSummary{
				DedupeKey:   "provider-" + item.ProviderID,
				Severity:    map[bool]string{true: "critical", false: "warning"}[item.Status == "unhealthy"],
				Status:      "open",
				Title:       fmt.Sprintf("Provider %s is %s", item.ProviderID, item.Status),
				Description: item.ErrorMessage,
				Metadata:    map[string]interface{}{"provider_id": item.ProviderID, "latency_ms": item.LatencyMs, "error_code": item.ErrorCode},
				LastSeenAt:  item.CheckedAt,
				CreatedAt:   item.CheckedAt,
			})
		}
	}
	var recentRequests, recentErrors int64
	if err := s.pg.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status_code >= 400)
		FROM request_logs
		WHERE created_at >= now() - interval '1 hour'`).Scan(&recentRequests, &recentErrors); err == nil && recentRequests > 0 {
		rate := float64(recentErrors) / float64(recentRequests)
		if rate >= 0.05 {
			_ = s.UpsertAlertEvent(ctx, &AlertSummary{
				DedupeKey:   "error-rate-1h",
				Severity:    map[bool]string{true: "critical", false: "warning"}[rate >= 0.20],
				Status:      "open",
				Title:       "Elevated gateway error rate",
				Description: fmt.Sprintf("%.2f%% errors in the last hour", rate*100),
				Metadata:    map[string]interface{}{"requests": recentRequests, "errors": recentErrors, "error_rate": rate},
				LastSeenAt:  now,
				CreatedAt:   now,
			})
		}
	}
	return s.ListAlertEvents(ctx, 100)
}

func (s *Store) UpsertAlertEvent(ctx context.Context, alert *AlertSummary) error {
	if alert == nil {
		return nil
	}
	if alert.DedupeKey == "" {
		alert.DedupeKey = alert.ID
	}
	if alert.DedupeKey == "" {
		return fmt.Errorf("alert dedupe_key is required")
	}
	if alert.Severity == "" {
		alert.Severity = "warning"
	}
	if alert.Status == "" {
		alert.Status = "open"
	}
	if alert.LastSeenAt.IsZero() {
		alert.LastSeenAt = time.Now().UTC()
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = alert.LastSeenAt
	}
	metadataBytes, err := json.Marshal(alert.Metadata)
	if err != nil {
		return fmt.Errorf("marshal alert event metadata: %w", err)
	}
	_, err = s.pg.Exec(ctx, `
		INSERT INTO alert_events (dedupe_key, rule_id, severity, status, title, description, metadata, first_seen_at, last_seen_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, 'open', $4, NULLIF($5, ''), $6, $7, $8)
		ON CONFLICT (dedupe_key) DO UPDATE SET
			severity = EXCLUDED.severity,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			metadata = EXCLUDED.metadata,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = now()`,
		alert.DedupeKey,
		alert.RuleID,
		alert.Severity,
		alert.Title,
		alert.Description,
		metadataBytes,
		alert.CreatedAt,
		alert.LastSeenAt,
	)
	if err != nil {
		return fmt.Errorf("upsert alert event: %w", err)
	}
	return nil
}

func (s *Store) ListAlertEvents(ctx context.Context, limit int) ([]AlertSummary, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, dedupe_key, COALESCE(rule_id::text, ''), severity, status, title,
		       COALESCE(description, ''), COALESCE(metadata, '{}'::jsonb), first_seen_at, last_seen_at,
		       COALESCE(acknowledged_by::text, ''), acknowledged_at, resolved_at, created_at
		FROM alert_events
		ORDER BY
			CASE status WHEN 'open' THEN 0 WHEN 'acknowledged' THEN 1 ELSE 2 END,
			last_seen_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query alert events: %w", err)
	}
	defer rows.Close()
	alerts := make([]AlertSummary, 0)
	for rows.Next() {
		alert, err := scanAlertEvent(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, *alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert events: %w", err)
	}
	return alerts, nil
}

func (s *Store) AcknowledgeAlertEvent(ctx context.Context, id, userID string) (*AlertSummary, error) {
	if _, err := s.pg.Exec(ctx, `
		UPDATE alert_events
		SET status = 'acknowledged', acknowledged_by = NULLIF($2, '')::uuid, acknowledged_at = now(), updated_at = now()
		WHERE id = $1 AND status != 'resolved'`, id, userID); err != nil {
		return nil, fmt.Errorf("acknowledge alert event: %w", err)
	}
	row := s.pg.QueryRow(ctx, `
		SELECT id::text, dedupe_key, COALESCE(rule_id::text, ''), severity, status, title,
		       COALESCE(description, ''), COALESCE(metadata, '{}'::jsonb), first_seen_at, last_seen_at,
		       COALESCE(acknowledged_by::text, ''), acknowledged_at, resolved_at, created_at
		FROM alert_events
		WHERE id = $1`, id)
	return scanAlertEvent(row)
}

func (s *Store) ResolveAlertEvent(ctx context.Context, id, userID string) (*AlertSummary, error) {
	if _, err := s.pg.Exec(ctx, `
		UPDATE alert_events
		SET status = 'resolved',
		    acknowledged_by = COALESCE(acknowledged_by, NULLIF($2, '')::uuid),
		    acknowledged_at = COALESCE(acknowledged_at, now()),
		    resolved_at = now(),
		    updated_at = now()
		WHERE id = $1`, id, userID); err != nil {
		return nil, fmt.Errorf("resolve alert event: %w", err)
	}
	row := s.pg.QueryRow(ctx, `
		SELECT id::text, dedupe_key, COALESCE(rule_id::text, ''), severity, status, title,
		       COALESCE(description, ''), COALESCE(metadata, '{}'::jsonb), first_seen_at, last_seen_at,
		       COALESCE(acknowledged_by::text, ''), acknowledged_at, resolved_at, created_at
		FROM alert_events
		WHERE id = $1`, id)
	return scanAlertEvent(row)
}

func scanAlertRule(row fileScanner) (*AlertRule, error) {
	var rule AlertRule
	var metadataBytes []byte
	if err := row.Scan(
		&rule.ID,
		&rule.Name,
		&rule.Metric,
		&rule.Operator,
		&rule.Threshold,
		&rule.Severity,
		&rule.WindowMinutes,
		&rule.Enabled,
		&metadataBytes,
		&rule.CreatedBy,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("alert rule not found")
		}
		return nil, fmt.Errorf("scan alert rule: %w", err)
	}
	rule.Metadata = map[string]interface{}{}
	_ = json.Unmarshal(metadataBytes, &rule.Metadata)
	return &rule, nil
}

func scanAlertEvent(row fileScanner) (*AlertSummary, error) {
	var alert AlertSummary
	var metadataBytes []byte
	if err := row.Scan(
		&alert.ID,
		&alert.DedupeKey,
		&alert.RuleID,
		&alert.Severity,
		&alert.Status,
		&alert.Title,
		&alert.Description,
		&metadataBytes,
		&alert.FirstSeenAt,
		&alert.LastSeenAt,
		&alert.AcknowledgedBy,
		&alert.AcknowledgedAt,
		&alert.ResolvedAt,
		&alert.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("alert event not found")
		}
		return nil, fmt.Errorf("scan alert event: %w", err)
	}
	alert.Metadata = map[string]interface{}{}
	_ = json.Unmarshal(metadataBytes, &alert.Metadata)
	return &alert, nil
}

func (s *Store) CreateFileRecord(ctx context.Context, file *FileRecord) (*FileRecord, error) {
	if file == nil {
		return nil, fmt.Errorf("file record is required")
	}
	if file.Purpose == "" {
		file.Purpose = "assistants"
	}
	if file.Metadata == nil {
		file.Metadata = map[string]interface{}{}
	}
	metadataBytes, err := json.Marshal(file.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}
	if _, err := s.pg.Exec(ctx, `
		INSERT INTO uploaded_files (
			id, user_id, api_key_id, workspace_id, filename, purpose, bytes,
			mime_type, storage_path, status, metadata
		) VALUES (
			$1, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7,
			NULLIF($8, ''), $9, 'uploaded', $10
		)`,
		file.ID,
		file.UserID,
		file.APIKeyID,
		file.WorkspaceID,
		file.Filename,
		file.Purpose,
		file.Bytes,
		file.MimeType,
		file.StoragePath,
		metadataBytes,
	); err != nil {
		return nil, fmt.Errorf("insert file record: %w", err)
	}
	return s.GetFileRecord(ctx, file.UserID, file.ID)
}

func (s *Store) GetFileRecord(ctx context.Context, userID, fileID string) (*FileRecord, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT id, user_id::text, COALESCE(api_key_id::text, ''), COALESCE(workspace_id::text, ''),
		       filename, purpose, bytes, COALESCE(mime_type, ''), storage_path, status,
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM uploaded_files
		WHERE id = $1 AND user_id = $2::uuid AND status != 'deleted'`, fileID, userID)
	return scanFileRecord(row)
}

func (s *Store) ListFileRecords(ctx context.Context, userID, purpose string, limit int) ([]FileRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, user_id::text, COALESCE(api_key_id::text, ''), COALESCE(workspace_id::text, ''),
		       filename, purpose, bytes, COALESCE(mime_type, ''), storage_path, status,
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM uploaded_files
		WHERE user_id = $1::uuid
		  AND status != 'deleted'
		  AND ($2 = '' OR purpose = $2)
		ORDER BY created_at DESC
		LIMIT $3`, userID, purpose, limit)
	if err != nil {
		return nil, fmt.Errorf("query file records: %w", err)
	}
	defer rows.Close()
	files := make([]FileRecord, 0)
	for rows.Next() {
		file, err := scanFileRecord(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file records: %w", err)
	}
	return files, nil
}

func (s *Store) DeleteFileRecord(ctx context.Context, userID, fileID string) (*FileRecord, error) {
	record, err := s.GetFileRecord(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	tag, err := s.pg.Exec(ctx, `
		UPDATE uploaded_files
		SET status = 'deleted', updated_at = now()
		WHERE id = $1 AND user_id = $2::uuid AND status != 'deleted'`, fileID, userID)
	if err != nil {
		return nil, fmt.Errorf("delete file record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("file not found")
	}
	record.Status = "deleted"
	return record, nil
}

func (s *Store) ListExpiredFileRecords(ctx context.Context, retentionDays, limit int) ([]FileRecord, error) {
	if retentionDays <= 0 {
		return []FileRecord{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, user_id::text, COALESCE(api_key_id::text, ''), COALESCE(workspace_id::text, ''),
		       filename, purpose, bytes, COALESCE(mime_type, ''), storage_path, status,
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM uploaded_files
		WHERE status != 'deleted'
		  AND created_at < now() - make_interval(days => $1)
		ORDER BY created_at ASC
		LIMIT $2`, retentionDays, limit)
	if err != nil {
		return nil, fmt.Errorf("query expired file records: %w", err)
	}
	defer rows.Close()
	files := make([]FileRecord, 0)
	for rows.Next() {
		file, err := scanFileRecord(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired file records: %w", err)
	}
	return files, nil
}

func (s *Store) MarkFileRecordDeletedByID(ctx context.Context, fileID string) error {
	tag, err := s.pg.Exec(ctx, `
		UPDATE uploaded_files
		SET status = 'deleted', updated_at = now()
		WHERE id = $1 AND status != 'deleted'`, fileID)
	if err != nil {
		return fmt.Errorf("mark file deleted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("file not found")
	}
	return nil
}

type fileScanner interface {
	Scan(dest ...interface{}) error
}

func scanFileRecord(row fileScanner) (*FileRecord, error) {
	var file FileRecord
	var metadataBytes []byte
	if err := row.Scan(
		&file.ID,
		&file.UserID,
		&file.APIKeyID,
		&file.WorkspaceID,
		&file.Filename,
		&file.Purpose,
		&file.Bytes,
		&file.MimeType,
		&file.StoragePath,
		&file.Status,
		&metadataBytes,
		&file.CreatedAt,
		&file.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("scan file record: %w", err)
	}
	file.Object = "file"
	file.Metadata = map[string]interface{}{}
	_ = json.Unmarshal(metadataBytes, &file.Metadata)
	return &file, nil
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
		SET status = $1::varchar,
		    result_data = COALESCE($2::jsonb, result_data),
		    completed_at = CASE
		        WHEN $1::varchar IN ('completed', 'failed') THEN now()
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

func (s *Store) CreateWorkflow(ctx context.Context, wf *Workflow) (*Workflow, error) {
	metadataBytes, _ := json.Marshal(wf.Metadata)
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin workflow tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	out := &Workflow{}
	var metadataRaw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO workflows (user_id, workspace_id, name, description, status, metadata)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, COALESCE(NULLIF($5, ''), 'active'), $6)
		RETURNING id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), name, description, status, metadata, created_at, updated_at`,
		wf.UserID, wf.WorkspaceID, wf.Name, wf.Description, wf.Status, metadataBytes,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.Name, &out.Description, &out.Status, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workflow: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	for i, step := range wf.Steps {
		if step.StepOrder == 0 {
			step.StepOrder = i + 1
		}
		configBytes, _ := json.Marshal(step.Config)
		var saved WorkflowStep
		var configRaw []byte
		err = tx.QueryRow(ctx, `
			INSERT INTO workflow_steps (workflow_id, step_order, name, step_type, model_id, tool_id, prompt_template, config)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id::text, workflow_id::text, step_order, name, step_type, model_id, tool_id, prompt_template, config, created_at, updated_at`,
			out.ID, step.StepOrder, step.Name, step.StepType, step.ModelID, step.ToolID, step.PromptTemplate, configBytes,
		).Scan(&saved.ID, &saved.WorkflowID, &saved.StepOrder, &saved.Name, &saved.StepType, &saved.ModelID, &saved.ToolID, &saved.PromptTemplate, &configRaw, &saved.CreatedAt, &saved.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert workflow step: %w", err)
		}
		_ = json.Unmarshal(configRaw, &saved.Config)
		out.Steps = append(out.Steps, saved)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit workflow tx: %w", err)
	}
	return out, nil
}

func (s *Store) ListWorkflows(ctx context.Context, userID string) ([]Workflow, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), name, description, status, metadata, created_at, updated_at
		FROM workflows
		WHERE user_id = NULLIF($1, '')::uuid OR $1 = ''
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query workflows: %w", err)
	}
	defer rows.Close()
	items := []Workflow{}
	for rows.Next() {
		var item Workflow
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.WorkspaceID, &item.Name, &item.Description, &item.Status, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	wf := &Workflow{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), name, description, status, metadata, created_at, updated_at
		FROM workflows WHERE id = $1`, workflowID,
	).Scan(&wf.ID, &wf.UserID, &wf.WorkspaceID, &wf.Name, &wf.Description, &wf.Status, &metadataRaw, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, fmt.Errorf("query workflow: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &wf.Metadata)
	steps, err := s.ListWorkflowSteps(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	wf.Steps = steps
	return wf, nil
}

func (s *Store) ListWorkflowSteps(ctx context.Context, workflowID string) ([]WorkflowStep, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workflow_id::text, step_order, name, step_type, model_id, tool_id, prompt_template, config, created_at, updated_at
		FROM workflow_steps WHERE workflow_id = $1 ORDER BY step_order`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("query workflow steps: %w", err)
	}
	defer rows.Close()
	items := []WorkflowStep{}
	for rows.Next() {
		var item WorkflowStep
		var configRaw []byte
		if err := rows.Scan(&item.ID, &item.WorkflowID, &item.StepOrder, &item.Name, &item.StepType, &item.ModelID, &item.ToolID, &item.PromptTemplate, &configRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow step: %w", err)
		}
		_ = json.Unmarshal(configRaw, &item.Config)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListTools(ctx context.Context) ([]Tool, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, display_name, description, tool_type, schema, is_enabled, created_at, updated_at FROM tools ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query tools: %w", err)
	}
	defer rows.Close()
	items := []Tool{}
	for rows.Next() {
		var item Tool
		var schemaRaw []byte
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Description, &item.ToolType, &schemaRaw, &item.IsEnabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tool: %w", err)
		}
		_ = json.Unmarshal(schemaRaw, &item.Schema)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateToolCredential(ctx context.Context, item *ToolCredential) (*ToolCredential, error) {
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &ToolCredential{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO tool_credentials (
			user_id, workspace_id, tool_id, name, secret_encrypted, secret_mask, metadata, status
		) VALUES (
			NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, COALESCE(NULLIF($8, ''), 'active')
		)
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), tool_id, name, secret_mask, metadata, status, last_used_at, created_at, updated_at`,
		item.UserID, item.WorkspaceID, item.ToolID, item.Name, item.SecretEncrypted, item.SecretMask, metadataBytes, item.Status,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.ToolID, &out.Name, &out.SecretMask, &metadataRaw, &out.Status, &out.LastUsedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert tool credential: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ListToolCredentials(ctx context.Context, userID string) ([]ToolCredential, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, user_id::text, COALESCE(workspace_id::text, ''), tool_id, name, secret_mask, metadata, status, last_used_at, created_at, updated_at
		FROM tool_credentials
		WHERE user_id = NULLIF($1, '')::uuid
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query tool credentials: %w", err)
	}
	defer rows.Close()
	items := []ToolCredential{}
	for rows.Next() {
		var item ToolCredential
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.WorkspaceID, &item.ToolID, &item.Name, &item.SecretMask, &metadataRaw, &item.Status, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tool credential: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RevokeToolCredential(ctx context.Context, credentialID, userID string) (*ToolCredential, error) {
	out := &ToolCredential{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		UPDATE tool_credentials
		SET status='revoked'
		WHERE id=$1 AND user_id = NULLIF($2, '')::uuid
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), tool_id, name, secret_mask, metadata, status, last_used_at, created_at, updated_at`,
		credentialID, userID,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.ToolID, &out.Name, &out.SecretMask, &metadataRaw, &out.Status, &out.LastUsedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("tool credential not found")
		}
		return nil, fmt.Errorf("revoke tool credential: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) CreateAgentSession(ctx context.Context, item *AgentSession) (*AgentSession, error) {
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &AgentSession{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO agent_sessions (user_id, workspace_id, workflow_id, name, status, metadata)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, COALESCE(NULLIF($5, ''), 'active'), $6)
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), COALESCE(workflow_id::text, ''), name, status, metadata,
		          COALESCE(last_run_id::text, ''), last_activity_at, created_at, updated_at`,
		item.UserID, item.WorkspaceID, item.WorkflowID, item.Name, item.Status, metadataBytes,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.WorkflowID, &out.Name, &out.Status, &metadataRaw, &out.LastRunID, &out.LastActivityAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert agent session: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ListAgentSessions(ctx context.Context, userID string, limit int) ([]AgentSession, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, user_id::text, COALESCE(workspace_id::text, ''), COALESCE(workflow_id::text, ''), name, status, metadata,
		       COALESCE(last_run_id::text, ''), last_activity_at, created_at, updated_at
		FROM agent_sessions
		WHERE user_id = NULLIF($1, '')::uuid
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query agent sessions: %w", err)
	}
	defer rows.Close()
	items := []AgentSession{}
	for rows.Next() {
		var item AgentSession
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.WorkspaceID, &item.WorkflowID, &item.Name, &item.Status, &metadataRaw, &item.LastRunID, &item.LastActivityAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent session: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetAgentSession(ctx context.Context, sessionID, userID string) (*AgentSession, error) {
	out := &AgentSession{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, user_id::text, COALESCE(workspace_id::text, ''), COALESCE(workflow_id::text, ''), name, status, metadata,
		       COALESCE(last_run_id::text, ''), last_activity_at, created_at, updated_at
		FROM agent_sessions
		WHERE id=$1 AND user_id = NULLIF($2, '')::uuid`, sessionID, userID,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.WorkflowID, &out.Name, &out.Status, &metadataRaw, &out.LastRunID, &out.LastActivityAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent session not found")
		}
		return nil, fmt.Errorf("query agent session: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) CloseAgentSession(ctx context.Context, sessionID, userID string) (*AgentSession, error) {
	out := &AgentSession{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		UPDATE agent_sessions
		SET status='closed'
		WHERE id=$1 AND user_id = NULLIF($2, '')::uuid
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), COALESCE(workflow_id::text, ''), name, status, metadata,
		          COALESCE(last_run_id::text, ''), last_activity_at, created_at, updated_at`,
		sessionID, userID,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.WorkflowID, &out.Name, &out.Status, &metadataRaw, &out.LastRunID, &out.LastActivityAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent session not found")
		}
		return nil, fmt.Errorf("close agent session: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) TouchAgentSessionRun(ctx context.Context, sessionID, runID string) error {
	if sessionID == "" {
		return nil
	}
	_, err := s.pg.Exec(ctx, `
		UPDATE agent_sessions
		SET last_run_id=$2, last_activity_at=now()
		WHERE id=$1`, sessionID, runID)
	if err != nil {
		return fmt.Errorf("touch agent session run: %w", err)
	}
	return nil
}

func (s *Store) CreatePromptTemplate(ctx context.Context, item *PromptTemplate) (*PromptTemplate, error) {
	variablesBytes, _ := json.Marshal(item.Variables)
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &PromptTemplate{}
	var variablesRaw, metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO prompt_templates (user_id, workspace_id, name, description, template, variables, metadata, status)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, COALESCE(NULLIF($8, ''), 'active'))
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), name, description, template, variables, metadata, status, created_at, updated_at`,
		item.UserID, item.WorkspaceID, item.Name, item.Description, item.Template, variablesBytes, metadataBytes, item.Status,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.Name, &out.Description, &out.Template, &variablesRaw, &metadataRaw, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert prompt template: %w", err)
	}
	_ = json.Unmarshal(variablesRaw, &out.Variables)
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ListPromptTemplates(ctx context.Context, userID string, limit int) ([]PromptTemplate, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, user_id::text, COALESCE(workspace_id::text, ''), name, description, template, variables, metadata, status, created_at, updated_at
		FROM prompt_templates
		WHERE user_id = NULLIF($1, '')::uuid
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query prompt templates: %w", err)
	}
	defer rows.Close()
	items := []PromptTemplate{}
	for rows.Next() {
		var item PromptTemplate
		var variablesRaw, metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.WorkspaceID, &item.Name, &item.Description, &item.Template, &variablesRaw, &metadataRaw, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt template: %w", err)
		}
		_ = json.Unmarshal(variablesRaw, &item.Variables)
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetPromptTemplate(ctx context.Context, templateID, userID string) (*PromptTemplate, error) {
	out := &PromptTemplate{}
	var variablesRaw, metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, user_id::text, COALESCE(workspace_id::text, ''), name, description, template, variables, metadata, status, created_at, updated_at
		FROM prompt_templates
		WHERE id=$1 AND user_id = NULLIF($2, '')::uuid`, templateID, userID,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.Name, &out.Description, &out.Template, &variablesRaw, &metadataRaw, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("prompt template not found")
		}
		return nil, fmt.Errorf("query prompt template: %w", err)
	}
	_ = json.Unmarshal(variablesRaw, &out.Variables)
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ArchivePromptTemplate(ctx context.Context, templateID, userID string) (*PromptTemplate, error) {
	out := &PromptTemplate{}
	var variablesRaw, metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		UPDATE prompt_templates
		SET status='archived'
		WHERE id=$1 AND user_id = NULLIF($2, '')::uuid
		RETURNING id::text, user_id::text, COALESCE(workspace_id::text, ''), name, description, template, variables, metadata, status, created_at, updated_at`,
		templateID, userID,
	).Scan(&out.ID, &out.UserID, &out.WorkspaceID, &out.Name, &out.Description, &out.Template, &variablesRaw, &metadataRaw, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("prompt template not found")
		}
		return nil, fmt.Errorf("archive prompt template: %w", err)
	}
	_ = json.Unmarshal(variablesRaw, &out.Variables)
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) CreateWorkflowRun(ctx context.Context, run *WorkflowRun) (*WorkflowRun, error) {
	inputBytes, _ := json.Marshal(run.Input)
	now := time.Now().UTC()
	out := &WorkflowRun{}
	var inputRaw, outputRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO workflow_runs (workflow_id, user_id, workspace_id, agent_session_id, status, input, started_at)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, 'running', $5, $6)
		RETURNING id::text, workflow_id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(agent_session_id::text, ''), status, input, output, total_cost_usd, started_at, completed_at, created_at`,
		run.WorkflowID, run.UserID, run.WorkspaceID, run.AgentSessionID, inputBytes, now,
	).Scan(&out.ID, &out.WorkflowID, &out.UserID, &out.WorkspaceID, &out.AgentSessionID, &out.Status, &inputRaw, &outputRaw, &out.TotalCostUSD, &out.StartedAt, &out.CompletedAt, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workflow run: %w", err)
	}
	_ = json.Unmarshal(inputRaw, &out.Input)
	_ = json.Unmarshal(outputRaw, &out.Output)
	return out, nil
}

func (s *Store) RecordWorkflowRunStep(ctx context.Context, step *WorkflowRunStep) (*WorkflowRunStep, error) {
	inputBytes := mustJSONBytes(step.Input)
	outputBytes := mustJSONBytes(step.Output)
	out := &WorkflowRunStep{}
	var inputRaw, outputRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO workflow_run_steps (run_id, workflow_step_id, step_order, name, step_type, status, input, output, latency_ms, cost_usd, error_message)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id::text, run_id::text, COALESCE(workflow_step_id::text, ''), step_order, name, step_type, status, input, output, latency_ms, cost_usd, error_message, created_at`,
		step.RunID, step.WorkflowStepID, step.StepOrder, step.Name, step.StepType, step.Status, inputBytes, outputBytes, step.LatencyMs, step.CostUSD, step.ErrorMessage,
	).Scan(&out.ID, &out.RunID, &out.WorkflowStepID, &out.StepOrder, &out.Name, &out.StepType, &out.Status, &inputRaw, &outputRaw, &out.LatencyMs, &out.CostUSD, &out.ErrorMessage, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workflow run step: %w", err)
	}
	_ = json.Unmarshal(inputRaw, &out.Input)
	_ = json.Unmarshal(outputRaw, &out.Output)
	return out, nil
}

func (s *Store) CompleteWorkflowRun(ctx context.Context, runID, status string, output map[string]interface{}, totalCost float64) error {
	outputBytes := mustJSONBytes(output)
	_, err := s.pg.Exec(ctx, `UPDATE workflow_runs SET status=$2, output=$3, total_cost_usd=$4, completed_at=now() WHERE id=$1`, runID, status, outputBytes, totalCost)
	if err != nil {
		return fmt.Errorf("complete workflow run: %w", err)
	}
	return nil
}

func mustJSONBytes(value interface{}) []byte {
	bytes, err := json.Marshal(value)
	if err != nil || bytes == nil || string(bytes) == "null" {
		return []byte("{}")
	}
	return bytes
}

func (s *Store) RecordAgentTrace(ctx context.Context, trace *AgentTrace) error {
	dataBytes, _ := json.Marshal(trace.Data)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO agent_traces (run_id, step_id, trace_type, message, data)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5)`,
		trace.RunID, trace.StepID, trace.TraceType, trace.Message, dataBytes)
	if err != nil {
		return fmt.Errorf("insert agent trace: %w", err)
	}
	return nil
}

func (s *Store) GetWorkflowRun(ctx context.Context, runID string) (*WorkflowRun, error) {
	run := &WorkflowRun{}
	var inputRaw, outputRaw []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id::text, workflow_id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(agent_session_id::text, ''), status, input, output, total_cost_usd, started_at, completed_at, created_at
		FROM workflow_runs WHERE id=$1`, runID,
	).Scan(&run.ID, &run.WorkflowID, &run.UserID, &run.WorkspaceID, &run.AgentSessionID, &run.Status, &inputRaw, &outputRaw, &run.TotalCostUSD, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("query workflow run: %w", err)
	}
	_ = json.Unmarshal(inputRaw, &run.Input)
	_ = json.Unmarshal(outputRaw, &run.Output)
	run.Steps, _ = s.ListWorkflowRunSteps(ctx, runID)
	run.Traces, _ = s.ListAgentTraces(ctx, runID)
	run.Webhooks, _ = s.ListWebhookDeliveries(ctx, runID)
	return run, nil
}

func (s *Store) ListWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]WorkflowRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workflow_id::text, COALESCE(user_id::text, ''), COALESCE(workspace_id::text, ''), COALESCE(agent_session_id::text, ''), status, input, output, total_cost_usd, started_at, completed_at, created_at
		FROM workflow_runs WHERE workflow_id=$1 ORDER BY created_at DESC LIMIT $2`, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("query workflow runs: %w", err)
	}
	defer rows.Close()
	items := []WorkflowRun{}
	for rows.Next() {
		var item WorkflowRun
		var inputRaw, outputRaw []byte
		if err := rows.Scan(&item.ID, &item.WorkflowID, &item.UserID, &item.WorkspaceID, &item.AgentSessionID, &item.Status, &inputRaw, &outputRaw, &item.TotalCostUSD, &item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		_ = json.Unmarshal(inputRaw, &item.Input)
		_ = json.Unmarshal(outputRaw, &item.Output)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListWorkflowRunSteps(ctx context.Context, runID string) ([]WorkflowRunStep, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, run_id::text, COALESCE(workflow_step_id::text, ''), step_order, name, step_type, status, input, output, latency_ms, cost_usd, error_message, created_at
		FROM workflow_run_steps WHERE run_id=$1 ORDER BY step_order`, runID)
	if err != nil {
		return nil, fmt.Errorf("query workflow run steps: %w", err)
	}
	defer rows.Close()
	items := []WorkflowRunStep{}
	for rows.Next() {
		var item WorkflowRunStep
		var inputRaw, outputRaw []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.WorkflowStepID, &item.StepOrder, &item.Name, &item.StepType, &item.Status, &inputRaw, &outputRaw, &item.LatencyMs, &item.CostUSD, &item.ErrorMessage, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow run step: %w", err)
		}
		_ = json.Unmarshal(inputRaw, &item.Input)
		_ = json.Unmarshal(outputRaw, &item.Output)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAgentTraces(ctx context.Context, runID string) ([]AgentTrace, error) {
	rows, err := s.pg.Query(ctx, `SELECT id::text, run_id::text, COALESCE(step_id::text, ''), trace_type, message, data, created_at FROM agent_traces WHERE run_id=$1 ORDER BY created_at`, runID)
	if err != nil {
		return nil, fmt.Errorf("query agent traces: %w", err)
	}
	defer rows.Close()
	items := []AgentTrace{}
	for rows.Next() {
		var item AgentTrace
		var dataRaw []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.StepID, &item.TraceType, &item.Message, &dataRaw, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent trace: %w", err)
		}
		_ = json.Unmarshal(dataRaw, &item.Data)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RecordWebhookDelivery(ctx context.Context, delivery *WebhookDelivery) error {
	if delivery == nil {
		return nil
	}
	if delivery.Payload == nil {
		delivery.Payload = map[string]interface{}{}
	}
	payloadBytes := mustJSONBytes(delivery.Payload)
	if delivery.MaxAttempts <= 0 {
		delivery.MaxAttempts = 3
	}
	err := s.pg.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (
			workflow_id, run_id, callback_url, event_type, status, response_status, error_message, payload, max_attempts
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING id::text, created_at, updated_at`,
		delivery.WorkflowID,
		delivery.RunID,
		delivery.CallbackURL,
		delivery.EventType,
		delivery.Status,
		delivery.ResponseStatus,
		delivery.ErrorMessage,
		payloadBytes,
		delivery.MaxAttempts,
	).Scan(&delivery.ID, &delivery.CreatedAt, &delivery.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert webhook delivery: %w", err)
	}
	return nil
}

func (s *Store) UpdateWebhookDeliveryResult(ctx context.Context, deliveryID, status string, responseStatus, attemptCount int, responseBody, errorMessage, signature string) error {
	deliveredExpr := "NULL"
	if status == "delivered" {
		deliveredExpr = "now()"
	}
	tag, err := s.pg.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $2,
		    response_status = $3,
		    attempt_count = $4,
		    response_body = LEFT($5, 4000),
		    error_message = LEFT($6, 1000),
		    signature = $7,
		    delivered_at = `+deliveredExpr+`,
		    updated_at = now()
		WHERE id = $1::uuid`,
		deliveryID, status, responseStatus, attemptCount, responseBody, errorMessage, signature)
	if err != nil {
		return fmt.Errorf("update webhook delivery: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook delivery not found")
	}
	return nil
}

func (s *Store) ListWebhookDeliveries(ctx context.Context, runID string) ([]WebhookDelivery, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, workflow_id::text, run_id::text, callback_url, event_type, status,
		       response_status, COALESCE(error_message, ''), attempt_count, max_attempts,
		       COALESCE(response_body, ''), COALESCE(signature, ''), delivered_at,
		       payload, created_at, updated_at
		FROM webhook_deliveries
		WHERE run_id = $1
		ORDER BY created_at DESC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query webhook deliveries: %w", err)
	}
	defer rows.Close()
	items := []WebhookDelivery{}
	for rows.Next() {
		var item WebhookDelivery
		var payloadRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.WorkflowID,
			&item.RunID,
			&item.CallbackURL,
			&item.EventType,
			&item.Status,
			&item.ResponseStatus,
			&item.ErrorMessage,
			&item.AttemptCount,
			&item.MaxAttempts,
			&item.ResponseBody,
			&item.Signature,
			&item.DeliveredAt,
			&payloadRaw,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}
		_ = json.Unmarshal(payloadRaw, &item.Payload)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateInferenceCluster(ctx context.Context, item *InferenceCluster) (*InferenceCluster, error) {
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &InferenceCluster{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO inference_clusters (name, region, network_mode, status, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, name, region, network_mode, status, metadata, created_at, updated_at`,
		item.Name, item.Region, item.NetworkMode, item.Status, metadataBytes,
	).Scan(&out.ID, &out.Name, &out.Region, &out.NetworkMode, &out.Status, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert inference cluster: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ListInferenceClusters(ctx context.Context) ([]InferenceCluster, error) {
	rows, err := s.pg.Query(ctx, `SELECT id::text, name, region, network_mode, status, metadata, created_at, updated_at FROM inference_clusters ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query inference clusters: %w", err)
	}
	defer rows.Close()
	items := []InferenceCluster{}
	for rows.Next() {
		var item InferenceCluster
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Region, &item.NetworkMode, &item.Status, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan inference cluster: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateInferenceNode(ctx context.Context, item *InferenceNode) (*InferenceNode, error) {
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &InferenceNode{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO inference_nodes (cluster_id, name, endpoint_url, gpu_type, gpu_count, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, cluster_id::text, name, endpoint_url, gpu_type, gpu_count, status, metadata, created_at, updated_at`,
		item.ClusterID, item.Name, item.EndpointURL, item.GPUType, item.GPUCount, item.Status, metadataBytes,
	).Scan(&out.ID, &out.ClusterID, &out.Name, &out.EndpointURL, &out.GPUType, &out.GPUCount, &out.Status, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert inference node: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)
	return out, nil
}

func (s *Store) ListInferenceNodes(ctx context.Context, clusterID string) ([]InferenceNode, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, cluster_id::text, name, endpoint_url, gpu_type, gpu_count, status, metadata, created_at, updated_at
		FROM inference_nodes
		WHERE cluster_id = $1 OR $1 = ''
		ORDER BY created_at DESC`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("query inference nodes: %w", err)
	}
	defer rows.Close()
	items := []InferenceNode{}
	for rows.Next() {
		var item InferenceNode
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.Name, &item.EndpointURL, &item.GPUType, &item.GPUCount, &item.Status, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan inference node: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateModelDeployment(ctx context.Context, item *ModelDeployment) (*ModelDeployment, error) {
	metadataBytes := mustJSONBytes(item.Metadata)
	out := &ModelDeployment{}
	var metadataRaw []byte
	err := s.pg.QueryRow(ctx, `
		INSERT INTO model_deployments (cluster_id, provider_id, model_id, upstream_model, runtime, endpoint_url, replicas, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider_id, model_id) DO UPDATE SET
			cluster_id=EXCLUDED.cluster_id, upstream_model=EXCLUDED.upstream_model, runtime=EXCLUDED.runtime,
			endpoint_url=EXCLUDED.endpoint_url, replicas=EXCLUDED.replicas, status=EXCLUDED.status,
			metadata=EXCLUDED.metadata, updated_at=now()
		RETURNING id::text, cluster_id::text, provider_id, model_id, upstream_model, runtime, endpoint_url, replicas, status, metadata, created_at, updated_at`,
		item.ClusterID, item.ProviderID, item.ModelID, item.UpstreamModel, item.Runtime, item.EndpointURL, item.Replicas, item.Status, metadataBytes,
	).Scan(&out.ID, &out.ClusterID, &out.ProviderID, &out.ModelID, &out.UpstreamModel, &out.Runtime, &out.EndpointURL, &out.Replicas, &out.Status, &metadataRaw, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert model deployment: %w", err)
	}
	_ = json.Unmarshal(metadataRaw, &out.Metadata)

	adapterType := "openai_compatible"
	if out.Runtime == "vllm" || out.Runtime == "sglang" {
		adapterType = "self_hosted"
	}
	if _, err := s.UpsertProviderAdmin(ctx, &Provider{ID: out.ProviderID, DisplayName: out.ProviderID, AdapterType: adapterType, BaseURL: out.EndpointURL, Config: map[string]interface{}{"runtime": out.Runtime}, IsEnabled: out.Status == "active"}); err != nil {
		return nil, err
	}
	if _, err := s.UpsertModelAdmin(ctx, &Model{ModelID: out.ModelID, DisplayName: out.ModelID, Modality: "text", Capabilities: []string{"chat", "self_hosted"}, PriceUnit: "per_1k_tokens", SupportsStream: true, Status: "active", Tags: []string{"self-hosted", out.Runtime}, Metadata: map[string]interface{}{"deployment_id": out.ID}}); err != nil {
		return nil, err
	}
	if _, err := s.UpsertModelProviderAdmin(ctx, out.ModelID, ProviderBinding{ProviderID: out.ProviderID, Priority: 1, UpstreamModel: out.UpstreamModel, CostMultiplier: 1, TimeoutMs: 60000, MaxRetries: 1, IsEnabled: out.Status == "active"}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListModelDeployments(ctx context.Context) ([]ModelDeployment, error) {
	rows, err := s.pg.Query(ctx, `SELECT id::text, cluster_id::text, provider_id, model_id, upstream_model, runtime, endpoint_url, replicas, status, metadata, created_at, updated_at FROM model_deployments ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query model deployments: %w", err)
	}
	defer rows.Close()
	items := []ModelDeployment{}
	for rows.Next() {
		var item ModelDeployment
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.ProviderID, &item.ModelID, &item.UpstreamModel, &item.Runtime, &item.EndpointURL, &item.Replicas, &item.Status, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan model deployment: %w", err)
		}
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}
