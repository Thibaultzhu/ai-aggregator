package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ai-aggregator/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Middleware struct {
	store     *storage.Store
	jwtSecret []byte
	keyPrefix string
}

type JWTClaims struct {
	UserID string
	Role   string
}

func NewMiddleware(store *storage.Store, jwtSecret, keyPrefix string) *Middleware {
	return &Middleware{
		store:     store,
		jwtSecret: []byte(jwtSecret),
		keyPrefix: keyPrefix,
	}
}

// RequireAuth validates either API Key (Bearer sk-aggr-xxx) or JWT.
// On success, sets user_id and auth_type in fiber.Locals.
func (m *Middleware) RequireAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "missing Authorization header",
				"type":    "authentication_error",
				"code":    "missing_auth",
			},
		})
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "invalid Authorization format, expected: Bearer <token>",
				"type":    "authentication_error",
				"code":    "invalid_auth_format",
			},
		})
	}

	token := parts[1]

	// API Key auth
	if strings.HasPrefix(token, m.keyPrefix) {
		return m.authenticateAPIKey(c, token)
	}

	// JWT auth
	return m.authenticateJWT(c, token)
}

// RequireJWT requires a valid JWT token (for user portal).
func (m *Middleware) RequireJWT(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(errorAuth("missing Authorization header"))
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(401).JSON(errorAuth("invalid Authorization format"))
	}

	return m.authenticateJWT(c, parts[1])
}

// RequireAdmin requires admin role JWT.
func (m *Middleware) RequireAdmin(c *fiber.Ctx) error {
	if err := m.RequireJWT(c); err != nil {
		return err
	}

	role := c.Locals("user_role")
	if role != "admin" && role != "super_admin" {
		return c.Status(403).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "admin access required",
				"type":    "permission_denied",
				"code":    "admin_required",
			},
		})
	}
	return nil
}

// ===== Internal =====

func (m *Middleware) authenticateAPIKey(c *fiber.Ctx, key string) error {
	// Hash the key and look it up
	keyHash := hashAPIKey(key)
	userID, keyID, workspaceID, projectID, perms, rateLimitRPM, rateLimitTPM, err := m.store.ValidateAPIKey(c.UserContext(), keyHash)
	if err != nil {
		return c.Status(401).JSON(errorAuth("invalid or expired API key"))
	}

	c.Locals("user_id", userID)
	c.Locals("api_key_id", keyID)
	c.Locals("workspace_id", workspaceID)
	c.Locals("project_id", projectID)
	c.Locals("auth_type", "api_key")
	c.Locals("permissions", perms)
	if rateLimitRPM > 0 {
		c.Locals("rate_limit_rpm", rateLimitRPM)
	}
	if rateLimitTPM > 0 {
		c.Locals("rate_limit_tpm", rateLimitTPM)
	}
	return c.Next()
}

func (m *Middleware) authenticateJWT(c *fiber.Ctx, tokenStr string) error {
	claims, err := ValidateJWT(m.jwtSecret, tokenStr)
	if err != nil {
		return c.Status(401).JSON(errorAuth("invalid or expired JWT"))
	}

	c.Locals("user_id", claims.UserID)
	c.Locals("user_role", claims.Role)
	c.Locals("auth_type", "jwt")
	return c.Next()
}

func ValidateJWT(secret []byte, tokenStr string) (*JWTClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired JWT")
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid JWT claims")
	}

	userID, _ := mapClaims["uid"].(string)
	role, _ := mapClaims["role"].(string)
	if userID == "" || role == "" {
		return nil, fmt.Errorf("invalid JWT claims")
	}
	return &JWTClaims{UserID: userID, Role: role}, nil
}

// ===== Helpers =====

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h)
}

func errorAuth(msg string) fiber.Map {
	return fiber.Map{
		"error": fiber.Map{
			"message": msg,
			"type":    "authentication_error",
		},
	}
}

// GetUserID extracts user_id from fiber context (set by auth middleware).
func GetUserID(c *fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok {
		return uid
	}
	return ""
}

// GenerateAPIKey creates a new API key with the given prefix.
// Returns the plaintext key (shown once to user), its SHA-256 hash (stored in DB), and any error.
func GenerateAPIKey(prefix string) (plainKey, hash string, err error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("crypto/rand read: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)
	plainKey = prefix + randomHex
	h := sha256.Sum256([]byte(plainKey))
	hash = hex.EncodeToString(h[:])
	return plainKey, hash, nil
}

// GenerateJWT creates a signed JWT for a user with the given claims.
// Claims include uid (userID), role, iat (issued at), and exp (expiration).
func GenerateJWT(secret []byte, userID, role string, expiresHours int) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":  userID,
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(time.Duration(expiresHours) * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
