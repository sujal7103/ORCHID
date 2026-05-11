package tools

import (
	"fmt"

	"clara-agents/internal/models"
)

// CredentialResolver is a function type for resolving credentials at runtime
// This is injected into tool args to provide access to credentials without
// exposing the credential service directly to tools
type CredentialResolver func(credentialID string) (*models.DecryptedCredential, error)

// ContextKey for passing credential resolver through args
const CredentialResolverKey = "__credential_resolver__"
const UserIDKey = "__user_id__"

// Note: CreateCredentialResolver is defined in services/credential_service.go
// to avoid import cycles (services imports tools, so tools cannot import services)

// ResolveWebhookURL resolves a webhook URL from either direct URL or credential ID
// Priority: 1. Direct webhook_url parameter, 2. credential_id lookup
func ResolveWebhookURL(args map[string]interface{}, integrationType string) (string, error) {
	// First, check for direct webhook_url
	if webhookURL, ok := args["webhook_url"].(string); ok && webhookURL != "" {
		return webhookURL, nil
	}

	// Check for credential_id
	credentialID, hasCredID := args["credential_id"].(string)
	if !hasCredID || credentialID == "" {
		return "", fmt.Errorf("either webhook_url or credential_id is required")
	}

	// Get credential resolver from args
	resolver, ok := args[CredentialResolverKey].(CredentialResolver)
	if !ok || resolver == nil {
		return "", fmt.Errorf("credential resolver not available")
	}

	// Resolve the credential
	cred, err := resolver(credentialID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve credential: %w", err)
	}

	// Verify integration type matches
	if cred.IntegrationType != integrationType {
		return "", fmt.Errorf("credential type mismatch: expected %s, got %s", integrationType, cred.IntegrationType)
	}

	// Extract webhook URL from credential data
	webhookURL, ok := cred.Data["webhook_url"].(string)
	if !ok || webhookURL == "" {
		// Try alternate key names
		if url, ok := cred.Data["url"].(string); ok && url != "" {
			webhookURL = url
		} else {
			return "", fmt.Errorf("credential does not contain a valid webhook URL")
		}
	}

	return webhookURL, nil
}

// ResolveAPIKey resolves an API key from either direct parameter or credential ID
func ResolveAPIKey(args map[string]interface{}, integrationType string, keyFieldName string) (string, error) {
	// First, check for direct API key
	if apiKey, ok := args[keyFieldName].(string); ok && apiKey != "" {
		return apiKey, nil
	}

	// Check for credential_id
	credentialID, hasCredID := args["credential_id"].(string)
	if !hasCredID || credentialID == "" {
		return "", fmt.Errorf("either %s or credential_id is required", keyFieldName)
	}

	// Get credential resolver from args
	resolver, ok := args[CredentialResolverKey].(CredentialResolver)
	if !ok || resolver == nil {
		return "", fmt.Errorf("credential resolver not available")
	}

	// Resolve the credential
	cred, err := resolver(credentialID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve credential: %w", err)
	}

	// Verify integration type matches
	if cred.IntegrationType != integrationType {
		return "", fmt.Errorf("credential type mismatch: expected %s, got %s", integrationType, cred.IntegrationType)
	}

	// Extract API key from credential data
	apiKey, ok := cred.Data[keyFieldName].(string)
	if !ok || apiKey == "" {
		// Try alternate key names
		if key, ok := cred.Data["api_key"].(string); ok && key != "" {
			apiKey = key
		} else if key, ok := cred.Data["token"].(string); ok && key != "" {
			apiKey = key
		} else {
			return "", fmt.Errorf("credential does not contain a valid API key")
		}
	}

	return apiKey, nil
}

// GetCredentialData retrieves all data from a credential by ID
func GetCredentialData(args map[string]interface{}, integrationType string) (map[string]interface{}, error) {
	// Check for credential_id
	credentialID, hasCredID := args["credential_id"].(string)
	if !hasCredID || credentialID == "" {
		return nil, fmt.Errorf("credential_id is required")
	}

	// Get credential resolver from args
	resolver, ok := args[CredentialResolverKey].(CredentialResolver)
	if !ok || resolver == nil {
		return nil, fmt.Errorf("credential resolver not available")
	}

	// Resolve the credential
	cred, err := resolver(credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve credential: %w", err)
	}

	// Verify integration type matches (if provided)
	if integrationType != "" && cred.IntegrationType != integrationType {
		return nil, fmt.Errorf("credential type mismatch: expected %s, got %s", integrationType, cred.IntegrationType)
	}

	return cred.Data, nil
}
