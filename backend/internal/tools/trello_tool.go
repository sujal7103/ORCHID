package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const trelloAPIBase = "https://api.trello.com/1"

// NewTrelloBoardsTool creates a tool for listing Trello boards
func NewTrelloBoardsTool() *Tool {
	return &Tool{
		Name:        "trello_boards",
		DisplayName: "List Trello Boards",
		Description: "List all boards accessible to the authenticated user. Authentication is handled automatically via configured credentials.",
		Icon:        "Layout",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"trello", "boards", "list", "project", "kanban"},
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
		Execute: executeTrelloBoards,
	}
}

// NewTrelloListsTool creates a tool for listing Trello lists
func NewTrelloListsTool() *Tool {
	return &Tool{
		Name:        "trello_lists",
		DisplayName: "List Trello Lists",
		Description: "List all lists in a Trello board. Authentication is handled automatically.",
		Icon:        "List",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"trello", "lists", "board", "columns"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"board_id": map[string]interface{}{
					"type":        "string",
					"description": "Trello Board ID",
				},
			},
			"required": []string{"board_id"},
		},
		Execute: executeTrelloLists,
	}
}

// NewTrelloCardsTool creates a tool for listing Trello cards
func NewTrelloCardsTool() *Tool {
	return &Tool{
		Name:        "trello_cards",
		DisplayName: "List Trello Cards",
		Description: "List cards from a Trello board or list. Authentication is handled automatically.",
		Icon:        "CreditCard",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"trello", "cards", "tasks", "items"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"board_id": map[string]interface{}{
					"type":        "string",
					"description": "Trello Board ID (required if list_id not provided)",
				},
				"list_id": map[string]interface{}{
					"type":        "string",
					"description": "Trello List ID (optional, filters cards to this list)",
				},
			},
			"required": []string{},
		},
		Execute: executeTrelloCards,
	}
}

// NewTrelloCreateCardTool creates a tool for creating Trello cards
func NewTrelloCreateCardTool() *Tool {
	return &Tool{
		Name:        "trello_create_card",
		DisplayName: "Create Trello Card",
		Description: "Create a new card in a Trello list. Authentication is handled automatically.",
		Icon:        "Plus",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"trello", "card", "create", "task", "add"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"list_id": map[string]interface{}{
					"type":        "string",
					"description": "Trello List ID where the card will be created",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Card title/name",
				},
				"desc": map[string]interface{}{
					"type":        "string",
					"description": "Card description",
				},
				"due": map[string]interface{}{
					"type":        "string",
					"description": "Due date (ISO 8601 format)",
				},
				"labels": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated label IDs",
				},
			},
			"required": []string{"list_id", "name"},
		},
		Execute: executeTrelloCreateCard,
	}
}

func trelloRequest(method, endpoint, apiKey, token string, body interface{}) (interface{}, error) {
	// Add auth params to URL
	u, _ := url.Parse(trelloAPIBase + endpoint)
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("token", token)
	u.RawQuery = q.Encode()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Trello API error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func getTrelloCredentials(args map[string]interface{}) (string, string, error) {
	credData, err := GetCredentialData(args, "trello")
	if err != nil {
		return "", "", fmt.Errorf("failed to get Trello credentials: %w", err)
	}

	apiKey, _ := credData["api_key"].(string)
	token, _ := credData["token"].(string)

	if apiKey == "" || token == "" {
		return "", "", fmt.Errorf("api_key and token are required")
	}

	return apiKey, token, nil
}

func executeTrelloBoards(args map[string]interface{}) (string, error) {
	apiKey, token, err := getTrelloCredentials(args)
	if err != nil {
		return "", err
	}

	result, err := trelloRequest("GET", "/members/me/boards", apiKey, token, nil)
	if err != nil {
		return "", err
	}

	boards, ok := result.([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	simplifiedBoards := make([]map[string]interface{}, 0)
	for _, b := range boards {
		board, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		simplifiedBoards = append(simplifiedBoards, map[string]interface{}{
			"id":     board["id"],
			"name":   board["name"],
			"url":    board["url"],
			"closed": board["closed"],
		})
	}

	response := map[string]interface{}{
		"success": true,
		"count":   len(simplifiedBoards),
		"boards":  simplifiedBoards,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeTrelloLists(args map[string]interface{}) (string, error) {
	apiKey, token, err := getTrelloCredentials(args)
	if err != nil {
		return "", err
	}

	boardID, _ := args["board_id"].(string)
	if boardID == "" {
		return "", fmt.Errorf("board_id is required")
	}

	endpoint := fmt.Sprintf("/boards/%s/lists", boardID)
	result, err := trelloRequest("GET", endpoint, apiKey, token, nil)
	if err != nil {
		return "", err
	}

	lists, ok := result.([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	simplifiedLists := make([]map[string]interface{}, 0)
	for _, l := range lists {
		list, ok := l.(map[string]interface{})
		if !ok {
			continue
		}
		simplifiedLists = append(simplifiedLists, map[string]interface{}{
			"id":     list["id"],
			"name":   list["name"],
			"closed": list["closed"],
			"pos":    list["pos"],
		})
	}

	response := map[string]interface{}{
		"success": true,
		"count":   len(simplifiedLists),
		"lists":   simplifiedLists,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeTrelloCards(args map[string]interface{}) (string, error) {
	apiKey, token, err := getTrelloCredentials(args)
	if err != nil {
		return "", err
	}

	boardID, _ := args["board_id"].(string)
	listID, _ := args["list_id"].(string)

	var endpoint string
	if listID != "" {
		endpoint = fmt.Sprintf("/lists/%s/cards", listID)
	} else if boardID != "" {
		endpoint = fmt.Sprintf("/boards/%s/cards", boardID)
	} else {
		return "", fmt.Errorf("either board_id or list_id is required")
	}

	result, err := trelloRequest("GET", endpoint, apiKey, token, nil)
	if err != nil {
		return "", err
	}

	cards, ok := result.([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	simplifiedCards := make([]map[string]interface{}, 0)
	for _, c := range cards {
		card, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		simplifiedCards = append(simplifiedCards, map[string]interface{}{
			"id":       card["id"],
			"name":     card["name"],
			"desc":     card["desc"],
			"url":      card["url"],
			"due":      card["due"],
			"closed":   card["closed"],
			"idList":   card["idList"],
			"labels":   card["labels"],
			"pos":      card["pos"],
		})
	}

	response := map[string]interface{}{
		"success": true,
		"count":   len(simplifiedCards),
		"cards":   simplifiedCards,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeTrelloCreateCard(args map[string]interface{}) (string, error) {
	apiKey, token, err := getTrelloCredentials(args)
	if err != nil {
		return "", err
	}

	listID, _ := args["list_id"].(string)
	name, _ := args["name"].(string)

	if listID == "" || name == "" {
		return "", fmt.Errorf("list_id and name are required")
	}

	// Build query params
	endpoint := "/cards"
	u, _ := url.Parse(trelloAPIBase + endpoint)
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("token", token)
	q.Set("idList", listID)
	q.Set("name", name)

	if desc, ok := args["desc"].(string); ok && desc != "" {
		q.Set("desc", desc)
	}
	if due, ok := args["due"].(string); ok && due != "" {
		q.Set("due", due)
	}
	if labels, ok := args["labels"].(string); ok && labels != "" {
		q.Set("idLabels", labels)
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Trello API error: %s", string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Card created successfully",
		"card_id": result["id"],
		"url":     result["url"],
		"name":    result["name"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

