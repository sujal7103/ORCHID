package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewRESTAPITool creates a generic REST API tool
func NewRESTAPITool() *Tool {
	return &Tool{
		Name:        "api_request",
		DisplayName: "REST API Request",
		Description: "Make HTTP requests to any REST API endpoint. Supports various authentication methods. Authentication is configured via credentials.",
		Icon:        "Globe",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"api", "rest", "http", "request", "get", "post", "put", "delete"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
					"description": "HTTP method (default: GET)",
				},
				"endpoint": map[string]interface{}{
					"type":        "string",
					"description": "API endpoint path (appended to base URL from credentials)",
				},
				"body": map[string]interface{}{
					"type":        "object",
					"description": "Request body for POST/PUT/PATCH requests",
				},
				"query_params": map[string]interface{}{
					"type":        "object",
					"description": "Query parameters as key-value pairs",
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "Additional headers as key-value pairs",
				},
			},
			"required": []string{"endpoint"},
		},
		Execute: executeRESTAPI,
	}
}

func executeRESTAPI(args map[string]interface{}) (string, error) {
	// Get credential data
	credData, err := GetCredentialData(args, "rest_api")
	if err != nil {
		return "", fmt.Errorf("failed to get REST API credentials: %w", err)
	}

	baseURL, _ := credData["base_url"].(string)
	if baseURL == "" {
		return "", fmt.Errorf("base_url is required in credentials")
	}

	authType, _ := credData["auth_type"].(string)
	authValue, _ := credData["auth_value"].(string)
	authHeaderName, _ := credData["auth_header_name"].(string)
	defaultHeaders, _ := credData["headers"].(string)

	// Get request parameters
	method := "GET"
	if m, ok := args["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	endpoint, _ := args["endpoint"].(string)
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is required")
	}

	// Build URL
	url := strings.TrimSuffix(baseURL, "/")
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	url += endpoint

	// Add query parameters
	if queryParams, ok := args["query_params"].(map[string]interface{}); ok && len(queryParams) > 0 {
		params := []string{}
		for k, v := range queryParams {
			params = append(params, fmt.Sprintf("%s=%v", k, v))
		}
		if strings.Contains(url, "?") {
			url += "&" + strings.Join(params, "&")
		} else {
			url += "?" + strings.Join(params, "&")
		}
	}

	// Build request body
	var reqBody io.Reader
	if body, ok := args["body"].(map[string]interface{}); ok && len(body) > 0 {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Create request
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type for requests with body
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Parse and apply default headers from credentials
	if defaultHeaders != "" {
		var defHeaders map[string]string
		if err := json.Unmarshal([]byte(defaultHeaders), &defHeaders); err == nil {
			for k, v := range defHeaders {
				req.Header.Set(k, v)
			}
		}
	}

	// Apply additional headers from request
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				req.Header.Set(k, strVal)
			}
		}
	}

	// Apply authentication
	switch authType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+authValue)
	case "basic":
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(authValue)))
	case "api_key_header":
		if authHeaderName == "" {
			authHeaderName = "X-API-Key"
		}
		req.Header.Set(authHeaderName, authValue)
	case "api_key_query":
		if strings.Contains(req.URL.RawQuery, "?") {
			req.URL.RawQuery += "&api_key=" + authValue
		} else {
			req.URL.RawQuery = "api_key=" + authValue
		}
	}

	// Execute request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Try to parse response as JSON
	var responseData interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		// Not JSON, use as string
		responseData = string(respBody)
	}

	// Build result
	result := map[string]interface{}{
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"data":        responseData,
	}

	// Include response headers
	respHeaders := map[string]string{}
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}
	result["headers"] = respHeaders

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

