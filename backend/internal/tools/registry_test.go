package tools

import (
	"errors"
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	// Note: We can't easily test GetRegistry() since it's a singleton,
	// so we'll create fresh registries for testing
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools in new registry, got %d", registry.Count())
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type": "string",
				},
			},
		},
		Execute: func(args map[string]interface{}) (string, error) {
			return "success", nil
		},
	}

	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", registry.Count())
	}
}

func TestRegistry_Register_EmptyName(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "",
		Description: "A test tool",
		Execute: func(args map[string]interface{}) (string, error) {
			return "success", nil
		},
	}

	err := registry.Register(tool)
	if err == nil {
		t.Error("Expected error for empty tool name, got nil")
	}
}

func TestRegistry_Register_NilExecute(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Execute:     nil,
	}

	err := registry.Register(tool)
	if err == nil {
		t.Error("Expected error for nil Execute function, got nil")
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Execute: func(args map[string]interface{}) (string, error) {
			return "success", nil
		},
	}

	// Register first time
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Try to register again
	err = registry.Register(tool)
	if err == nil {
		t.Error("Expected error for duplicate tool registration, got nil")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Execute: func(args map[string]interface{}) (string, error) {
			return "success", nil
		},
	}

	registry.Register(tool)

	// Get existing tool
	retrieved, exists := registry.Get("test_tool")
	if !exists {
		t.Error("Expected tool to exist")
	}

	if retrieved.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got %s", retrieved.Name)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	_, exists := registry.Get("nonexistent_tool")
	if exists {
		t.Error("Expected tool to not exist")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	// Register multiple tools
	tools := []*Tool{
		{
			Name:        "tool1",
			Description: "First tool",
			Parameters: map[string]interface{}{
				"type": "object",
			},
			Execute: func(args map[string]interface{}) (string, error) {
				return "success", nil
			},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			Parameters: map[string]interface{}{
				"type": "object",
			},
			Execute: func(args map[string]interface{}) (string, error) {
				return "success", nil
			},
		},
	}

	for _, tool := range tools {
		registry.Register(tool)
	}

	// List tools
	toolsList := registry.List()
	if len(toolsList) != 2 {
		t.Errorf("Expected 2 tools in list, got %d", len(toolsList))
	}

	// Verify format
	for _, toolDef := range toolsList {
		if toolDef["type"] != "function" {
			t.Error("Expected tool type to be 'function'")
		}

		function, ok := toolDef["function"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected function to be a map")
		}

		if function["name"] == nil {
			t.Error("Expected function to have a name")
		}

		if function["description"] == nil {
			t.Error("Expected function to have a description")
		}

		if function["parameters"] == nil {
			t.Error("Expected function to have parameters")
		}
	}
}

func TestRegistry_Execute(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	expectedResult := "test result"
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Execute: func(args map[string]interface{}) (string, error) {
			return expectedResult, nil
		},
	}

	registry.Register(tool)

	// Execute tool
	result, err := registry.Execute("test_tool", nil)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if result != expectedResult {
		t.Errorf("Expected result %s, got %s", expectedResult, result)
	}
}

func TestRegistry_Execute_WithArgs(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "echo_tool",
		Description: "Echoes the input",
		Execute: func(args map[string]interface{}) (string, error) {
			input, ok := args["input"].(string)
			if !ok {
				return "", errors.New("input must be a string")
			}
			return input, nil
		},
	}

	registry.Register(tool)

	// Execute with args
	args := map[string]interface{}{
		"input": "hello world",
	}

	result, err := registry.Execute("echo_tool", args)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if result != "hello world" {
		t.Errorf("Expected result 'hello world', got %s", result)
	}
}

func TestRegistry_Execute_Error(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	expectedError := errors.New("execution failed")
	tool := &Tool{
		Name:        "error_tool",
		Description: "Always fails",
		Execute: func(args map[string]interface{}) (string, error) {
			return "", expectedError
		},
	}

	registry.Register(tool)

	// Execute tool
	_, err := registry.Execute("error_tool", nil)
	if err == nil {
		t.Error("Expected error from tool execution, got nil")
	}

	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestRegistry_Execute_NotFound(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	_, err := registry.Execute("nonexistent_tool", nil)
	if err == nil {
		t.Error("Expected error for nonexistent tool, got nil")
	}
}

func TestRegistry_Count(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools initially, got %d", registry.Count())
	}

	// Register tools
	for i := 0; i < 5; i++ {
		tool := &Tool{
			Name:        string(rune('a' + i)),
			Description: "Test tool",
			Execute: func(args map[string]interface{}) (string, error) {
				return "success", nil
			},
		}
		registry.Register(tool)
	}

	if registry.Count() != 5 {
		t.Errorf("Expected 5 tools, got %d", registry.Count())
	}
}

func TestRegistry_ThreadSafety(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tool := &Tool{
				Name:        string(rune('a' + (id % 26))),
				Description: "Test tool",
				Execute: func(args map[string]interface{}) (string, error) {
					return "success", nil
				},
			}
			// Ignore duplicate errors
			_ = registry.Register(tool)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			registry.Get(string(rune('a' + (id % 26))))
			registry.List()
			registry.Count()
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	if registry.Count() < 0 || registry.Count() > 26 {
		t.Errorf("Unexpected tool count after concurrent operations: %d", registry.Count())
	}
}

func TestGetRegistry_Singleton(t *testing.T) {
	// Get registry multiple times
	r1 := GetRegistry()
	r2 := GetRegistry()

	// Should be the same instance
	if r1 != r2 {
		t.Error("Expected GetRegistry() to return the same instance")
	}
}

func TestGetRegistry_HasBuiltInTools(t *testing.T) {
	registry := GetRegistry()

	// Check for built-in tools
	expectedTools := []string{"get_current_time", "search_web"}

	for _, toolName := range expectedTools {
		_, exists := registry.Get(toolName)
		if !exists {
			t.Errorf("Expected built-in tool %s to be registered", toolName)
		}
	}
}

func TestRegistry_List_EmptyRegistry(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	list := registry.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d tools", len(list))
	}
}

func TestTool_ExecuteWithComplexArgs(t *testing.T) {
	registry := &Registry{
		tools: make(map[string]*Tool),
	}

	tool := &Tool{
		Name:        "complex_tool",
		Description: "Handles complex arguments",
		Execute: func(args map[string]interface{}) (string, error) {
			// Test nested maps
			nested, ok := args["nested"].(map[string]interface{})
			if !ok {
				return "", errors.New("nested must be a map")
			}

			value, ok := nested["key"].(string)
			if !ok {
				return "", errors.New("nested.key must be a string")
			}

			return value, nil
		},
	}

	registry.Register(tool)

	args := map[string]interface{}{
		"nested": map[string]interface{}{
			"key": "test_value",
		},
	}

	result, err := registry.Execute("complex_tool", args)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if result != "test_value" {
		t.Errorf("Expected result 'test_value', got %s", result)
	}
}
