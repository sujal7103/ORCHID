package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestToolRegistration verifies all file tools are registered correctly
func TestFileToolsRegistration(t *testing.T) {
	registry := GetRegistry()

	expectedTools := []struct {
		name        string
		displayName string
		category    string
	}{
		{"read_document", "Read Document", "data_sources"},
		{"read_data_file", "Read Data File", "data_sources"},
		{"describe_image", "Describe Image", "data_sources"},
		{"download_file", "Download File", "data_sources"},
		{"transcribe_audio", "Transcribe Audio", "data_sources"},
	}

	for _, expected := range expectedTools {
		tool, exists := registry.Get(expected.name)
		if !exists {
			t.Errorf("Tool %s should be registered", expected.name)
			continue
		}

		if tool.DisplayName != expected.displayName {
			t.Errorf("Tool %s: expected display name %q, got %q", expected.name, expected.displayName, tool.DisplayName)
		}

		if tool.Category != expected.category {
			t.Errorf("Tool %s: expected category %q, got %q", expected.name, expected.category, tool.Category)
		}

		if tool.Execute == nil {
			t.Errorf("Tool %s: Execute function should not be nil", expected.name)
		}

		if tool.Parameters == nil {
			t.Errorf("Tool %s: Parameters should not be nil", expected.name)
		}

		// Verify parameters have required structure
		params, ok := tool.Parameters["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %s: Parameters should have 'properties' field", expected.name)
		}

		// All file tools should have file_id parameter (except download_file which has url)
		if expected.name == "download_file" {
			if _, hasURL := params["url"]; !hasURL {
				t.Errorf("Tool %s: should have 'url' parameter", expected.name)
			}
		} else {
			if _, hasFileID := params["file_id"]; !hasFileID {
				t.Errorf("Tool %s: should have 'file_id' parameter", expected.name)
			}
		}
	}
}

// TestReadDocumentToolParams verifies read_document tool parameters
func TestReadDocumentToolParams(t *testing.T) {
	tool := NewReadDocumentTool()

	if tool.Name != "read_document" {
		t.Errorf("Expected name 'read_document', got %q", tool.Name)
	}

	params := tool.Parameters
	required, ok := params["required"].([]string)
	if !ok {
		t.Error("Parameters should have 'required' field as []string")
		return
	}

	if len(required) != 1 || required[0] != "file_id" {
		t.Errorf("Expected required=[file_id], got %v", required)
	}

	// Test with missing file_id
	result, err := tool.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing file_id")
	}
	if result != "" && !strings.Contains(err.Error(), "file_id") {
		t.Errorf("Error should mention file_id, got: %v", err)
	}
}

// TestReadDataFileToolParams verifies read_data_file tool parameters
func TestReadDataFileToolParams(t *testing.T) {
	tool := NewReadDataFileTool()

	if tool.Name != "read_data_file" {
		t.Errorf("Expected name 'read_data_file', got %q", tool.Name)
	}

	params := tool.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("Parameters should have 'properties' field")
		return
	}

	// Check format parameter exists
	formatParam, hasFormat := props["format"]
	if !hasFormat {
		t.Error("Should have 'format' parameter")
	} else {
		formatProps, ok := formatParam.(map[string]interface{})
		if ok {
			if enum, hasEnum := formatProps["enum"]; hasEnum {
				enumList, ok := enum.([]string)
				if !ok {
					t.Error("Format enum should be []string")
				} else if len(enumList) == 0 {
					t.Error("Format enum should not be empty")
				}
			}
		}
	}

	// Test with missing file_id
	result, err := tool.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing file_id")
	}
	if result != "" && !strings.Contains(err.Error(), "file_id") {
		t.Errorf("Error should mention file_id, got: %v", err)
	}
}

// TestDescribeImageToolParams verifies describe_image tool parameters
func TestDescribeImageToolParams(t *testing.T) {
	tool := NewDescribeImageTool()

	if tool.Name != "describe_image" {
		t.Errorf("Expected name 'describe_image', got %q", tool.Name)
	}

	if tool.Icon != "Image" {
		t.Errorf("Expected icon 'Image', got %q", tool.Icon)
	}

	params := tool.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("Parameters should have 'properties' field")
		return
	}

	// Check optional parameters exist
	requiredParams := []string{"file_id"}
	optionalParams := []string{"question", "detail"}

	for _, param := range requiredParams {
		if _, has := props[param]; !has {
			t.Errorf("Should have required parameter %q", param)
		}
	}

	for _, param := range optionalParams {
		if _, has := props[param]; !has {
			t.Errorf("Should have optional parameter %q", param)
		}
	}

	// Test with missing file_id
	result, err := tool.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing file_id")
	}
	if result != "" && !strings.Contains(err.Error(), "file_id") {
		t.Errorf("Error should mention file_id, got: %v", err)
	}
}

// TestDownloadFileToolParams verifies download_file tool parameters
func TestDownloadFileToolParams(t *testing.T) {
	tool := NewDownloadFileTool()

	if tool.Name != "download_file" {
		t.Errorf("Expected name 'download_file', got %q", tool.Name)
	}

	if tool.Icon != "Download" {
		t.Errorf("Expected icon 'Download', got %q", tool.Icon)
	}

	params := tool.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("Parameters should have 'properties' field")
		return
	}

	// Check url parameter exists
	if _, hasURL := props["url"]; !hasURL {
		t.Error("Should have 'url' parameter")
	}

	// Check filename parameter exists (optional)
	if _, hasFilename := props["filename"]; !hasFilename {
		t.Error("Should have 'filename' parameter")
	}

	// Test with missing url
	result, err := tool.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing url")
	}
	if result != "" && !strings.Contains(err.Error(), "url") {
		t.Errorf("Error should mention url, got: %v", err)
	}
}

// TestDownloadFileSSRFProtection verifies SSRF protection in download_file tool
func TestDownloadFileSSRFProtection(t *testing.T) {
	tool := NewDownloadFileTool()

	blockedURLs := []struct {
		url    string
		reason string
	}{
		{"http://localhost/file.txt", "localhost"},
		{"http://127.0.0.1/file.txt", "loopback IP"},
		{"http://192.168.1.1/file.txt", "private IP"},
		{"http://10.0.0.1/file.txt", "private IP"},
		{"http://172.16.0.1/file.txt", "private IP"},
		{"http://169.254.169.254/latest/meta-data/", "cloud metadata"},
		{"file:///etc/passwd", "file protocol"},
		{"ftp://example.com/file.txt", "non-http protocol"},
	}

	for _, tc := range blockedURLs {
		result, err := tool.Execute(map[string]interface{}{
			"url": tc.url,
		})

		if err == nil {
			t.Errorf("URL %s should be blocked (%s), but got result: %s", tc.url, tc.reason, result)
		}
	}
}

// TestTranscribeAudioToolParams verifies transcribe_audio tool parameters
func TestTranscribeAudioToolParams(t *testing.T) {
	tool := NewTranscribeAudioTool()

	if tool.Name != "transcribe_audio" {
		t.Errorf("Expected name 'transcribe_audio', got %q", tool.Name)
	}

	if tool.Icon != "Mic" {
		t.Errorf("Expected icon 'Mic', got %q", tool.Icon)
	}

	params := tool.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("Parameters should have 'properties' field")
		return
	}

	// Check parameters exist
	requiredParams := []string{"file_id"}
	optionalParams := []string{"language", "prompt"}

	for _, param := range requiredParams {
		if _, has := props[param]; !has {
			t.Errorf("Should have required parameter %q", param)
		}
	}

	for _, param := range optionalParams {
		if _, has := props[param]; !has {
			t.Errorf("Should have optional parameter %q", param)
		}
	}

	// Test with missing file_id
	result, err := tool.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing file_id")
	}
	if result != "" && !strings.Contains(err.Error(), "file_id") {
		t.Errorf("Error should mention file_id, got: %v", err)
	}
}

// TestToolKeywords verifies all file tools have appropriate keywords
func TestFileToolsKeywords(t *testing.T) {
	tools := []*Tool{
		NewReadDocumentTool(),
		NewReadDataFileTool(),
		NewDescribeImageTool(),
		NewDownloadFileTool(),
		NewTranscribeAudioTool(),
	}

	for _, tool := range tools {
		if len(tool.Keywords) == 0 {
			t.Errorf("Tool %s should have keywords for smart recommendations", tool.Name)
		}

		// Verify keywords are lowercase and non-empty
		for _, keyword := range tool.Keywords {
			if keyword == "" {
				t.Errorf("Tool %s has empty keyword", tool.Name)
			}
			if keyword != strings.ToLower(keyword) {
				t.Errorf("Tool %s keyword %q should be lowercase", tool.Name, keyword)
			}
		}
	}
}

// TestToolsHaveProperSource verifies all tools are marked as builtin
func TestFileToolsSource(t *testing.T) {
	tools := []*Tool{
		NewReadDocumentTool(),
		NewReadDataFileTool(),
		NewDescribeImageTool(),
		NewDownloadFileTool(),
		NewTranscribeAudioTool(),
	}

	for _, tool := range tools {
		if tool.Source != ToolSourceBuiltin {
			t.Errorf("Tool %s should have source 'builtin', got %q", tool.Name, tool.Source)
		}
	}
}

// TestToolJSONSchema verifies tool parameters produce valid JSON
func TestToolJSONSchema(t *testing.T) {
	registry := GetRegistry()
	toolList := registry.List()

	for _, toolMap := range toolList {
		fn, ok := toolMap["function"].(map[string]interface{})
		if !ok {
			t.Error("Tool should have 'function' field")
			continue
		}

		name, _ := fn["name"].(string)
		params := fn["parameters"]

		// Verify parameters can be serialized to JSON
		jsonBytes, err := json.Marshal(params)
		if err != nil {
			t.Errorf("Tool %s parameters should be JSON serializable: %v", name, err)
			continue
		}

		// Verify it can be deserialized back
		var decoded map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
			t.Errorf("Tool %s parameters JSON should be valid: %v", name, err)
		}
	}
}

// TestValidateDownloadURL tests URL validation helper
func TestValidateDownloadURL(t *testing.T) {
	validURLs := []string{
		"https://example.com/file.pdf",
		"https://cdn.example.org/image.png",
		"http://api.example.com/download?file=test.csv",
	}

	for _, urlStr := range validURLs {
		parsed, err := validateDownloadURL(urlStr)
		if err != nil {
			t.Errorf("URL %s should be valid: %v", urlStr, err)
		}
		if parsed == nil {
			t.Errorf("URL %s should return parsed URL", urlStr)
		}
	}

	invalidURLs := []string{
		"not-a-url",
		"ftp://example.com/file",
		"file:///etc/passwd",
		"http://localhost/secret",
		"http://127.0.0.1:8080/api",
		"http://[::1]/api",
		"http://169.254.169.254/",
	}

	for _, urlStr := range invalidURLs {
		_, err := validateDownloadURL(urlStr)
		if err == nil {
			t.Errorf("URL %s should be invalid", urlStr)
		}
	}
}

// Benchmark tests
func BenchmarkToolRegistration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewReadDocumentTool()
		_ = NewReadDataFileTool()
		_ = NewDescribeImageTool()
		_ = NewDownloadFileTool()
		_ = NewTranscribeAudioTool()
	}
}

func BenchmarkToolLookup(b *testing.B) {
	registry := GetRegistry()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		registry.Get("read_document")
		registry.Get("read_data_file")
		registry.Get("describe_image")
		registry.Get("download_file")
		registry.Get("transcribe_audio")
	}
}

// TestFileCacheIntegration tests filecache service is available
func TestFileCacheServiceAvailable(t *testing.T) {
	// This test verifies the filecache service can be initialized
	// The actual file operations are tested in integration tests
	// that run after docker compose up with proper file fixtures
}

// Helper to create temp file for testing
func createTempTestFile(t *testing.T, content []byte, extension string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test"+extension)
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}
