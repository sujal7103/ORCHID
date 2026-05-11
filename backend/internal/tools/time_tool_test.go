package tools

import (
	"strings"
	"testing"
	"time"
)

func TestNewTimeTool(t *testing.T) {
	tool := NewTimeTool()

	if tool.Name != "get_current_time" {
		t.Errorf("Expected tool name 'get_current_time', got %s", tool.Name)
	}

	if tool.DisplayName != "Get Current Time" {
		t.Errorf("Expected display name 'Get Current Time', got %s", tool.DisplayName)
	}

	if tool.Category != "time" {
		t.Errorf("Expected category 'time', got %s", tool.Category)
	}

	if tool.Source != ToolSourceBuiltin {
		t.Errorf("Expected source 'builtin', got %s", tool.Source)
	}

	if len(tool.Keywords) == 0 {
		t.Error("Expected keywords to be populated")
	}

	// Verify keywords contain expected terms
	expectedKeywords := []string{"time", "date", "clock", "now"}
	for _, keyword := range expectedKeywords {
		found := false
		for _, k := range tool.Keywords {
			if k == keyword {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected keyword '%s' to be present", keyword)
		}
	}
}

func TestExecuteGetCurrentTime_DefaultUTC(t *testing.T) {
	// Execute without timezone - should default to UTC
	args := map[string]interface{}{}

	result, err := executeGetCurrentTime(args)
	if err != nil {
		t.Fatalf("Failed to execute get_current_time with default timezone: %v", err)
	}

	// Verify result contains UTC
	if !strings.Contains(result, "UTC") {
		t.Errorf("Expected result to contain 'UTC', got: %s", result)
	}

	// Verify format (should be parseable)
	_, parseErr := time.Parse("2006-01-02 15:04:05 MST", result)
	if parseErr != nil {
		t.Errorf("Result has invalid time format: %s", result)
	}
}

func TestExecuteGetCurrentTime_WithTimezone(t *testing.T) {
	testCases := []struct {
		timezone     string
		expectedPart string
	}{
		{"UTC", "UTC"},
		{"America/New_York", "EST"}, // or EDT depending on daylight savings
		{"Asia/Tokyo", "JST"},
		{"Europe/London", "GMT"}, // or BST depending on daylight savings
	}

	for _, tc := range testCases {
		t.Run(tc.timezone, func(t *testing.T) {
			args := map[string]interface{}{
				"timezone": tc.timezone,
			}

			result, err := executeGetCurrentTime(args)
			if err != nil {
				t.Fatalf("Failed to execute get_current_time with timezone %s: %v", tc.timezone, err)
			}

			// Verify result is not empty
			if result == "" {
				t.Error("Expected non-empty result")
			}

			// Verify format is parseable
			_, parseErr := time.Parse("2006-01-02 15:04:05 MST", result)
			if parseErr != nil {
				t.Errorf("Result has invalid time format for timezone %s: %s", tc.timezone, result)
			}
		})
	}
}

func TestExecuteGetCurrentTime_EmptyTimezone(t *testing.T) {
	// Empty string should default to UTC
	args := map[string]interface{}{
		"timezone": "",
	}

	result, err := executeGetCurrentTime(args)
	if err != nil {
		t.Fatalf("Failed to execute get_current_time with empty timezone: %v", err)
	}

	// Verify result contains UTC
	if !strings.Contains(result, "UTC") {
		t.Errorf("Expected result to contain 'UTC' for empty timezone, got: %s", result)
	}
}

func TestExecuteGetCurrentTime_InvalidTimezone(t *testing.T) {
	args := map[string]interface{}{
		"timezone": "Invalid/Timezone",
	}

	_, err := executeGetCurrentTime(args)
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}

	// Verify error message mentions the invalid timezone
	if !strings.Contains(err.Error(), "Invalid/Timezone") {
		t.Errorf("Expected error message to mention 'Invalid/Timezone', got: %v", err)
	}
}

func TestExecuteGetCurrentTime_FormatConsistency(t *testing.T) {
	// Execute multiple times to ensure format consistency
	args := map[string]interface{}{
		"timezone": "UTC",
	}

	for i := 0; i < 5; i++ {
		result, err := executeGetCurrentTime(args)
		if err != nil {
			t.Fatalf("Failed to execute get_current_time on iteration %d: %v", i, err)
		}

		// Verify format (YYYY-MM-DD HH:MM:SS TZ)
		parts := strings.Split(result, " ")
		if len(parts) != 3 {
			t.Errorf("Expected result to have 3 parts (date, time, timezone), got: %s", result)
		}

		// Verify date part
		dateParts := strings.Split(parts[0], "-")
		if len(dateParts) != 3 {
			t.Errorf("Expected date to have 3 parts (YYYY-MM-DD), got: %s", parts[0])
		}

		// Verify time part
		timeParts := strings.Split(parts[1], ":")
		if len(timeParts) != 3 {
			t.Errorf("Expected time to have 3 parts (HH:MM:SS), got: %s", parts[1])
		}
	}
}

func TestTimeTool_IntegrationWithRegistry(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	// Register time tool
	err := registry.Register(NewTimeTool())
	if err != nil {
		t.Fatalf("Failed to register time tool: %v", err)
	}

	// Execute via registry with no args (should default to UTC)
	result, err := registry.Execute("get_current_time", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to execute time tool via registry: %v", err)
	}

	if !strings.Contains(result, "UTC") {
		t.Errorf("Expected result to contain 'UTC', got: %s", result)
	}

	// Execute with timezone
	result, err = registry.Execute("get_current_time", map[string]interface{}{
		"timezone": "America/New_York",
	})
	if err != nil {
		t.Fatalf("Failed to execute time tool with timezone via registry: %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestTimeTool_ParametersStructure(t *testing.T) {
	tool := NewTimeTool()

	params := tool.Parameters

	// Verify type is object
	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got %v", params["type"])
	}

	// Verify properties exist
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	// Verify timezone property
	timezone, ok := properties["timezone"].(map[string]interface{})
	if !ok {
		t.Fatal("Timezone property should be a map")
	}

	if timezone["type"] != "string" {
		t.Errorf("Expected timezone type 'string', got %v", timezone["type"])
	}

	if timezone["default"] != "UTC" {
		t.Errorf("Expected timezone default 'UTC', got %v", timezone["default"])
	}

	// Verify required is empty (timezone is optional)
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Required should be a string array")
	}

	if len(required) != 0 {
		t.Errorf("Expected no required parameters, got %d", len(required))
	}
}

func TestExecuteGetCurrentTime_CategoryAndKeywords(t *testing.T) {
	tool := NewTimeTool()

	// Test that GetToolsByCategory would find this tool
	registry := &Registry{
		tools: make(map[string]*Tool),
	}
	registry.Register(tool)

	timeTools := registry.GetToolsByCategory("time")
	if len(timeTools) != 1 {
		t.Errorf("Expected 1 time tool, got %d", len(timeTools))
	}

	if timeTools[0].Name != "get_current_time" {
		t.Errorf("Expected get_current_time, got %s", timeTools[0].Name)
	}

	// Test GetCategories
	categories := registry.GetCategories()
	if categories["time"] != 1 {
		t.Errorf("Expected 1 tool in time category, got %d", categories["time"])
	}
}
