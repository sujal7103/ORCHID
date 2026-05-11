package execution

import (
	"clara-agents/internal/models"
	"clara-agents/internal/services"
	"clara-agents/internal/tools"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// ToolExecutor executes tool blocks using the tool registry
// This executor runs tools directly without LLM involvement for faster, deterministic execution
type ToolExecutor struct {
	registry          *tools.Registry
	credentialService *services.CredentialService
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(registry *tools.Registry, credentialService *services.CredentialService) *ToolExecutor {
	return &ToolExecutor{
		registry:          registry,
		credentialService: credentialService,
	}
}

// deepInterpolate recursively interpolates {{...}} templates in nested structures
func deepInterpolate(value interface{}, inputs map[string]any) interface{} {
	switch v := value.(type) {
	case string:
		if !strings.Contains(v, "{{") {
			return v
		}
		// If the entire string is a single {{path}} placeholder, resolve to the raw value
		// (preserves non-string types like maps, slices, numbers)
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") &&
			strings.Count(trimmed, "{{") == 1 {
			resolvedPath := strings.TrimPrefix(trimmed, "{{")
			resolvedPath = strings.TrimSuffix(resolvedPath, "}}")
			resolvedPath = strings.TrimSpace(resolvedPath)
			resolved := ResolvePath(inputs, resolvedPath)
			if resolved != nil {
				return resolved
			}
		}
		// Mixed text with one or more {{...}} placeholders — interpolate as template string
		return InterpolateTemplate(v, inputs)
	case map[string]interface{}:
		// Handle nested maps
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = deepInterpolate(val, inputs)
		}
		return result
	case []interface{}:
		// Handle arrays
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = deepInterpolate(val, inputs)
		}
		return result
	default:
		// Return primitives as-is (numbers, booleans, nil)
		return v
	}
}

// Execute runs a tool block
func (e *ToolExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	toolName := getString(config, "toolName", "")
	if toolName == "" {
		return nil, fmt.Errorf("toolName is required for tool execution block")
	}

	log.Printf("🔧 [TOOL-EXEC] Block '%s': executing tool '%s'", block.Name, toolName)

	// Get tool from registry
	tool, exists := e.registry.Get(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Map inputs to tool arguments based on argumentMapping config
	argMapping := getMap(config, "argumentMapping")
	args := make(map[string]interface{})

	log.Printf("🔧 [TOOL-EXEC] Block '%s' config: %+v", block.Name, config)
	log.Printf("🔧 [TOOL-EXEC] Block '%s' argumentMapping: %+v", block.Name, argMapping)
	log.Printf("🔧 [TOOL-EXEC] Block '%s' inputs keys: %v", block.Name, getInputKeys(inputs))

	if argMapping != nil {
		for argName, inputPath := range argMapping {
			// Use deep interpolation to handle nested structures
			interpolated := deepInterpolate(inputPath, inputs)

			// Log the interpolation result
			if pathStr, ok := inputPath.(string); ok && strings.HasPrefix(pathStr, "{{") {
				log.Printf("🔧 [TOOL-EXEC] Interpolated '%s': %v", argName, interpolated)
			} else if _, isMap := inputPath.(map[string]interface{}); isMap {
				log.Printf("🔧 [TOOL-EXEC] Deep interpolated object '%s'", argName)
			} else {
				log.Printf("🔧 [TOOL-EXEC] Using literal value for '%s': %v", argName, inputPath)
			}

			if interpolated != nil {
				args[argName] = interpolated
			}
		}
	} else {
		// If no argument mapping, pass all inputs as args
		for k, v := range inputs {
			// Skip internal fields
			if len(k) > 0 && k[0] == '_' {
				continue
			}
			args[k] = v
		}
	}

	// Extract userID from inputs for credential resolution (uses __user_id__ convention)
	userID, _ := inputs["__user_id__"].(string)

	// Inject credentials for tools that need them
	e.injectCredentials(ctx, toolName, args, userID, config)

	log.Printf("🔧 [TOOL-EXEC] Tool '%s' args: %v", toolName, args)

	// Execute tool
	result, err := tool.Execute(args)

	// Clean up internal keys from args (don't log them)
	delete(args, tools.CredentialResolverKey)
	delete(args, tools.UserIDKey)

	if err != nil {
		log.Printf("❌ [TOOL-EXEC] Tool '%s' failed: %v", toolName, err)
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	log.Printf("✅ [TOOL-EXEC] Tool '%s' completed, result_len=%d", toolName, len(result))

	// Try to parse result as JSON for structured output
	var parsedResult any
	if err := json.Unmarshal([]byte(result), &parsedResult); err != nil {
		// Not JSON, use as string
		parsedResult = result
	}

	return map[string]any{
		"response": parsedResult, // Primary output key for consistency with other blocks
		"result":   parsedResult, // Kept for backwards compatibility
		"data":     parsedResult, // For structured data access
		"toolName": toolName,
		"raw":      result,
	}, nil
}

// injectCredentials adds credential resolver and auto-discovers credentials for tools that need them
func (e *ToolExecutor) injectCredentials(ctx context.Context, toolName string, args map[string]interface{}, userID string, config map[string]any) {
	if e.credentialService == nil || userID == "" {
		return
	}

	// Inject credential resolver for tools that need authentication
	// Cast to tools.CredentialResolver type for proper type assertion in credential_helper.go
	resolver := tools.CredentialResolver(e.credentialService.CreateCredentialResolver(userID))
	args[tools.CredentialResolverKey] = resolver
	args[tools.UserIDKey] = userID

	// Auto-inject credential_id for tools that need it
	toolIntegrationType := tools.GetIntegrationTypeForTool(toolName)
	if toolIntegrationType == "" {
		return
	}

	var credentialID string

	// First, try to find from explicitly configured credentials in block config
	if credentials, ok := config["credentials"].([]interface{}); ok && len(credentials) > 0 {
		for _, credID := range credentials {
			if credIDStr, ok := credID.(string); ok {
				cred, err := resolver(credIDStr)
				if err == nil && cred != nil && cred.IntegrationType == toolIntegrationType {
					credentialID = credIDStr
					log.Printf("🔐 [TOOL-EXEC] Found credential_id=%s from block config for tool=%s",
						credentialID, toolName)
					break
				}
			}
		}
	}

	// If no credential found in block config, try runtime auto-discovery from user's credentials
	if credentialID == "" {
		log.Printf("🔍 [TOOL-EXEC] No credentials in block config for tool=%s, trying runtime auto-discovery...", toolName)
		userCreds, err := e.credentialService.ListByUserAndType(ctx, userID, toolIntegrationType)
		if err != nil {
			log.Printf("⚠️ [TOOL-EXEC] Failed to fetch user credentials: %v", err)
		} else if len(userCreds) == 1 {
			// Exactly one credential of this type - auto-use it
			credentialID = userCreds[0].ID
			log.Printf("🔐 [TOOL-EXEC] Runtime auto-discovered single credential: %s (%s) for tool=%s",
				userCreds[0].Name, credentialID, toolName)
		} else if len(userCreds) > 1 {
			log.Printf("⚠️ [TOOL-EXEC] Multiple credentials (%d) found for %s - cannot auto-select. Configure in block settings.",
				len(userCreds), toolIntegrationType)
		} else {
			log.Printf("⚠️ [TOOL-EXEC] No %s credentials found for user. Please add one in Credentials Manager.",
				toolIntegrationType)
		}
	}

	// Inject the credential_id if we found one
	if credentialID != "" {
		args["credential_id"] = credentialID
		log.Printf("🔐 [TOOL-EXEC] Auto-injected credential_id=%s for tool=%s (type=%s)",
			credentialID, toolName, toolIntegrationType)
	}
}

// getInputKeys returns sorted keys from inputs map for logging
func getInputKeys(inputs map[string]any) []string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		// Skip internal fields for cleaner logging
		if !strings.HasPrefix(k, "__") {
			keys = append(keys, k)
		}
	}
	return keys
}
