package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
)

// DeduplicateExecutor removes duplicate items from an array by a key field.
type DeduplicateExecutor struct{}

func NewDeduplicateExecutor() *DeduplicateExecutor {
	return &DeduplicateExecutor{}
}

func (e *DeduplicateExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := StripTemplateBraces(getString(config, "arrayField", "response"))
	keyField := getString(config, "keyField", "id")
	keep := getString(config, "keep", "first")

	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("deduplicate: field '%s' not found in inputs", arrayField)
	}

	arr, ok := toSlice(rawArray)
	if !ok {
		return nil, fmt.Errorf("deduplicate: field '%s' is not an array", arrayField)
	}

	originalCount := len(arr)

	if keep == "last" {
		// Reverse iterate so the last occurrence wins
		seen := make(map[string]bool)
		result := make([]any, 0, len(arr))

		// Walk backwards to find last occurrences
		for i := len(arr) - 1; i >= 0; i-- {
			key := extractKeyValue(arr[i], keyField)
			if !seen[key] {
				seen[key] = true
				result = append(result, arr[i])
			}
		}

		// Reverse result to maintain original order
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}

		log.Printf("🔄 [DEDUP] Block '%s': %d → %d items (keep=last, key=%s)",
			block.Name, originalCount, len(result), keyField)

		return map[string]any{
			"response":      result,
			"data":          result,
			"originalCount": originalCount,
			"removedCount":  originalCount - len(result),
		}, nil
	}

	// Default: keep first
	seen := make(map[string]bool)
	result := make([]any, 0, len(arr))

	for _, item := range arr {
		key := extractKeyValue(item, keyField)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	log.Printf("🔄 [DEDUP] Block '%s': %d → %d items (keep=first, key=%s)",
		block.Name, originalCount, len(result), keyField)

	return map[string]any{
		"response":      result,
		"data":          result,
		"originalCount": originalCount,
		"removedCount":  originalCount - len(result),
	}, nil
}

func extractKeyValue(item any, keyField string) string {
	if itemMap, ok := item.(map[string]any); ok {
		v := ResolvePath(itemMap, keyField)
		if v != nil {
			return fmt.Sprintf("%v", v)
		}
	}
	return fmt.Sprintf("%v", item)
}
