package services

import (
	"testing"
)

// TestNormalizeContent tests content normalization for deduplication
func TestNormalizeContent(t *testing.T) {
	service := &MemoryStorageService{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic normalization",
			input:    "User prefers dark mode",
			expected: "user prefers dark mode",
		},
		{
			name:     "Remove punctuation",
			input:    "User's name is John, and he likes coffee!",
			expected: "users name is john and he likes coffee",
		},
		{
			name:     "Collapse whitespace",
			input:    "User   likes    lots   of   spaces",
			expected: "user likes lots of spaces",
		},
		{
			name:     "Mixed case and punctuation",
			input:    "User PREFERS Dark-Mode!!!",
			expected: "user prefers dark mode",
		},
		{
			name:     "Trim whitespace",
			input:    "  user prefers dark mode  ",
			expected: "user prefers dark mode",
		},
		{
			name:     "Numbers preserved",
			input:    "User is 25 years old",
			expected: "user is 25 years old",
		},
		{
			name:     "Special characters removed",
			input:    "User's email: john@example.com",
			expected: "users email johnexamplecom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.normalizeContent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestCalculateHash ensures consistent hashing
func TestCalculateHash(t *testing.T) {
	service := &MemoryStorageService{}

	tests := []struct {
		name    string
		input1  string
		input2  string
		shouldMatch bool
	}{
		{
			name:    "Identical strings",
			input1:  "user prefers dark mode",
			input2:  "user prefers dark mode",
			shouldMatch: true,
		},
		{
			name:    "Different strings",
			input1:  "user prefers dark mode",
			input2:  "user prefers light mode",
			shouldMatch: false,
		},
		{
			name:    "Empty string",
			input1:  "",
			input2:  "",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := service.calculateHash(tt.input1)
			hash2 := service.calculateHash(tt.input2)

			if tt.shouldMatch && hash1 != hash2 {
				t.Errorf("Expected hashes to match: %s != %s", hash1, hash2)
			}

			if !tt.shouldMatch && hash1 == hash2 {
				t.Errorf("Expected hashes to differ, both got: %s", hash1)
			}

			// Verify SHA-256 produces 64 character hex string
			if len(hash1) != 64 {
				t.Errorf("Expected 64 character hash, got %d", len(hash1))
			}
		})
	}
}

// TestDeduplicationLogic tests the deduplication flow
func TestDeduplicationLogic(t *testing.T) {
	service := &MemoryStorageService{}

	// Test cases that should be considered duplicates after normalization
	duplicates := []struct {
		original string
		variant  string
	}{
		{
			original: "User prefers dark mode",
			variant:  "USER PREFERS DARK MODE",
		},
		{
			original: "User prefers dark mode",
			variant:  "User prefers dark-mode!",
		},
		{
			original: "User likes coffee",
			variant:  "User   likes   coffee!!!",
		},
		{
			original: "User name is John",
			variant:  "  User name is John  ",
		},
	}

	for _, tt := range duplicates {
		t.Run(tt.original, func(t *testing.T) {
			normalized1 := service.normalizeContent(tt.original)
			normalized2 := service.normalizeContent(tt.variant)

			hash1 := service.calculateHash(normalized1)
			hash2 := service.calculateHash(normalized2)

			if hash1 != hash2 {
				t.Errorf("Expected duplicates to have same hash:\n  Original: %q -> %q -> %s\n  Variant:  %q -> %q -> %s",
					tt.original, normalized1, hash1,
					tt.variant, normalized2, hash2,
				)
			}
		})
	}

	// Test cases that should NOT be considered duplicates
	nonDuplicates := []struct {
		content1 string
		content2 string
	}{
		{
			content1: "User prefers dark mode",
			content2: "User prefers light mode",
		},
		{
			content1: "User likes coffee",
			content2: "User likes tea",
		},
		{
			content1: "User is 25 years old",
			content2: "User is 30 years old",
		},
	}

	for i, tt := range nonDuplicates {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			normalized1 := service.normalizeContent(tt.content1)
			normalized2 := service.normalizeContent(tt.content2)

			hash1 := service.calculateHash(normalized1)
			hash2 := service.calculateHash(normalized2)

			if hash1 == hash2 {
				t.Errorf("Expected different hashes:\n  Content1: %q -> %q\n  Content2: %q -> %q\n  Both got: %s",
					tt.content1, normalized1,
					tt.content2, normalized2,
					hash1,
				)
			}
		})
	}
}

// TestContentHashCollisions checks for hash collision resistance
func TestContentHashCollisions(t *testing.T) {
	service := &MemoryStorageService{}

	// Generate 1000 different normalized contents with guaranteed uniqueness
	contents := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		// Use index to guarantee uniqueness
		contents[i] = service.normalizeContent(string(rune('a'+(i%26))) + " test content " + string(rune('0'+(i%10))) + string(rune('0'+((i/10)%10))) + string(rune('0'+((i/100)%10))))
	}

	// Calculate hashes and check for collisions
	hashes := make(map[string]string)
	collisionCount := 0
	for _, content := range contents {
		hash := service.calculateHash(content)

		if existingContent, exists := hashes[hash]; exists {
			// Only report as collision if content is actually different
			if existingContent != content {
				t.Errorf("Hash collision detected!\n  Hash: %s\n  Content1: %q\n  Content2: %q",
					hash, existingContent, content)
				collisionCount++
			}
		} else {
			hashes[hash] = content
		}
	}

	if collisionCount > 0 {
		t.Errorf("Found %d true hash collisions", collisionCount)
	}

	t.Logf("Generated %d unique hashes without collisions", len(hashes))
}

// TestNormalizationEdgeCases tests edge cases in normalization
func TestNormalizationEdgeCases(t *testing.T) {
	service := &MemoryStorageService{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Only punctuation",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "     ",
			expected: "",
		},
		{
			name:     "Unicode characters",
			input:    "User likes café ☕",
			expected: "user likes caf",
		},
		{
			name:     "Mixed alphanumeric",
			input:    "abc123DEF456",
			expected: "abc123def456",
		},
		{
			name:     "Newlines and tabs",
			input:    "User\nprefers\tdark\nmode",
			expected: "user prefers dark mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.normalizeContent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestHashConsistency ensures hashing is deterministic
func TestHashConsistency(t *testing.T) {
	service := &MemoryStorageService{}

	content := "user prefers dark mode"

	// Calculate hash multiple times
	hash1 := service.calculateHash(content)
	hash2 := service.calculateHash(content)
	hash3 := service.calculateHash(content)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash should be deterministic, got different values: %s, %s, %s", hash1, hash2, hash3)
	}
}

// TestMemoryCategories ensures category values are valid
func TestMemoryCategories(t *testing.T) {
	validCategories := []string{
		"personal_info",
		"preferences",
		"context",
		"fact",
		"instruction",
	}

	// This test documents the expected categories
	t.Logf("Valid memory categories: %v", validCategories)

	for _, category := range validCategories {
		if category == "" {
			t.Errorf("Category should not be empty")
		}
	}
}

// BenchmarkNormalizeContent benchmarks content normalization
func BenchmarkNormalizeContent(b *testing.B) {
	service := &MemoryStorageService{}
	content := "User prefers dark mode and likes to use the application at night!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.normalizeContent(content)
	}
}

// BenchmarkCalculateHash benchmarks hash calculation
func BenchmarkCalculateHash(b *testing.B) {
	service := &MemoryStorageService{}
	content := "user prefers dark mode and likes to use the application at night"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.calculateHash(content)
	}
}

// BenchmarkDeduplicationPipeline benchmarks the full deduplication pipeline
func BenchmarkDeduplicationPipeline(b *testing.B) {
	service := &MemoryStorageService{}
	content := "User prefers dark mode and likes to use the application at night!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalized := service.normalizeContent(content)
		service.calculateHash(normalized)
	}
}
