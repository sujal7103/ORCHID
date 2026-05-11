package services

import (
	"context"
	"log"
	"strings"

	"clara-agents/internal/tools"
)

// CortexToolSelector selects tools for daemons based on role and task needs.
// MCP tools (user's desktop client) are ALWAYS included — they're the user's local tools.
// Built-in tools are filtered by credentials and optionally narrowed by the predictor.
type CortexToolSelector struct {
	toolRegistry     *tools.Registry
	toolService      *ToolService
	toolPredictorSvc *ToolPredictorService
}

// NewCortexToolSelector creates a new tool selector
func NewCortexToolSelector(
	toolRegistry *tools.Registry,
	toolPredictorSvc *ToolPredictorService,
) *CortexToolSelector {
	return &CortexToolSelector{
		toolRegistry:     toolRegistry,
		toolPredictorSvc: toolPredictorSvc,
	}
}

// SetToolService sets the tool service (late dependency injection)
func (s *CortexToolSelector) SetToolService(svc *ToolService) {
	s.toolService = svc
}

// SelectToolsForDaemon returns the tool definitions a daemon should have access to.
// Strategy:
//   - ALL MCP tools are always included (user's desktop client tools)
//   - Built-in tools are credential-filtered, then narrowed by predictor if too many
func (s *CortexToolSelector) SelectToolsForDaemon(
	ctx context.Context,
	userID string,
	role string,
	toolsNeeded []string,
	taskSummary string,
) ([]map[string]interface{}, []string) {
	// 1. Get ALL MCP tools — always included, no filtering
	mcpTools := s.toolRegistry.GetMCPTools(userID)

	// 2. Get credential-filtered built-in tools
	var builtinTools []map[string]interface{}
	if s.toolService != nil {
		builtinTools = s.toolService.GetAvailableTools(ctx, userID)
	} else {
		builtinTools = s.toolRegistry.GetUserTools(userID)
	}

	// Remove MCP tools from builtinTools (they're already in mcpTools)
	mcpNames := map[string]bool{}
	for _, t := range mcpTools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				mcpNames[name] = true
			}
		}
	}
	var filteredBuiltin []map[string]interface{}
	for _, t := range builtinTools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok && !mcpNames[name] {
				filteredBuiltin = append(filteredBuiltin, t)
			}
		}
	}

	// 3. If too many built-in tools, use predictor to narrow them
	if len(filteredBuiltin) > 20 && s.toolPredictorSvc != nil && taskSummary != "" {
		predicted, err := s.toolPredictorSvc.PredictTools(
			ctx, userID, "", taskSummary, filteredBuiltin, nil,
		)
		if err == nil && len(predicted) > 0 {
			filteredBuiltin = predicted
		}
	}

	// 4. Hard cap: if total tools would exceed maxTotalTools, trim built-in to fit.
	// MCP tools are always prioritized (user's local desktop tools).
	const maxTotalTools = 100
	if len(mcpTools)+len(filteredBuiltin) > maxTotalTools {
		allowedBuiltin := maxTotalTools - len(mcpTools)
		if allowedBuiltin < 0 {
			allowedBuiltin = 0
		}
		if len(filteredBuiltin) > allowedBuiltin {
			log.Printf("[TOOL-SELECTOR] Capping built-in tools from %d to %d (max total %d, MCP=%d)",
				len(filteredBuiltin), allowedBuiltin, maxTotalTools, len(mcpTools))
			filteredBuiltin = filteredBuiltin[:allowedBuiltin]
		}
	}

	// 5. Combine: ALL MCP tools + filtered built-in tools
	combined := make([]map[string]interface{}, 0, len(mcpTools)+len(filteredBuiltin))
	combined = append(combined, mcpTools...)
	combined = append(combined, filteredBuiltin...)

	// Collect names for logging
	var names []string
	for _, t := range combined {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				names = append(names, name)
			}
		}
	}

	log.Printf("[TOOL-SELECTOR] User %s: %d MCP tools + %d built-in = %d total for daemon role=%s",
		userID, len(mcpTools), len(filteredBuiltin), len(combined), role)

	return combined, names
}

// HandleSearchToolsRequest handles the search_available_tools meta-tool invocation
func (s *CortexToolSelector) HandleSearchToolsRequest(
	userID string,
	query string,
) ([]map[string]interface{}, string) {
	allTools := s.toolRegistry.GetUserTools(userID)
	query = strings.ToLower(query)
	var matches []map[string]interface{}
	var descriptions []string

	for _, toolDef := range allTools {
		funcDef, ok := toolDef["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := funcDef["name"].(string)
		desc, _ := funcDef["description"].(string)

		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(desc), query) {
			matches = append(matches, toolDef)
			descriptions = append(descriptions, name+": "+desc)
		}
	}

	summary := "No matching tools found for: " + query
	if len(matches) > 0 {
		summary = strings.Join(descriptions, "\n")
	}

	return matches, summary
}
