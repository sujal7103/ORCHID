package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
)

// MergeExecutor combines data from multiple upstream blocks.
// Modes: append (concat arrays), merge_by_key (join by shared key), combine_all (object keyed by source).
type MergeExecutor struct{}

func NewMergeExecutor() *MergeExecutor {
	return &MergeExecutor{}
}

func (e *MergeExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config
	mode := getString(config, "mode", "append")
	keyField := getString(config, "keyField", "id")

	// Collect upstream data — inputs has keys like "upstream-block-name.response"
	// We group by source block (strip internal keys)
	sources := make(map[string]any)
	for k, v := range inputs {
		if strings.HasPrefix(k, "_") {
			continue
		}
		sources[k] = v
	}

	log.Printf("🔗 [MERGE] Block '%s': mode=%s, %d source keys", block.Name, mode, len(sources))

	var merged any
	var err error

	switch mode {
	case "append":
		merged, err = mergeAppend(sources)
	case "merge_by_key":
		merged, err = mergeByKey(sources, keyField)
	case "combine_all":
		merged = sources
	default:
		return nil, fmt.Errorf("merge: unknown mode '%s'", mode)
	}

	if err != nil {
		return nil, err
	}

	return map[string]any{
		"response":    merged,
		"data":        merged,
		"sourceCount": len(sources),
	}, nil
}

// mergeAppend concatenates all arrays found in source values.
func mergeAppend(sources map[string]any) ([]any, error) {
	result := make([]any, 0)
	for _, v := range sources {
		if arr, ok := toSlice(v); ok {
			result = append(result, arr...)
		} else if v != nil {
			// Non-array values get appended as single items
			result = append(result, v)
		}
	}
	return result, nil
}

// mergeByKey joins arrays of objects by a shared key field.
func mergeByKey(sources map[string]any, keyField string) ([]any, error) {
	// Build a map of key → merged object
	merged := make(map[string]map[string]any)
	insertOrder := make([]string, 0)

	for _, v := range sources {
		arr, ok := toSlice(v)
		if !ok {
			continue
		}
		for _, item := range arr {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%v", itemMap[keyField])
			if _, exists := merged[key]; !exists {
				merged[key] = make(map[string]any)
				insertOrder = append(insertOrder, key)
			}
			// Merge fields into existing entry
			for k, val := range itemMap {
				merged[key][k] = val
			}
		}
	}

	result := make([]any, 0, len(insertOrder))
	for _, key := range insertOrder {
		result = append(result, merged[key])
	}
	return result, nil
}
