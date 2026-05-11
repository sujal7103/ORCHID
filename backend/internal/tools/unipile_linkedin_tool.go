package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// NewUnipileLinkedInSendMessageTool creates a tool for sending LinkedIn DMs via Unipile.
func NewUnipileLinkedInSendMessageTool() *Tool {
	return &Tool{
		Name:        "unipile_linkedin_send_message",
		DisplayName: "Unipile LinkedIn - Send Message",
		Description: "Send a LinkedIn direct message via Unipile with optional image/file attachment. To message an existing conversation, provide chat_id (get IDs from unipile_linkedin_list_chats). To start a NEW conversation, provide account_id (get from unipile_list_accounts) and attendee_id (get from unipile_list_attendees or unipile_linkedin_search_profiles). Optionally attach an image or document via attachment_url. Authentication is handled automatically.",
		Icon:        "Linkedin",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"linkedin", "unipile", "message", "dm", "send", "messaging", "direct message", "image", "attachment"},
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
					"description": "The ID of an existing LinkedIn conversation to send the message to.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Your Unipile LinkedIn account ID (get from unipile_list_accounts). Required when starting a new conversation.",
				},
				"attendee_id": map[string]interface{}{
					"type":        "string",
					"description": "The recipient's LinkedIn provider ID (get from unipile_list_attendees or unipile_linkedin_search_profiles). Required when starting a new conversation.",
				},
				"attachment_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of an image, PDF, or document to attach to the message (max 15MB). Supports images, PDFs, videos, and documents.",
				},
			},
			"required": []string{"text"},
		},
		Execute: executeUnipileLinkedInSendMessage,
	}
}

func executeUnipileLinkedInSendMessage(args map[string]interface{}) (string, error) {
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
		// Send to existing conversation
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
		return "", fmt.Errorf("either chat_id (existing conversation) or both account_id and attendee_id (new conversation) are required")
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
		"message":      "LinkedIn message sent successfully",
		"message_sent": true,
		"response":     resp,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// NewUnipileLinkedInListChatsTool creates a tool for listing LinkedIn conversations via Unipile.
func NewUnipileLinkedInListChatsTool() *Tool {
	return &Tool{
		Name:        "unipile_linkedin_list_chats",
		DisplayName: "Unipile LinkedIn - List Chats",
		Description: "List LinkedIn message conversations via Unipile. Returns recent conversations with metadata. Optionally filter by account_id if multiple LinkedIn accounts are connected.",
		Icon:        "Linkedin",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"linkedin", "unipile", "chats", "conversations", "list", "inbox", "dm"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter conversations to a specific Unipile LinkedIn account (optional).",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of conversations to return (default: 50, max: 100).",
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Pagination cursor from a previous response to get the next page.",
				},
			},
			"required": []string{},
		},
		Execute: executeUnipileLinkedInListChats,
	}
}

func executeUnipileLinkedInListChats(args map[string]interface{}) (string, error) {
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

// NewUnipileLinkedInGetMessagesTool creates a tool for getting messages from a LinkedIn conversation.
func NewUnipileLinkedInGetMessagesTool() *Tool {
	return &Tool{
		Name:        "unipile_linkedin_get_messages",
		DisplayName: "Unipile LinkedIn - Get Messages",
		Description: "Get messages from a specific LinkedIn conversation via Unipile. Returns message history for the given chat_id.",
		Icon:        "Linkedin",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"linkedin", "unipile", "messages", "history", "conversation", "read", "dm"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"chat_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the LinkedIn conversation to retrieve messages from.",
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
		Execute: executeUnipileLinkedInGetMessages,
	}
}

func executeUnipileLinkedInGetMessages(args map[string]interface{}) (string, error) {
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

// NewUnipileLinkedInSearchProfilesTool creates a tool for searching LinkedIn profiles via Unipile.
func NewUnipileLinkedInSearchProfilesTool() *Tool {
	return &Tool{
		Name:        "unipile_linkedin_search_profiles",
		DisplayName: "Unipile LinkedIn - Search Profiles",
		Description: "Search LinkedIn profiles via Unipile. Find people by name, company, title, or keywords. Requires an account_id for the LinkedIn account to search from.",
		Icon:        "Linkedin",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"linkedin", "unipile", "search", "profiles", "people", "find", "recruit"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "The Unipile LinkedIn account ID to search from.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query: name, company, title, or keywords to find LinkedIn profiles.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of profiles to return (default: 25, max: 100).",
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Pagination cursor from a previous response to get the next page.",
				},
			},
			"required": []string{"account_id", "query"},
		},
		Execute: executeUnipileLinkedInSearchProfiles,
	}
}

func executeUnipileLinkedInSearchProfiles(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	accountID, _ := args["account_id"].(string)
	if accountID == "" {
		return "", fmt.Errorf("account_id is required")
	}

	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	payload := map[string]interface{}{
		"account_id": accountID,
		"query":      query,
	}

	limit := 25
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	payload["limit"] = limit

	if cursor, ok := args["cursor"].(string); ok && cursor != "" {
		payload["cursor"] = cursor
	}

	body, statusCode, err := client.PostJSON("/linkedin/search", payload)
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
