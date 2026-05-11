package services

import (
	"clara-agents/internal/models"
	"context"
	"strings"
	"testing"
)

func TestNewAPIKeyService(t *testing.T) {
	// Test creation without MongoDB (nil)
	service := NewAPIKeyService(nil, nil)
	if service == nil {
		t.Fatal("Expected non-nil API key service")
	}
}

func TestAPIKeyService_GenerateKey(t *testing.T) {
	service := NewAPIKeyService(nil, nil)

	key, err := service.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Check prefix
	if !strings.HasPrefix(key, APIKeyPrefix) {
		t.Errorf("Expected key to start with '%s', got '%s'", APIKeyPrefix, key[:len(APIKeyPrefix)])
	}

	// Check length (prefix + 64 hex chars)
	expectedLen := len(APIKeyPrefix) + APIKeyLength*2
	if len(key) != expectedLen {
		t.Errorf("Expected key length %d, got %d", expectedLen, len(key))
	}

	// Generate another key - should be different
	key2, err := service.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate second key: %v", err)
	}

	if key == key2 {
		t.Error("Generated keys should be unique")
	}
}

func TestAPIKeyService_HashAndVerify(t *testing.T) {
	service := NewAPIKeyService(nil, nil)

	key, _ := service.GenerateKey()

	// Hash the key
	hash, err := service.HashKey(key)
	if err != nil {
		t.Fatalf("Failed to hash key: %v", err)
	}

	// Hash should not be empty
	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// Hash should not equal the key
	if hash == key {
		t.Error("Hash should not equal the original key")
	}

	// Verify correct key
	if !service.VerifyKey(key, hash) {
		t.Error("VerifyKey should return true for correct key")
	}

	// Verify wrong key
	wrongKey := key + "x"
	if service.VerifyKey(wrongKey, hash) {
		t.Error("VerifyKey should return false for wrong key")
	}
}

func TestAPIKeyModel_Scopes(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		check    string
		expected bool
	}{
		{
			name:     "exact match",
			scopes:   []string{"execute:*", "read:executions"},
			check:    "execute:*",
			expected: true,
		},
		{
			name:     "wildcard execute",
			scopes:   []string{"execute:*"},
			check:    "execute:agent-123",
			expected: true,
		},
		{
			name:     "specific agent",
			scopes:   []string{"execute:agent-123"},
			check:    "execute:agent-123",
			expected: true,
		},
		{
			name:     "wrong agent",
			scopes:   []string{"execute:agent-123"},
			check:    "execute:agent-456",
			expected: false,
		},
		{
			name:     "full access",
			scopes:   []string{"*"},
			check:    "execute:agent-123",
			expected: true,
		},
		{
			name:     "no match",
			scopes:   []string{"read:executions"},
			check:    "execute:*",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &models.APIKey{Scopes: tt.scopes}
			result := key.HasScope(tt.check)
			if result != tt.expected {
				t.Errorf("HasScope(%s) = %v, expected %v", tt.check, result, tt.expected)
			}
		})
	}
}

func TestAPIKeyModel_HasExecuteScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		agentID  string
		expected bool
	}{
		{
			name:     "wildcard execute",
			scopes:   []string{"execute:*"},
			agentID:  "agent-123",
			expected: true,
		},
		{
			name:     "specific agent match",
			scopes:   []string{"execute:agent-123"},
			agentID:  "agent-123",
			expected: true,
		},
		{
			name:     "specific agent no match",
			scopes:   []string{"execute:agent-456"},
			agentID:  "agent-123",
			expected: false,
		},
		{
			name:     "no execute scope",
			scopes:   []string{"read:executions"},
			agentID:  "agent-123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &models.APIKey{Scopes: tt.scopes}
			result := key.HasExecuteScope(tt.agentID)
			if result != tt.expected {
				t.Errorf("HasExecuteScope(%s) = %v, expected %v", tt.agentID, result, tt.expected)
			}
		})
	}
}

func TestAPIKeyModel_IsValid(t *testing.T) {
	key := &models.APIKey{}

	// Should be valid by default (no revocation, no expiration)
	if !key.IsValid() {
		t.Error("New key should be valid")
	}

	if key.IsRevoked() {
		t.Error("New key should not be revoked")
	}

	if key.IsExpired() {
		t.Error("New key should not be expired")
	}
}

func TestAPIKeyListItem_Conversion(t *testing.T) {
	key := &models.APIKey{
		KeyPrefix:   "clv_test1234",
		Name:        "Test Key",
		Description: "A test key",
		Scopes:      []string{"execute:*"},
	}

	item := key.ToListItem()

	if item.KeyPrefix != key.KeyPrefix {
		t.Errorf("KeyPrefix mismatch: got %s, want %s", item.KeyPrefix, key.KeyPrefix)
	}
	if item.Name != key.Name {
		t.Errorf("Name mismatch: got %s, want %s", item.Name, key.Name)
	}
	if item.Description != key.Description {
		t.Errorf("Description mismatch: got %s, want %s", item.Description, key.Description)
	}
	if len(item.Scopes) != len(key.Scopes) {
		t.Errorf("Scopes length mismatch: got %d, want %d", len(item.Scopes), len(key.Scopes))
	}
}

func TestIsValidScope(t *testing.T) {
	tests := []struct {
		scope    string
		expected bool
	}{
		{"execute:*", true},
		{"read:executions", true},
		{"read:*", true},
		{"*", true},
		{"execute:agent-123", true},
		{"invalid", false},
		{"write:*", false},
		{"delete:*", false},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			result := models.IsValidScope(tt.scope)
			if result != tt.expected {
				t.Errorf("IsValidScope(%s) = %v, expected %v", tt.scope, result, tt.expected)
			}
		})
	}
}

func TestAPIKeyService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require MongoDB
	_ = context.Background()
	service := NewAPIKeyService(nil, nil)
	if service == nil {
		t.Fatal("Expected non-nil service")
	}
}
