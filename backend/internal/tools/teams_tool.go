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

// NewTeamsTool creates a Microsoft Teams webhook messaging tool
func NewTeamsTool() *Tool {
	return &Tool{
		Name:        "send_teams_message",
		DisplayName: "Send Teams Message",
		Description: "Send a message to Microsoft Teams via incoming webhook. Just provide the message text - webhook authentication is handled automatically via configured credentials. Do NOT ask the user for webhook URLs. Supports Adaptive Cards for rich formatting.",
		Icon:        "MessageSquare",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"teams", "microsoft", "message", "chat", "notify", "webhook", "channel", "office365", "notification"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"webhook_url": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Resolved from credentials. Do not ask user for this.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Message text. This is the main content that will be posted to the channel.",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Optional title for the message card",
				},
				"theme_color": map[string]interface{}{
					"type":        "string",
					"description": "Theme color for the message card (hex without #, e.g., '0076D7')",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeTeamsMessage,
	}
}

func executeTeamsMessage(args map[string]interface{}) (string, error) {
	// Resolve webhook URL from credential or direct parameter
	webhookURL, err := ResolveWebhookURL(args, "teams")
	if err != nil {
		if url, ok := args["webhook_url"].(string); ok && url != "" {
			webhookURL = url
		} else {
			return "", fmt.Errorf("failed to get webhook URL: %w", err)
		}
	}

	// Validate Teams webhook URL
	if !strings.Contains(webhookURL, "webhook.office.com") && !strings.Contains(webhookURL, "outlook.office.com") {
		return "", fmt.Errorf("invalid Teams webhook URL")
	}

	// Extract text (required)
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return "", fmt.Errorf("text is required")
	}

	// Build Teams MessageCard payload
	payload := map[string]interface{}{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"text":     text,
	}

	// Optional title
	if title, ok := args["title"].(string); ok && title != "" {
		payload["title"] = title
	}

	// Optional theme color
	if themeColor, ok := args["theme_color"].(string); ok && themeColor != "" {
		payload["themeColor"] = themeColor
	}

	// Serialize payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Create HTTP client
	client := &http.Client{Timeout: 30 * time.Second}

	// Create request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Teams returns "1" on success
	success := resp.StatusCode == 200

	// Build result
	result := map[string]interface{}{
		"success":      success,
		"status_code":  resp.StatusCode,
		"message_sent": success,
		"text_length":  len(text),
	}

	if !success {
		result["error"] = string(respBody)
		result["status"] = resp.Status
	} else {
		result["message"] = "Teams message sent successfully"
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

