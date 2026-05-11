package execution

import (
	"clara-agents/internal/models"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPRequestExecutor executes HTTP request blocks — universal REST API calls without LLM
type HTTPRequestExecutor struct {
	client *http.Client
}

func NewHTTPRequestExecutor() *HTTPRequestExecutor {
	return &HTTPRequestExecutor{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *HTTPRequestExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	method := strings.ToUpper(getString(config, "method", "GET"))
	rawURL := getString(config, "url", "")
	if rawURL == "" {
		return nil, fmt.Errorf("url is required for http_request block")
	}

	// Interpolate templates in URL
	reqURL := InterpolateTemplate(rawURL, inputs)

	// Merge queryParams into URL (backward compat for old workflows)
	if qp := getMap(config, "queryParams"); qp != nil {
		if parsedURL, parseErr := url.Parse(reqURL); parseErr == nil {
			q := parsedURL.Query()
			for k, v := range qp {
				if strVal, ok := v.(string); ok {
					q.Set(k, InterpolateTemplate(strVal, inputs))
				}
			}
			parsedURL.RawQuery = q.Encode()
			reqURL = parsedURL.String()
		}
	}

	// Build body
	var bodyReader io.Reader
	if bodyRaw, ok := config["body"]; ok && bodyRaw != nil {
		switch b := bodyRaw.(type) {
		case string:
			if b != "" {
				interpolated := InterpolateTemplate(b, inputs)
				bodyReader = strings.NewReader(interpolated)
			}
		case map[string]any:
			interpolated := InterpolateMapValues(b, inputs)
			jsonBody, err := json.Marshal(interpolated)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
			bodyReader = strings.NewReader(string(jsonBody))
		}
	}

	log.Printf("🌐 [HTTP-REQ] Block '%s': %s %s", block.Name, method, reqURL)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if headers := getMap(config, "headers"); headers != nil {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, InterpolateTemplate(strVal, inputs))
			}
		}
	}

	// Default content type for requests with body
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply authentication
	authType := getString(config, "authType", "none")
	authConfig := getMap(config, "authConfig")
	if authConfig != nil {
		switch authType {
		case "bearer":
			if token, ok := authConfig["token"].(string); ok && token != "" {
				req.Header.Set("Authorization", "Bearer "+InterpolateTemplate(token, inputs))
			}
		case "basic":
			username, _ := authConfig["username"].(string)
			password, _ := authConfig["password"].(string)
			if username != "" {
				encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				req.Header.Set("Authorization", "Basic "+encoded)
			}
		case "api_key":
			key, _ := authConfig["key"].(string)
			headerName := getString(authConfig, "headerName", "X-API-Key")
			if key != "" {
				req.Header.Set(headerName, InterpolateTemplate(key, inputs))
			}
		}
	}

	// Execute
	resp, err := e.client.Do(req)
	if err != nil {
		// Classify network/timeout errors for retry decisions
		return nil, ClassifyError(err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("🌐 [HTTP-REQ] Block '%s': status=%d, body_len=%d", block.Name, resp.StatusCode, len(responseBody))

	// Parse response
	var parsedBody any
	if err := json.Unmarshal(responseBody, &parsedBody); err != nil {
		parsedBody = string(responseBody)
	}

	// Collect response headers
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	result := map[string]any{
		"response": parsedBody,
		"data":     parsedBody,
		"status":   resp.StatusCode,
		"headers":  respHeaders,
		"raw":      string(responseBody),
	}

	// For non-2xx responses, return a classified error (enables smart retries)
	// but still include the response data so downstream blocks can inspect it
	if resp.StatusCode >= 400 {
		classifiedErr := ClassifyHTTPError(resp.StatusCode, string(responseBody))
		// Parse Retry-After header for 429 responses
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, parseErr := fmt.Sscanf(retryAfter, "%d", &classifiedErr.RetryAfter); parseErr != nil || seconds == 0 {
				classifiedErr.RetryAfter = 60 // Default if Retry-After is not a number
			}
		}
		log.Printf("⚠️ [HTTP-REQ] Block '%s': HTTP %d — %s [retryable=%v]",
			block.Name, resp.StatusCode, classifiedErr.Category.String(), classifiedErr.Retryable)
		return result, classifiedErr
	}

	return result, nil
}
