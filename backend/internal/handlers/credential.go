package handlers

import (
	"clara-agents/internal/models"
	"clara-agents/internal/services"
	"log"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CredentialHandler handles credential management endpoints
type CredentialHandler struct {
	credentialService *services.CredentialService
	credentialTester  *CredentialTester
}

// NewCredentialHandler creates a new credential handler
func NewCredentialHandler(credentialService *services.CredentialService) *CredentialHandler {
	return &CredentialHandler{
		credentialService: credentialService,
		credentialTester:  NewCredentialTester(credentialService),
	}
}

// Create creates a new credential
// POST /api/credentials
func (h *CredentialHandler) Create(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req models.CreateCredentialRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	if req.IntegrationType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Integration type is required",
		})
	}

	if req.Data == nil || len(req.Data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Credential data is required",
		})
	}

	result, err := h.credentialService.Create(c.Context(), userID, &req)
	if err != nil {
		log.Printf("❌ [CREDENTIAL] Failed to create credential: %v", err)

		// Check for validation error
		if _, ok := err.(*models.CredentialValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create credential",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

// List lists all credentials for the user
// GET /api/credentials
func (h *CredentialHandler) List(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	integrationType := c.Query("type") // Optional filter by type

	var credentials []*models.CredentialListItem
	var err error

	if integrationType != "" {
		credentials, err = h.credentialService.ListByUserAndType(c.Context(), userID, integrationType)
	} else {
		credentials, err = h.credentialService.ListByUser(c.Context(), userID)
	}

	if err != nil {
		log.Printf("❌ [CREDENTIAL] Failed to list credentials: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list credentials",
		})
	}

	return c.JSON(models.GetCredentialsResponse{
		Credentials: credentials,
		Total:       len(credentials),
	})
}

// Get retrieves a specific credential (metadata only)
// GET /api/credentials/:id
func (h *CredentialHandler) Get(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	credIDStr := c.Params("id")

	credID, err := primitive.ObjectIDFromHex(credIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid credential ID",
		})
	}

	credential, err := h.credentialService.GetByIDAndUser(c.Context(), credID, userID)
	if err != nil {
		if err.Error() == "credential not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Credential not found",
			})
		}
		log.Printf("❌ [CREDENTIAL] Failed to get credential: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get credential",
		})
	}

	return c.JSON(credential.ToListItem())
}

// Update updates a credential
// PUT /api/credentials/:id
func (h *CredentialHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	credIDStr := c.Params("id")

	credID, err := primitive.ObjectIDFromHex(credIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid credential ID",
		})
	}

	var req models.UpdateCredentialRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// At least one field must be provided
	if req.Name == "" && req.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least name or data must be provided",
		})
	}

	result, err := h.credentialService.Update(c.Context(), credID, userID, &req)
	if err != nil {
		if err.Error() == "credential not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Credential not found",
			})
		}
		log.Printf("❌ [CREDENTIAL] Failed to update credential: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update credential",
		})
	}

	return c.JSON(result)
}

// Delete permanently deletes a credential
// DELETE /api/credentials/:id
func (h *CredentialHandler) Delete(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	credIDStr := c.Params("id")

	credID, err := primitive.ObjectIDFromHex(credIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid credential ID",
		})
	}

	if err := h.credentialService.Delete(c.Context(), credID, userID); err != nil {
		if err.Error() == "credential not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Credential not found",
			})
		}
		log.Printf("❌ [CREDENTIAL] Failed to delete credential: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete credential",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Credential deleted successfully",
	})
}

// Test tests a credential by making a real API call
// POST /api/credentials/:id/test
func (h *CredentialHandler) Test(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	credIDStr := c.Params("id")

	credID, err := primitive.ObjectIDFromHex(credIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid credential ID",
		})
	}

	// Get and decrypt the credential
	decrypted, err := h.credentialService.GetDecrypted(c.Context(), userID, credID)
	if err != nil {
		if err.Error() == "credential not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Credential not found",
			})
		}
		log.Printf("❌ [CREDENTIAL] Failed to get credential for testing: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get credential",
		})
	}

	// Test the credential
	result := h.credentialTester.Test(c.Context(), decrypted)

	// Update test status
	status := "failed"
	if result.Success {
		status = "success"
	}
	if err := h.credentialService.UpdateTestStatus(c.Context(), credID, userID, status, nil); err != nil {
		log.Printf("⚠️ [CREDENTIAL] Failed to update test status: %v", err)
	}

	return c.JSON(result)
}

// GetIntegrations returns all available integrations
// GET /api/integrations
func (h *CredentialHandler) GetIntegrations(c *fiber.Ctx) error {
	categories := models.GetIntegrationsByCategory()
	return c.JSON(models.GetIntegrationsResponse{
		Categories: categories,
	})
}

// GetIntegration returns a specific integration
// GET /api/integrations/:id
func (h *CredentialHandler) GetIntegration(c *fiber.Ctx) error {
	integrationID := c.Params("id")

	integration, exists := models.GetIntegration(integrationID)
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Integration not found",
		})
	}

	return c.JSON(integration)
}

// GetCredentialsByIntegration returns credentials grouped by integration type
// GET /api/credentials/by-integration
func (h *CredentialHandler) GetCredentialsByIntegration(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	credentials, err := h.credentialService.ListByUser(c.Context(), userID)
	if err != nil {
		log.Printf("❌ [CREDENTIAL] Failed to list credentials: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list credentials",
		})
	}

	// Group by integration type
	groupedMap := make(map[string]*models.CredentialsByIntegration)
	for _, cred := range credentials {
		if _, exists := groupedMap[cred.IntegrationType]; !exists {
			integration, _ := models.GetIntegration(cred.IntegrationType)
			groupedMap[cred.IntegrationType] = &models.CredentialsByIntegration{
				IntegrationType: cred.IntegrationType,
				Integration:     integration,
				Credentials:     []*models.CredentialListItem{},
			}
		}
		groupedMap[cred.IntegrationType].Credentials = append(
			groupedMap[cred.IntegrationType].Credentials,
			cred,
		)
	}

	// Convert to slice
	var integrations []models.CredentialsByIntegration
	for _, group := range groupedMap {
		integrations = append(integrations, *group)
	}

	return c.JSON(models.GetCredentialsByIntegrationResponse{
		Integrations: integrations,
	})
}

// GetCredentialReferences returns credential references for LLM context
// GET /api/credentials/references
func (h *CredentialHandler) GetCredentialReferences(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Optional filter by integration types
	var integrationTypes []string
	if types := c.Query("types"); types != "" {
		// Parse comma-separated types
		// For simplicity, just pass nil to get all
		// In production, you'd parse the query param
	}

	refs, err := h.credentialService.GetCredentialReferences(c.Context(), userID, integrationTypes)
	if err != nil {
		log.Printf("❌ [CREDENTIAL] Failed to get credential references: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get credential references",
		})
	}

	return c.JSON(fiber.Map{
		"credentials": refs,
	})
}
