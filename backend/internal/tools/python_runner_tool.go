package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"clara-agents/internal/e2b"
)

// NewPythonRunnerTool creates a new Python Code Runner tool
func NewPythonRunnerTool() *Tool {
	return &Tool{
		Name:        "run_python",
		DisplayName: "Python Code Runner",
		Description: `Execute Python code with custom pip dependencies. Install packages on-the-fly and retrieve generated files. Max 5 minutes execution time.

⚠️ CRITICAL: This tool CANNOT access user-uploaded files!
- If user uploaded a file (CSV, Excel, JSON, etc.), use 'analyze_data' tool instead
- Files uploaded by users are NOT accessible by filename in this sandbox
- This tool runs in an isolated environment with NO access to local files

USE THIS TOOL FOR:
- Running Python scripts that need specific pip packages (torch, transformers, etc.)
- Generating NEW files (model weights, processed data, images, PDFs)
- Quick computations, API calls, web scraping
- Processing data from URLs (not local files)
- Code that doesn't need user-uploaded input files

DO NOT USE FOR:
- Analyzing user-uploaded CSV/Excel/JSON files → use 'analyze_data' instead
- Reading local files by filename → they don't exist in sandbox
- Any task requiring access to files the user shared in chat

Remember: Install dependencies and run code in the same session - no persistence between calls.`,
		Icon:     "Terminal",
		Source:   ToolSourceBuiltin,
		Category: "computation",
		Keywords: []string{"python", "code", "execute", "run", "script", "programming", "processing", "compute", "pip", "packages", "dependencies"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "Python code to execute",
				},
				"dependencies": map[string]interface{}{
					"type":        "array",
					"description": "Pip packages to install before execution (e.g., ['torch', 'transformers', 'requests'])",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"output_files": map[string]interface{}{
					"type":        "array",
					"description": "File paths to retrieve after execution (e.g., ['model.pt', 'output.csv'])",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"code"},
		},
		Execute: executePythonRunner,
	}
}

func executePythonRunner(args map[string]interface{}) (string, error) {
	// Extract code (required)
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return "", fmt.Errorf("code is required")
	}

	// Extract dependencies (optional)
	var dependencies []string
	if depsRaw, ok := args["dependencies"].([]interface{}); ok {
		for _, dep := range depsRaw {
			if depStr, ok := dep.(string); ok {
				dependencies = append(dependencies, depStr)
			}
		}
	}

	// Extract output files (optional)
	var outputFiles []string
	if filesRaw, ok := args["output_files"].([]interface{}); ok {
		for _, file := range filesRaw {
			if fileStr, ok := file.(string); ok {
				outputFiles = append(outputFiles, fileStr)
			}
		}
	}

	// Build request
	req := e2b.AdvancedExecuteRequest{
		Code:         code,
		Timeout:      300, // 5 minutes for complex tasks like Playwright, ML training, etc.
		Dependencies: dependencies,
		OutputFiles:  outputFiles,
	}

	// Execute
	e2bService := e2b.GetE2BExecutorService()
	result, err := e2bService.ExecuteAdvanced(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to execute code: %w", err)
	}

	if !result.Success {
		errorMsg := "execution failed"
		if result.Error != nil {
			errorMsg = *result.Error
		}
		if result.Stderr != "" {
			errorMsg += "\nStderr: " + result.Stderr
		}
		return "", fmt.Errorf("%s", errorMsg)
	}

	// Build response
	response := map[string]interface{}{
		"success": true,
		"stdout":  result.Stdout,
	}

	// Include stderr if present
	if result.Stderr != "" {
		response["stderr"] = result.Stderr
	}

	// Include install output if dependencies were installed
	if result.InstallOutput != "" {
		response["install_output"] = result.InstallOutput
	}

	// Include plots if any
	if len(result.Plots) > 0 {
		response["plots"] = result.Plots
		response["plot_count"] = len(result.Plots)
	}

	// Include files if any were retrieved
	if len(result.Files) > 0 {
		response["files"] = result.Files
		response["file_count"] = len(result.Files)
	}

	// Include execution time
	if result.ExecutionTime != nil {
		response["execution_time"] = *result.ExecutionTime
	}

	jsonResponse, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResponse), nil
}
