package execution

import (
	"context"
	"testing"

	"clara-agents/internal/models"
)

// TestFileReferenceDetection tests isFileReference helper
func TestFileReferenceDetection(t *testing.T) {
	testCases := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "valid file reference",
			value:    map[string]any{"file_id": "abc123", "filename": "test.pdf"},
			expected: true,
		},
		{
			name:     "file_id only",
			value:    map[string]any{"file_id": "abc123"},
			expected: true,
		},
		{
			name:     "no file_id",
			value:    map[string]any{"filename": "test.pdf"},
			expected: false,
		},
		{
			name:     "string value",
			value:    "just a string",
			expected: false,
		},
		{
			name:     "number value",
			value:    123,
			expected: false,
		},
		{
			name:     "nil value",
			value:    nil,
			expected: false,
		},
		{
			name:     "empty map",
			value:    map[string]any{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isFileReference(tc.value)
			if result != tc.expected {
				t.Errorf("isFileReference(%v) = %v, expected %v", tc.value, result, tc.expected)
			}
		})
	}
}

// TestGetFileType tests MIME type categorization
func TestGetFileType(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected string
	}{
		// Images
		{"image/jpeg", "image"},
		{"image/png", "image"},
		{"image/gif", "image"},
		{"image/webp", "image"},
		{"image/svg+xml", "image"},

		// Audio
		{"audio/mpeg", "audio"},
		{"audio/wav", "audio"},
		{"audio/mp4", "audio"},
		{"audio/ogg", "audio"},

		// Documents
		{"application/pdf", "document"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "document"},
		{"application/msword", "document"},

		// Data files
		{"application/json", "data"},
		{"text/csv", "data"},
		{"text/plain", "data"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "data"},

		// Unknown defaults to data
		{"application/octet-stream", "data"},
		{"video/mp4", "data"},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			result := getFileType(tc.mimeType)
			if result != tc.expected {
				t.Errorf("getFileType(%s) = %s, expected %s", tc.mimeType, result, tc.expected)
			}
		})
	}
}

// TestFileReferenceStruct tests FileReference structure
func TestFileReferenceStruct(t *testing.T) {
	ref := FileReference{
		FileID:   "file-123",
		Filename: "document.pdf",
		MimeType: "application/pdf",
		Size:     12345,
		Type:     "document",
	}

	if ref.FileID == "" {
		t.Error("FileID should be set")
	}
	if ref.Type != "document" {
		t.Errorf("Type should be 'document', got %s", ref.Type)
	}
}

// TestVariableExecutorCreation tests executor creation
func TestVariableExecutorCreation(t *testing.T) {
	executor := NewVariableExecutor()
	if executor == nil {
		t.Fatal("NewVariableExecutor should return non-nil executor")
	}
}

// TestVariableReadOperation tests read operation
func TestVariableReadOperation(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Read Variable",
		Config: map[string]any{
			"operation":    "read",
			"variableName": "testVar",
		},
	}

	inputs := map[string]any{
		"testVar": "hello world",
	}

	result, err := executor.Execute(context.Background(), block, inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result["value"] != "hello world" {
		t.Errorf("Expected 'hello world', got %v", result["value"])
	}
	if result["testVar"] != "hello world" {
		t.Errorf("Expected testVar='hello world', got %v", result["testVar"])
	}
}

// TestVariableReadWithDefault tests read operation with default value
func TestVariableReadWithDefault(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Read Variable",
		Config: map[string]any{
			"operation":    "read",
			"variableName": "missingVar",
			"defaultValue": "default value",
		},
	}

	inputs := map[string]any{}

	result, err := executor.Execute(context.Background(), block, inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result["value"] != "default value" {
		t.Errorf("Expected 'default value', got %v", result["value"])
	}
}

// TestVariableReadMissing tests read of missing variable without default
func TestVariableReadMissing(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Read Variable",
		Config: map[string]any{
			"operation":    "read",
			"variableName": "missingVar",
		},
	}

	inputs := map[string]any{}

	result, err := executor.Execute(context.Background(), block, inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should return nil for missing variable
	if result["value"] != nil {
		t.Errorf("Expected nil, got %v", result["value"])
	}
}

// TestVariableSetOperation tests set operation
func TestVariableSetOperation(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Set Variable",
		Config: map[string]any{
			"operation":    "set",
			"variableName": "newVar",
			"value":        "set value",
		},
	}

	inputs := map[string]any{}

	result, err := executor.Execute(context.Background(), block, inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result["value"] != "set value" {
		t.Errorf("Expected 'set value', got %v", result["value"])
	}
	if result["newVar"] != "set value" {
		t.Errorf("Expected newVar='set value', got %v", result["newVar"])
	}
}

// TestVariableSetFromExpression tests set operation with expression
func TestVariableSetFromExpression(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Set Variable",
		Config: map[string]any{
			"operation":       "set",
			"variableName":    "result",
			"valueExpression": "sourceData",
		},
	}

	inputs := map[string]any{
		"sourceData": "value from source",
	}

	result, err := executor.Execute(context.Background(), block, inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result["value"] != "value from source" {
		t.Errorf("Expected 'value from source', got %v", result["value"])
	}
}

// TestVariableMissingName tests error for missing variable name
func TestVariableMissingName(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Bad Block",
		Config: map[string]any{
			"operation": "read",
			// Missing variableName
		},
	}

	inputs := map[string]any{}

	_, err := executor.Execute(context.Background(), block, inputs)
	if err == nil {
		t.Error("Expected error for missing variableName")
	}
}

// TestVariableUnknownOperation tests error for unknown operation
func TestVariableUnknownOperation(t *testing.T) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Bad Block",
		Config: map[string]any{
			"operation":    "invalid",
			"variableName": "test",
		},
	}

	inputs := map[string]any{}

	_, err := executor.Execute(context.Background(), block, inputs)
	if err == nil {
		t.Error("Expected error for unknown operation")
	}
}

// TestGetKeysHelper tests getKeys helper function
func TestGetKeysHelper(t *testing.T) {
	m := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	keys := getKeys(m)
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all keys are present (order doesn't matter)
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	for _, expected := range []string{"a", "b", "c"} {
		if !keyMap[expected] {
			t.Errorf("Expected key %s not found", expected)
		}
	}
}

// Benchmark tests
func BenchmarkVariableRead(b *testing.B) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Read Variable",
		Config: map[string]any{
			"operation":    "read",
			"variableName": "testVar",
		},
	}

	inputs := map[string]any{
		"testVar": "test value",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, block, inputs)
	}
}

func BenchmarkVariableSet(b *testing.B) {
	executor := NewVariableExecutor()

	block := models.Block{
		ID:   "var-block",
		Name: "Set Variable",
		Config: map[string]any{
			"operation":    "set",
			"variableName": "testVar",
			"value":        "test value",
		},
	}

	inputs := map[string]any{}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, block, inputs)
	}
}

func BenchmarkIsFileReference(b *testing.B) {
	testCases := []any{
		map[string]any{"file_id": "abc123"},
		"just a string",
		123,
		nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			isFileReference(tc)
		}
	}
}

func BenchmarkGetFileType(b *testing.B) {
	mimeTypes := []string{
		"image/jpeg",
		"audio/mpeg",
		"application/pdf",
		"application/json",
		"text/plain",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, mime := range mimeTypes {
			getFileType(mime)
		}
	}
}
