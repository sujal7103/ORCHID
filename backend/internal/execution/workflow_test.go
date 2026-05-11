package execution

import (
	"clara-agents/internal/models"
	"context"
	"testing"
)

// TestVariableExecutorOutputsCorrectKeys tests that variable blocks output both "value" and the variable name
func TestVariableExecutorOutputsCorrectKeys(t *testing.T) {
	executor := NewVariableExecutor()

	tests := []struct {
		name         string
		block        models.Block
		inputs       map[string]any
		expectedKeys []string
	}{
		{
			name: "Read operation with defaultValue should output both value and variableName",
			block: models.Block{
				Name: "Start",
				Type: "variable",
				Config: map[string]any{
					"operation":    "read",
					"variableName": "input",
					"defaultValue": "test value",
				},
			},
			inputs:       map[string]any{},
			expectedKeys: []string{"value", "input"},
		},
		{
			name: "Read operation with existing input should output both keys",
			block: models.Block{
				Name: "Read Input",
				Type: "variable",
				Config: map[string]any{
					"operation":    "read",
					"variableName": "query",
				},
			},
			inputs:       map[string]any{"query": "search term"},
			expectedKeys: []string{"value", "query"},
		},
		{
			name: "Set operation should output both keys",
			block: models.Block{
				Name: "Set Variable",
				Type: "variable",
				Config: map[string]any{
					"operation":    "set",
					"variableName": "result",
					"value":        "new value",
				},
			},
			inputs:       map[string]any{},
			expectedKeys: []string{"value", "result"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			output, err := executor.Execute(ctx, tt.block, tt.inputs)

			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			// Check that all expected keys are present
			for _, key := range tt.expectedKeys {
				if _, ok := output[key]; !ok {
					t.Errorf("Expected key '%s' not found in output. Got: %+v", key, output)
				}
			}

			// For "read" operation, both keys should have the same value
			if tt.block.Config["operation"] == "read" {
				if output["value"] != output[tt.block.Config["variableName"].(string)] {
					t.Errorf("'value' and variable name key should have same value. Got: %+v", output)
				}
			}
		})
	}
}

// TestInterpolateTemplate tests the template interpolation function
func TestInterpolateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		inputs   map[string]any
		expected string
	}{
		{
			name:     "Simple variable interpolation",
			template: "Search for {{input}}",
			inputs:   map[string]any{"input": "test query"},
			expected: "Search for test query",
		},
		{
			name:     "Multiple variables",
			template: "{{user}} searched for {{query}}",
			inputs:   map[string]any{"user": "John", "query": "golang"},
			expected: "John searched for golang",
		},
		{
			name:     "Nested object access",
			template: "Result: {{output.response}}",
			inputs: map[string]any{
				"output": map[string]any{
					"response": "success",
				},
			},
			expected: "Result: success",
		},
		{
			name:     "Missing variable should keep original",
			template: "Value: {{missing}}",
			inputs:   map[string]any{"other": "value"},
			expected: "Value: {{missing}}",
		},
		{
			name:     "Number conversion",
			template: "Count: {{count}}",
			inputs:   map[string]any{"count": 42},
			expected: "Count: 42",
		},
		{
			name:     "Boolean conversion",
			template: "Active: {{active}}",
			inputs:   map[string]any{"active": true},
			expected: "Active: true",
		},
		{
			name:     "Typed struct slice with array access",
			template: "{{ai-agent.response}} download: {{ai-agent.files[0].download_url}}",
			inputs: map[string]any{
				"ai-agent": map[string]any{
					"response": "Here is your file",
					"files": []GeneratedFile{
						{Filename: "test.pdf", DownloadURL: "http://example.com/test.pdf"},
					},
				},
			},
			expected: "Here is your file download: http://example.com/test.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpolateTemplate(tt.template, tt.inputs)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestInterpolateMapValues tests the map value interpolation function
func TestInterpolateMapValues(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		inputs   map[string]any
		expected map[string]any
	}{
		{
			name: "Interpolate string values in map",
			data: map[string]any{
				"query": "{{input}}",
				"type":  "search",
			},
			inputs: map[string]any{"input": "test query"},
			expected: map[string]any{
				"query": "test query",
				"type":  "search",
			},
		},
		{
			name: "Nested map interpolation",
			data: map[string]any{
				"params": map[string]any{
					"q": "{{query}}",
				},
			},
			inputs: map[string]any{"query": "golang"},
			expected: map[string]any{
				"params": map[string]any{
					"q": "golang",
				},
			},
		},
		{
			name: "Array interpolation",
			data: map[string]any{
				"items": []any{"{{first}}", "{{second}}"},
			},
			inputs: map[string]any{"first": "a", "second": "b"},
			expected: map[string]any{
				"items": []any{"a", "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpolateMapValues(tt.data, tt.inputs)

			// Deep comparison
			if !mapsEqual(result, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

// TestWorkflowDataFlow tests end-to-end data flow through a simple workflow
func TestWorkflowDataFlow(t *testing.T) {
	// Create a simple workflow: Start -> Block A -> Block B
	workflow := &models.Workflow{
		ID: "test-workflow",
		Blocks: []models.Block{
			{
				ID:   "start",
				Name: "Start",
				Type: "variable",
				Config: map[string]any{
					"operation":    "read",
					"variableName": "input",
					"defaultValue": "test value",
				},
			},
		},
		Connections: []models.Connection{},
		Variables:   []models.Variable{},
	}

	// Test variable executor output
	varExec := NewVariableExecutor()
	ctx := context.Background()

	startOutput, err := varExec.Execute(ctx, workflow.Blocks[0], map[string]any{})
	if err != nil {
		t.Fatalf("Start block failed: %v", err)
	}

	t.Logf("Start block output: %+v", startOutput)

	// Verify Start block outputs both keys
	if _, ok := startOutput["value"]; !ok {
		t.Error("Start block should output 'value' key")
	}
	if _, ok := startOutput["input"]; !ok {
		t.Error("Start block should output 'input' key")
	}

	// Simulate engine passing Start output to next block
	nextBlockInputs := map[string]any{}
	// Copy globalInputs (workflow input)
	nextBlockInputs["input"] = "test value"
	// Add Start block outputs
	for k, v := range startOutput {
		nextBlockInputs[k] = v
	}

	t.Logf("Next block would receive inputs: %+v", nextBlockInputs)

	// Test interpolation with these inputs
	template := "Process {{input}}"
	result := InterpolateTemplate(template, nextBlockInputs)
	expected := "Process test value"

	if result != expected {
		t.Errorf("Interpolation failed. Expected '%s', got '%s'", expected, result)
	}
}

// TestWorkflowEngineBlockInputConstruction tests how engine.go constructs blockInputs
func TestWorkflowEngineBlockInputConstruction(t *testing.T) {
	// Simulate what engine.go does at lines 129-154
	workflow := &models.Workflow{
		ID: "test",
		Blocks: []models.Block{
			{ID: "start", Name: "Start", Type: "variable"},
			{ID: "block2", Name: "Block 2", Type: "llm_inference"},
		},
		Connections: []models.Connection{
			{
				ID:              "conn1",
				SourceBlockID:   "start",
				TargetBlockID:   "block2",
				SourceOutput:    "output",
				TargetInput:     "input",
			},
		},
	}

	// Initial workflow input
	workflowInput := map[string]any{
		"input": "GUVI HCL Scam",
	}

	// Global inputs (what engine.go builds)
	globalInputs := make(map[string]any)
	for k, v := range workflowInput {
		globalInputs[k] = v
	}

	// Start block outputs
	startBlockOutput := map[string]any{
		"value": "GUVI HCL Scam",
		"input": "GUVI HCL Scam",
	}

	// Block outputs storage
	blockOutputs := map[string]map[string]any{
		"start": startBlockOutput,
	}

	// Build inputs for block2 (what engine.go does)
	blockInputs := make(map[string]any)
	for k, v := range globalInputs {
		blockInputs[k] = v
	}

	// Add outputs from connected upstream blocks
	for _, conn := range workflow.Connections {
		if conn.TargetBlockID == "block2" {
			if output, ok := blockOutputs[conn.SourceBlockID]; ok {
				// Add under source block name
				blockInputs["Start"] = output

				// Also add fields directly
				for k, v := range output {
					blockInputs[k] = v
				}
			}
		}
	}

	t.Logf("Block2 inputs: %+v", blockInputs)

	// Verify block2 has access to "input" key
	if _, ok := blockInputs["input"]; !ok {
		t.Error("Block2 should have 'input' key in inputs")
	}

	if blockInputs["input"] != "GUVI HCL Scam" {
		t.Errorf("Block2 input should be 'GUVI HCL Scam', got: %v", blockInputs["input"])
	}

	// Test interpolation would work
	template := "{{input}}"
	result := InterpolateTemplate(template, blockInputs)
	if result != "GUVI HCL Scam" {
		t.Errorf("Interpolation should resolve to 'GUVI HCL Scam', got: '%s'", result)
	}
}

// Helper function for deep map comparison
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		// Handle different types
		switch av := v.(type) {
		case map[string]any:
			bvm, ok := bv.(map[string]any)
			if !ok || !mapsEqual(av, bvm) {
				return false
			}
		case []any:
			bva, ok := bv.([]any)
			if !ok || !slicesEqual(av, bva) {
				return false
			}
		default:
			if v != bv {
				return false
			}
		}
	}
	return true
}

func slicesEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
