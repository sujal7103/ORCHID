package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// NewUnipileWhatsAppSendMessageTool creates a tool for sending WhatsApp messages via Unipile.
func NewUnipileWhatsAppSendMessageTool() *Tool {
	return &Tool{
		Name:        "unipile_whatsapp_send_message",
		DisplayName: "Unipile WhatsApp - Send Message",
		Description: "Send a WhatsApp message via Unipile with optional image/file attachment. To message an existing chat, provide chat_id (get IDs from unipile_whatsapp_list_chats). To start a NEW conversation with a phone number, provide account_id (get from unipile_list_accounts) and attendee_id (the phone number in international format like +1234567890). Optionally attach an image or document via attachment_url. Authentication is handled automatically.",
		Icon:        "MessageCircle",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"whatsapp", "unipile", "message", "chat", "send", "messaging", "wa", "image", "attachment"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The message text to send.",
				},
				"chat_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of an existing chat to send the message to. Use this for ongoing conversations.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Your Unipile WhatsApp account ID (get from unipile_list_accounts). Required when starting a new conversation.",
				},
				"attendee_id": map[string]interface{}{
					"type":        "string",
					"description": "The recipient's phone number in international format (e.g. +1234567890). Required when starting a new conversation. You can also use an attendee ID from unipile_list_attendees.",
				},
				"attachment_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of an image, PDF, or document to attach to the message (max 15MB). Supports images, PDFs, videos, and documents.",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeUnipileWhatsAppSendMessage,
	}
}

func executeUnipileWhatsAppSendMessage(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	chatID, _ := args["chat_id"].(string)
	accountID, _ := args["account_id"].(string)
	attendeeID, _ := args["attendee_id"].(string)
	attachmentURL, _ := args["attachment_url"].(string)

	var body []byte
	var statusCode int

	if chatID != "" {
		// Send to existing chat
		fields := map[string]string{"text": text}
		body, statusCode, err = client.PostMultipartWithAttachment("/chats/"+chatID+"/messages", fields, attachmentURL)
	} else if accountID != "" && attendeeID != "" {
		// Start new conversation
		fields := map[string]string{
			"text":          text,
			"account_id":    accountID,
			"attendees_ids": attendeeID,
		}
		body, statusCode, err = client.PostMultipartWithAttachment("/chats", fields, attachmentURL)
	} else {
		return "", fmt.Errorf("either chat_id (existing chat) or both account_id and attendee_id (new chat) are required")
	}

	if err != nil {
		return "", err
	}

	if statusCode < 200 || statusCode >= 300 {
		return "", formatUnipileError(statusCode, body)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"success":      true,
		"message":      "WhatsApp message sent successfully",
		"message_sent": true,
		"response":     resp,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// NewUnipileWhatsAppListChatsTool creates a tool for listing WhatsApp chats via Unipile.
func NewUnipileWhatsAppListChatsTool() *Tool {
	return &Tool{
		Name:        "unipile_whatsapp_list_chats",
		DisplayName: "Unipile WhatsApp - List Chats",
		Description: "List WhatsApp chat conversations via Unipile. Returns recent chats with metadata. Optionally filter by account_id if multiple WhatsApp accounts are connected.",
		Icon:        "MessageCircle",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"whatsapp", "unipile", "chats", "conversations", "list", "inbox"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter chats to a specific Unipile WhatsApp account (optional).",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of chats to return (default: 50, max: 100).",
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Pagination cursor from a previous response to get the next page.",
				},
			},
			"required": []string{},
		},
		Execute: executeUnipileWhatsAppListChats,
	}
}

func executeUnipileWhatsAppListChats(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	listParams := unipileListParams(args, 50)
	for k, v := range listParams {
		params.Set(k, v)
	}

	if accountID, ok := args["account_id"].(string); ok && accountID != "" {
		params.Set("account_id", accountID)
	}

	body, statusCode, err := client.Get("/chats", params)
	if err != nil {
		return "", err
	}

	if statusCode < 200 || statusCode >= 300 {
		return "", formatUnipileError(statusCode, body)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"data":    resp,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// NewUnipileWhatsAppGetMessagesTool creates a tool for getting messages from a WhatsApp chat.
func NewUnipileWhatsAppGetMessagesTool() *Tool {
	return &Tool{
		Name:        "unipile_whatsapp_get_messages",
		DisplayName: "Unipile WhatsApp - Get Messages",
		Description: "Get messages from a specific WhatsApp chat via Unipile. Returns message history for the given chat_id.",
		Icon:        "MessageCircle",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"whatsapp", "unipile", "messages", "history", "chat", "read"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"chat_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the chat to retrieve messages from.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of messages to return (default: 50, max: 100).",
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Pagination cursor from a previous response to get the next page.",
				},
			},
			"required": []string{"chat_id"},
		},
		Execute: executeUnipileWhatsAppGetMessages,
	}
}

func executeUnipileWhatsAppGetMessages(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	chatID, _ := args["chat_id"].(string)
	if chatID == "" {
		return "", fmt.Errorf("chat_id is required")
	}

	params := url.Values{}
	listParams := unipileListParams(args, 50)
	for k, v := range listParams {
		params.Set(k, v)
	}

	body, statusCode, err := client.Get("/chats/"+chatID+"/messages", params)
	if err != nil {
		return "", err
	}

	if statusCode < 200 || statusCode >= 300 {
		return "", formatUnipileError(statusCode, body)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"data":    resp,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// NewUnipileWhatsAppSendToPhoneTool creates a simplified tool for sending cold WhatsApp messages directly to a phone number.
// It auto-discovers the WhatsApp account so the user only needs to provide a phone number and message text.
func NewUnipileWhatsAppSendToPhoneTool() *Tool {
	return &Tool{
		Name:        "unipile_whatsapp_send_to_phone",
		DisplayName: "Unipile WhatsApp - Send to Phone Number",
		Description: "Send a WhatsApp message directly to a phone number (cold message) with optional image/file attachment. Just provide the phone number in international format (e.g. +1234567890) and the message text. The WhatsApp account is auto-detected from your connected Unipile accounts. Perfect for cold outreach — no need to look up account_id or chat_id first.",
		Icon:        "Phone",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"whatsapp", "unipile", "phone", "cold", "outreach", "send", "sms", "direct", "image", "attachment"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"phone_number": map[string]interface{}{
					"type":        "string",
					"description": "The recipient's phone number in international format (e.g. +1234567890).",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The message text to send.",
				},
				"attachment_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of an image, PDF, or document to attach to the message (max 15MB). Supports images, PDFs, videos, and documents.",
				},
			},
			"required": []string{"phone_number", "text"},
		},
		Execute: executeUnipileWhatsAppSendToPhone,
	}
}

func executeUnipileWhatsAppSendToPhone(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	phoneNumber, _ := args["phone_number"].(string)
	if phoneNumber == "" {
		return "", fmt.Errorf("phone_number is required")
	}

	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	// Auto-discover the WhatsApp account
	accountsBody, statusCode, err := client.Get("/accounts", nil)
	if err != nil {
		return "", fmt.Errorf("failed to list accounts to find WhatsApp account: %w", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return "", formatUnipileError(statusCode, accountsBody)
	}

	var accountsResp map[string]interface{}
	if err := json.Unmarshal(accountsBody, &accountsResp); err != nil {
		return "", fmt.Errorf("failed to parse accounts response: %w", err)
	}

	// Find the first WHATSAPP account
	var whatsappAccountID string
	if items, ok := accountsResp["items"].([]interface{}); ok {
		for _, item := range items {
			if acc, ok := item.(map[string]interface{}); ok {
				accType, _ := acc["type"].(string)
				if strings.EqualFold(accType, "WHATSAPP") {
					whatsappAccountID, _ = acc["id"].(string)
					break
				}
			}
		}
	}

	if whatsappAccountID == "" {
		return "", fmt.Errorf("no WhatsApp account found on your Unipile instance. Please connect a WhatsApp account in Unipile first")
	}

	// Send the message using the discovered account and the phone number as attendee
	attachmentURL, _ := args["attachment_url"].(string)
	fields := map[string]string{
		"text":          text,
		"account_id":    whatsappAccountID,
		"attendees_ids": phoneNumber,
	}
	body, statusCode, err := client.PostMultipartWithAttachment("/chats", fields, attachmentURL)
	if err != nil {
		return "", err
	}

	if statusCode < 200 || statusCode >= 300 {
		return "", formatUnipileError(statusCode, body)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"success":      true,
		"message":      fmt.Sprintf("WhatsApp message sent to %s", phoneNumber),
		"message_sent": true,
		"account_used": whatsappAccountID,
		"response":     resp,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
