package execution

import (
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// TransformExecutor sets, deletes, or renames fields on the data flowing through.
// Pure data transformation — no HTTP calls, no LLM.
type TransformExecutor struct{}

func NewTransformExecutor() *TransformExecutor {
	return &TransformExecutor{}
}

func (e *TransformExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	// Start with a shallow copy of input data (skip internal keys)
	data := make(map[string]any)
	for k, v := range inputs {
		if !strings.HasPrefix(k, "__") && !strings.HasPrefix(k, "_") {
			data[k] = v
		}
	}

	// Get operations list
	rawOps, ok := config["operations"]
	if !ok {
		// No operations — passthrough mode, just forward the input
		log.Printf("🔄 [TRANSFORM] Block '%s': no operations, passthrough", block.Name)
		return map[string]any{
			"response": data,
			"data":     data,
		}, nil
	}

	ops, ok := rawOps.([]interface{})
	if !ok {
		return nil, fmt.Errorf("operations must be an array")
	}

	log.Printf("🔄 [TRANSFORM] Block '%s': applying %d operations", block.Name, len(ops))

	for i, rawOp := range ops {
		op, ok := rawOp.(map[string]interface{})
		if !ok {
			continue
		}

		field, _ := op["field"].(string)
		expression, _ := op["expression"].(string)
		operation, _ := op["operation"].(string)

		if field == "" {
			continue
		}

		switch operation {
		case "set":
			// Set field to a literal or resolved expression
			if strings.Contains(expression, "{{") {
				data[field] = InterpolateTemplate(expression, inputs)
			} else {
				// Try parsing as JSON first (for objects/arrays/numbers/booleans)
				var parsed any
				if err := json.Unmarshal([]byte(expression), &parsed); err == nil {
					data[field] = parsed
				} else {
					data[field] = expression
				}
			}

		case "template":
			// Always interpolate as template string
			data[field] = InterpolateTemplate(expression, inputs)

		case "delete":
			delete(data, field)

		case "rename":
			// expression = new field name
			if val, exists := data[field]; exists {
				data[expression] = val
				delete(data, field)
			}

		case "extract":
			// Extract a nested path into a top-level field
			// expression = path to extract (e.g., "response.data.items")
			extracted := ResolvePath(inputs, StripTemplateBraces(expression))
			if extracted != nil {
				data[field] = extracted
			}

		default:
			log.Printf("⚠️ [TRANSFORM] Block '%s': unknown operation '%s' at index %d", block.Name, operation, i)
		}
	}

	log.Printf("✅ [TRANSFORM] Block '%s': output has %d keys", block.Name, len(data))

	return map[string]any{
		"response": data,
		"data":     data,
	}, nil
}
