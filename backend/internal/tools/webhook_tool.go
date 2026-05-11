package tools

import (
	"bytes"
	"clara-agents/internal/security"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewWebhookTool creates a new generic webhook/HTTP tool
func NewWebhookTool() *Tool {
	return &Tool{
		Name:        "send_webhook",
		DisplayName: "Send Webhook",
		Description: "Send HTTP requests to any URL. Use for APIs, webhooks, notifications, and integrations. Supports GET, POST, PUT, DELETE methods with custom headers and body.",
		Icon:        "Send",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"webhook", "http", "api", "request", "post", "get", "send", "notify", "integration", "rest"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of a stored webhook/REST API credential (preferred over raw url)",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Target URL - only use if no credential_id is provided (must include http:// or https://)",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP method to use",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "POST",
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP headers to include (optional). Example: {\"Authorization\": \"Bearer token\"}",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Request body (typically JSON string for POST/PUT requests)",
				},
				"content_type": map[string]interface{}{
					"type":        "string",
					"description": "Content-Type header value",
					"default":     "application/json",
				},
			},
			"required": []string{},
		},
		Execute: executeWebhook,
	}
}

func executeWebhook(args map[string]interface{}) (string, error) {
	// Resolve URL from credential or direct parameter
	// Try multiple integration types that use webhooks/URLs
	url, err := ResolveWebhookURL(args, "custom_webhook")
	if err != nil {
		// Try rest_api type
		url, err = ResolveWebhookURL(args, "rest_api")
		if err != nil {
			// Fallback: check for direct url if credential resolution failed
			if u, ok := args["url"].(string); ok && u != "" {
				url = u
			} else {
				return "", fmt.Errorf("either url or credential_id is required")
			}
		}
	}

	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("url must start with http:// or https://")
	}

	// SSRF protection: block requests to internal/private networks
	if err := security.ValidateURLForSSRF(url); err != nil {
		return "", fmt.Errorf("SSRF protection: %w", err)
	}

	// Extract method (default: POST)
	method := "POST"
	if m, ok := args["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Extract body
	body := ""
	if b, ok := args["body"].(string); ok {
		body = b
	}

	// Extract content type (default: application/json)
	contentType := "application/json"
	if ct, ok := args["content_type"].(string); ok && ct != "" {
		contentType = ct
	}

	// Extract headers
	headers := make(map[string]string)
	if headersRaw, ok := args["headers"].(map[string]interface{}); ok {
		for key, value := range headersRaw {
			headers[key] = fmt.Sprintf("%v", value)
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	var req *http.Request

	if body != "" {
		req, err = http.NewRequest(method, url, bytes.NewBufferString(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type for requests with body
	if body != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

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

	// Truncate response if too long
	respBodyStr := string(respBody)
	if len(respBodyStr) > 5000 {
		respBodyStr = respBodyStr[:5000] + "... (truncated)"
	}

	// Build result
	result := map[string]interface{}{
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"url":         url,
		"method":      method,
		"response":    respBodyStr,
	}

	// Add response headers
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}
	result["response_headers"] = respHeaders

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
