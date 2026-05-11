package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
)

// LimitExecutor takes the first or last N items from an array with optional offset.
type LimitExecutor struct{}

func NewLimitExecutor() *LimitExecutor {
	return &LimitExecutor{}
}

func (e *LimitExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := StripTemplateBraces(getString(config, "arrayField", "response"))
	count := getInt(config, "count", 10)
	position := getString(config, "position", "first")
	offset := getInt(config, "offset", 0)

	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("limit: field '%s' not found in inputs", arrayField)
	}

	arr, ok := toSlice(rawArray)
	if !ok {
		return nil, fmt.Errorf("limit: field '%s' is not an array", arrayField)
	}

	total := len(arr)
	var result []any

	switch position {
	case "last":
		start := total - count - offset
		if start < 0 {
			start = 0
		}
		end := total - offset
		if end < 0 {
			end = 0
		}
		if end > total {
			end = total
		}
		if start >= end {
			result = []any{}
		} else {
			result = arr[start:end]
		}
	default: // "first"
		start := offset
		if start > total {
			start = total
		}
		end := start + count
		if end > total {
			end = total
		}
		result = arr[start:end]
	}

	log.Printf("✂️ [LIMIT] Block '%s': %d items → %d (position=%s, offset=%d, count=%d)",
		block.Name, total, len(result), position, offset, count)

	return map[string]any{
		"response":      result,
		"data":          result,
		"totalItems":    total,
		"returnedItems": len(result),
	}, nil
}
