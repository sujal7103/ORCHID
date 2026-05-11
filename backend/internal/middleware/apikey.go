package middleware

import (
	"clara-agents/internal/models"
	"clara-agents/internal/services"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// APIKeyMiddleware validates API keys for programmatic access
// This middleware checks the X-API-Key header and validates the key
func APIKeyMiddleware(apiKeyService *services.APIKeyService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get API key from header
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing API key. Include X-API-Key header.",
			})
		}

		// Validate the key
		key, err := apiKeyService.ValidateKey(c.Context(), apiKey)
		if err != nil {
			log.Printf("❌ [APIKEY-AUTH] Invalid key attempt: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired API key",
			})
		}

		// Check if revoked
		if key.IsRevoked() {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key has been revoked",
			})
		}

		// Store key info in context for handlers
		c.Locals("api_key", key)
		c.Locals("user_id", key.UserID)
		c.Locals("auth_type", "api_key")

		log.Printf("🔑 [APIKEY-AUTH] Authenticated via API key %s (user: %s)", key.KeyPrefix, key.UserID)

		return c.Next()
	}
}

// APIKeyOrJWTMiddleware allows authentication via either API key or JWT
// Checks API key first, falls back to JWT
func APIKeyOrJWTMiddleware(apiKeyService *services.APIKeyService, jwtMiddleware fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check for API key first
		apiKey := c.Get("X-API-Key")
		if apiKey != "" {
			// Validate the key
			key, err := apiKeyService.ValidateKey(c.Context(), apiKey)
			if err != nil {
				log.Printf("❌ [APIKEY-AUTH] Invalid key: %v", err)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid or expired API key",
				})
			}

			if key.IsRevoked() {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "API key has been revoked",
				})
			}

			// Authenticated via API key
			c.Locals("api_key", key)
			c.Locals("user_id", key.UserID)
			c.Locals("auth_type", "api_key")

			log.Printf("🔑 [APIKEY-AUTH] Authenticated via API key %s", key.KeyPrefix)
			return c.Next()
		}

		// Fall back to JWT middleware
		return jwtMiddleware(c)
	}
}

// RequireScope middleware checks if the authenticated API key has a specific scope
func RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if authenticated via API key
		authType, _ := c.Locals("auth_type").(string)
		if authType != "api_key" {
			// JWT auth - allow through (JWT has full access)
			return c.Next()
		}

		// Get API key from context
		key, ok := c.Locals("api_key").(*models.APIKey)
		if !ok {
			// Fallback - allow through
			return c.Next()
		}

		// Check if key has required scope
		if !key.HasScope(scope) {
			log.Printf("🚫 [APIKEY-AUTH] Scope denied: %s (has: %v)", scope, key.Scopes)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "API key does not have required permission: " + scope,
			})
		}

		return c.Next()
	}
}

// RequireExecuteScope middleware checks if the API key can execute a specific agent
func RequireExecuteScope(agentIDParam string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if authenticated via API key
		authType, _ := c.Locals("auth_type").(string)
		if authType != "api_key" {
			// JWT auth - allow through
			return c.Next()
		}

		// Get agent ID from params
		agentID := c.Params(agentIDParam)
		if agentID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Missing agent ID",
			})
		}

		// Get API key from context
		key, ok := c.Locals("api_key").(*models.APIKey)
		if !ok {
			return c.Next()
		}

		// Check if key can execute this agent
		if !key.HasExecuteScope(agentID) {
			log.Printf("🚫 [APIKEY-AUTH] Execute denied for agent %s (has: %v)", agentID, key.Scopes)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "API key cannot execute this agent",
			})
		}

		return c.Next()
	}
}

// RateLimitByAPIKey applies rate limiting based on API key limits
func RateLimitByAPIKey(redisService *services.RedisService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to API key auth
		authType, _ := c.Locals("auth_type").(string)
		if authType != "api_key" {
			return c.Next()
		}

		// Get API key from context
		key, ok := c.Locals("api_key").(*models.APIKey)
		if !ok || redisService == nil {
			return c.Next()
		}

		// Get rate limits
		var perMinute, perHour int64 = 60, 1000 // Defaults
		if key.RateLimit != nil {
			perMinute = key.RateLimit.RequestsPerMinute
			perHour = key.RateLimit.RequestsPerHour
		}

		// Check rate limits using Redis
		keyPrefix := key.KeyPrefix
		ctx := c.Context()

		// Check per-minute limit
		minuteKey := "ratelimit:minute:" + keyPrefix
		count, err := redisService.Incr(ctx, minuteKey)
		if err != nil {
			log.Printf("⚠️ [RATE-LIMIT] Redis error: %v", err)
			return c.Next() // Allow on error
		}

		if count == 1 {
			// First request, set expiry
			redisService.Expire(ctx, minuteKey, 60)
		}

		if count > perMinute {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Rate limit exceeded (per minute)",
				"retry_after": "60 seconds",
			})
		}

		// Check per-hour limit
		hourKey := "ratelimit:hour:" + keyPrefix
		hourCount, err := redisService.Incr(ctx, hourKey)
		if err != nil {
			return c.Next()
		}

		if hourCount == 1 {
			redisService.Expire(ctx, hourKey, 3600)
		}

		if hourCount > perHour {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Rate limit exceeded (per hour)",
				"retry_after": "3600 seconds",
			})
		}

		// Add rate limit headers
		c.Set("X-RateLimit-Limit-Minute", formatInt64(perMinute))
		c.Set("X-RateLimit-Remaining-Minute", formatInt64(max(0, perMinute-count)))
		c.Set("X-RateLimit-Limit-Hour", formatInt64(perHour))
		c.Set("X-RateLimit-Remaining-Hour", formatInt64(max(0, perHour-hourCount)))

		return c.Next()
	}
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
