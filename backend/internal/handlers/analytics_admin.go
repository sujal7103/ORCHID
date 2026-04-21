package handlers

import (
	"clara-agents/internal/health"
	"clara-agents/internal/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AdminAnalyticsHandler serves all admin analytics, insights, health, user, and autopilot endpoints.
type AdminAnalyticsHandler struct {
	analyticsService *services.AnalyticsService
	modelService     *services.ModelService
	healthService    *health.Service
}

// NewAdminAnalyticsHandler creates a new AdminAnalyticsHandler.
func NewAdminAnalyticsHandler(
	analyticsService *services.AnalyticsService,
	modelService *services.ModelService,
	healthService *health.Service,
) *AdminAnalyticsHandler {
	return &AdminAnalyticsHandler{
		analyticsService: analyticsService,
		modelService:     modelService,
		healthService:    healthService,
	}
}

// ── Analytics ─────────────────────────────────────────────────────────────────

// GetOverviewStats returns system overview statistics.
func (h *AdminAnalyticsHandler) GetOverviewStats(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetOverviewStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetProviderAnalytics returns usage analytics per provider.
func (h *AdminAnalyticsHandler) GetProviderAnalytics(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetProviderAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetModelAnalytics returns usage analytics per model.
func (h *AdminAnalyticsHandler) GetModelAnalytics(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetModelAnalytics(c.Context(), h.modelService)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetChatAnalytics returns chat usage statistics.
func (h *AdminAnalyticsHandler) GetChatAnalytics(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetChatAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetAgentAnalytics returns agent activity analytics.
func (h *AdminAnalyticsHandler) GetAgentAnalytics(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetAgentAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// MigrateChatTimestamps fixes existing chat sessions that don't have proper startedAt timestamps.
func (h *AdminAnalyticsHandler) MigrateChatTimestamps(c *fiber.Ctx) error {
	n, err := h.analyticsService.MigrateChatSessionTimestamps(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"success":          true,
		"message":          "Migration complete",
		"sessions_updated": n,
	})
}

// ── Users + GDPR ──────────────────────────────────────────────────────────────

// GetUsers returns a GDPR-compliant paginated user list.
func (h *AdminAnalyticsHandler) GetUsers(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	tier := c.Query("tier")
	search := c.Query("search")

	users, totalCount, err := h.analyticsService.GetUserListGDPR(c.Context(), page, pageSize, tier, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"users":       users,
		"total_count": totalCount,
		"page":        page,
		"page_size":   pageSize,
		"gdpr_notice": "User data is anonymized. Only aggregate analytics and email domains are shown.",
	})
}

// GetGDPRPolicy returns the static GDPR data policy.
func (h *AdminAnalyticsHandler) GetGDPRPolicy(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data_collected": []string{
			"Email address (hashed for analytics)",
			"Chat session metadata (start time, message count, model used)",
			"Agent execution logs (agent ID, trigger type, timestamp)",
			"Authentication tokens (stored as hashed values)",
		},
		"data_retention_days": 365,
		"purpose":             "Service operation, abuse prevention, and aggregate usage analytics",
		"legal_basis":         "Legitimate interest (service operation) and user consent for analytics",
		"user_rights":         []string{"Access", "Rectification", "Erasure", "Data portability", "Restrict processing"},
	})
}

// ── Health Dashboard ──────────────────────────────────────────────────────────

// GetHealthDashboard returns the health status of all registered providers.
func (h *AdminAnalyticsHandler) GetHealthDashboard(c *fiber.Ctx) error {
	if h.healthService == nil {
		return c.JSON(fiber.Map{
			"summary": fiber.Map{
				"total":     0,
				"healthy":   0,
				"unhealthy": 0,
				"cooldown":  0,
				"unknown":   0,
			},
			"capabilities": fiber.Map{},
			"providers":    []fiber.Map{},
		})
	}

	entries := h.healthService.GetAllRegistered()

	// Summary counts
	totalCount := 0
	healthyCount := 0
	unhealthyCount := 0
	cooldownCount := 0
	unknownCount := 0

	// Capabilities map: capability -> {healthy, unhealthy, cooldown, unknown}
	capMap := make(map[string]map[string]int)

	// Providers array
	providers := make([]fiber.Map, 0, len(entries))

	for _, entry := range entries {
		totalCount++

		cap := string(entry.Capability)
		if _, exists := capMap[cap]; !exists {
			capMap[cap] = map[string]int{
				"healthy":   0,
				"unhealthy": 0,
				"cooldown":  0,
				"unknown":   0,
			}
		}

		var statusStr string
		switch entry.Status {
		case health.StatusHealthy:
			statusStr = "healthy"
			healthyCount++
			capMap[cap]["healthy"]++
		case health.StatusUnhealthy:
			statusStr = "unhealthy"
			unhealthyCount++
			capMap[cap]["unhealthy"]++
		case health.StatusCooldown:
			statusStr = "cooldown"
			cooldownCount++
			capMap[cap]["cooldown"]++
		default:
			statusStr = "unknown"
			unknownCount++
			capMap[cap]["unknown"]++
		}

		// Format optional time fields as RFC3339 or nil
		var lastChecked interface{}
		if !entry.LastChecked.IsZero() {
			lastChecked = entry.LastChecked.Format(time.RFC3339)
		}

		var lastSuccess interface{}
		if !entry.LastSuccessAt.IsZero() {
			lastSuccess = entry.LastSuccessAt.Format(time.RFC3339)
		}

		var cooldownUntil interface{}
		if !entry.CooldownUntil.IsZero() {
			cooldownUntil = entry.CooldownUntil.Format(time.RFC3339)
		}

		providers = append(providers, fiber.Map{
			"provider_id":    entry.ProviderID,
			"provider_name":  entry.ProviderName,
			"model_name":     entry.ModelName,
			"capability":     string(entry.Capability),
			"status":         statusStr,
			"failure_count":  entry.FailureCount,
			"last_error":     entry.LastError,
			"last_checked":   lastChecked,
			"last_success":   lastSuccess,
			"cooldown_until": cooldownUntil,
			"priority":       entry.Priority,
		})
	}

	// Convert capMap to fiber.Map
	capabilities := fiber.Map{}
	for cap, counts := range capMap {
		capabilities[cap] = counts
	}

	return c.JSON(fiber.Map{
		"summary": fiber.Map{
			"total":     totalCount,
			"healthy":   healthyCount,
			"unhealthy": unhealthyCount,
			"cooldown":  cooldownCount,
			"unknown":   unknownCount,
		},
		"capabilities": capabilities,
		"providers":    providers,
	})
}

// ── Insights (stubs) ──────────────────────────────────────────────────────────

// GetInsightsOverview returns a zeroed insights overview for the current 30-day period.
func (h *AdminAnalyticsHandler) GetInsightsOverview(c *fiber.Ctx) error {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	return c.JSON(fiber.Map{
		"period": fiber.Map{
			"start": start.Format(time.RFC3339),
			"end":   now.Format(time.RFC3339),
		},
		"users": fiber.Map{
			"totalActive": 0,
			"trend":       "0%",
			"new7d":       0,
			"churned7d":   0,
			"atRisk":      0,
		},
		"features": fiber.Map{},
		"feedback": fiber.Map{
			"bugs7d":    0,
			"features7d": 0,
			"npsAvg":    0,
			"npsTrend":  "0%",
		},
		"errors": fiber.Map{
			"total7d": 0,
			"trend":   "0%",
			"topType": "",
		},
	})
}

// GetInsightsMetrics returns an empty metrics list.
func (h *AdminAnalyticsHandler) GetInsightsMetrics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"metrics": []interface{}{},
	})
}

// GetHealthDistribution returns zeroed health distribution data.
func (h *AdminAnalyticsHandler) GetHealthDistribution(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"healthy": 0,
		"medium":  0,
		"high":    0,
		"signals": fiber.Map{},
	})
}

// GetActivationFunnel returns an empty activation funnel.
func (h *AdminAnalyticsHandler) GetActivationFunnel(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"funnel":        []interface{}{},
		"biggestDropOff": "",
		"dropOffRate":   0,
	})
}

// GetFeedbackStream returns an empty feedback stream.
func (h *AdminAnalyticsHandler) GetFeedbackStream(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"feedback":   []interface{}{},
		"totalCount": 0,
	})
}

// GetCollectionStats returns raw document counts and daily series from major MongoDB collections.
func (h *AdminAnalyticsHandler) GetCollectionStats(c *fiber.Ctx) error {
	result, err := h.analyticsService.GetCollectionStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// BackfillMetrics is a stub that acknowledges the backfill request.
func (h *AdminAnalyticsHandler) BackfillMetrics(c *fiber.Ctx) error {
	days := c.QueryInt("days", 90)
	return c.JSON(fiber.Map{
		"message":   "Backfill not yet implemented",
		"days":      days,
		"processed": 0,
	})
}

// ── AutoPilot (stubs) ─────────────────────────────────────────────────────────

// GetAutoPilotContext returns a zeroed AutoPilot context payload.
func (h *AdminAnalyticsHandler) GetAutoPilotContext(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"generatedAt": time.Now().Format(time.RFC3339),
		"period":      "30d",
		"context": fiber.Map{
			"users":    0,
			"features": 0,
			"feedback": 0,
			"errors":   0,
			"health":   0,
			"funnel":   0,
			"trends":   0,
		},
		"potentialActions": []interface{}{},
	})
}

// AnalyzeWithAI returns a stub response indicating AI analysis is not yet configured.
func (h *AdminAnalyticsHandler) AnalyzeWithAI(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"analysis":  "AutoPilot AI analysis is not yet configured. Add a model provider in /admin to enable AI-powered insights.",
		"model":     "",
		"tokens":    fiber.Map{"input": 0, "output": 0},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
