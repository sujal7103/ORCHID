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

const (
	notionAPIBase    = "https://api.notion.com/v1"
	notionAPIVersion = "2022-06-28"
)

// NewNotionSearchTool creates a tool for searching Notion workspace
func NewNotionSearchTool() *Tool {
	return &Tool{
		Name:        "notion_search",
		DisplayName: "Search Notion",
		Description: "Search across all pages and databases in a Notion workspace. Use this to find existing content by title or text. Authentication is handled automatically via configured credentials.",
		Icon:        "Search",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"notion", "search", "find", "query", "workspace", "pages", "databases"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query text. Searches titles and content.",
				},
				"filter_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"page", "database"},
					"description": "Filter results by type: 'page' for pages only, 'database' for databases only. Leave empty for all.",
				},
				"page_size": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results to return (1-100, default 10)",
				},
			},
			"required": []string{"query"},
		},
		Execute: executeNotionSearch,
	}
}

// NewNotionQueryDatabaseTool creates a tool for querying Notion databases
func NewNotionQueryDatabaseTool() *Tool {
	return &Tool{
		Name:        "notion_query_database",
		DisplayName: "Query Notion Database",
		Description: "Query and filter entries from a Notion database. Use this to get tasks, contacts, content items, or any structured data. Supports filtering by properties and sorting. Authentication is handled automatically.",
		Icon:        "Database",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"notion", "database", "query", "filter", "tasks", "entries", "records", "table"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"database_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the Notion database to query. Found in the database URL after the workspace name.",
				},
				"filter": map[string]interface{}{
					"type":        "object",
					"description": "Optional filter object. Example: {\"property\": \"Status\", \"select\": {\"equals\": \"Done\"}}",
				},
				"sorts": map[string]interface{}{
					"type":        "array",
					"description": "Optional sort array. Example: [{\"property\": \"Created\", \"direction\": \"descending\"}]",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
				"page_size": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results (1-100, default 10)",
				},
			},
			"required": []string{"database_id"},
		},
		Execute: executeNotionQueryDatabase,
	}
}

// NewNotionCreatePageTool creates a tool for creating pages in Notion
func NewNotionCreatePageTool() *Tool {
	return &Tool{
		Name:        "notion_create_page",
		DisplayName: "Create Notion Page",
		Description: "Create a new page or database entry in Notion. Can create standalone pages or add entries to databases (like tasks, contacts, etc.). Authentication is handled automatically.",
		Icon:        "FilePlus",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"notion", "create", "page", "add", "new", "task", "entry", "record"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"parent_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"database_id", "page_id"},
					"description": "Type of parent: 'database_id' to add entry to a database, 'page_id' to create subpage",
				},
				"parent_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the parent database or page",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title of the page (for standalone pages or the Name/Title property of database entries)",
				},
				"properties": map[string]interface{}{
					"type":        "object",
					"description": "Properties for database entries. Example: {\"Status\": {\"select\": {\"name\": \"In Progress\"}}, \"Priority\": {\"select\": {\"name\": \"High\"}}}",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Optional text content to add to the page body (will be added as paragraph blocks)",
				},
				"icon_emoji": map[string]interface{}{
					"type":        "string",
					"description": "Optional emoji icon for the page (e.g., '📝', '✅', '🎯')",
				},
			},
			"required": []string{"parent_type", "parent_id"},
		},
		Execute: executeNotionCreatePage,
	}
}

// NewNotionUpdatePageTool creates a tool for updating Notion pages
func NewNotionUpdatePageTool() *Tool {
	return &Tool{
		Name:        "notion_update_page",
		DisplayName: "Update Notion Page",
		Description: "Update properties of an existing Notion page or database entry. Use this to change status, update fields, or modify page metadata. Authentication is handled automatically.",
		Icon:        "Edit",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"notion", "update", "edit", "modify", "change", "status", "properties"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"page_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the page to update",
				},
				"properties": map[string]interface{}{
					"type":        "object",
					"description": "Properties to update. Example: {\"Status\": {\"select\": {\"name\": \"Done\"}}, \"Completed\": {\"checkbox\": true}}",
				},
				"icon_emoji": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Update the page icon emoji",
				},
			},
			"required": []string{"page_id"},
		},
		Execute: executeNotionUpdatePage,
	}
}

// Helper function to make Notion API requests
func notionRequest(method, endpoint, apiKey string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, notionAPIBase+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Notion-Version", notionAPIVersion)
	req.Header.Set("Content-Type", "application/json")

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

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		errMsg := "Notion API error"
		if msg, ok := result["message"].(string); ok {
			errMsg = msg
		}
		return nil, fmt.Errorf("%s (status %d)", errMsg, resp.StatusCode)
	}

	return result, nil
}

// Extract simplified page data from Notion response
func simplifyNotionPage(page map[string]interface{}) map[string]interface{} {
	simplified := map[string]interface{}{
		"id":  page["id"],
		"url": page["url"],
	}

	// Extract title from properties
	if props, ok := page["properties"].(map[string]interface{}); ok {
		simplifiedProps := make(map[string]interface{})
		for name, prop := range props {
			propMap, ok := prop.(map[string]interface{})
			if !ok {
				continue
			}
			propType, _ := propMap["type"].(string)

			switch propType {
			case "title":
				if titleArr, ok := propMap["title"].([]interface{}); ok && len(titleArr) > 0 {
					if titleObj, ok := titleArr[0].(map[string]interface{}); ok {
						if plainText, ok := titleObj["plain_text"].(string); ok {
							simplifiedProps[name] = plainText
							if name == "Name" || name == "Title" {
								simplified["title"] = plainText
							}
						}
					}
				}
			case "rich_text":
				if textArr, ok := propMap["rich_text"].([]interface{}); ok && len(textArr) > 0 {
					if textObj, ok := textArr[0].(map[string]interface{}); ok {
						simplifiedProps[name] = textObj["plain_text"]
					}
				}
			case "select":
				if sel, ok := propMap["select"].(map[string]interface{}); ok {
					simplifiedProps[name] = sel["name"]
				}
			case "multi_select":
				if multiSel, ok := propMap["multi_select"].([]interface{}); ok {
					names := make([]string, 0)
					for _, s := range multiSel {
						if selObj, ok := s.(map[string]interface{}); ok {
							if n, ok := selObj["name"].(string); ok {
								names = append(names, n)
							}
						}
					}
					simplifiedProps[name] = names
				}
			case "checkbox":
				simplifiedProps[name] = propMap["checkbox"]
			case "number":
				simplifiedProps[name] = propMap["number"]
			case "date":
				if dateObj, ok := propMap["date"].(map[string]interface{}); ok {
					simplifiedProps[name] = dateObj["start"]
				}
			case "status":
				if status, ok := propMap["status"].(map[string]interface{}); ok {
					simplifiedProps[name] = status["name"]
				}
			case "url":
				simplifiedProps[name] = propMap["url"]
			case "email":
				simplifiedProps[name] = propMap["email"]
			case "phone_number":
				simplifiedProps[name] = propMap["phone_number"]
			}
		}
		simplified["properties"] = simplifiedProps
	}

	// Extract icon
	if icon, ok := page["icon"].(map[string]interface{}); ok {
		if emoji, ok := icon["emoji"].(string); ok {
			simplified["icon"] = emoji
		}
	}

	// Extract created/updated times
	if created, ok := page["created_time"].(string); ok {
		simplified["created_time"] = created
	}
	if updated, ok := page["last_edited_time"].(string); ok {
		simplified["last_edited_time"] = updated
	}

	return simplified
}

func executeNotionSearch(args map[string]interface{}) (string, error) {
	// Resolve API key
	apiKey, err := ResolveAPIKey(args, "notion", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get Notion API key: %w", err)
	}

	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Build request body
	body := map[string]interface{}{
		"query": query,
	}

	// Add filter if specified
	if filterType, ok := args["filter_type"].(string); ok && filterType != "" {
		body["filter"] = map[string]interface{}{
			"value":    filterType,
			"property": "object",
		}
	}

	// Page size
	pageSize := 10
	if ps, ok := args["page_size"].(float64); ok && ps > 0 {
		pageSize = int(ps)
		if pageSize > 100 {
			pageSize = 100
		}
	}
	body["page_size"] = pageSize

	// Make API request
	result, err := notionRequest("POST", "/search", apiKey, body)
	if err != nil {
		return "", err
	}

	// Simplify results
	simplified := map[string]interface{}{
		"query":        query,
		"total_found":  0,
		"results":      []interface{}{},
	}

	if results, ok := result["results"].([]interface{}); ok {
		simplified["total_found"] = len(results)
		simplifiedResults := make([]interface{}, 0, len(results))
		for _, r := range results {
			if page, ok := r.(map[string]interface{}); ok {
				simplifiedResults = append(simplifiedResults, simplifyNotionPage(page))
			}
		}
		simplified["results"] = simplifiedResults
	}

	jsonResult, _ := json.MarshalIndent(simplified, "", "  ")
	return string(jsonResult), nil
}

func executeNotionQueryDatabase(args map[string]interface{}) (string, error) {
	// Resolve API key
	apiKey, err := ResolveAPIKey(args, "notion", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get Notion API key: %w", err)
	}

	databaseID, _ := args["database_id"].(string)
	if databaseID == "" {
		return "", fmt.Errorf("database_id is required")
	}

	// Clean up database ID (remove hyphens if present in URL format)
	databaseID = strings.ReplaceAll(databaseID, "-", "")

	// Build request body
	body := map[string]interface{}{}

	// Add filter if specified
	if filter, ok := args["filter"].(map[string]interface{}); ok {
		body["filter"] = filter
	}

	// Add sorts if specified
	if sorts, ok := args["sorts"].([]interface{}); ok {
		body["sorts"] = sorts
	}

	// Page size
	pageSize := 10
	if ps, ok := args["page_size"].(float64); ok && ps > 0 {
		pageSize = int(ps)
		if pageSize > 100 {
			pageSize = 100
		}
	}
	body["page_size"] = pageSize

	// Make API request
	result, err := notionRequest("POST", "/databases/"+databaseID+"/query", apiKey, body)
	if err != nil {
		return "", err
	}

	// Simplify results
	simplified := map[string]interface{}{
		"database_id": databaseID,
		"total_found": 0,
		"entries":     []interface{}{},
	}

	if results, ok := result["results"].([]interface{}); ok {
		simplified["total_found"] = len(results)
		entries := make([]interface{}, 0, len(results))
		for _, r := range results {
			if page, ok := r.(map[string]interface{}); ok {
				entries = append(entries, simplifyNotionPage(page))
			}
		}
		simplified["entries"] = entries
	}

	jsonResult, _ := json.MarshalIndent(simplified, "", "  ")
	return string(jsonResult), nil
}

func executeNotionCreatePage(args map[string]interface{}) (string, error) {
	// Resolve API key
	apiKey, err := ResolveAPIKey(args, "notion", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get Notion API key: %w", err)
	}

	parentType, _ := args["parent_type"].(string)
	parentID, _ := args["parent_id"].(string)

	if parentType == "" || parentID == "" {
		return "", fmt.Errorf("parent_type and parent_id are required")
	}

	// Clean up parent ID
	parentID = strings.ReplaceAll(parentID, "-", "")

	// Build request body
	body := map[string]interface{}{
		"parent": map[string]interface{}{
			parentType: parentID,
		},
	}

	// Build properties
	properties := map[string]interface{}{}

	// Add title if provided
	if title, ok := args["title"].(string); ok && title != "" {
		// For database entries, use "Name" or check for title property
		// For pages, use "title" type
		if parentType == "database_id" {
			properties["Name"] = map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]interface{}{"content": title}},
				},
			}
		} else {
			properties["title"] = map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]interface{}{"content": title}},
				},
			}
		}
	}

	// Merge additional properties
	if additionalProps, ok := args["properties"].(map[string]interface{}); ok {
		for k, v := range additionalProps {
			properties[k] = v
		}
	}

	if len(properties) > 0 {
		body["properties"] = properties
	}

	// Add icon if provided
	if iconEmoji, ok := args["icon_emoji"].(string); ok && iconEmoji != "" {
		body["icon"] = map[string]interface{}{
			"type":  "emoji",
			"emoji": iconEmoji,
		}
	}

	// Add content as children blocks
	if content, ok := args["content"].(string); ok && content != "" {
		// Split content into paragraphs
		paragraphs := strings.Split(content, "\n\n")
		children := make([]map[string]interface{}, 0, len(paragraphs))

		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			children = append(children, map[string]interface{}{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"type": "text", "text": map[string]interface{}{"content": para}},
					},
				},
			})
		}

		if len(children) > 0 {
			body["children"] = children
		}
	}

	// Make API request
	result, err := notionRequest("POST", "/pages", apiKey, body)
	if err != nil {
		return "", err
	}

	// Build response
	response := map[string]interface{}{
		"success":    true,
		"message":    "Page created successfully",
		"page_id":    result["id"],
		"url":        result["url"],
		"created_at": result["created_time"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeNotionUpdatePage(args map[string]interface{}) (string, error) {
	// Resolve API key
	apiKey, err := ResolveAPIKey(args, "notion", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get Notion API key: %w", err)
	}

	pageID, _ := args["page_id"].(string)
	if pageID == "" {
		return "", fmt.Errorf("page_id is required")
	}

	// Clean up page ID
	pageID = strings.ReplaceAll(pageID, "-", "")

	// Build request body
	body := map[string]interface{}{}

	// Add properties if specified
	if properties, ok := args["properties"].(map[string]interface{}); ok && len(properties) > 0 {
		body["properties"] = properties
	}

	// Add icon if specified
	if iconEmoji, ok := args["icon_emoji"].(string); ok && iconEmoji != "" {
		body["icon"] = map[string]interface{}{
			"type":  "emoji",
			"emoji": iconEmoji,
		}
	}

	if len(body) == 0 {
		return "", fmt.Errorf("at least one property to update is required")
	}

	// Make API request
	result, err := notionRequest("PATCH", "/pages/"+pageID, apiKey, body)
	if err != nil {
		return "", err
	}

	// Build response
	response := map[string]interface{}{
		"success":     true,
		"message":     "Page updated successfully",
		"page_id":     result["id"],
		"url":         result["url"],
		"updated_at":  result["last_edited_time"],
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}
