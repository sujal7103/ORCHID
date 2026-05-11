package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Credential represents an encrypted credential for external integrations
type Credential struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          string             `bson:"userId" json:"userId"`
	Name            string             `bson:"name" json:"name"`
	IntegrationType string             `bson:"integrationType" json:"integrationType"`

	// Encrypted data - NEVER exposed to frontend or LLM
	EncryptedData string `bson:"encryptedData" json:"-"` // json:"-" ensures it's never serialized

	// Metadata (safe to expose)
	Metadata CredentialMetadata `bson:"metadata" json:"metadata"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// CredentialMetadata contains non-sensitive information about a credential
type CredentialMetadata struct {
	MaskedPreview string     `bson:"maskedPreview" json:"maskedPreview"` // e.g., "https://discord...xxx"
	Icon          string     `bson:"icon,omitempty" json:"icon,omitempty"`
	LastUsedAt    *time.Time `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	UsageCount    int64      `bson:"usageCount" json:"usageCount"`
	LastTestAt    *time.Time `bson:"lastTestAt,omitempty" json:"lastTestAt,omitempty"`
	TestStatus    string     `bson:"testStatus,omitempty" json:"testStatus,omitempty"` // "success", "failed", "pending"
}

// CredentialListItem is a safe representation for listing credentials
// Never includes the encrypted data
type CredentialListItem struct {
	ID              string             `json:"id"`
	UserID          string             `json:"userId"`
	Name            string             `json:"name"`
	IntegrationType string             `json:"integrationType"`
	Metadata        CredentialMetadata `json:"metadata"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

// ToListItem converts a Credential to a safe list representation
func (c *Credential) ToListItem() *CredentialListItem {
	return &CredentialListItem{
		ID:              c.ID.Hex(),
		UserID:          c.UserID,
		Name:            c.Name,
		IntegrationType: c.IntegrationType,
		Metadata:        c.Metadata,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// DecryptedCredential is used internally by tools to access credential data
// This should NEVER be returned to the frontend or exposed to the LLM
type DecryptedCredential struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	IntegrationType string                 `json:"integrationType"`
	Data            map[string]interface{} `json:"data"` // The actual credential values
}

// CreateCredentialRequest is the request body for creating a credential
type CreateCredentialRequest struct {
	Name            string                 `json:"name" validate:"required,min=1,max=100"`
	IntegrationType string                 `json:"integrationType" validate:"required"`
	Data            map[string]interface{} `json:"data" validate:"required"` // Will be encrypted
}

// UpdateCredentialRequest is the request body for updating a credential
type UpdateCredentialRequest struct {
	Name string                 `json:"name,omitempty"`
	Data map[string]interface{} `json:"data,omitempty"` // Will be re-encrypted if provided
}

// TestCredentialResponse is returned after testing a credential
type TestCredentialResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"` // Additional info (sanitized)
}

// CredentialReference is used in block configs to reference credentials
// The LLM only sees the Name, tools use the ID to fetch the actual credential
type CredentialReference struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	IntegrationType string `json:"integrationType"`
}

// Integration represents a supported external integration type
type Integration struct {
	ID          string             `json:"id"`          // e.g., "discord", "notion"
	Name        string             `json:"name"`        // e.g., "Discord", "Notion"
	Description string             `json:"description"` // Short description
	Icon        string             `json:"icon"`        // Icon identifier (lucide or custom)
	Category    string             `json:"category"`    // e.g., "communication", "productivity"
	Fields      []IntegrationField `json:"fields"`      // Required/optional fields
	Tools       []string           `json:"tools"`       // Which tools use this integration
	DocsURL     string             `json:"docsUrl,omitempty"`
	ComingSoon  bool               `json:"comingSoon,omitempty"` // If true, integration is not yet available
}

// IntegrationField defines a field required for an integration
type IntegrationField struct {
	Key         string   `json:"key"`                   // e.g., "webhook_url", "api_key"
	Label       string   `json:"label"`                 // e.g., "Webhook URL", "API Key"
	Type        string   `json:"type"`                  // "api_key", "webhook_url", "token", "text", "select", "json"
	Required    bool     `json:"required"`              // Is this field required?
	Placeholder string   `json:"placeholder,omitempty"` // Placeholder text
	HelpText    string   `json:"helpText,omitempty"`    // Help text for the user
	Options     []string `json:"options,omitempty"`     // For select type
	Default     string   `json:"default,omitempty"`     // Default value
	Sensitive   bool     `json:"sensitive"`             // Should this be masked in UI?
}

// IntegrationCategory represents a category of integrations
type IntegrationCategory struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Icon         string        `json:"icon"`
	Integrations []Integration `json:"integrations"`
}

// GetIntegrationsResponse is the response for listing available integrations
type GetIntegrationsResponse struct {
	Categories []IntegrationCategory `json:"categories"`
}

// GetCredentialsResponse is the response for listing user credentials
type GetCredentialsResponse struct {
	Credentials []*CredentialListItem `json:"credentials"`
	Total       int                   `json:"total"`
}

// CredentialsByIntegration groups credentials by integration type
type CredentialsByIntegration struct {
	IntegrationType string               `json:"integrationType"`
	Integration     Integration          `json:"integration"`
	Credentials     []*CredentialListItem `json:"credentials"`
}

// GetCredentialsByIntegrationResponse groups credentials by integration
type GetCredentialsByIntegrationResponse struct {
	Integrations []CredentialsByIntegration `json:"integrations"`
}
