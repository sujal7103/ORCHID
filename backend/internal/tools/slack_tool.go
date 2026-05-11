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

// NewSlackTool creates a Slack webhook messaging tool
func NewSlackTool() *Tool {
	return &Tool{
		Name:        "send_slack_message",
		DisplayName: "Send Slack Message",
		Description: "Send a message to Slack via incoming webhook. Just provide the message text - webhook authentication is handled automatically via configured credentials. Do NOT ask the user for webhook URLs. Supports Block Kit for rich formatting.",
		Icon:        "Hash",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"slack", "message", "chat", "notify", "webhook", "channel", "workspace", "notification"},
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
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Override the default webhook username (optional)",
				},
				"icon_emoji": map[string]interface{}{
					"type":        "string",
					"description": "Override the default webhook icon with an emoji (e.g., ':robot_face:')",
				},
				"icon_url": map[string]interface{}{
					"type":        "string",
					"description": "Override the default webhook icon with an image URL (optional)",
				},
				"channel": map[string]interface{}{
					"type":        "string",
					"description": "Override the default channel (e.g., '#general' or '@username'). Requires additional webhook permissions.",
				},
				"unfurl_links": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable/disable link unfurling (default: true)",
				},
				"unfurl_media": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable/disable media unfurling (default: true)",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeSlackMessage,
	}
}

func executeSlackMessage(args map[string]interface{}) (string, error) {
	// Resolve webhook URL from credential or direct parameter
	webhookURL, err := ResolveWebhookURL(args, "slack")
	if err != nil {
		// Fallback: check for direct webhook_url if credential resolution failed
		if url, ok := args["webhook_url"].(string); ok && url != "" {
			webhookURL = url
		} else {
			return "", fmt.Errorf("failed to get webhook URL: %w", err)
		}
	}

	// Validate Slack webhook URL
	if !strings.Contains(webhookURL, "hooks.slack.com") {
		return "", fmt.Errorf("invalid Slack webhook URL (must contain hooks.slack.com)")
	}

	// Extract text (required)
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return "", fmt.Errorf("text is required")
	}

	// Slack has a soft limit of ~40,000 characters but recommend keeping under 4000
	if len(text) > 40000 {
		text = text[:39997] + "..."
	}

	// Build Slack webhook payload
	payload := map[string]interface{}{
		"text": text,
	}

	// Optional username override
	if username, ok := args["username"].(string); ok && username != "" {
		payload["username"] = username
	}

	// Optional icon emoji
	if iconEmoji, ok := args["icon_emoji"].(string); ok && iconEmoji != "" {
		payload["icon_emoji"] = iconEmoji
	}

	// Optional icon URL
	if iconURL, ok := args["icon_url"].(string); ok && iconURL != "" {
		payload["icon_url"] = iconURL
	}

	// Optional channel override
	if channel, ok := args["channel"].(string); ok && channel != "" {
		payload["channel"] = channel
	}

	// Optional unfurl settings
	if unfurlLinks, ok := args["unfurl_links"].(bool); ok {
		payload["unfurl_links"] = unfurlLinks
	}

	if unfurlMedia, ok := args["unfurl_media"].(bool); ok {
		payload["unfurl_media"] = unfurlMedia
	}

	// Serialize payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

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

	// Slack returns "ok" on success
	success := resp.StatusCode == 200 && string(respBody) == "ok"

	// Build result
	result := map[string]interface{}{
		"success":       success,
		"status_code":   resp.StatusCode,
		"message_sent":  success,
		"text_length":   len(text),
	}

	// Include response body for debugging
	if !success {
		result["error"] = string(respBody)
		result["status"] = resp.Status
	} else {
		result["message"] = "Slack message sent successfully"
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
