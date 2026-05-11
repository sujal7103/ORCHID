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

// NewGoogleChatTool creates a Google Chat webhook messaging tool
func NewGoogleChatTool() *Tool {
	return &Tool{
		Name:        "send_google_chat_message",
		DisplayName: "Send Google Chat Message",
		Description: "Send a message to Google Chat (Google Workspace) via webhook. Just provide the message content - webhook authentication is handled automatically via configured credentials. Do NOT ask the user for webhook URLs. Supports cards for rich formatting.",
		Icon:        "MessageCircle",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"google", "chat", "workspace", "message", "notify", "webhook", "space", "notification", "gchat"},
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
					"description": "Simple text message to send. Supports basic formatting with *bold*, _italic_, and ~strikethrough~.",
				},
				"card_title": map[string]interface{}{
					"type":        "string",
					"description": "Title for a card message (optional, for rich formatting)",
				},
				"card_subtitle": map[string]interface{}{
					"type":        "string",
					"description": "Subtitle for the card (optional)",
				},
				"card_text": map[string]interface{}{
					"type":        "string",
					"description": "Main text content for the card (optional)",
				},
				"card_image_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of an image to display in the card (optional, must be publicly accessible)",
				},
				"button_text": map[string]interface{}{
					"type":        "string",
					"description": "Text for a button in the card (optional)",
				},
				"button_url": map[string]interface{}{
					"type":        "string",
					"description": "URL the button should open (optional, required if button_text is set)",
				},
				"thread_key": map[string]interface{}{
					"type":        "string",
					"description": "Thread key for threading messages together (optional)",
				},
			},
			"required": []string{},
		},
		Execute: executeGoogleChatMessage,
	}
}

func executeGoogleChatMessage(args map[string]interface{}) (string, error) {
	// Resolve webhook URL from credential or direct parameter
	webhookURL, err := ResolveWebhookURL(args, "google_chat")
	if err != nil {
		// Fallback: check for direct webhook_url if credential resolution failed
		if url, ok := args["webhook_url"].(string); ok && url != "" {
			webhookURL = url
		} else {
			return "", fmt.Errorf("failed to get webhook URL: %w", err)
		}
	}

	// Validate Google Chat webhook URL
	if !strings.Contains(webhookURL, "chat.googleapis.com") {
		return "", fmt.Errorf("invalid Google Chat webhook URL")
	}

	// Extract text content
	text, _ := args["text"].(string)

	// Check for card fields
	cardTitle, hasCardTitle := args["card_title"].(string)
	cardSubtitle, _ := args["card_subtitle"].(string)
	cardText, _ := args["card_text"].(string)
	cardImageURL, _ := args["card_image_url"].(string)
	buttonText, _ := args["button_text"].(string)
	buttonURL, _ := args["button_url"].(string)

	// Check for thread key
	threadKey, hasThreadKey := args["thread_key"].(string)

	// Build Google Chat webhook payload
	payload := map[string]interface{}{}

	// Determine if we're sending a card or simple text
	hasCard := hasCardTitle || cardText != "" || cardImageURL != ""

	if hasCard {
		// Build card payload (Google Chat Cards v2 format)
		card := buildGoogleChatCard(cardTitle, cardSubtitle, cardText, cardImageURL, buttonText, buttonURL)
		payload["cardsV2"] = []map[string]interface{}{
			{
				"cardId": "card_" + fmt.Sprintf("%d", time.Now().Unix()),
				"card":   card,
			},
		}
		// Add text as fallback for clients that don't support cards
		if text != "" {
			payload["text"] = text
		}
	} else if text != "" {
		payload["text"] = text
	} else {
		return "", fmt.Errorf("either text or card content is required")
	}

	// Add thread key if provided
	if hasThreadKey && threadKey != "" {
		webhookURL = webhookURL + "&messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD"
		payload["thread"] = map[string]interface{}{
			"threadKey": threadKey,
		}
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Serialize payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

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

	// Build result
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := map[string]interface{}{
		"success":      success,
		"status_code":  resp.StatusCode,
		"status":       resp.Status,
		"message_sent": success,
	}

	if text != "" {
		result["text_length"] = len(text)
	}
	if hasCard {
		result["card_sent"] = true
		if cardTitle != "" {
			result["card_title"] = cardTitle
		}
	}
	if hasThreadKey {
		result["thread_key"] = threadKey
	}

	// Include response body if there's an error
	if !success && len(respBody) > 0 {
		result["error"] = string(respBody)
	}

	// Add success message
	if success {
		if hasCard {
			result["message"] = "Google Chat card message sent successfully"
		} else {
			result["message"] = "Google Chat message sent successfully"
		}
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// buildGoogleChatCard builds a Google Chat Card v2 structure
func buildGoogleChatCard(title, subtitle, text, imageURL, buttonText, buttonURL string) map[string]interface{} {
	card := map[string]interface{}{}

	// Add header if title is provided
	if title != "" {
		header := map[string]interface{}{
			"title": title,
		}
		if subtitle != "" {
			header["subtitle"] = subtitle
		}
		card["header"] = header
	}

	// Build sections
	sections := []map[string]interface{}{}

	// Text section
	if text != "" {
		sections = append(sections, map[string]interface{}{
			"widgets": []map[string]interface{}{
				{
					"textParagraph": map[string]interface{}{
						"text": text,
					},
				},
			},
		})
	}

	// Image section
	if imageURL != "" {
		sections = append(sections, map[string]interface{}{
			"widgets": []map[string]interface{}{
				{
					"image": map[string]interface{}{
						"imageUrl": imageURL,
					},
				},
			},
		})
	}

	// Button section
	if buttonText != "" && buttonURL != "" {
		sections = append(sections, map[string]interface{}{
			"widgets": []map[string]interface{}{
				{
					"buttonList": map[string]interface{}{
						"buttons": []map[string]interface{}{
							{
								"text": buttonText,
								"onClick": map[string]interface{}{
									"openLink": map[string]interface{}{
										"url": buttonURL,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	if len(sections) > 0 {
		card["sections"] = sections
	}

	return card
}
