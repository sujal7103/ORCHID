package handlers

import (
	"clara-agents/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ExecutionHandler handles execution history endpoints.
type ExecutionHandler struct {
	executionService *services.ExecutionService
}

// NewExecutionHandler creates a new ExecutionHandler.
func NewExecutionHandler(executionService *services.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{executionService: executionService}
}

// ListByAgent returns paginated executions for a specific agent.
// GET /api/agents/:id/executions
func (h *ExecutionHandler) ListByAgent(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	agentID := c.Params("id")
	opts := &services.ListExecutionsOptions{
		Limit:  parseIntQuery(c, "limit", 20),
		Page:   parseIntQuery(c, "page", 1),
		Status: c.Query("status"),
	}
	result, err := h.executionService.ListByAgent(c.Context(), agentID, userID, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// ListByUser returns all executions for the authenticated user across agents.
// GET /api/executions
func (h *ExecutionHandler) ListByUser(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	opts := &services.ListExecutionsOptions{
		Limit:  parseIntQuery(c, "limit", 20),
		Page:   parseIntQuery(c, "page", 1),
		Status: c.Query("status"),
	}
	result, err := h.executionService.ListByUser(c.Context(), userID, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetByID returns a single execution record by ID.
// GET /api/executions/:id
func (h *ExecutionHandler) GetByID(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	executionID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid execution ID"})
	}
	record, err := h.executionService.GetByIDAndUser(c.Context(), executionID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "execution not found"})
	}
	return c.JSON(record)
}

// parseIntQuery reads a query param as int with a fallback default.
func parseIntQuery(c *fiber.Ctx, key string, defaultVal int) int {
	v := c.QueryInt(key, defaultVal)
	if v <= 0 {
		return defaultVal
	}
	return v
}
