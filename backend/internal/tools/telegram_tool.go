package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NewTelegramTool creates a Telegram Bot API messaging tool
func NewTelegramTool() *Tool {
	return &Tool{
		Name:        "send_telegram_message",
		DisplayName: "Send Telegram Message",
		Description: "Send a message to Telegram via Bot API. Just provide the message content and chat ID - bot authentication is handled automatically via configured credentials. Do NOT ask the user for bot tokens. Supports markdown and HTML formatting.",
		Icon:        "Send",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"telegram", "message", "chat", "notify", "bot", "notification", "messenger"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"bot_token": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Resolved from credentials. Do not ask user for this.",
				},
				"chat_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Resolved from credentials. The chat ID to send messages to. Can be overridden for dynamic use cases.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Message text (max 4096 characters). This is the main message content.",
				},
				"parse_mode": map[string]interface{}{
					"type":        "string",
					"description": "Text formatting mode: 'MarkdownV2', 'HTML', or 'Markdown' (legacy). Optional.",
					"enum":        []string{"MarkdownV2", "HTML", "Markdown"},
				},
				"disable_notification": map[string]interface{}{
					"type":        "boolean",
					"description": "Send message silently without notification sound. Optional, defaults to false.",
				},
				"disable_link_preview": map[string]interface{}{
					"type":        "boolean",
					"description": "Disable link previews for URLs in the message. Optional, defaults to false.",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeTelegramMessage,
	}
}

func executeTelegramMessage(args map[string]interface{}) (string, error) {
	// Get all credential data (both bot_token and chat_id)
	credData, err := GetCredentialData(args, "telegram")
	if err != nil {
		return "", fmt.Errorf("failed to get Telegram credentials: %w. Please configure Telegram credentials first.", err)
	}

	// Extract bot_token from credential
	botToken, ok := credData["bot_token"].(string)
	if !ok || botToken == "" {
		return "", fmt.Errorf("Telegram credentials incomplete: bot_token is required")
	}

	// Extract chat_id - first check args (for override), then fall back to credential
	chatID, _ := args["chat_id"].(string)
	if chatID == "" {
		// Fall back to chat_id from credential
		if credChatID, ok := credData["chat_id"].(string); ok {
			chatID = credChatID
		}
	}

	if chatID == "" {
		return "", fmt.Errorf("chat_id is required - either provide it in the request or configure a default in Telegram credentials")
	}

	// Extract text (required)
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return "", fmt.Errorf("text is required")
	}

	// Truncate text if too long (Telegram limit is 4096)
	if len(text) > 4096 {
		text = text[:4093] + "..."
	}

	// Build Telegram API payload
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	// Optional parse_mode
	if parseMode, ok := args["parse_mode"].(string); ok && parseMode != "" {
		payload["parse_mode"] = parseMode
	}

	// Optional disable_notification
	if disableNotification, ok := args["disable_notification"].(bool); ok && disableNotification {
		payload["disable_notification"] = true
	}

	// Optional link_preview_options (replaces deprecated disable_web_page_preview)
	if disablePreview, ok := args["disable_link_preview"].(bool); ok && disablePreview {
		payload["link_preview_options"] = map[string]interface{}{
			"is_disabled": true,
		}
	}

	// Build API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

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
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
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

	// Parse Telegram response
	var telegramResp map[string]interface{}
	if err := json.Unmarshal(respBody, &telegramResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if request was successful
	success := false
	if ok, exists := telegramResp["ok"].(bool); exists {
		success = ok
	}

	// Build result
	result := map[string]interface{}{
		"success":       success,
		"status_code":   resp.StatusCode,
		"message_sent":  success,
		"chat_id":       chatID,
		"text_length":   len(text),
	}

	// Include message_id if successful
	if success {
		if msgResult, ok := telegramResp["result"].(map[string]interface{}); ok {
			if msgID, ok := msgResult["message_id"].(float64); ok {
				result["message_id"] = int(msgID)
			}
		}
		result["message"] = "Telegram message sent successfully"
	} else {
		// Include error details
		if description, ok := telegramResp["description"].(string); ok {
			result["error"] = description
		}
		if errorCode, ok := telegramResp["error_code"].(float64); ok {
			result["error_code"] = int(errorCode)
		}
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
