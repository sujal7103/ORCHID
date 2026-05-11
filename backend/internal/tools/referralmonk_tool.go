package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NewReferralMonkWhatsAppTool creates a ReferralMonk WhatsApp tool
func NewReferralMonkWhatsAppTool() *Tool {
	return &Tool{
		Name:        "referralmonk_whatsapp",
		DisplayName: "ReferralMonk WhatsApp",
		Description: `Send WhatsApp messages via ReferralMonk with template support.

Features:
- Send templated WhatsApp messages for campaigns
- Support for template parameters (up to 3 parameters)
- International number support with country code
- External message ID tracking for analytics

Use this for marketing campaigns, nurture flows, and templated notifications.
Numbers must include country code (e.g., 917550002919 for India).`,
		Icon:     "MessageSquare",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"referralmonk", "whatsapp", "template", "campaign", "message", "marketing", "ahaguru"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"mobile": map[string]interface{}{
					"type":        "string",
					"description": "Mobile number with country code (e.g., 917550002919 for +91 7550002919)",
				},
				"template_name": map[string]interface{}{
					"type":        "string",
					"description": "WhatsApp template name/ID (e.g., demo_session_01)",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Language code for template (default: en)",
					"default":     "en",
				},
				"param_1": map[string]interface{}{
					"type":        "string",
					"description": "First template parameter (e.g., user name)",
				},
				"param_2": map[string]interface{}{
					"type":        "string",
					"description": "Second template parameter (e.g., link or URL)",
				},
				"param_3": map[string]interface{}{
					"type":        "string",
					"description": "Third template parameter (e.g., signature or team name)",
				},
				"external_message_id": map[string]interface{}{
					"type":        "string",
					"description": "External message ID for tracking (auto-generated if not provided)",
				},
			},
			"required": []string{"mobile", "template_name"},
		},
		Execute: executeReferralMonkWhatsApp,
	}
}

func executeReferralMonkWhatsApp(args map[string]interface{}) (string, error) {
	// Get ReferralMonk credentials
	credData, err := GetCredentialData(args, "referralmonk")
	if err != nil {
		return "", fmt.Errorf("failed to get ReferralMonk credentials: %w", err)
	}

	apiToken, _ := credData["api_token"].(string)
	apiSecret, _ := credData["api_secret"].(string)

	if apiToken == "" || apiSecret == "" {
		return "", fmt.Errorf("ReferralMonk credentials incomplete: api_token and api_secret are required")
	}

	// Extract parameters
	mobile, _ := args["mobile"].(string)
	templateName, _ := args["template_name"].(string)
	language, _ := args["language"].(string)
	param1, _ := args["param_1"].(string)
	param2, _ := args["param_2"].(string)
	param3, _ := args["param_3"].(string)
	externalMsgID, _ := args["external_message_id"].(string)

	// Validate required fields
	if mobile == "" {
		return "", fmt.Errorf("'mobile' number is required")
	}
	if templateName == "" {
		return "", fmt.Errorf("'template_name' is required")
	}

	// Set defaults
	if language == "" {
		language = "en"
	}
	if externalMsgID == "" {
		externalMsgID = fmt.Sprintf("msg_%d", time.Now().Unix())
	}

	// Build template parameters array - each param is a separate object
	var parameters []map[string]interface{}
	if param1 != "" {
		parameters = append(parameters, map[string]interface{}{
			"type": "text",
			"text": param1,
		})
	}
	if param2 != "" {
		parameters = append(parameters, map[string]interface{}{
			"type": "text",
			"text": param2,
		})
	}
	if param3 != "" {
		parameters = append(parameters, map[string]interface{}{
			"type": "text",
			"text": param3,
		})
	}

	// Build ReferralMonk API payload
	payload := map[string]interface{}{
		"template_name": templateName,
		"channel":       "whatsapp",
		"recipients": []map[string]interface{}{
			{
				"mobile":            mobile,
				"language":          language,
				"externalMessageId": externalMsgID,
				"components": []map[string]interface{}{
					{
						"type":       "body",
						"parameters": parameters,
					},
				},
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API request to ReferralMonk
	apiURL := "https://ahaguru.referralmonk.com/api/campaign"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Token", apiToken)
	req.Header.Set("Api-Secret", apiSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, _ := io.ReadAll(resp.Body)

	// Check for errors
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ReferralMonk API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var apiResponse map[string]interface{}
	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		// If JSON parse fails, return raw response
		apiResponse = map[string]interface{}{
			"raw_response": string(respBody),
		}
	}

	// Build output
	output := map[string]interface{}{
		"success":             true,
		"mobile":              mobile,
		"template_name":       templateName,
		"external_message_id": externalMsgID,
		"language":            language,
		"status_code":         resp.StatusCode,
		"response":            apiResponse,
	}

	jsonResult, _ := json.MarshalIndent(output, "", "  ")
	return string(jsonResult), nil
}
