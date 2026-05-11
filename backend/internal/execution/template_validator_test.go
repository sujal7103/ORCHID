package execution

import (
	"clara-agents/internal/models"
	"testing"
)

func TestValidateTemplateReferences_ValidRefs(t *testing.T) {
	workflow := &models.Workflow{
		Blocks: []models.Block{
			{ID: "block-1", NormalizedID: "start", Name: "Start", Config: map[string]any{
				"variableName": "input",
			}},
			{ID: "block-2", NormalizedID: "search-web", Name: "Search Web", Config: map[string]any{
				"prompt": "Search for {{start.value}}",
				"query":  "{{input}}",
			}},
		},
		Variables: []models.Variable{
			{Name: "input"},
		},
	}

	warnings := ValidateTemplateReferences(workflow)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for valid refs, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateTemplateReferences_InvalidBlockName(t *testing.T) {
	workflow := &models.Workflow{
		Blocks: []models.Block{
			{ID: "block-1", NormalizedID: "start", Name: "Start", Config: map[string]any{}},
			{ID: "block-2", NormalizedID: "agent", Name: "Agent", Config: map[string]any{
				"prompt": "Use data from {{misspelled-block.response}}",
			}},
		},
	}

	warnings := ValidateTemplateReferences(workflow)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].BlockName != "Agent" {
		t.Errorf("expected warning on Agent block, got %s", warnings[0].BlockName)
	}
	if warnings[0].Reference != "{{misspelled-block.response}}" {
		t.Errorf("unexpected reference: %s", warnings[0].Reference)
	}
}

func TestValidateTemplateReferences_NestedConfig(t *testing.T) {
	workflow := &models.Workflow{
		Blocks: []models.Block{
			{ID: "b1", NormalizedID: "fetch", Name: "Fetch", Config: map[string]any{}},
			{ID: "b2", NormalizedID: "process", Name: "Process", Config: map[string]any{
				"argumentMapping": map[string]any{
					"url":  "https://api.example.com/{{fetch.data.id}}",
					"body": "{{typo-block.output}}",
				},
			}},
		},
	}

	warnings := ValidateTemplateReferences(workflow)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for typo-block, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Reference != "{{typo-block.output}}" {
		t.Errorf("unexpected reference: %s", warnings[0].Reference)
	}
}

func TestValidateTemplateReferences_RuntimeKeysIgnored(t *testing.T) {
	workflow := &models.Workflow{
		Blocks: []models.Block{
			{ID: "b1", NormalizedID: "agent", Name: "Agent", Config: map[string]any{
				"prompt": "Process {{response}} and {{item}} at {{index}}",
			}},
		},
	}

	warnings := ValidateTemplateReferences(workflow)
	if len(warnings) != 0 {
		t.Errorf("runtime keys should not trigger warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestExtractTemplateRefs(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"plain string", "hello", 0},
		{"single ref", "{{block.field}}", 1},
		{"multiple refs", "{{a.x}} and {{b.y}}", 2},
		{"nested map", map[string]any{"key": "{{ref.val}}"}, 1},
		{"array", []any{"{{one.a}}", "{{two.b}}"}, 2},
		{"nil", nil, 0},
		{"number", 42, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := extractTemplateRefs(tt.input)
			if len(refs) != tt.expected {
				t.Errorf("expected %d refs, got %d: %v", tt.expected, len(refs), refs)
			}
		})
	}
}
