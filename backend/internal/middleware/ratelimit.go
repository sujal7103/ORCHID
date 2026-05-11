package middleware

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	// Global limits (per IP)
	GlobalAPIMax        int           // Max requests per minute for all API endpoints
	GlobalAPIExpiration time.Duration // Expiration window

	// Public endpoint limits (per IP) - read-only, cacheable
	PublicReadMax        int
	PublicReadExpiration time.Duration

	// Authenticated endpoint limits (per user ID)
	AuthenticatedMax        int
	AuthenticatedExpiration time.Duration

	// Heavy operation limits
	TranscribeMax        int
	TranscribeExpiration time.Duration

	// WebSocket/Connection limits (per IP)
	WebSocketMax        int
	WebSocketExpiration time.Duration

	// Image proxy limits (per IP) - can be abused for bandwidth
	ImageProxyMax        int
	ImageProxyExpiration time.Duration
}

// DefaultRateLimitConfig returns production-safe defaults
// These are designed to prevent abuse while avoiding false positives
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		// Global: 200/min = ~3.3 req/sec - very generous for normal use
		GlobalAPIMax:        200,
		GlobalAPIExpiration: 1 * time.Minute,

		// Public read endpoints: 120/min = 2 req/sec
		PublicReadMax:        120,
		PublicReadExpiration: 1 * time.Minute,

		// Authenticated operations: 60/min = 1 req/sec average
		AuthenticatedMax:        60,
		AuthenticatedExpiration: 1 * time.Minute,

		// Transcription: 10/min (expensive GPU operation)
		TranscribeMax:        10,
		TranscribeExpiration: 1 * time.Minute,

		// WebSocket: 20 connections/min in production
		WebSocketMax:        20,
		WebSocketExpiration: 1 * time.Minute,

		// Image proxy: 60/min (bandwidth protection)
		ImageProxyMax:        60,
		ImageProxyExpiration: 1 * time.Minute,
	}
}

// LoadRateLimitConfig loads config from environment variables with defaults
func LoadRateLimitConfig() *RateLimitConfig {
	config := DefaultRateLimitConfig()

	// Allow environment overrides for tuning
	if v := os.Getenv("RATE_LIMIT_GLOBAL_API"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.GlobalAPIMax = n
		}
	}

	if v := os.Getenv("RATE_LIMIT_PUBLIC_READ"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.PublicReadMax = n
		}
	}

	if v := os.Getenv("RATE_LIMIT_AUTHENTICATED"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.AuthenticatedMax = n
		}
	}

	if v := os.Getenv("RATE_LIMIT_WEBSOCKET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.WebSocketMax = n
		}
	}

	if v := os.Getenv("RATE_LIMIT_IMAGE_PROXY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.ImageProxyMax = n
		}
	}

	// Development mode: more lenient limits
	if os.Getenv("ENVIRONMENT") == "development" {
		config.GlobalAPIMax = 1000       // Very high for dev
		config.WebSocketMax = 100        // Keep high for dev
		config.ImageProxyMax = 200       // More lenient
		log.Println("⚠️  [RATE-LIMIT] Development mode: using relaxed rate limits")
	}

	return config
}

// GlobalAPIRateLimiter creates a rate limiter for all API requests
// This is the first line of defense against DDoS
func GlobalAPIRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.GlobalAPIMax,
		Expiration: config.GlobalAPIExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "global:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("🚫 [RATE-LIMIT] Global limit reached for IP: %s", c.IP())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Too many requests. Please slow down.",
				"retry_after": int(config.GlobalAPIExpiration.Seconds()),
			})
		},
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
	})
}

// PublicReadRateLimiter for public read-only endpoints
func PublicReadRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.PublicReadMax,
		Expiration: config.PublicReadExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "public:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("⚠️  [RATE-LIMIT] Public endpoint limit reached for IP: %s on %s", c.IP(), c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Too many requests to this endpoint.",
				"retry_after": int(config.PublicReadExpiration.Seconds()),
			})
		},
	})
}

// AuthenticatedRateLimiter for authenticated endpoints (uses user ID)
func AuthenticatedRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.AuthenticatedMax,
		Expiration: config.AuthenticatedExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			// Use user ID if available, fall back to IP
			if userID, ok := c.Locals("user_id").(string); ok && userID != "" && userID != "anonymous" {
				return "auth:" + userID
			}
			return "auth-ip:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			userID, _ := c.Locals("user_id").(string)
			log.Printf("⚠️  [RATE-LIMIT] Auth endpoint limit reached for user: %s on %s", userID, c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Too many requests. Please wait before trying again.",
				"retry_after": int(config.AuthenticatedExpiration.Seconds()),
			})
		},
	})
}

// TranscribeRateLimiter for expensive audio transcription
func TranscribeRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.TranscribeMax,
		Expiration: config.TranscribeExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			if userID, ok := c.Locals("user_id").(string); ok && userID != "" && userID != "anonymous" {
				return "transcribe:" + userID
			}
			return "transcribe-ip:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("⚠️  [RATE-LIMIT] Transcription limit reached for: %v", c.Locals("user_id"))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Transcription rate limit reached. Please wait before transcribing more audio.",
				"retry_after": int(config.TranscribeExpiration.Seconds()),
			})
		},
	})
}

// WebSocketRateLimiter for WebSocket connection attempts
func WebSocketRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.WebSocketMax,
		Expiration: config.WebSocketExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "ws:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("🚫 [RATE-LIMIT] WebSocket connection limit reached for IP: %s", c.IP())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Too many connection attempts. Please wait before reconnecting.",
				"retry_after": int(config.WebSocketExpiration.Seconds()),
			})
		},
	})
}

// ImageProxyRateLimiter for image proxy requests (bandwidth protection)
func ImageProxyRateLimiter(config *RateLimitConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.ImageProxyMax,
		Expiration: config.ImageProxyExpiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			return "imgproxy:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Printf("⚠️  [RATE-LIMIT] Image proxy limit reached for IP: %s", c.IP())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Too many image requests. Please wait.",
				"retry_after": int(config.ImageProxyExpiration.Seconds()),
			})
		},
	})
}

// SlowdownMiddleware adds progressive delays for rapid requests
// This discourages automated attacks without hard blocking
func SlowdownMiddleware(threshold int, delay time.Duration) fiber.Handler {
	// Use a simple in-memory counter (for single-instance deployments)
	// For multi-instance, use Redis-backed rate limiting
	return func(c *fiber.Ctx) error {
		// This is a placeholder - the limiter middleware handles the actual limiting
		// This could be enhanced to add progressive delays before hard blocking
		return c.Next()
	}
}
