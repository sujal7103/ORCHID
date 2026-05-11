package middleware

import (
	"clara-agents/internal/config"

	"github.com/gofiber/fiber/v2"
)

// AdminMiddleware checks if the authenticated user is a superadmin.
// In the community edition, users with role "admin" (first registered user)
// are also treated as superadmins.
func AdminMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok || userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		isSuperadmin := false

		// Check if user has admin role (set by JWT claims)
		if role, ok := c.Locals("user_role").(string); ok && role == "admin" {
			isSuperadmin = true
		}

		// Also check the explicit superadmin list from env
		if !isSuperadmin {
			for _, adminID := range cfg.SuperadminUserIDs {
				if adminID == userID {
					isSuperadmin = true
					break
				}
			}
		}

		if !isSuperadmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Superadmin access required",
			})
		}

		// Store admin flag for handlers to use
		c.Locals("is_superadmin", true)
		return c.Next()
	}
}

// IsSuperadmin is a helper function to check if a user ID is a superadmin
func IsSuperadmin(userID string, cfg *config.Config) bool {
	for _, adminID := range cfg.SuperadminUserIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}
