package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// formatUnipileError returns a user-friendly error message from a Unipile API error response.
func formatUnipileError(statusCode int, body []byte) error {
	var errResp map[string]interface{}
	if err := json.Unmarshal(body, &errResp); err == nil {
		if msg, ok := errResp["message"].(string); ok {
			return fmt.Errorf("Unipile API error (%d): %s", statusCode, msg)
		}
		if msg, ok := errResp["error"].(string); ok {
			return fmt.Errorf("Unipile API error (%d): %s", statusCode, msg)
		}
	}

	switch statusCode {
	case 401:
		return fmt.Errorf("Unipile authentication failed: check your DSN and access token")
	case 403:
		return fmt.Errorf("Unipile access forbidden: your account may not have permission for this action")
	case 404:
		return fmt.Errorf("Unipile resource not found: check that the ID or account exists")
	case 429:
		return fmt.Errorf("Unipile rate limit exceeded: please wait before retrying")
	default:
		return fmt.Errorf("Unipile API error (%d): %s", statusCode, string(body))
	}
}

// unipileListParams builds common query parameters for Unipile list endpoints.
func unipileListParams(args map[string]interface{}, defaultLimit int) map[string]string {
	params := make(map[string]string)

	limit := defaultLimit
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	params["limit"] = strconv.Itoa(limit)

	if cursor, ok := args["cursor"].(string); ok && cursor != "" {
		params["cursor"] = cursor
	}

	return params
}
