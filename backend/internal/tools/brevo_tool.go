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

// NewBrevoTool creates a Brevo (formerly SendInBlue) email sending tool
func NewBrevoTool() *Tool {
	return &Tool{
		Name:        "send_brevo_email",
		DisplayName: "Send Email (Brevo)",
		Description: `Send emails via Brevo (formerly SendInBlue) API. Supports transactional emails, marketing campaigns, and templates.

Features:
- Send to single or multiple recipients (to, cc, bcc)
- HTML and plain text email bodies
- Custom sender name and reply-to address
- Template support with dynamic parameters
- Attachment support via URL

Authentication is handled automatically via configured Brevo credentials. Do NOT ask users for API keys.
The sender email (from_email) can be configured in credentials as default, or overridden per-request.`,
		Icon:     "Mail",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"brevo", "sendinblue", "email", "send", "mail", "message", "notification", "newsletter", "transactional", "marketing"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Recipient email address(es). For multiple recipients, separate with commas (e.g., 'user1@example.com, user2@example.com')",
				},
				"from_email": map[string]interface{}{
					"type":        "string",
					"description": "Sender email address (optional). Must be a verified sender in Brevo. If not provided, uses the default from_email configured in credentials.",
				},
				"from_name": map[string]interface{}{
					"type":        "string",
					"description": "Sender display name (optional, e.g., 'John Doe' or 'My Company')",
				},
				"subject": map[string]interface{}{
					"type":        "string",
					"description": "Email subject line",
				},
				"text_content": map[string]interface{}{
					"type":        "string",
					"description": "Plain text email body. Either text_content or html_content (or both) must be provided.",
				},
				"html_content": map[string]interface{}{
					"type":        "string",
					"description": "HTML email body for rich formatting. Either text_content or html_content (or both) must be provided.",
				},
				"cc": map[string]interface{}{
					"type":        "string",
					"description": "CC recipient(s). For multiple, separate with commas.",
				},
				"bcc": map[string]interface{}{
					"type":        "string",
					"description": "BCC recipient(s). For multiple, separate with commas.",
				},
				"reply_to": map[string]interface{}{
					"type":        "string",
					"description": "Reply-to email address (optional)",
				},
				"template_id": map[string]interface{}{
					"type":        "number",
					"description": "Brevo template ID to use instead of html_content/text_content (optional)",
				},
				"params": map[string]interface{}{
					"type":        "object",
					"description": "Template parameters as key-value pairs (optional, used with template_id)",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Tags to categorize this email (optional)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"to", "subject"},
		},
		Execute: executeBrevoEmail,
	}
}

// BrevoRecipient represents an email recipient
type BrevoRecipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// BrevoSender represents the email sender
type BrevoSender struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// BrevoRequest represents the Brevo API request
type BrevoRequest struct {
	Sender      BrevoSender      `json:"sender"`
	To          []BrevoRecipient `json:"to"`
	CC          []BrevoRecipient `json:"cc,omitempty"`
	BCC         []BrevoRecipient `json:"bcc,omitempty"`
	ReplyTo     *BrevoRecipient  `json:"replyTo,omitempty"`
	Subject     string           `json:"subject"`
	HTMLContent string           `json:"htmlContent,omitempty"`
	TextContent string           `json:"textContent,omitempty"`
	TemplateID  int              `json:"templateId,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
}

func executeBrevoEmail(args map[string]interface{}) (string, error) {
	// Get all credential data first
	credData, credErr := GetCredentialData(args, "brevo")

	// Resolve API key from credential
	apiKey, err := ResolveAPIKey(args, "brevo", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get Brevo API key: %w. Please configure Brevo credentials first.", err)
	}

	// Validate API key format (Brevo keys start with "xkeysib-")
	if !strings.HasPrefix(apiKey, "xkeysib-") {
		return "", fmt.Errorf("invalid Brevo API key format (should start with 'xkeysib-')")
	}

	// Extract required parameters
	toStr, ok := args["to"].(string)
	if !ok || toStr == "" {
		return "", fmt.Errorf("'to' email address is required")
	}

	// Get from_email - first check args, then fall back to credential data
	fromEmail, _ := args["from_email"].(string)
	if fromEmail == "" && credErr == nil && credData != nil {
		if credFromEmail, ok := credData["from_email"].(string); ok {
			fromEmail = credFromEmail
		}
	}
	if fromEmail == "" {
		return "", fmt.Errorf("'from_email' is required - either provide it in the request or configure a default in Brevo credentials")
	}

	subject, ok := args["subject"].(string)
	if !ok || subject == "" {
		return "", fmt.Errorf("'subject' is required")
	}

	// Extract content
	textContent, _ := args["text_content"].(string)
	htmlContent, _ := args["html_content"].(string)
	templateID := 0
	if tid, ok := args["template_id"].(float64); ok {
		templateID = int(tid)
	}

	// Require either content or template
	if textContent == "" && htmlContent == "" && templateID == 0 {
		return "", fmt.Errorf("either 'text_content', 'html_content', or 'template_id' is required")
	}

	// Parse recipient email addresses
	toRecipients := parseBrevoEmailList(toStr)
	if len(toRecipients) == 0 {
		return "", fmt.Errorf("at least one valid 'to' email address is required")
	}

	// Build sender
	sender := BrevoSender{Email: fromEmail}
	if fromName, ok := args["from_name"].(string); ok && fromName != "" {
		sender.Name = fromName
	}

	// Build request
	request := BrevoRequest{
		Sender:  sender,
		To:      toRecipients,
		Subject: subject,
	}

	// Add content or template
	if templateID > 0 {
		request.TemplateID = templateID
		if params, ok := args["params"].(map[string]interface{}); ok {
			request.Params = params
		}
	} else {
		if htmlContent != "" {
			request.HTMLContent = htmlContent
		}
		if textContent != "" {
			request.TextContent = textContent
		}
	}

	// Parse CC recipients
	if ccStr, ok := args["cc"].(string); ok && ccStr != "" {
		request.CC = parseBrevoEmailList(ccStr)
	}

	// Parse BCC recipients
	if bccStr, ok := args["bcc"].(string); ok && bccStr != "" {
		request.BCC = parseBrevoEmailList(bccStr)
	}

	// Add reply-to if provided
	if replyTo, ok := args["reply_to"].(string); ok && replyTo != "" {
		request.ReplyTo = &BrevoRecipient{Email: replyTo}
	}

	// Add tags if provided
	if tags, ok := args["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				request.Tags = append(request.Tags, tagStr)
			}
		}
	}

	// Serialize request
	jsonPayload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to serialize request: %w", err)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request to Brevo API
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Accept", "application/json")

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

	// Brevo returns 201 Created on success
	success := resp.StatusCode == 201

	// Build result
	result := map[string]interface{}{
		"success":     success,
		"status_code": resp.StatusCode,
		"email_sent":  success,
		"recipients":  len(toRecipients),
		"subject":     subject,
		"from":        fromEmail,
	}

	if len(request.CC) > 0 {
		result["cc_count"] = len(request.CC)
	}
	if len(request.BCC) > 0 {
		result["bcc_count"] = len(request.BCC)
	}
	if templateID > 0 {
		result["template_id"] = templateID
	}

	// Parse response
	if len(respBody) > 0 {
		var apiResp map[string]interface{}
		if err := json.Unmarshal(respBody, &apiResp); err == nil {
			if messageId, ok := apiResp["messageId"].(string); ok {
				result["message_id"] = messageId
			}
			if !success {
				result["error"] = apiResp
			}
		} else if !success {
			result["error"] = string(respBody)
		}
	}

	if success {
		result["message"] = fmt.Sprintf("Email sent successfully to %d recipient(s)", len(toRecipients))
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// parseBrevoEmailList parses a comma-separated list of email addresses
func parseBrevoEmailList(emailStr string) []BrevoRecipient {
	var recipients []BrevoRecipient
	parts := strings.Split(emailStr, ",")
	for _, part := range parts {
		email := strings.TrimSpace(part)
		if email != "" && strings.Contains(email, "@") {
			recipients = append(recipients, BrevoRecipient{Email: email})
		}
	}
	return recipients
}

