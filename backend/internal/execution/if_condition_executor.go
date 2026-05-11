package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
)

// IfConditionExecutor evaluates a condition and routes data to true/false branches.
// The DAG engine checks the "branch" output key against Connection.SourceOutput
// to determine which downstream blocks to execute.
type IfConditionExecutor struct{}

func NewIfConditionExecutor() *IfConditionExecutor {
	return &IfConditionExecutor{}
}

func (e *IfConditionExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	field := StripTemplateBraces(getString(config, "field", "response"))
	operator := getString(config, "operator", "is_true")
	compareValue := config["value"]

	// Resolve the field value from inputs
	fieldValue := ResolvePath(inputs, field)

	log.Printf("🔀 [IF] Block '%s': field=%s operator=%s compareValue=%v fieldValue=%v",
		block.Name, field, operator, compareValue, fieldValue)

	result := evaluateCondition(fieldValue, operator, compareValue)

	branch := "false"
	if result {
		branch = "true"
	}

	log.Printf("🔀 [IF] Block '%s': condition result=%v, branch=%s", block.Name, result, branch)

	return map[string]any{
		"response": fieldValue,
		"data":     fieldValue,
		"branch":   branch,
		"result":   result,
	}, nil
}

func evaluateCondition(fieldValue any, operator string, compareValue any) bool {
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", compareValue)
	case "neq":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", compareValue)
	case "contains":
		return strings.Contains(
			strings.ToLower(fmt.Sprintf("%v", fieldValue)),
			strings.ToLower(fmt.Sprintf("%v", compareValue)),
		)
	case "not_contains":
		return !strings.Contains(
			strings.ToLower(fmt.Sprintf("%v", fieldValue)),
			strings.ToLower(fmt.Sprintf("%v", compareValue)),
		)
	case "gt":
		return toFloat(fieldValue) > toFloat(compareValue)
	case "lt":
		return toFloat(fieldValue) < toFloat(compareValue)
	case "gte":
		return toFloat(fieldValue) >= toFloat(compareValue)
	case "lte":
		return toFloat(fieldValue) <= toFloat(compareValue)
	case "is_empty":
		return fieldValue == nil || fmt.Sprintf("%v", fieldValue) == ""
	case "not_empty":
		return fieldValue != nil && fmt.Sprintf("%v", fieldValue) != ""
	case "is_true":
		return isTruthy(fieldValue)
	case "is_false":
		return !isTruthy(fieldValue)
	case "starts_with":
		return strings.HasPrefix(
			strings.ToLower(fmt.Sprintf("%v", fieldValue)),
			strings.ToLower(fmt.Sprintf("%v", compareValue)),
		)
	case "ends_with":
		return strings.HasSuffix(
			strings.ToLower(fmt.Sprintf("%v", fieldValue)),
			strings.ToLower(fmt.Sprintf("%v", compareValue)),
		)
	default:
		log.Printf("⚠️ [IF] Unknown operator: %s, defaulting to is_true", operator)
		return isTruthy(fieldValue)
	}
}

func toFloat(v any) float64 {
	switch f := v.(type) {
	case float64:
		return f
	case float32:
		return float64(f)
	case int:
		return float64(f)
	case int64:
		return float64(f)
	case string:
		var result float64
		fmt.Sscanf(f, "%f", &result)
		return result
	default:
		return 0
	}
}

func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != "" && strings.ToLower(val) != "false" && val != "0"
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return true
	}
}
