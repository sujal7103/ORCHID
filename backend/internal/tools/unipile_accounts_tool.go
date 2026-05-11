package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// NewUnipileListAccountsTool creates a tool for listing connected Unipile accounts.
func NewUnipileListAccountsTool() *Tool {
	return &Tool{
		Name:        "unipile_list_accounts",
		DisplayName: "Unipile - List Connected Accounts",
		Description: "List all connected accounts (WhatsApp, LinkedIn, etc.) on your Unipile instance. Use this first to discover your account_id values, which are needed to send new messages or search profiles. Each account has a type (WHATSAPP, LINKEDIN, etc.) and a unique ID.",
		Icon:        "Users",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"unipile", "accounts", "list", "whatsapp", "linkedin", "connected"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
			},
			"required": []string{},
		},
		Execute: executeUnipileListAccounts,
	}
}

func executeUnipileListAccounts(args map[string]interface{}) (string, error) {
	client, err := NewUnipileClientFromArgs(args)
	if err != nil {
		return "", err
	}

	body, statusCode, err := client.Get("/accounts", nil)
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

// NewUnipileListAttendeesTool creates a tool for listing/searching contacts across accounts.
func NewUnipileListAttendeesTool() *Tool {
	return &Tool{
		Name:        "unipile_list_attendees",
		DisplayName: "Unipile - List Contacts",
		Description: "List or search contacts/attendees across your connected Unipile accounts. Use this to find a person's attendee_id which is needed to start a new conversation. You can filter by account_id to only see contacts from a specific WhatsApp or LinkedIn account.",
		Icon:        "Users",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"unipile", "contacts", "attendees", "search", "people", "phone", "find"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter contacts to a specific connected account (optional). Use unipile_list_accounts to find your account IDs.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of contacts to return (default: 50, max: 100).",
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Pagination cursor from a previous response to get the next page.",
				},
			},
			"required": []string{},
		},
		Execute: executeUnipileListAttendees,
	}
}

func executeUnipileListAttendees(args map[string]interface{}) (string, error) {
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

	body, statusCode, err := client.Get("/attendees", params)
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
