package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewTwilioSMSTool creates a Twilio SMS tool
func NewTwilioSMSTool() *Tool {
	return &Tool{
		Name:        "twilio_send_sms",
		DisplayName: "Twilio SMS",
		Description: `Send SMS or MMS messages via Twilio.

Features:
- Send text messages to any phone number
- Send MMS with media attachments
- Support for international numbers

Numbers must be in E.164 format (e.g., +1234567890).
Authentication is handled automatically via configured Twilio credentials.`,
		Icon:     "Phone",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"twilio", "sms", "text", "message", "phone", "mms"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Destination phone number in E.164 format (e.g., +1234567890)",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "The text message content (max 1600 characters)",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Twilio phone number to send from (uses default if not specified)",
				},
				"media_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of media to include (for MMS)",
				},
			},
			"required": []string{"to", "body"},
		},
		Execute: executeTwilioSMS,
	}
}

// NewTwilioWhatsAppTool creates a Twilio WhatsApp tool
func NewTwilioWhatsAppTool() *Tool {
	return &Tool{
		Name:        "twilio_send_whatsapp",
		DisplayName: "Twilio WhatsApp",
		Description: `Send WhatsApp messages via Twilio.

Features:
- Send WhatsApp messages
- Send media attachments
- Requires a Twilio WhatsApp-enabled number

Numbers must be in E.164 format (e.g., +1234567890).
Authentication is handled automatically via configured Twilio credentials.`,
		Icon:     "MessageSquare",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"twilio", "whatsapp", "message", "chat"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Destination WhatsApp number in E.164 format (e.g., +1234567890)",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "The message content",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Twilio WhatsApp number to send from",
				},
				"media_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of media to include",
				},
			},
			"required": []string{"to", "body"},
		},
		Execute: executeTwilioWhatsApp,
	}
}

func executeTwilioSMS(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "twilio")
	if err != nil {
		return "", fmt.Errorf("failed to get Twilio credentials: %w", err)
	}

	accountSID, _ := credData["account_sid"].(string)
	authToken, _ := credData["auth_token"].(string)
	defaultFrom, _ := credData["from_number"].(string)

	if accountSID == "" || authToken == "" {
		return "", fmt.Errorf("Twilio credentials incomplete: account_sid and auth_token are required")
	}

	to, _ := args["to"].(string)
	body, _ := args["body"].(string)
	from, _ := args["from"].(string)
	mediaURL, _ := args["media_url"].(string)

	if to == "" {
		return "", fmt.Errorf("'to' phone number is required")
	}
	if body == "" {
		return "", fmt.Errorf("'body' message content is required")
	}

	if from == "" {
		from = defaultFrom
	}
	if from == "" {
		return "", fmt.Errorf("'from' phone number is required")
	}

	// Build form data
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", from)
	data.Set("Body", body)
	if mediaURL != "" {
		data.Set("MediaUrl", mediaURL)
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(accountSID + ":" + authToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if msg, ok := result["message"].(string); ok {
			errMsg = msg
		}
		return "", fmt.Errorf("Twilio API error: %s", errMsg)
	}

	output := map[string]interface{}{
		"success":     true,
		"message_sid": result["sid"],
		"status":      result["status"],
		"to":          result["to"],
		"from":        result["from"],
	}
	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}

func executeTwilioWhatsApp(args map[string]interface{}) (string, error) {
	credData, err := GetCredentialData(args, "twilio")
	if err != nil {
		return "", fmt.Errorf("failed to get Twilio credentials: %w", err)
	}

	accountSID, _ := credData["account_sid"].(string)
	authToken, _ := credData["auth_token"].(string)
	defaultFrom, _ := credData["from_number"].(string)

	if accountSID == "" || authToken == "" {
		return "", fmt.Errorf("Twilio credentials incomplete: account_sid and auth_token are required")
	}

	to, _ := args["to"].(string)
	body, _ := args["body"].(string)
	from, _ := args["from"].(string)
	mediaURL, _ := args["media_url"].(string)

	if to == "" {
		return "", fmt.Errorf("'to' phone number is required")
	}
	if body == "" {
		return "", fmt.Errorf("'body' message content is required")
	}

	// Format WhatsApp numbers
	if !strings.HasPrefix(to, "whatsapp:") {
		to = "whatsapp:" + to
	}
	if from == "" {
		from = defaultFrom
	}
	if from == "" {
		return "", fmt.Errorf("'from' WhatsApp number is required")
	}
	if !strings.HasPrefix(from, "whatsapp:") {
		from = "whatsapp:" + from
	}

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", from)
	data.Set("Body", body)
	if mediaURL != "" {
		data.Set("MediaUrl", mediaURL)
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(accountSID + ":" + authToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if msg, ok := result["message"].(string); ok {
			errMsg = msg
		}
		return "", fmt.Errorf("Twilio API error: %s", errMsg)
	}

	output := map[string]interface{}{
		"success":     true,
		"message_sid": result["sid"],
		"status":      result["status"],
		"to":          result["to"],
		"from":        result["from"],
	}
	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}
