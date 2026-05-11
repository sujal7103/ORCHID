package execution

import (
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// FormatInput represents the input data to be formatted
type FormatInput struct {
	// RawData is the unstructured data from task execution (can be string, map, slice, etc.)
	RawData any

	// ToolResults contains results from tool executions (if any)
	ToolResults []map[string]any

	// LLMResponse is the final LLM text response (if any)
	LLMResponse string

	// Context provides additional context for formatting (e.g., original task description)
	Context string
}

// FormatOutput represents the result of schema formatting
type FormatOutput struct {
	// Data is the validated, schema-compliant structured output
	Data map[string]any

	// RawJSON is the raw JSON string returned by the formatter
	RawJSON string

	// Model is the model ID used for formatting
	Model string

	// Tokens contains token usage information
	Tokens models.TokenUsage

	// Success indicates whether formatting succeeded
	Success bool

	// Error contains any error message if formatting failed
	Error string
}

// FormatToSchema formats the given input data into the specified JSON schema
// This is a method on AgentBlockExecutor so it can reuse the existing LLM call infrastructure
func (e *AgentBlockExecutor) FormatToSchema(
	ctx context.Context,
	input FormatInput,
	schema *models.JSONSchema,
	modelID string,
) (*FormatOutput, error) {
	log.Printf("📐 [FORMAT-SCHEMA] Starting schema formatting with model=%s", modelID)

	if schema == nil {
		return nil, fmt.Errorf("schema is required for FormatToSchema")
	}

	// Resolve model using existing method
	provider, resolvedModelID, err := e.resolveModel(modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model: %w", err)
	}

	log.Printf("📐 [FORMAT-SCHEMA] Resolved model %s -> %s (provider: %s)",
		modelID, resolvedModelID, provider.Name)

	// Build the formatting prompt
	systemPrompt, userPrompt := buildFormattingPrompts(input, schema)

	// Build messages
	messages := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	// Make the LLM call with native structured output if supported
	// Use existing callLLMWithRetryAndSchema for consistency
	response, _, err := e.callLLMWithRetryAndSchema(ctx, provider, resolvedModelID, messages, nil, 0.1, nil, schema)
	if err != nil {
		return &FormatOutput{
			Success: false,
			Error:   fmt.Sprintf("LLM call failed: %v", err),
			Model:   resolvedModelID,
		}, nil
	}

	log.Printf("📐 [FORMAT-SCHEMA] LLM response received, length=%d chars", len(response.Content))

	// Parse and validate the output
	output, err := parseAndValidateSchema(response.Content, schema)
	if err != nil {
		log.Printf("⚠️ [FORMAT-SCHEMA] Validation failed: %v", err)
		return &FormatOutput{
			Success: false,
			Error:   fmt.Sprintf("validation failed: %v", err),
			RawJSON: response.Content,
			Model:   resolvedModelID,
			Tokens: models.TokenUsage{
				Input:  response.InputTokens,
				Output: response.OutputTokens,
			},
		}, nil
	}

	log.Printf("✅ [FORMAT-SCHEMA] Successfully formatted data to schema")

	return &FormatOutput{
		Data:    output,
		RawJSON: response.Content,
		Model:   resolvedModelID,
		Tokens: models.TokenUsage{
			Input:  response.InputTokens,
			Output: response.OutputTokens,
		},
		Success: true,
	}, nil
}

// buildFormattingPrompts creates the system and user prompts for schema formatting
func buildFormattingPrompts(input FormatInput, schema *models.JSONSchema) (string, string) {
	// Build schema description
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

	// Build data description
	var dataBuilder strings.Builder

	// Add tool results if present
	if len(input.ToolResults) > 0 {
		dataBuilder.WriteString("## Tool Execution Results\n")
		for i, tr := range input.ToolResults {
			trJSON, _ := json.MarshalIndent(tr, "", "  ")
			dataBuilder.WriteString(fmt.Sprintf("### Tool Result %d\n```json\n%s\n```\n\n", i+1, string(trJSON)))
		}
	}

	// Add LLM response if present
	if input.LLMResponse != "" {
		dataBuilder.WriteString("## LLM Response\n")
		dataBuilder.WriteString("```\n")
		dataBuilder.WriteString(input.LLMResponse)
		dataBuilder.WriteString("\n```\n\n")
	}

	// Add raw data if present and different from LLM response
	if input.RawData != nil {
		rawStr := fmt.Sprintf("%v", input.RawData)
		if rawStr != input.LLMResponse {
			dataBuilder.WriteString("## Additional Data\n")
			if rawJSON, err := json.MarshalIndent(input.RawData, "", "  "); err == nil {
				dataBuilder.WriteString("```json\n")
				dataBuilder.WriteString(string(rawJSON))
				dataBuilder.WriteString("\n```\n\n")
			} else {
				dataBuilder.WriteString("```\n")
				dataBuilder.WriteString(rawStr)
				dataBuilder.WriteString("\n```\n\n")
			}
		}
	}

	systemPrompt := fmt.Sprintf(`You are a precise data formatter. Your ONLY task is to extract data from the provided sources and format it as JSON matching the required schema.

## CRITICAL RULES
1. Respond with ONLY valid JSON - no explanations, no markdown code blocks, no extra text
2. The JSON must exactly match the required schema structure
3. Extract ALL relevant data from the provided sources
4. If data is missing for a required field, use reasonable defaults or null
5. DO NOT invent or fabricate data - only use what's provided
6. DO NOT include fields not in the schema

## Required Output Schema
%s

## Data Fields Explanation
%s`, string(schemaJSON), buildSchemaFieldsExplanation(schema))

	contextNote := ""
	if input.Context != "" {
		contextNote = fmt.Sprintf("\n\n## Context\n%s", input.Context)
	}

	userPrompt := fmt.Sprintf(`Format the following data into the required JSON schema.

%s%s

Respond with ONLY the JSON object, nothing else.`, dataBuilder.String(), contextNote)

	return systemPrompt, userPrompt
}

// buildSchemaFieldsExplanation creates a human-readable explanation of schema fields
func buildSchemaFieldsExplanation(schema *models.JSONSchema) string {
	if schema == nil || schema.Properties == nil {
		return "No specific field requirements."
	}

	var builder strings.Builder
	for fieldName, fieldSchema := range schema.Properties {
		builder.WriteString(fmt.Sprintf("- **%s**: ", fieldName))
		if fieldSchema.Description != "" {
			builder.WriteString(fieldSchema.Description)
		} else if fieldSchema.Type != "" {
			builder.WriteString(fmt.Sprintf("(%s)", fieldSchema.Type))
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

// parseAndValidateSchema parses JSON and validates against schema
func parseAndValidateSchema(content string, schema *models.JSONSchema) (map[string]any, error) {
	// Extract JSON from content (handle markdown code blocks if any slipped through)
	jsonContent := extractJSONFromContent(content)

	// Parse JSON
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonContent), &output); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	for _, required := range schema.Required {
		if _, exists := output[required]; !exists {
			return nil, fmt.Errorf("missing required field: %s", required)
		}
	}

	// Validate property types
	for propName, propSchema := range schema.Properties {
		val, exists := output[propName]
		if !exists {
			continue // Not required, skip
		}

		if err := validateFieldType(val, propSchema.Type); err != nil {
			return nil, fmt.Errorf("field %s: %w", propName, err)
		}
	}

	return output, nil
}

// extractJSONFromContent extracts JSON from content that may have markdown wrappers
func extractJSONFromContent(content string) string {
	content = strings.TrimSpace(content)

	// Check for markdown JSON code block
	jsonBlockRegex := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	if matches := jsonBlockRegex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find JSON object
	start := strings.Index(content, "{")
	if start == -1 {
		return content
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(content); i++ {
		if content[i] == '{' {
			depth++
		} else if content[i] == '}' {
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}

	return content[start:]
}

// validateFieldType checks if a value matches the expected JSON schema type
func validateFieldType(val any, expectedType string) error {
	if val == nil {
		return nil // null is valid for any type
	}

	switch expectedType {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
	case "number", "integer":
		switch val.(type) {
		case float64, float32, int, int32, int64:
			// OK
		default:
			return fmt.Errorf("expected number, got %T", val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", val)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("expected array, got %T", val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("expected object, got %T", val)
		}
	}

	return nil
}
