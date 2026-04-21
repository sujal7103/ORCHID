package handlers

import (
	"clara-agents/internal/services"
	"clara-agents/internal/tools"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ToolsHandler handles tool-related requests
type ToolsHandler struct {
	registry    *tools.Registry
	toolService *services.ToolService
}

// NewToolsHandler creates a new tools handler
func NewToolsHandler(registry *tools.Registry, toolService *services.ToolService) *ToolsHandler {
	return &ToolsHandler{
		registry:    registry,
		toolService: toolService,
	}
}

// ToolResponse represents a tool in the API response
type ToolResponse struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon"`
	Category    string                 `json:"category"`
	Keywords    []string               `json:"keywords"`
	Source      string                 `json:"source"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// CategoryResponse represents a category with its tools
type CategoryResponse struct {
	Name  string         `json:"name"`
	Count int            `json:"count"`
	Tools []ToolResponse `json:"tools"`
}

// ListTools returns all tools available to the authenticated user, grouped by category
func (h *ToolsHandler) ListTools(c *fiber.Ctx) error {
	// Extract user ID from auth middleware
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get all tools for the user (built-in + MCP)
	toolsList := h.registry.GetUserTools(userID)

	// Group tools by category
	categoryMap := make(map[string][]ToolResponse)

	for _, toolDef := range toolsList {
		function, ok := toolDef["function"].(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := function["name"].(string)
		description, _ := function["description"].(string)

		// Get the actual tool to extract metadata
		tool, exists := h.registry.GetUserTool(userID, name)
		if !exists {
			continue
		}

		toolResponse := ToolResponse{
			Name:        tool.Name,
			DisplayName: tool.DisplayName,
			Description: description,
			Icon:        tool.Icon,
			Category:    tool.Category,
			Keywords:    tool.Keywords,
			Source:      string(tool.Source),
			Parameters:  tool.Parameters,
		}

		// Group by category (default to "other" if no category)
		category := tool.Category
		if category == "" {
			category = "other"
		}

		categoryMap[category] = append(categoryMap[category], toolResponse)
	}

	// Convert map to array of CategoryResponse
	categories := make([]CategoryResponse, 0, len(categoryMap))
	for categoryName, categoryTools := range categoryMap {
		categories = append(categories, CategoryResponse{
			Name:  categoryName,
			Count: len(categoryTools),
			Tools: categoryTools,
		})
	}

	// Sort categories alphabetically
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return c.JSON(fiber.Map{
		"categories": categories,
		"total":      h.registry.CountUserTools(userID),
	})
}

// ListRegistry is an alias for ListTools served at /api/tools/registry.
// GET /api/tools/registry
func (h *ToolsHandler) ListRegistry(c *fiber.Ctx) error {
	return h.ListTools(c)
}

// AvailableToolResponse represents a tool with credential metadata
type AvailableToolResponse struct {
	Name               string   `json:"name"`
	DisplayName        string   `json:"display_name"`
	Description        string   `json:"description"`
	Icon               string   `json:"icon"`
	Category           string   `json:"category"`
	Keywords           []string `json:"keywords"`
	Source             string   `json:"source"`
	RequiresCredential bool     `json:"requires_credential"`
	IntegrationType    string   `json:"integration_type,omitempty"`
}

// GetAvailableTools returns tools filtered by user's credentials
// Only tools that don't require credentials OR tools where user has configured credentials are returned
func (h *ToolsHandler) GetAvailableTools(c *fiber.Ctx) error {
	// Extract user ID from auth middleware
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// If no tool service available, fall back to all tools
	if h.toolService == nil {
		return h.ListTools(c)
	}

	// Get filtered tools from tool service
	filteredTools := h.toolService.GetAvailableTools(c.Context(), userID)

	// Build response with metadata
	toolResponses := make([]AvailableToolResponse, 0, len(filteredTools))
	categoryMap := make(map[string][]AvailableToolResponse)

	for _, toolDef := range filteredTools {
		function, ok := toolDef["function"].(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := function["name"].(string)
		description, _ := function["description"].(string)

		// Get the actual tool to extract metadata
		tool, exists := h.registry.GetUserTool(userID, name)
		if !exists {
			continue
		}

		// Check if tool requires credentials
		integrationType := tools.GetIntegrationTypeForTool(name)
		requiresCredential := integrationType != ""

		toolResponse := AvailableToolResponse{
			Name:               tool.Name,
			DisplayName:        tool.DisplayName,
			Description:        description,
			Icon:               tool.Icon,
			Category:           tool.Category,
			Keywords:           tool.Keywords,
			Source:             string(tool.Source),
			RequiresCredential: requiresCredential,
			IntegrationType:    integrationType,
		}

		toolResponses = append(toolResponses, toolResponse)

		// Group by category
		category := tool.Category
		if category == "" {
			category = "other"
		}
		categoryMap[category] = append(categoryMap[category], toolResponse)
	}

	// Convert to category response format
	categories := make([]struct {
		Name  string                  `json:"name"`
		Count int                     `json:"count"`
		Tools []AvailableToolResponse `json:"tools"`
	}, 0, len(categoryMap))

	for categoryName, categoryTools := range categoryMap {
		categories = append(categories, struct {
			Name  string                  `json:"name"`
			Count int                     `json:"count"`
			Tools []AvailableToolResponse `json:"tools"`
		}{
			Name:  categoryName,
			Count: len(categoryTools),
			Tools: categoryTools,
		})
	}

	// Sort categories alphabetically
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	// Get total count for comparison
	allToolsCount := h.registry.CountUserTools(userID)
	filteredCount := allToolsCount - len(filteredTools)

	return c.JSON(fiber.Map{
		"categories":     categories,
		"total":          len(filteredTools),
		"filtered_count": filteredCount, // Number of tools filtered out due to missing credentials
	})
}

// RecommendToolsRequest represents the request body for tool recommendations
type RecommendToolsRequest struct {
	BlockName        string `json:"block_name"`
	BlockDescription string `json:"block_description"`
	BlockType        string `json:"block_type"`
}

// ToolRecommendation represents a recommended tool with a score
type ToolRecommendation struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	Source      string   `json:"source"`
	Score       int      `json:"score"`
	Reason      string   `json:"reason"`
}

// RecommendTools returns scored and ranked tool recommendations based on block context
func (h *ToolsHandler) RecommendTools(c *fiber.Ctx) error {
	// Extract user ID from auth middleware
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Parse request body
	var req RecommendToolsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Tokenize block context (name + description)
	context := strings.ToLower(req.BlockName + " " + req.BlockDescription)
	contextTokens := tokenize(context)

	// Get all tools for the user
	toolsList := h.registry.GetUserTools(userID)

	// Score each tool based on keyword matching
	recommendations := []ToolRecommendation{}

	for _, toolDef := range toolsList {
		function, ok := toolDef["function"].(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := function["name"].(string)

		// Get the actual tool to extract metadata
		tool, exists := h.registry.GetUserTool(userID, name)
		if !exists {
			continue
		}

		// Calculate match score
		score, matchedKeywords := calculateMatchScore(contextTokens, tool.Keywords)

		// Only include tools with a score > 0
		if score > 0 {
			reason := "Matches: " + strings.Join(matchedKeywords, ", ")

			recommendations = append(recommendations, ToolRecommendation{
				Name:        tool.Name,
				DisplayName: tool.DisplayName,
				Description: tool.Description,
				Icon:        tool.Icon,
				Category:    tool.Category,
				Keywords:    tool.Keywords,
				Source:      string(tool.Source),
				Score:       score,
				Reason:      reason,
			})
		}
	}

	// Sort recommendations by score (descending)
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// Limit to top 10 recommendations
	if len(recommendations) > 10 {
		recommendations = recommendations[:10]
	}

	return c.JSON(fiber.Map{
		"recommendations": recommendations,
		"count":           len(recommendations),
	})
}

// tokenize splits a string into lowercase tokens
func tokenize(text string) []string {
	// Replace common separators with spaces
	text = strings.ReplaceAll(text, "-", " ")
	text = strings.ReplaceAll(text, "_", " ")
	text = strings.ReplaceAll(text, "/", " ")

	// Split by whitespace
	tokens := strings.Fields(text)

	// Deduplicate tokens
	tokenSet := make(map[string]bool)
	uniqueTokens := []string{}
	for _, token := range tokens {
		token = strings.ToLower(token)
		if !tokenSet[token] && token != "" {
			tokenSet[token] = true
			uniqueTokens = append(uniqueTokens, token)
		}
	}

	return uniqueTokens
}

// calculateMatchScore calculates how well a tool matches the context tokens
func calculateMatchScore(contextTokens []string, keywords []string) (int, []string) {
	score := 0
	matchedKeywords := []string{}

	// Normalize keywords to lowercase
	normalizedKeywords := make([]string, len(keywords))
	for i, keyword := range keywords {
		normalizedKeywords[i] = strings.ToLower(keyword)
	}

	// Check each context token against keywords
	for _, token := range contextTokens {
		for _, keyword := range normalizedKeywords {
			// Exact match
			if token == keyword {
				score += 10
				matchedKeywords = append(matchedKeywords, keyword)
				break
			}

			// Partial match (substring)
			if strings.Contains(keyword, token) || strings.Contains(token, keyword) {
				score += 5
				matchedKeywords = append(matchedKeywords, keyword)
				break
			}
		}
	}

	return score, matchedKeywords
}
