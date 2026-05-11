package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
)

// FilterExecutor filters array items by conditions.
// Reuses evaluateCondition() from if_condition_executor.go.
type FilterExecutor struct{}

func NewFilterExecutor() *FilterExecutor {
	return &FilterExecutor{}
}

func (e *FilterExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := StripTemplateBraces(getString(config, "arrayField", "response"))
	mode := getString(config, "mode", "include")

	// Resolve the array from inputs
	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("filter: field '%s' not found in inputs", arrayField)
	}

	arr, ok := toSlice(rawArray)
	if !ok {
		return nil, fmt.Errorf("filter: field '%s' is not an array", arrayField)
	}

	// Parse conditions
	rawConditions, _ := config["conditions"].([]interface{})
	if len(rawConditions) == 0 {
		// No conditions — passthrough
		log.Printf("🔍 [FILTER] Block '%s': no conditions, passthrough (%d items)", block.Name, len(arr))
		return map[string]any{
			"response":      arr,
			"data":          arr,
			"originalCount": len(arr),
			"filteredCount": len(arr),
		}, nil
	}

	type condition struct {
		field    string
		operator string
		value    string
	}

	conditions := make([]condition, 0, len(rawConditions))
	for _, rc := range rawConditions {
		cMap, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}
		conditions = append(conditions, condition{
			field:    fmt.Sprintf("%v", cMap["field"]),
			operator: fmt.Sprintf("%v", cMap["operator"]),
			value:    fmt.Sprintf("%v", cMap["value"]),
		})
	}

	// Filter
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		allMatch := true
		for _, cond := range conditions {
			var fieldValue any
			if itemMap, ok := item.(map[string]any); ok {
				fieldValue = ResolvePath(itemMap, cond.field)
			} else {
				fieldValue = item
			}

			compareValue := cond.value
			// Support template interpolation in compare value
			if len(compareValue) > 4 && compareValue[:2] == "{{" {
				compareValue = InterpolateTemplate(compareValue, inputs)
			}

			match := evaluateCondition(fieldValue, cond.operator, compareValue)
			if !match {
				allMatch = false
				break
			}
		}

		keep := (mode == "include" && allMatch) || (mode == "exclude" && !allMatch)
		if keep {
			result = append(result, item)
		}
	}

	log.Printf("🔍 [FILTER] Block '%s': %d → %d items (mode=%s)", block.Name, len(arr), len(result), mode)

	return map[string]any{
		"response":      result,
		"data":          result,
		"originalCount": len(arr),
		"filteredCount": len(result),
	}, nil
}

// toSlice converts various slice types to []any.
func toSlice(v any) ([]any, bool) {
	if arr, ok := v.([]any); ok {
		return arr, true
	}
	if arr, ok := v.([]interface{}); ok {
		return arr, true
	}
	// Handle typed slices via reflection-free check for common JSON types
	if arr, ok := v.([]map[string]any); ok {
		result := make([]any, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result, true
	}
	if arr, ok := v.([]string); ok {
		result := make([]any, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result, true
	}
	if arr, ok := v.([]float64); ok {
		result := make([]any, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result, true
	}
	return nil, false
}
