package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
)

// ForEachExecutor iterates over an array and collects each item.
// Downstream blocks connected to the "loop_body" output receive each item in sequence.
// The "done" output fires after all items are processed with aggregated results.
//
// Config:
//   - arrayField: path to the array in upstream output (e.g. "response" or "response.items")
//   - itemVariable: variable name for the current item (default "item")
//   - maxIterations: safety limit (default 100)
type ForEachExecutor struct{}

func NewForEachExecutor() *ForEachExecutor {
	return &ForEachExecutor{}
}

func (e *ForEachExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := getString(config, "arrayField", "response")
	itemVariable := getString(config, "itemVariable", "item")
	maxIterations := getInt(config, "maxIterations", 100)

	// Strip {{}} braces if user used template syntax (e.g., "{{webhook.data.items}}" → "webhook.data.items")
	arrayField = StripTemplateBraces(arrayField)

	// Resolve the array from inputs
	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("for_each: field '%s' not found in inputs", arrayField)
	}

	// Convert to slice
	var items []any
	switch v := rawArray.(type) {
	case []any:
		items = v
	default:
		// If it's not an array, wrap it as a single-item array
		items = []any{rawArray}
	}

	totalItems := len(items)
	if totalItems == 0 {
		log.Printf("🔄 [FOR_EACH] Block '%s': empty array, skipping iterations", block.Name)
		return map[string]any{
			"response":   []any{},
			"data":       []any{},
			"items":      []any{},
			"totalItems": 0,
			"branch":     "*",
		}, nil
	}

	// Apply safety limit
	if totalItems > maxIterations {
		log.Printf("⚠️ [FOR_EACH] Block '%s': array has %d items, capping at %d", block.Name, totalItems, maxIterations)
		items = items[:maxIterations]
	}

	log.Printf("🔄 [FOR_EACH] Block '%s': iterating over %d items (field=%s, var=%s)",
		block.Name, len(items), arrayField, itemVariable)

	// Build per-item outputs that downstream blocks can access
	// Each item is available as loop-block.item (or whatever itemVariable is set to)
	// We also provide the full results array and iteration metadata
	results := make([]any, len(items))
	for i, item := range items {
		results[i] = map[string]any{
			"index": i,
			"value": item,
		}
	}

	output := map[string]any{
		"response":   items,     // The full array for downstream
		"data":       items,     // Alias
		"items":      results,   // Structured [{index, value}, ...]
		"totalItems": len(items),
		itemVariable: items,     // Available as {{loop-block.item}} (array)
		"branch":     "*",       // Wildcard: fire both "Each" and "Done" downstream branches
	}

	log.Printf("✅ [FOR_EACH] Block '%s': completed %d iterations", block.Name, len(items))

	return output, nil
}

func getInt(config map[string]any, key string, defaultVal int) int {
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return defaultVal
}
