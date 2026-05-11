package middleware

import (
	"clara-agents/pkg/auth"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// NewJWTMiddleware creates a required JWT auth middleware from a secret key string.
// This is the primary auth middleware used by protected routes.
func NewJWTMiddleware(secret string) fiber.Handler {
	if secret == "" {
		log.Println("⚠️  JWT_SECRET not set — auth middleware will reject all requests in production")
	}
	jwtAuth, err := auth.NewLocalJWTAuth(secret, 15*time.Minute, 168*time.Hour)
	if err != nil {
		log.Printf("⚠️  Failed to init JWT auth: %v", err)
		return LocalAuthMiddleware(nil)
	}
	return LocalAuthMiddleware(jwtAuth)
}

// NewOptionalJWTMiddleware creates an optional JWT auth middleware.
// Requests without a token are allowed; token claims are attached if present.
func NewOptionalJWTMiddleware(secret string) fiber.Handler {
	jwtAuth, err := auth.NewLocalJWTAuth(secret, 15*time.Minute, 168*time.Hour)
	if err != nil {
		return OptionalLocalAuthMiddleware(nil)
	}
	return OptionalLocalAuthMiddleware(jwtAuth)
}
