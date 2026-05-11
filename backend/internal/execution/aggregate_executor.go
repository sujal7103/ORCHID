package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
)

// AggregateExecutor groups array items and computes aggregations.
// Supports: count, sum, avg, min, max, first, last, concat, collect.
type AggregateExecutor struct{}

func NewAggregateExecutor() *AggregateExecutor {
	return &AggregateExecutor{}
}

type aggOp struct {
	outputField string
	operation   string
	field       string
}

func (e *AggregateExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := StripTemplateBraces(getString(config, "arrayField", "response"))
	groupByField := getString(config, "groupBy", "")

	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("aggregate: field '%s' not found in inputs", arrayField)
	}

	arr, ok := toSlice(rawArray)
	if !ok {
		return nil, fmt.Errorf("aggregate: field '%s' is not an array", arrayField)
	}

	// Parse operations
	rawOps, _ := config["operations"].([]interface{})
	if len(rawOps) == 0 {
		return map[string]any{
			"response": map[string]any{"count": len(arr)},
			"data":     map[string]any{"count": len(arr)},
		}, nil
	}

	ops := make([]aggOp, 0, len(rawOps))
	for _, ro := range rawOps {
		opMap, ok := ro.(map[string]interface{})
		if !ok {
			continue
		}
		ops = append(ops, aggOp{
			outputField: fmt.Sprintf("%v", opMap["outputField"]),
			operation:   fmt.Sprintf("%v", opMap["operation"]),
			field:       fmt.Sprintf("%v", opMap["field"]),
		})
	}

	log.Printf("📊 [AGGREGATE] Block '%s': %d items, %d ops, groupBy='%s'",
		block.Name, len(arr), len(ops), groupByField)

	if groupByField == "" {
		// No grouping — aggregate entire array
		result := computeAggregations(arr, ops)
		return map[string]any{
			"response": result,
			"data":     result,
		}, nil
	}

	// Group by field
	groups := make(map[string][]any)
	groupOrder := make([]string, 0)
	for _, item := range arr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%v", itemMap[groupByField])
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], item)
	}

	result := make([]any, 0, len(groupOrder))
	for _, key := range groupOrder {
		groupResult := computeAggregations(groups[key], ops)
		groupResult[groupByField] = key
		result = append(result, groupResult)
	}

	return map[string]any{
		"response": result,
		"data":     result,
	}, nil
}

func computeAggregations(items []any, ops []aggOp) map[string]any {
	result := make(map[string]any)

	for _, op := range ops {
		switch op.operation {
		case "count":
			result[op.outputField] = len(items)

		case "sum":
			sum := 0.0
			for _, item := range items {
				sum += extractFloat(item, op.field)
			}
			result[op.outputField] = sum

		case "avg":
			if len(items) == 0 {
				result[op.outputField] = 0.0
				continue
			}
			sum := 0.0
			for _, item := range items {
				sum += extractFloat(item, op.field)
			}
			result[op.outputField] = sum / float64(len(items))

		case "min":
			minVal := math.MaxFloat64
			for _, item := range items {
				v := extractFloat(item, op.field)
				if v < minVal {
					minVal = v
				}
			}
			if len(items) == 0 {
				result[op.outputField] = 0.0
			} else {
				result[op.outputField] = minVal
			}

		case "max":
			maxVal := -math.MaxFloat64
			for _, item := range items {
				v := extractFloat(item, op.field)
				if v > maxVal {
					maxVal = v
				}
			}
			if len(items) == 0 {
				result[op.outputField] = 0.0
			} else {
				result[op.outputField] = maxVal
			}

		case "first":
			if len(items) > 0 {
				result[op.outputField] = extractValue(items[0], op.field)
			}

		case "last":
			if len(items) > 0 {
				result[op.outputField] = extractValue(items[len(items)-1], op.field)
			}

		case "concat":
			parts := make([]string, 0, len(items))
			for _, item := range items {
				v := extractValue(item, op.field)
				if v != nil {
					parts = append(parts, fmt.Sprintf("%v", v))
				}
			}
			result[op.outputField] = strings.Join(parts, ", ")

		case "collect":
			collected := make([]any, 0, len(items))
			for _, item := range items {
				v := extractValue(item, op.field)
				if v != nil {
					collected = append(collected, v)
				}
			}
			result[op.outputField] = collected
		}
	}

	return result
}

func extractFloat(item any, field string) float64 {
	v := extractValue(item, field)
	return toFloat(v)
}

func extractValue(item any, field string) any {
	if field == "" {
		return item
	}
	if itemMap, ok := item.(map[string]any); ok {
		return ResolvePath(itemMap, field)
	}
	return nil
}
