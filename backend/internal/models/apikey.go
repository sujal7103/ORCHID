package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID  string             `bson:"userId" json:"userId"`

	// Key info (hash stored, never plain text)
	KeyPrefix string `bson:"keyPrefix" json:"keyPrefix"`     // First 8 chars for display (e.g., "clv_a1b2")
	KeyHash   string `bson:"keyHash" json:"-"`               // bcrypt hash, never exposed in JSON
	PlainKey  string `bson:"plainKey,omitempty" json:"key"` // TEMPORARY: Plain key for early platform phase

	// Metadata
	Name        string `bson:"name" json:"name"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`

	// Permissions
	Scopes []string `bson:"scopes" json:"scopes"` // e.g., ["execute:*"], ["execute:agent-123", "read:executions"]

	// Rate limits (tier-based defaults can be overridden)
	RateLimit *APIKeyRateLimit `bson:"rateLimit,omitempty" json:"rateLimit,omitempty"`

	// Status
	LastUsedAt *time.Time `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"` // Soft delete
	ExpiresAt  *time.Time `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"` // Optional expiration

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// APIKeyRateLimit defines custom rate limits for an API key
type APIKeyRateLimit struct {
	RequestsPerMinute int64 `bson:"requestsPerMinute" json:"requestsPerMinute"`
	RequestsPerHour   int64 `bson:"requestsPerHour" json:"requestsPerHour"`
}

// IsRevoked returns true if the API key has been revoked
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsExpired returns true if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid returns true if the API key is not revoked and not expired
func (k *APIKey) IsValid() bool {
	return !k.IsRevoked() && !k.IsExpired()
}

// HasScope checks if the API key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope || s == "*" {
			return true
		}
		// Check wildcard patterns like "execute:*"
		if matchWildcardScope(s, scope) {
			return true
		}
	}
	return false
}

// HasExecuteScope checks if the API key can execute a specific agent
func (k *APIKey) HasExecuteScope(agentID string) bool {
	// Check for universal execute permission
	if k.HasScope("execute:*") {
		return true
	}
	// Check for specific agent permission
	return k.HasScope("execute:" + agentID)
}

// HasReadExecutionsScope checks if the API key can read executions
func (k *APIKey) HasReadExecutionsScope() bool {
	return k.HasScope("read:executions") || k.HasScope("read:*") || k.HasScope("*")
}

// matchWildcardScope checks if a wildcard scope matches a target scope
// e.g., "execute:*" matches "execute:agent-123"
func matchWildcardScope(pattern, target string) bool {
	if len(pattern) < 2 || pattern[len(pattern)-1] != '*' {
		return false
	}
	prefix := pattern[:len(pattern)-1] // Remove the '*'
	return len(target) >= len(prefix) && target[:len(prefix)] == prefix
}

// CreateAPIKeyRequest is the request body for creating an API key
type CreateAPIKeyRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Scopes      []string          `json:"scopes"`                   // Required: what the key can do
	RateLimit   *APIKeyRateLimit  `json:"rateLimit,omitempty"`      // Optional: custom rate limits
	ExpiresIn   int               `json:"expiresIn,omitempty"`      // Optional: expiration in days
}

// CreateAPIKeyResponse is returned after creating an API key
// This is the ONLY time the full key is returned
type CreateAPIKeyResponse struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`       // Full API key (ONLY shown once)
	KeyPrefix string    `json:"keyPrefix"` // Display prefix
	Name      string    `json:"name"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// APIKeyListItem is a safe representation of an API key for listing
// Never includes the key hash
type APIKeyListItem struct {
	ID          string     `json:"id"`
	KeyPrefix   string     `json:"keyPrefix"`
	Key         string     `json:"key,omitempty"` // TEMPORARY: Plain key for early platform phase
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Scopes      []string   `json:"scopes"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	IsRevoked   bool       `json:"isRevoked"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// ToListItem converts an APIKey to a safe list representation
func (k *APIKey) ToListItem() *APIKeyListItem {
	return &APIKeyListItem{
		ID:          k.ID.Hex(),
		KeyPrefix:   k.KeyPrefix,
		Key:         k.PlainKey, // TEMPORARY: Include plain key for early platform phase
		Name:        k.Name,
		Description: k.Description,
		Scopes:      k.Scopes,
		LastUsedAt:  k.LastUsedAt,
		ExpiresAt:   k.ExpiresAt,
		IsRevoked:   k.IsRevoked(),
		CreatedAt:   k.CreatedAt,
	}
}

// TriggerAgentRequest is the request body for triggering an agent via API
type TriggerAgentRequest struct {
	Input map[string]interface{} `json:"input,omitempty"`

	// EnableBlockChecker enables block completion validation (optional)
	// When true, each block is checked to ensure it accomplished its job
	EnableBlockChecker bool `json:"enable_block_checker,omitempty"`

	// CheckerModelID is the model to use for block checking (optional)
	// Defaults to gpt-4o-mini for fast, cheap validation
	CheckerModelID string `json:"checker_model_id,omitempty"`
}

// TriggerAgentResponse is returned after triggering an agent
type TriggerAgentResponse struct {
	ExecutionID string `json:"executionId"`
	Status      string `json:"status"` // "queued" or "running"
	Message     string `json:"message"`
}

// ValidScopes lists all valid API key scopes
var ValidScopes = []string{
	"execute:*",       // Execute any agent
	"upload",          // Upload files for workflow inputs
	"read:executions", // Read execution history
	"read:*",          // Read all resources
	"*",               // Full access (admin)
}

// IsValidScope checks if a scope is valid
func IsValidScope(scope string) bool {
	// Check exact match
	for _, valid := range ValidScopes {
		if scope == valid {
			return true
		}
	}
	// Check agent-specific execute scope (execute:agent-xxx)
	if len(scope) > 8 && scope[:8] == "execute:" {
		return true
	}
	return false
}
