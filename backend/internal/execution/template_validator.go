package execution

import (
	"clara-agents/internal/models"
	"fmt"
	"log"
	"regexp"
	"strings"
)

var templateRefRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// TemplateWarning represents a potentially broken template reference.
type TemplateWarning struct {
	BlockName string // The block containing the reference
	Reference string // The full {{...}} reference
	Reason    string // Why it's suspicious
}

// ValidateTemplateReferences scans all block configs for {{block-name.field}} references
// and checks that the block-name part matches an actual block. Returns warnings for
// unresolvable references. This catches typos before execution starts.
func ValidateTemplateReferences(workflow *models.Workflow) []TemplateWarning {
	// Build set of valid block identifiers (IDs and normalized IDs)
	validBlockNames := make(map[string]bool)
	for _, block := range workflow.Blocks {
		if block.ID != "" {
			validBlockNames[block.ID] = true
		}
		if block.NormalizedID != "" {
			validBlockNames[block.NormalizedID] = true
		}
		// Also accept the raw Name lowercased/normalized for fuzzy matching
		validBlockNames[strings.ToLower(block.Name)] = true
	}

	// Workflow variables are also valid top-level references
	for _, v := range workflow.Variables {
		validBlockNames[v.Name] = true
	}

	// Common global input keys that resolve at runtime (not block references)
	runtimeKeys := map[string]bool{
		"input": true, "value": true, "response": true, "data": true,
		"result": true, "output": true, "item": true, "index": true,
		"__user_id__": true, "_workflowModelId": true,
	}

	var warnings []TemplateWarning
	for _, block := range workflow.Blocks {
		refs := extractTemplateRefs(block.Config)
		for _, ref := range refs {
			// Extract the top-level name (before the first dot)
			topLevel := ref
			if idx := strings.Index(ref, "."); idx > 0 {
				topLevel = ref[:idx]
			}
			topLevel = strings.TrimSpace(topLevel)

			// Skip runtime keys and valid block names
			if runtimeKeys[topLevel] || validBlockNames[topLevel] {
				continue
			}

			warnings = append(warnings, TemplateWarning{
				BlockName: block.Name,
				Reference: fmt.Sprintf("{{%s}}", ref),
				Reason:    fmt.Sprintf("'%s' does not match any block ID, normalized ID, or workflow variable", topLevel),
			})
		}
	}
	return warnings
}

// ValidateBlockConfigs checks that each block has the required config fields for its type.
// Returns warnings for blocks with missing required configuration.
func ValidateBlockConfigs(workflow *models.Workflow) []TemplateWarning {
	// Required config fields by block type
	requiredFields := map[string][]string{
		"code_block":    {"prompt"},        // agent blocks need a prompt (or systemPrompt)
		"llm_inference": {"prompt"},        // LLM blocks need a prompt
		"http_request":  {"url"},           // HTTP blocks need a URL
		"if_condition":  {"condition"},     // Conditionals need a condition
		"for_each":      {"arraySource"},   // Loops need an array source
		"inline_code":   {"code"},          // Inline code needs code
	}

	// Alternative fields: if any of these exist, the requirement is satisfied
	alternativeFields := map[string]map[string][]string{
		"code_block": {"prompt": {"systemPrompt", "prompt"}},
	}

	var warnings []TemplateWarning
	for _, block := range workflow.Blocks {
		fields, ok := requiredFields[block.Type]
		if !ok {
			continue
		}
		for _, field := range fields {
			// Check if the field (or any alternative) exists and is non-empty
			if hasConfigField(block.Config, field) {
				continue
			}
			// Check alternatives
			if alts, ok := alternativeFields[block.Type]; ok {
				if altFields, ok := alts[field]; ok {
					found := false
					for _, alt := range altFields {
						if hasConfigField(block.Config, alt) {
							found = true
							break
						}
					}
					if found {
						continue
					}
				}
			}
			warnings = append(warnings, TemplateWarning{
				BlockName: block.Name,
				Reference: field,
				Reason:    fmt.Sprintf("block type '%s' requires config field '%s'", block.Type, field),
			})
		}
	}
	return warnings
}

func hasConfigField(config map[string]any, field string) bool {
	val, ok := config[field]
	if !ok {
		return false
	}
	// Check non-empty for strings
	if s, ok := val.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return val != nil
}

// LogTemplateWarnings validates and logs any template reference and config issues.
func LogTemplateWarnings(workflow *models.Workflow) {
	warnings := ValidateTemplateReferences(workflow)
	warnings = append(warnings, ValidateBlockConfigs(workflow)...)
	if len(warnings) == 0 {
		return
	}
	for _, w := range warnings {
		log.Printf("⚠️ [TEMPLATE] Block '%s': reference %s — %s", w.BlockName, w.Reference, w.Reason)
	}
}

// extractTemplateRefs recursively walks a value and extracts all {{...}} reference paths.
func extractTemplateRefs(value any) []string {
	switch v := value.(type) {
	case string:
		matches := templateRefRe.FindAllStringSubmatch(v, -1)
		refs := make([]string, 0, len(matches))
		for _, m := range matches {
			if len(m) > 1 {
				refs = append(refs, strings.TrimSpace(m[1]))
			}
		}
		return refs
	case map[string]any:
		var refs []string
		for _, val := range v {
			refs = append(refs, extractTemplateRefs(val)...)
		}
		return refs
	case []any:
		var refs []string
		for _, item := range v {
			refs = append(refs, extractTemplateRefs(item)...)
		}
		return refs
	default:
		return nil
	}
}
