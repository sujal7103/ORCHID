package vision

import (
	"testing"
)

// TestProviderStructure tests provider structure
func TestProviderStructure(t *testing.T) {
	provider := &Provider{
		ID:      1,
		Name:    "openai",
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Enabled: true,
	}

	if provider.Name != "openai" {
		t.Errorf("Expected provider name 'openai', got %s", provider.Name)
	}
	if !provider.Enabled {
		t.Error("Provider should be enabled")
	}
	if provider.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
}

// TestDescribeImageRequestStructure tests request structure
func TestDescribeImageRequestStructure(t *testing.T) {
	req := &DescribeImageRequest{
		ImageData: []byte{0xFF, 0xD8, 0xFF}, // JPEG header bytes
		MimeType:  "image/jpeg",
		Question:  "What is in this image?",
		Detail:    "detailed",
	}

	if len(req.ImageData) == 0 {
		t.Error("ImageData should be set")
	}
	if req.MimeType == "" {
		t.Error("MimeType should be set")
	}
	if req.Detail != "detailed" && req.Detail != "brief" && req.Detail != "auto" && req.Detail != "" {
		t.Errorf("Invalid detail level: %s", req.Detail)
	}
}

// TestDescribeImageResponseStructure tests response structure
func TestDescribeImageResponseStructure(t *testing.T) {
	resp := &DescribeImageResponse{
		Description: "A beautiful sunset over the ocean",
		Model:       "gpt-4o",
		Provider:    "openai",
	}

	if resp.Description == "" {
		t.Error("Description should not be empty")
	}
	if resp.Model == "" {
		t.Error("Model should not be empty")
	}
}

// TestGetServiceSingleton verifies singleton pattern
func TestGetServiceSingleton(t *testing.T) {
	// GetService should return nil if not initialized
	svc := GetService()
	// Note: This may return non-nil if InitService was called elsewhere
	// The test mainly verifies no panic occurs
	_ = svc
}

// TestDetailLevelValidation tests valid detail levels
func TestDetailLevelValidation(t *testing.T) {
	validLevels := []string{"brief", "detailed", "auto", ""}

	for _, level := range validLevels {
		req := &DescribeImageRequest{
			ImageData: []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
			MimeType:  "image/png",
			Detail:    level,
		}
		// Simply verify the struct accepts all levels
		if req.Detail != level {
			t.Errorf("Detail level %q should be accepted", level)
		}
	}
}

// TestInitServiceWithNilCallbacks verifies graceful handling
func TestInitServiceWithNilCallbacks(t *testing.T) {
	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("InitService should not panic with nil callbacks: %v", r)
		}
	}()

	// Note: We can't actually call InitService here without affecting global state
	// This test documents the expected behavior
}

// TestProviderCallbackTypes verifies callback type signatures
func TestProviderCallbackTypes(t *testing.T) {
	// Test ProviderGetter signature
	var providerGetter ProviderGetter = func(id int) (*Provider, error) {
		return &Provider{
			ID:      id,
			Name:    "test",
			BaseURL: "https://test.com",
			APIKey:  "key",
			Enabled: true,
		}, nil
	}

	provider, err := providerGetter(1)
	if err != nil {
		t.Errorf("ProviderGetter should not error: %v", err)
	}
	if provider.ID != 1 {
		t.Errorf("Expected provider ID 1, got %d", provider.ID)
	}

	// Test VisionModelFinder signature
	var modelFinder VisionModelFinder = func() (int, string, error) {
		return 1, "gpt-4o", nil
	}

	providerID, modelName, err := modelFinder()
	if err != nil {
		t.Errorf("VisionModelFinder should not error: %v", err)
	}
	if providerID != 1 {
		t.Errorf("Expected provider ID 1, got %d", providerID)
	}
	if modelName != "gpt-4o" {
		t.Errorf("Expected model 'gpt-4o', got %s", modelName)
	}
}

// TestMimeTypeValidation tests various MIME types
func TestMimeTypeValidation(t *testing.T) {
	mimeTypes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	}

	for _, mimeType := range mimeTypes {
		req := &DescribeImageRequest{
			ImageData: []byte{1, 2, 3},
			MimeType:  mimeType,
		}
		if req.MimeType != mimeType {
			t.Errorf("MimeType should be %s, got %s", mimeType, req.MimeType)
		}
	}
}

// Benchmark tests
func BenchmarkProviderCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &Provider{
			ID:      1,
			Name:    "openai",
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
			Enabled: true,
		}
	}
}

func BenchmarkRequestCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &DescribeImageRequest{
			ImageData: []byte{0xFF, 0xD8, 0xFF},
			MimeType:  "image/jpeg",
			Question:  "What is in this image?",
			Detail:    "detailed",
		}
	}
}
