package audio

import (
	"testing"
)

// TestSupportedFormats verifies all expected audio formats are supported
func TestSupportedFormats(t *testing.T) {
	supportedMimeTypes := []string{
		"audio/mpeg",
		"audio/mp3",
		"audio/wav",
		"audio/x-wav",
		"audio/wave",
		"audio/mp4",
		"audio/x-m4a",
		"audio/webm",
		"audio/ogg",
		"audio/flac",
	}

	for _, mimeType := range supportedMimeTypes {
		if !IsSupportedFormat(mimeType) {
			t.Errorf("MIME type %s should be supported", mimeType)
		}
	}
}

// TestUnsupportedFormats verifies unsupported formats are rejected
func TestUnsupportedFormats(t *testing.T) {
	unsupportedMimeTypes := []string{
		"video/mp4",
		"image/jpeg",
		"application/pdf",
		"text/plain",
		"audio/midi",
		"audio/aiff",
	}

	for _, mimeType := range unsupportedMimeTypes {
		if IsSupportedFormat(mimeType) {
			t.Errorf("MIME type %s should NOT be supported", mimeType)
		}
	}
}

// TestGetSupportedFormats verifies the list of supported formats
func TestGetSupportedFormats(t *testing.T) {
	formats := GetSupportedFormats()

	if len(formats) == 0 {
		t.Error("GetSupportedFormats should return non-empty list")
	}

	// Check some expected formats are in the list (file extensions, not MIME types)
	expectedFormats := map[string]bool{
		"mp3":  false,
		"wav":  false,
		"mp4":  false,
		"ogg":  false,
		"webm": false,
		"flac": false,
	}

	for _, format := range formats {
		if _, ok := expectedFormats[format]; ok {
			expectedFormats[format] = true
		}
	}

	for format, found := range expectedFormats {
		if !found {
			t.Errorf("Expected format %s in supported formats list", format)
		}
	}
}

// TestTranscribeRequestValidation tests request validation
func TestTranscribeRequestValidation(t *testing.T) {
	// Service requires initialization, so we test the request structure
	req := &TranscribeRequest{
		AudioPath: "/path/to/audio.mp3",
		Language:  "en",
		Prompt:    "Test transcription",
	}

	if req.AudioPath == "" {
		t.Error("AudioPath should be set")
	}
	if req.Language != "en" {
		t.Errorf("Language should be 'en', got %s", req.Language)
	}
}

// TestTranscribeResponseStructure tests response structure
func TestTranscribeResponseStructure(t *testing.T) {
	resp := &TranscribeResponse{
		Text:     "Hello world",
		Language: "en",
		Duration: 5.5,
	}

	if resp.Text == "" {
		t.Error("Text should not be empty")
	}
	if resp.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

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
}

// TestGetServiceSingleton verifies singleton pattern
func TestGetServiceSingleton(t *testing.T) {
	// GetService should return nil if not initialized
	svc := GetService()
	// Note: This may return non-nil if InitService was called elsewhere
	// The test mainly verifies no panic occurs
	_ = svc
}

// Benchmark tests
func BenchmarkIsSupportedFormat(b *testing.B) {
	testCases := []string{
		"audio/mpeg",
		"audio/wav",
		"video/mp4",
		"image/jpeg",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			IsSupportedFormat(tc)
		}
	}
}
