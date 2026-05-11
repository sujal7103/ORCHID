package execution

import (
	"bytes"
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// InlineCodeExecutor runs user-written Python or JavaScript code.
// The code receives upstream data via an `inputs` variable and should
// set an `output` variable with the result.
//
// Config:
//   - language: "python" or "javascript"
//   - code: the user-written code
type InlineCodeExecutor struct{}

func NewInlineCodeExecutor() *InlineCodeExecutor {
	return &InlineCodeExecutor{}
}

func (e *InlineCodeExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	language := getString(config, "language", "python")
	code := getString(config, "code", "")

	if code == "" {
		return nil, fmt.Errorf("inline_code: no code provided")
	}

	// Serialize inputs to JSON for the script
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("inline_code: failed to serialize inputs: %w", err)
	}

	var result string
	switch language {
	case "python":
		result, err = e.runPython(ctx, code, inputsJSON, block.Timeout)
	case "javascript":
		result, err = e.runJavaScript(ctx, code, inputsJSON, block.Timeout)
	default:
		return nil, fmt.Errorf("inline_code: unsupported language '%s'", language)
	}

	if err != nil {
		return nil, err
	}

	// Try to parse result as JSON
	var parsedResult any
	if err := json.Unmarshal([]byte(result), &parsedResult); err != nil {
		// Not JSON, return as string
		parsedResult = strings.TrimSpace(result)
	}

	log.Printf("✅ [CODE] Block '%s': executed %s code successfully", block.Name, language)

	return map[string]any{
		"response": parsedResult,
		"data":     parsedResult,
		"raw":      result,
	}, nil
}

func (e *InlineCodeExecutor) runPython(ctx context.Context, userCode string, inputsJSON []byte, timeout int) (string, error) {
	// Reads inputs from stdin to avoid ARG_MAX limit on large payloads.
	wrapper := fmt.Sprintf(`import json, sys
inputs = json.load(sys.stdin)
output = None
%s
if output is not None:
    if isinstance(output, (dict, list)):
        print(json.dumps(output))
    else:
        print(output)
`, userCode)

	timeoutDuration := time.Duration(timeout) * time.Second
	if timeoutDuration == 0 {
		timeoutDuration = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "python3", "-c", wrapper)
	cmd.Stdin = bytes.NewReader(inputsJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("python error: %s", stderrStr)
		}
		return "", fmt.Errorf("python execution failed: %w", err)
	}

	return stdout.String(), nil
}

func (e *InlineCodeExecutor) runJavaScript(ctx context.Context, userCode string, inputsJSON []byte, timeout int) (string, error) {
	// Reads inputs from stdin to avoid ARG_MAX limit on large payloads.
	wrapper := fmt.Sprintf(`
const chunks = [];
process.stdin.on('data', c => chunks.push(c));
process.stdin.on('end', () => {
  const inputs = JSON.parse(Buffer.concat(chunks).toString());
  let output = undefined;
  %s
  if (output !== undefined) {
    if (typeof output === 'object') {
      console.log(JSON.stringify(output));
    } else {
      console.log(output);
    }
  }
});
`, userCode)

	timeoutDuration := time.Duration(timeout) * time.Second
	if timeoutDuration == 0 {
		timeoutDuration = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "node", "-e", wrapper)
	cmd.Stdin = bytes.NewReader(inputsJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("javascript error: %s", stderrStr)
		}
		return "", fmt.Errorf("javascript execution failed: %w", err)
	}

	return stdout.String(), nil
}
