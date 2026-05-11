package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"clara-agents/internal/e2b"
	"clara-agents/internal/security"
)

// NewAPITesterTool creates a new API Tester tool
func NewAPITesterTool() *Tool {
	return &Tool{
		Name:        "test_api",
		DisplayName: "API Tester",
		Description: "Test REST API endpoints with various HTTP methods (GET, POST, PUT, DELETE, PATCH). Sends requests, validates responses, measures response times, and displays status codes, headers, and response bodies. Useful for API testing, debugging, and validation.",
		Icon:        "Network",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"api", "test", "http", "rest", "endpoint", "request", "response", "get", "post", "put", "delete", "patch", "web service", "debugging"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "API endpoint URL (must include http:// or https://)",
					"pattern":     "^https?://.*$",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP method to use",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "GET",
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP headers to include in the request (optional)",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Request body (JSON string, optional)",
				},
				"expected_status": map[string]interface{}{
					"type":        "number",
					"description": "Expected HTTP status code for validation (optional)",
					"minimum":     100,
					"maximum":     599,
				},
			},
			"required": []string{"url"},
		},
		Execute: executeAPITester,
	}
}

func executeAPITester(args map[string]interface{}) (string, error) {
	// Extract parameters
	url, ok := args["url"].(string)
	if !ok {
		return "", fmt.Errorf("url must be a string")
	}

	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("url must start with http:// or https://")
	}

	// SSRF protection: block requests to internal/private networks
	if err := security.ValidateURLForSSRF(url); err != nil {
		return "", fmt.Errorf("SSRF protection: %w", err)
	}

	method := "GET"
	if m, ok := args["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	headers := make(map[string]string)
	if headersRaw, ok := args["headers"].(map[string]interface{}); ok {
		for key, value := range headersRaw {
			headers[key] = fmt.Sprintf("%v", value)
		}
	}

	body := ""
	if b, ok := args["body"].(string); ok {
		body = b
	}

	expectedStatus := 0
	if es, ok := args["expected_status"].(float64); ok {
		expectedStatus = int(es)
	}

	// Generate Python code
	pythonCode := generateAPITestCode(url, method, headers, body, expectedStatus)

	// Execute code
	e2bService := e2b.GetE2BExecutorService()
	result, err := e2bService.Execute(context.Background(), pythonCode, 30)
	if err != nil {
		return "", fmt.Errorf("failed to execute API test: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return "", fmt.Errorf("API test failed: %s", *result.Error)
		}
		return "", fmt.Errorf("API test failed with stderr: %s", result.Stderr)
	}

	// Format response
	response := map[string]interface{}{
		"success": true,
		"url":     url,
		"method":  method,
		"output":  result.Stdout,
	}

	jsonResponse, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResponse), nil
}

func generateAPITestCode(url, method string, headers map[string]string, body string, expectedStatus int) string {
	// Build headers dict
	headersStr := ""
	if len(headers) > 0 {
		headerParts := []string{}
		for key, value := range headers {
			// Escape single quotes in values
			escapedValue := strings.ReplaceAll(value, "'", "\\'")
			headerParts = append(headerParts, fmt.Sprintf("    '%s': '%s'", key, escapedValue))
		}
		headersStr = fmt.Sprintf("{\n%s\n}", strings.Join(headerParts, ",\n"))
	} else {
		headersStr = "{}"
	}

	// Escape body for Python string
	escapedBody := strings.ReplaceAll(body, "'", "\\'")
	escapedBody = strings.ReplaceAll(escapedBody, "\n", "\\n")

	code := fmt.Sprintf(`import requests
import json
import time

print("=" * 80)
print("🔌 API TESTER")
print("=" * 80)

# Request configuration
url = '%s'
method = '%s'
headers = %s
`, url, method, headersStr)

	if body != "" {
		code += fmt.Sprintf(`
body = '''%s'''
`, escapedBody)
	} else {
		code += `
body = None
`
	}

	code += fmt.Sprintf(`
print(f"\n📡 Testing API Endpoint")
print("-" * 80)
print(f"URL: {url}")
print(f"Method: {method}")

if headers:
    print(f"\n📋 Headers:")
    for key, value in headers.items():
        print(f"  {key}: {value}")

if body:
    print(f"\n📦 Request Body:")
    try:
        # Try to pretty-print if it's JSON
        body_dict = json.loads(body)
        print(json.dumps(body_dict, indent=2))
    except:
        print(body[:500])  # Print first 500 chars if not JSON

print(f"\n⏳ Sending request...")

# Send request
start_time = time.time()

try:
    if method == 'GET':
        response = requests.get(url, headers=headers, timeout=10)
    elif method == 'POST':
        response = requests.post(url, headers=headers, data=body, timeout=10)
    elif method == 'PUT':
        response = requests.put(url, headers=headers, data=body, timeout=10)
    elif method == 'DELETE':
        response = requests.delete(url, headers=headers, timeout=10)
    elif method == 'PATCH':
        response = requests.patch(url, headers=headers, data=body, timeout=10)
    else:
        raise ValueError(f"Unsupported method: {method}")

    elapsed_time = time.time() - start_time

    print(f"\n✅ Request completed in {elapsed_time:.3f}s")

    # Response details
    print(f"\n📊 RESPONSE")
    print("-" * 80)
    print(f"Status Code: {response.status_code} {response.reason}")
`)

	if expectedStatus > 0 {
		code += fmt.Sprintf(`
    # Validate status code
    if response.status_code == %d:
        print(f"✅ Status code matches expected: %d")
    else:
        print(f"❌ Status code mismatch! Expected: %d, Got: {response.status_code}")
`, expectedStatus, expectedStatus, expectedStatus)
	}

	code += `
    print(f"Response Time: {elapsed_time:.3f}s")
    print(f"Content Length: {len(response.content)} bytes")

    # Response headers
    print(f"\n📋 Response Headers:")
    for key, value in response.headers.items():
        print(f"  {key}: {value}")

    # Response body
    print(f"\n📦 Response Body:")
    print("-" * 80)

    content_type = response.headers.get('Content-Type', '')

    if 'application/json' in content_type:
        try:
            json_data = response.json()
            print(json.dumps(json_data, indent=2))
        except:
            print(response.text[:2000])
    else:
        print(response.text[:2000])

    if len(response.text) > 2000:
        print(f"\n... (Total length: {len(response.text)} characters)")

    # Status code interpretation
    print(f"\n💡 Status Code Interpretation:")
    if 200 <= response.status_code < 300:
        print(f"  ✅ Success - Request completed successfully")
    elif 300 <= response.status_code < 400:
        print(f"  🔄 Redirect - Resource moved to another location")
    elif 400 <= response.status_code < 500:
        print(f"  ❌ Client Error - Problem with the request")
    elif 500 <= response.status_code < 600:
        print(f"  🚨 Server Error - Problem on the server side")

except requests.exceptions.Timeout:
    print(f"\n⏱️  Request timed out after 10 seconds")
    print(f"💡 Tip: The server is taking too long to respond")

except requests.exceptions.ConnectionError:
    print(f"\n🔌 Connection failed")
    print(f"💡 Tip: Check if the URL is correct and the server is accessible")

except requests.exceptions.RequestException as e:
    print(f"\n❌ Request failed: {e}")

except Exception as e:
    print(f"\n❌ Unexpected error: {e}")

print("\n" + "=" * 80)
print("✅ API TEST COMPLETE")
print("=" * 80)
`

	return code
}
