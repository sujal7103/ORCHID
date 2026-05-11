package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// SortExecutor sorts array items by one or more fields.
type SortExecutor struct{}

func NewSortExecutor() *SortExecutor {
	return &SortExecutor{}
}

func (e *SortExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	arrayField := StripTemplateBraces(getString(config, "arrayField", "response"))

	rawArray := ResolvePath(inputs, arrayField)
	if rawArray == nil {
		return nil, fmt.Errorf("sort: field '%s' not found in inputs", arrayField)
	}

	arr, ok := toSlice(rawArray)
	if !ok {
		return nil, fmt.Errorf("sort: field '%s' is not an array", arrayField)
	}

	// Parse sort fields
	rawSortBy, _ := config["sortBy"].([]interface{})
	if len(rawSortBy) == 0 {
		log.Printf("📋 [SORT] Block '%s': no sortBy fields, passthrough", block.Name)
		return map[string]any{
			"response":   arr,
			"data":       arr,
			"totalItems": len(arr),
		}, nil
	}

	type sortField struct {
		field     string
		direction string // "asc" or "desc"
		sortType  string // "string", "number", "date"
	}

	sortFields := make([]sortField, 0, len(rawSortBy))
	for _, rs := range rawSortBy {
		sMap, ok := rs.(map[string]interface{})
		if !ok {
			continue
		}
		sf := sortField{
			field:     fmt.Sprintf("%v", sMap["field"]),
			direction: "asc",
			sortType:  "auto",
		}
		if d, ok := sMap["direction"].(string); ok {
			sf.direction = d
		}
		if t, ok := sMap["type"].(string); ok {
			sf.sortType = t
		}
		sortFields = append(sortFields, sf)
	}

	// Make a copy to avoid mutating the original
	sorted := make([]any, len(arr))
	copy(sorted, arr)

	sort.SliceStable(sorted, func(i, j int) bool {
		for _, sf := range sortFields {
			a := extractValue(sorted[i], sf.field)
			b := extractValue(sorted[j], sf.field)

			cmp := compareValues(a, b, sf.sortType)
			if cmp == 0 {
				continue
			}

			if sf.direction == "desc" {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})

	log.Printf("📋 [SORT] Block '%s': sorted %d items by %d fields", block.Name, len(sorted), len(sortFields))

	return map[string]any{
		"response":   sorted,
		"data":       sorted,
		"totalItems": len(sorted),
	}, nil
}

// compareValues returns -1, 0, or 1
func compareValues(a, b any, sortType string) int {
	if sortType == "number" || sortType == "auto" {
		af := toFloat(a)
		bf := toFloat(b)
		if af != bf || sortType == "number" {
			if af < bf {
				return -1
			}
			if af > bf {
				return 1
			}
			return 0
		}
	}

	if sortType == "date" {
		at := parseDate(fmt.Sprintf("%v", a))
		bt := parseDate(fmt.Sprintf("%v", b))
		if at.Before(bt) {
			return -1
		}
		if at.After(bt) {
			return 1
		}
		return 0
	}

	// String comparison (default/fallback)
	as := strings.ToLower(fmt.Sprintf("%v", a))
	bs := strings.ToLower(fmt.Sprintf("%v", b))
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func parseDate(s string) time.Time {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
		"01/02/2006",
		"Jan 2, 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
