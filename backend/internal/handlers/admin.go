package handlers

import (
	"clara-agents/internal/models"
	"clara-agents/internal/services"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
)

// AdminHandler handles admin operations (provider + model management).
type AdminHandler struct {
	providerService *services.ProviderService
	modelService    *services.ModelService
	modelMgmtSvc    *services.ModelManagementService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	providerService *services.ProviderService,
	modelService *services.ModelService,
	modelMgmtSvc *services.ModelManagementService,
) *AdminHandler {
	return &AdminHandler{
		providerService: providerService,
		modelService:    modelService,
		modelMgmtSvc:    modelMgmtSvc,
	}
}

// ── Admin Status ──────────────────────────────────────────────────────────────

// GetAdminStatus returns basic info about the authenticated admin user.
// GET /api/admin/me
func (h *AdminHandler) GetAdminStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	email := c.Locals("email")
	if email == nil {
		email = ""
	}
	return c.JSON(fiber.Map{
		"is_admin": true,
		"user_id":  userID,
		"email":    email,
	})
}

// ── Provider Management ───────────────────────────────────────────────────────

// GetProviders returns all providers with model counts and metadata.
// GET /api/admin/providers
func (h *AdminHandler) GetProviders(c *fiber.Ctx) error {
	providers, err := h.providerService.GetAllIncludingDisabled()
	if err != nil {
		log.Printf("❌ [ADMIN] Failed to get providers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get providers",
		})
	}

	var providerViews []fiber.Map
	for _, provider := range providers {
		mods, _ := h.modelService.GetByProvider(provider.ID, false)
		modelCount := len(mods)

		aliases, _ := h.modelService.LoadAllAliasesFromDB()
		providerAliases := make(map[string]interface{})
		if aliases[provider.ID] != nil {
			providerAliases = convertAliasesMapToInterface(aliases[provider.ID])
		}

		filters, _ := h.providerService.GetFilters(provider.ID)

		recommended, _ := h.modelService.LoadAllRecommendedModelsFromDB()
		var recommendedModels interface{}
		if recommended[provider.ID] != nil {
			recommendedModels = recommended[provider.ID]
		}

		providerViews = append(providerViews, fiber.Map{
			"id":                 provider.ID,
			"name":               provider.Name,
			"base_url":           provider.BaseURL,
			"enabled":            provider.Enabled,
			"audio_only":         provider.AudioOnly,
			"favicon":            provider.Favicon,
			"model_count":        modelCount,
			"model_aliases":      providerAliases,
			"filters":            filters,
			"recommended_models": recommendedModels,
		})
	}

	return c.JSON(fiber.Map{"providers": providerViews})
}

// CreateProvider creates a new provider.
// POST /api/admin/providers
func (h *AdminHandler) CreateProvider(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		BaseURL       string `json:"base_url"`
		APIKey        string `json:"api_key"`
		Enabled       *bool  `json:"enabled"`
		AudioOnly     *bool  `json:"audio_only"`
		ImageOnly     *bool  `json:"image_only"`
		ImageEditOnly *bool  `json:"image_edit_only"`
		Secure        *bool  `json:"secure"`
		DefaultModel  string `json:"default_model"`
		SystemPrompt  string `json:"system_prompt"`
		Favicon       string `json:"favicon"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Name == "" || req.BaseURL == "" || req.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, base_url, and api_key are required",
		})
	}

	cfg := models.ProviderConfig{
		Name:          req.Name,
		BaseURL:       req.BaseURL,
		APIKey:        req.APIKey,
		Enabled:       req.Enabled != nil && *req.Enabled,
		AudioOnly:     req.AudioOnly != nil && *req.AudioOnly,
		ImageOnly:     req.ImageOnly != nil && *req.ImageOnly,
		ImageEditOnly: req.ImageEditOnly != nil && *req.ImageEditOnly,
		Secure:        req.Secure != nil && *req.Secure,
		DefaultModel:  req.DefaultModel,
		SystemPrompt:  req.SystemPrompt,
		Favicon:       req.Favicon,
	}

	provider, err := h.providerService.Create(cfg)
	if err != nil {
		log.Printf("❌ [ADMIN] Create provider: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create provider: %v", err),
		})
	}
	log.Printf("✅ [ADMIN] Created provider: %s (ID %d)", provider.Name, provider.ID)
	return c.Status(fiber.StatusCreated).JSON(provider)
}

// UpdateProvider updates an existing provider.
// PUT /api/admin/providers/:id
func (h *AdminHandler) UpdateProvider(c *fiber.Ctx) error {
	providerID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid provider ID"})
	}

	var req struct {
		Name          *string `json:"name"`
		BaseURL       *string `json:"base_url"`
		APIKey        *string `json:"api_key"`
		Enabled       *bool   `json:"enabled"`
		AudioOnly     *bool   `json:"audio_only"`
		ImageOnly     *bool   `json:"image_only"`
		ImageEditOnly *bool   `json:"image_edit_only"`
		Secure        *bool   `json:"secure"`
		DefaultModel  *string `json:"default_model"`
		SystemPrompt  *string `json:"system_prompt"`
		Favicon       *string `json:"favicon"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	existing, err := h.providerService.GetByID(providerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Provider not found"})
	}

	cfg := models.ProviderConfig{
		Name:         existing.Name,
		BaseURL:      existing.BaseURL,
		APIKey:       existing.APIKey,
		Enabled:      existing.Enabled,
		AudioOnly:    existing.AudioOnly,
		SystemPrompt: existing.SystemPrompt,
		Favicon:      existing.Favicon,
	}
	if req.Name != nil {
		cfg.Name = *req.Name
	}
	if req.BaseURL != nil {
		cfg.BaseURL = *req.BaseURL
	}
	if req.APIKey != nil {
		cfg.APIKey = *req.APIKey
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.AudioOnly != nil {
		cfg.AudioOnly = *req.AudioOnly
	}
	if req.ImageOnly != nil {
		cfg.ImageOnly = *req.ImageOnly
	}
	if req.ImageEditOnly != nil {
		cfg.ImageEditOnly = *req.ImageEditOnly
	}
	if req.Secure != nil {
		cfg.Secure = *req.Secure
	}
	if req.DefaultModel != nil {
		cfg.DefaultModel = *req.DefaultModel
	}
	if req.SystemPrompt != nil {
		cfg.SystemPrompt = *req.SystemPrompt
	}
	if req.Favicon != nil {
		cfg.Favicon = *req.Favicon
	}

	if err := h.providerService.Update(providerID, cfg); err != nil {
		log.Printf("❌ [ADMIN] Update provider %d: %v", providerID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update provider: %v", err),
		})
	}

	updated, err := h.providerService.GetByID(providerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve updated provider",
		})
	}
	log.Printf("✅ [ADMIN] Updated provider: %s (ID %d)", updated.Name, updated.ID)
	return c.JSON(updated)
}

// DeleteProvider deletes a provider.
// DELETE /api/admin/providers/:id
func (h *AdminHandler) DeleteProvider(c *fiber.Ctx) error {
	providerID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid provider ID"})
	}

	provider, err := h.providerService.GetByID(providerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Provider not found"})
	}

	if err := h.providerService.Delete(providerID); err != nil {
		log.Printf("❌ [ADMIN] Delete provider %d: %v", providerID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to delete provider: %v", err),
		})
	}
	log.Printf("✅ [ADMIN] Deleted provider: %s (ID %d)", provider.Name, provider.ID)
	return c.JSON(fiber.Map{"message": "Provider deleted successfully"})
}

// ToggleProvider enables or disables a provider.
// PUT /api/admin/providers/:id/toggle
func (h *AdminHandler) ToggleProvider(c *fiber.Ctx) error {
	providerID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid provider ID"})
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	provider, err := h.providerService.GetByID(providerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Provider not found"})
	}

	cfg := models.ProviderConfig{
		Name:         provider.Name,
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey,
		Enabled:      req.Enabled,
		AudioOnly:    provider.AudioOnly,
		SystemPrompt: provider.SystemPrompt,
		Favicon:      provider.Favicon,
	}
	if err := h.providerService.Update(providerID, cfg); err != nil {
		log.Printf("❌ [ADMIN] Toggle provider %d: %v", providerID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to toggle provider: %v", err),
		})
	}

	updated, err := h.providerService.GetByID(providerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve updated provider",
		})
	}
	log.Printf("✅ [ADMIN] Toggled provider %s enabled=%v", updated.Name, updated.Enabled)
	return c.JSON(updated)
}

// FetchModelsFromProvider fetches models from provider API and stores them.
// POST /api/admin/providers/:id/fetch
func (h *AdminHandler) FetchModelsFromProvider(c *fiber.Ctx) error {
	providerID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid provider ID"})
	}
	if h.modelMgmtSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Model management service unavailable",
		})
	}

	count, err := h.modelMgmtSvc.FetchModelsFromProvider(c.Context(), providerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch models: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"success":       true,
		"models_fetched": count,
		"message":       fmt.Sprintf("Fetched %d models from provider", count),
	})
}

// SyncProviderToJSON syncs provider models to providers.json format.
// POST /api/admin/providers/:id/sync
func (h *AdminHandler) SyncProviderToJSON(c *fiber.Ctx) error {
	providerID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid provider ID"})
	}
	if h.modelMgmtSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Model management service unavailable",
		})
	}

	_, err = h.modelMgmtSvc.FetchModelsFromProvider(c.Context(), providerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Sync failed: %v", err),
		})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Provider synced successfully"})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func convertAliasesMapToInterface(aliases map[string]models.ModelAlias) map[string]interface{} {
	result := make(map[string]interface{})
	for key, alias := range aliases {
		result[key] = fiber.Map{
			"actual_model":                      alias.ActualModel,
			"display_name":                      alias.DisplayName,
			"description":                       alias.Description,
			"supports_vision":                   alias.SupportsVision,
			"agents":                            alias.Agents,
			"smart_tool_router":                 alias.SmartToolRouter,
			"free_tier":                         alias.FreeTier,
			"structured_output_support":         alias.StructuredOutputSupport,
			"structured_output_compliance":      alias.StructuredOutputCompliance,
			"structured_output_warning":         alias.StructuredOutputWarning,
			"structured_output_speed_ms":        alias.StructuredOutputSpeedMs,
			"structured_output_badge":           alias.StructuredOutputBadge,
			"memory_extractor":                  alias.MemoryExtractor,
			"memory_selector":                   alias.MemorySelector,
		}
	}
	return result
}
