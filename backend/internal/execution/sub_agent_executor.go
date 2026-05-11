package execution

import (
	"bytes"
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SubAgentExecutor triggers another agent via the internal API and waits for its result.
//
// Config:
//   - agentId: ID of the agent to call
//   - inputMapping: template for the input (e.g., "{{upstream.response}}")
//   - waitForCompletion: whether to poll for completion (default true)
//   - timeoutSeconds: max wait time (default 120)
type SubAgentExecutor struct {
	internalBaseURL string
	serviceKey      string
}

func NewSubAgentExecutor(internalBaseURL, serviceKey string) *SubAgentExecutor {
	return &SubAgentExecutor{
		internalBaseURL: internalBaseURL,
		serviceKey:      serviceKey,
	}
}

func (e *SubAgentExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	agentID := getString(config, "agentId", "")
	if agentID == "" {
		return nil, fmt.Errorf("sub_agent: no agentId configured")
	}

	inputMapping := getString(config, "inputMapping", "{{input}}")
	waitForCompletion := getBool(config, "waitForCompletion", true)
	timeoutSeconds := getInt(config, "timeoutSeconds", 120)

	// Resolve the input using template interpolation
	resolvedInput := InterpolateTemplate(inputMapping, inputs)

	log.Printf("🔗 [SUB_AGENT] Block '%s': triggering agent %s (wait=%v, timeout=%ds)",
		block.Name, agentID, waitForCompletion, timeoutSeconds)

	// Extract user ID from context if available
	userID, _ := ctx.Value("userID").(string)

	// Trigger the sub-agent via internal API
	triggerURL := fmt.Sprintf("%s/api/trigger/%s", e.internalBaseURL, agentID)

	// Build trigger payload
	payload := map[string]any{
		"input": map[string]any{
			"message": resolvedInput,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("sub_agent: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", triggerURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("sub_agent: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service-Key", e.serviceKey)
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sub_agent: failed to trigger agent: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sub_agent: trigger failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var triggerResp map[string]any
	if err := json.Unmarshal(body, &triggerResp); err != nil {
		return nil, fmt.Errorf("sub_agent: failed to parse trigger response: %w", err)
	}

	executionID, _ := triggerResp["executionId"].(string)
	if executionID == "" {
		// If no executionId, the response might be the direct result (sync mode)
		log.Printf("✅ [SUB_AGENT] Block '%s': got direct response from agent %s", block.Name, agentID)
		return map[string]any{
			"response": triggerResp,
			"data":     triggerResp,
		}, nil
	}

	if !waitForCompletion {
		log.Printf("✅ [SUB_AGENT] Block '%s': triggered agent %s (execution: %s), not waiting",
			block.Name, agentID, executionID)
		return map[string]any{
			"response":    executionID,
			"data":        triggerResp,
			"executionId": executionID,
		}, nil
	}

	// Poll for completion
	statusURL := fmt.Sprintf("%s/api/trigger/status/%s", e.internalBaseURL, executionID)
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		statusReq, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
		if err != nil {
			continue
		}
		statusReq.Header.Set("X-Internal-Service-Key", e.serviceKey)
		if userID != "" {
			statusReq.Header.Set("X-User-ID", userID)
		}

		statusResp, err := client.Do(statusReq)
		if err != nil {
			continue
		}

		statusBody, _ := io.ReadAll(statusResp.Body)
		statusResp.Body.Close()

		var statusResult map[string]any
		if err := json.Unmarshal(statusBody, &statusResult); err != nil {
			continue
		}

		status, _ := statusResult["status"].(string)
		switch status {
		case "completed":
			log.Printf("✅ [SUB_AGENT] Block '%s': agent %s completed (execution: %s)",
				block.Name, agentID, executionID)

			result, _ := statusResult["result"]
			return map[string]any{
				"response":    result,
				"data":        statusResult,
				"executionId": executionID,
			}, nil

		case "failed":
			errMsg, _ := statusResult["error"].(string)
			return nil, fmt.Errorf("sub_agent: agent %s failed: %s", agentID, errMsg)
		}

		// Still running, continue polling
	}

	return nil, fmt.Errorf("sub_agent: agent %s timed out after %ds (execution: %s)",
		agentID, timeoutSeconds, executionID)
}

func getBool(config map[string]any, key string, defaultVal bool) bool {
	if v, ok := config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}
