package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewPostHogCaptureTool creates a PostHog event capture tool
func NewPostHogCaptureTool() *Tool {
	return &Tool{
		Name:        "posthog_capture",
		DisplayName: "PostHog Capture Event",
		Description: `Capture events in PostHog for product analytics.

Send user actions and behaviors to PostHog for analysis.
Authentication is handled automatically via configured PostHog credentials.`,
		Icon:     "BarChart2",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"posthog", "analytics", "capture", "event", "metrics"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"event": map[string]interface{}{
					"type":        "string",
					"description": "The name of the event to capture",
				},
				"distinct_id": map[string]interface{}{
					"type":        "string",
					"description": "The unique user identifier",
				},
				"properties": map[string]interface{}{
					"type":        "object",
					"description": "Additional properties to capture with the event",
				},
			},
			"required": []string{"event", "distinct_id"},
		},
		Execute: executePostHogCapture,
	}
}

// NewPostHogIdentifyTool creates a PostHog user identify tool
func NewPostHogIdentifyTool() *Tool {
	return &Tool{
		Name:        "posthog_identify",
		DisplayName: "PostHog Identify User",
		Description: `Identify users in PostHog.

Set user properties for segmentation and personalization.
Authentication is handled automatically via configured PostHog credentials.`,
		Icon:     "User",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"posthog", "identify", "user", "profile"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"distinct_id": map[string]interface{}{
					"type":        "string",
					"description": "The unique user identifier",
				},
				"properties": map[string]interface{}{
					"type":        "object",
					"description": "User properties to set",
				},
				"set_once_properties": map[string]interface{}{
					"type":        "object",
					"description": "Properties to set only if not already set",
				},
			},
			"required": []string{"distinct_id"},
		},
		Execute: executePostHogIdentify,
	}
}

// NewPostHogQueryTool creates a PostHog query tool
func NewPostHogQueryTool() *Tool {
	return &Tool{
		Name:        "posthog_query",
		DisplayName: "PostHog Query",
		Description: `Query PostHog data using HogQL.

Requires a Personal API Key to be configured.
Authentication is handled automatically via configured PostHog credentials.`,
		Icon:     "Database",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"posthog", "query", "hogql", "data", "analytics"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "HogQL query to execute",
				},
			},
			"required": []string{"query"},
		},
		Execute: executePostHogQuery,
	}
}

func getPostHogHost(credData map[string]interface{}) string {
	if host, ok := credData["host"].(string); ok && host != "" {
		return strings.TrimSuffix(host, "/")
	}
	return "https://app.posthog.com"
}

func executePostHogCapture(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "posthog")
	if err != nil {
		return "", fmt.Errorf("failed to get PostHog credentials: %w", err)
	}

	apiKey, _ := credData["api_key"].(string)
	if apiKey == "" {
		return "", fmt.Errorf("PostHog api_key not configured")
	}

	event, _ := args["event"].(string)
	distinctID, _ := args["distinct_id"].(string)
	if event == "" || distinctID == "" {
		return "", fmt.Errorf("'event' and 'distinct_id' are required")
	}

	host := getPostHogHost(credData)
	payload := map[string]interface{}{
		"api_key":     apiKey,
		"event":       event,
		"distinct_id": distinctID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if props, ok := args["properties"].(map[string]interface{}); ok {
		payload["properties"] = props
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", host+"/capture/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PostHog API error: %s", string(body))
	}

	output := map[string]interface{}{
		"success":     true,
		"event":       event,
		"distinct_id": distinctID,
	}
	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

func executePostHogIdentify(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "posthog")
	if err != nil {
		return "", fmt.Errorf("failed to get PostHog credentials: %w", err)
	}

	apiKey, _ := credData["api_key"].(string)
	if apiKey == "" {
		return "", fmt.Errorf("PostHog api_key not configured")
	}

	distinctID, _ := args["distinct_id"].(string)
	if distinctID == "" {
		return "", fmt.Errorf("'distinct_id' is required")
	}

	host := getPostHogHost(credData)
	payload := map[string]interface{}{
		"api_key":     apiKey,
		"event":       "$identify",
		"distinct_id": distinctID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	props := make(map[string]interface{})
	if p, ok := args["properties"].(map[string]interface{}); ok {
		props["$set"] = p
	}
	if p, ok := args["set_once_properties"].(map[string]interface{}); ok {
		props["$set_once"] = p
	}
	if len(props) > 0 {
		payload["properties"] = props
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", host+"/capture/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PostHog API error: %s", string(body))
	}

	output := map[string]interface{}{
		"success":     true,
		"distinct_id": distinctID,
	}
	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

func executePostHogQuery(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "posthog")
	if err != nil {
		return "", fmt.Errorf("failed to get PostHog credentials: %w", err)
	}

	personalAPIKey, _ := credData["personal_api_key"].(string)
	if personalAPIKey == "" {
		return "", fmt.Errorf("PostHog personal_api_key not configured (required for queries)")
	}

	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("'query' is required")
	}

	host := getPostHogHost(credData)
	payload := map[string]interface{}{
		"query": map[string]interface{}{
			"kind":  "HogQLQuery",
			"query": query,
		},
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", host+"/api/projects/@current/query/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+personalAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if msg, ok := result["detail"].(string); ok {
			errMsg = msg
		}
		return "", fmt.Errorf("PostHog API error: %s", errMsg)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
