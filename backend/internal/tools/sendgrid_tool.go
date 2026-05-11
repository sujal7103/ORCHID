package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Email rate limiter - prevents sending more than 1 email per recipient per minute
var (
	emailRateLimiter = make(map[string]time.Time)
	rateLimiterMutex sync.RWMutex
	rateLimitWindow  = 1 * time.Minute
)

// checkRateLimit checks if an email can be sent to the recipient
// Returns true if allowed, false if rate limited
func checkRateLimit(email string) (bool, time.Duration) {
	rateLimiterMutex.RLock()
	lastSent, exists := emailRateLimiter[strings.ToLower(email)]
	rateLimiterMutex.RUnlock()

	if !exists {
		return true, 0
	}

	elapsed := time.Since(lastSent)
	if elapsed < rateLimitWindow {
		return false, rateLimitWindow - elapsed
	}

	return true, 0
}

// recordEmailSent records that an email was sent to a recipient
func recordEmailSent(email string) {
	rateLimiterMutex.Lock()
	emailRateLimiter[strings.ToLower(email)] = time.Now()
	rateLimiterMutex.Unlock()
}

// cleanupOldRateLimits removes expired entries (called periodically)
func cleanupOldRateLimits() {
	rateLimiterMutex.Lock()
	defer rateLimiterMutex.Unlock()

	now := time.Now()
	for email, lastSent := range emailRateLimiter {
		if now.Sub(lastSent) > rateLimitWindow {
			delete(emailRateLimiter, email)
		}
	}
}

// NewSendGridTool creates a SendGrid email sending tool
func NewSendGridTool() *Tool {
	// Start a background cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			cleanupOldRateLimits()
		}
	}()

	return &Tool{
		Name:        "send_email",
		DisplayName: "Send Email (SendGrid)",
		Description: `Send emails via SendGrid API. Supports plain text and HTML emails with file attachments.

Features:
- Send to single or multiple recipients (to, cc, bcc)
- HTML and plain text email bodies
- Custom sender name and reply-to address
- File attachments via URL (supports generated PDFs, secure files, and external URLs)
- Rate limited: 1 email per recipient per minute to prevent spam

ATTACHMENTS: To attach files (like generated PDFs), use file_url with the full download URL.
For secure files, use the download_url returned by file-generating tools (e.g., /api/files/{id}?code={code}).

Authentication is handled automatically via configured SendGrid credentials. Do NOT ask users for API keys.
The sender email (from_email) can be configured in credentials as default, or overridden per-request.

IMPORTANT: This tool has rate limiting. If you've already sent an email to a recipient within the last minute, the tool will return a message indicating the email was already sent. Do NOT retry sending - wait for the cooldown period.`,
		Icon:     "Mail",
		Source:   ToolSourceBuiltin,
		Category: "integration",
		Keywords: []string{"email", "sendgrid", "send", "mail", "message", "notification", "newsletter", "transactional", "attachment"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"api_key": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Resolved from credentials. Do not ask user for this.",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Recipient email address(es). For multiple recipients, separate with commas (e.g., 'user1@example.com, user2@example.com')",
				},
				"from_email": map[string]interface{}{
					"type":        "string",
					"description": "Sender email address (optional). Must be a verified sender in SendGrid. If not provided, uses the default from_email configured in credentials.",
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
				"file_url": map[string]interface{}{
					"type":        "string",
					"description": "URL to download a file to attach. Supports both absolute URLs and relative paths starting with /api/files/. For secure files generated by other tools, use the full download_url including the access code (e.g., '/api/files/{id}?code={code}').",
				},
				"file_name": map[string]interface{}{
					"type":        "string",
					"description": "Filename for the attached file (optional, will be inferred from URL or Content-Disposition if not provided)",
				},
				"attachments": map[string]interface{}{
					"type":        "array",
					"description": "Alternative: File attachments as array of objects with 'content' (base64), 'filename', and 'type' (mime type). Use file_url for simpler attachment.",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Base64 encoded file content",
							},
							"filename": map[string]interface{}{
								"type":        "string",
								"description": "Filename with extension",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "MIME type (e.g., 'application/pdf', 'image/png')",
							},
						},
					},
				},
			},
			"required": []string{"to", "subject"},
		},
		Execute: executeSendGridEmail,
	}
}

// SendGridPersonalization represents email recipient personalization
type SendGridPersonalization struct {
	To  []SendGridEmail `json:"to"`
	CC  []SendGridEmail `json:"cc,omitempty"`
	BCC []SendGridEmail `json:"bcc,omitempty"`
}

// SendGridEmail represents an email address with optional name
type SendGridEmail struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// SendGridContent represents email content (text or html)
type SendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SendGridAttachment represents an email attachment
type SendGridAttachment struct {
	Content     string `json:"content"`
	Filename    string `json:"filename"`
	Type        string `json:"type,omitempty"`
	Disposition string `json:"disposition,omitempty"`
}

// SendGridRequest represents the full SendGrid API request
type SendGridRequest struct {
	Personalizations []SendGridPersonalization `json:"personalizations"`
	From             SendGridEmail             `json:"from"`
	ReplyTo          *SendGridEmail            `json:"reply_to,omitempty"`
	Subject          string                    `json:"subject"`
	Content          []SendGridContent         `json:"content"`
	Attachments      []SendGridAttachment      `json:"attachments,omitempty"`
}

func executeSendGridEmail(args map[string]interface{}) (string, error) {
	// Get all credential data first (we need both api_key and from_email)
	credData, credErr := GetCredentialData(args, "sendgrid")

	// Resolve API key from credential or direct parameter
	apiKey, err := ResolveAPIKey(args, "sendgrid", "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get SendGrid API key: %w. Please configure SendGrid credentials first.", err)
	}

	// Validate API key format
	if !strings.HasPrefix(apiKey, "SG.") {
		return "", fmt.Errorf("invalid SendGrid API key format (should start with 'SG.')")
	}

	// Extract required parameters
	toStr, ok := args["to"].(string)
	if !ok || toStr == "" {
		return "", fmt.Errorf("'to' email address is required")
	}

	// Get from_email - first check args, then fall back to credential data
	fromEmail, _ := args["from_email"].(string)
	if fromEmail == "" && credErr == nil && credData != nil {
		// Try to get from credential data
		if credFromEmail, ok := credData["from_email"].(string); ok {
			fromEmail = credFromEmail
		}
	}
	if fromEmail == "" {
		return "", fmt.Errorf("'from_email' is required - either provide it in the request or configure a default in SendGrid credentials")
	}

	subject, ok := args["subject"].(string)
	if !ok || subject == "" {
		return "", fmt.Errorf("'subject' is required")
	}

	// Extract content (at least one is required)
	textContent, _ := args["text_content"].(string)
	htmlContent, _ := args["html_content"].(string)

	if textContent == "" && htmlContent == "" {
		return "", fmt.Errorf("either 'text_content' or 'html_content' is required")
	}

	// Parse recipient email addresses
	toEmails := parseEmailList(toStr)
	if len(toEmails) == 0 {
		return "", fmt.Errorf("at least one valid 'to' email address is required")
	}

	// Check rate limits for all recipients BEFORE sending
	var rateLimitedEmails []string
	var allowedEmails []SendGridEmail
	for _, email := range toEmails {
		allowed, waitTime := checkRateLimit(email.Email)
		if !allowed {
			rateLimitedEmails = append(rateLimitedEmails, fmt.Sprintf("%s (wait %ds)", email.Email, int(waitTime.Seconds())))
		} else {
			allowedEmails = append(allowedEmails, email)
		}
	}

	// If ALL recipients are rate limited, return early with a clear message
	if len(allowedEmails) == 0 {
		result := map[string]interface{}{
			"success":        false,
			"already_sent":   true,
			"rate_limited":   true,
			"message":        "Email already sent to all recipients within the last minute. Please wait before sending again.",
			"blocked_emails": rateLimitedEmails,
			"cooldown":       "1 minute per recipient",
		}
		jsonResult, _ := json.MarshalIndent(result, "", "  ")
		return string(jsonResult), nil
	}

	// Build personalization with only allowed emails
	personalization := SendGridPersonalization{
		To: allowedEmails,
	}

	// Parse CC recipients (also rate limited)
	if ccStr, ok := args["cc"].(string); ok && ccStr != "" {
		ccEmails := parseEmailList(ccStr)
		for _, email := range ccEmails {
			allowed, _ := checkRateLimit(email.Email)
			if allowed {
				personalization.CC = append(personalization.CC, email)
			}
		}
	}

	// Parse BCC recipients (also rate limited)
	if bccStr, ok := args["bcc"].(string); ok && bccStr != "" {
		bccEmails := parseEmailList(bccStr)
		for _, email := range bccEmails {
			allowed, _ := checkRateLimit(email.Email)
			if allowed {
				personalization.BCC = append(personalization.BCC, email)
			}
		}
	}

	// Build from address
	from := SendGridEmail{Email: fromEmail}
	if fromName, ok := args["from_name"].(string); ok && fromName != "" {
		from.Name = fromName
	}

	// Build content array
	var sgContent []SendGridContent
	if textContent != "" {
		sgContent = append(sgContent, SendGridContent{
			Type:  "text/plain",
			Value: textContent,
		})
	}
	if htmlContent != "" {
		sgContent = append(sgContent, SendGridContent{
			Type:  "text/html",
			Value: htmlContent,
		})
	}

	// Build request
	request := SendGridRequest{
		Personalizations: []SendGridPersonalization{personalization},
		From:             from,
		Subject:          subject,
		Content:          sgContent,
	}

	// Add reply-to if provided
	if replyTo, ok := args["reply_to"].(string); ok && replyTo != "" {
		request.ReplyTo = &SendGridEmail{Email: replyTo}
	}

	// Track if we attached a file via URL
	var fileAttached bool
	var attachedFileName string

	// Handle file_url attachment (like Discord does)
	if fileURL, ok := args["file_url"].(string); ok && fileURL != "" {
		fileName, _ := args["file_name"].(string)
		fileData, resolvedFileName, mimeType, fetchErr := fetchFileForEmail(fileURL, fileName)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch file from URL: %w", fetchErr)
		}

		// Encode file content as base64 for SendGrid
		base64Content := base64.StdEncoding.EncodeToString(fileData)

		attachment := SendGridAttachment{
			Content:     base64Content,
			Filename:    resolvedFileName,
			Type:        mimeType,
			Disposition: "attachment",
		}
		request.Attachments = append(request.Attachments, attachment)
		fileAttached = true
		attachedFileName = resolvedFileName
	}

	// Add manual attachments if provided (legacy support)
	if attachments, ok := args["attachments"].([]interface{}); ok && len(attachments) > 0 {
		for _, att := range attachments {
			if attMap, ok := att.(map[string]interface{}); ok {
				attachment := SendGridAttachment{
					Disposition: "attachment",
				}
				if content, ok := attMap["content"].(string); ok {
					attachment.Content = content
				}
				if filename, ok := attMap["filename"].(string); ok {
					attachment.Filename = filename
				}
				if mimeType, ok := attMap["type"].(string); ok {
					attachment.Type = mimeType
				}
				if attachment.Content != "" && attachment.Filename != "" {
					request.Attachments = append(request.Attachments, attachment)
				}
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
		Timeout: 60 * time.Second, // Increased timeout for large attachments
	}

	// Create request to SendGrid API
	req, err := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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

	// SendGrid returns 202 Accepted on success
	success := resp.StatusCode == 202

	// Record successful sends for rate limiting
	if success {
		for _, email := range allowedEmails {
			recordEmailSent(email.Email)
		}
		for _, email := range personalization.CC {
			recordEmailSent(email.Email)
		}
		for _, email := range personalization.BCC {
			recordEmailSent(email.Email)
		}
	}

	// Build result
	result := map[string]interface{}{
		"success":     success,
		"status_code": resp.StatusCode,
		"email_sent":  success,
		"recipients":  len(allowedEmails),
		"subject":     subject,
		"from":        fromEmail,
	}

	// Include rate limited info if some were blocked
	if len(rateLimitedEmails) > 0 {
		result["rate_limited_emails"] = rateLimitedEmails
		result["partial_send"] = true
	}

	if len(personalization.CC) > 0 {
		result["cc_count"] = len(personalization.CC)
	}
	if len(personalization.BCC) > 0 {
		result["bcc_count"] = len(personalization.BCC)
	}
	if len(request.Attachments) > 0 {
		result["attachments_count"] = len(request.Attachments)
	}
	if fileAttached {
		result["file_attached"] = true
		result["attached_file"] = attachedFileName
	}

	// Include response for debugging
	if !success {
		// Parse error response
		var errorResp map[string]interface{}
		if err := json.Unmarshal(respBody, &errorResp); err == nil {
			result["error"] = errorResp
		} else {
			result["error"] = string(respBody)
		}
		result["status"] = resp.Status
	} else {
		if fileAttached {
			result["message"] = fmt.Sprintf("Email with attachment '%s' sent successfully to %d recipient(s)", attachedFileName, len(allowedEmails))
		} else {
			result["message"] = fmt.Sprintf("Email sent successfully to %d recipient(s)", len(allowedEmails))
		}
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// fetchFileForEmail fetches a file from a URL for email attachment
// Returns: file content, filename, mime type, error
func fetchFileForEmail(fileURL, providedFileName string) ([]byte, string, string, error) {
	// Resolve relative URLs to absolute URLs
	actualURL := fileURL
	if strings.HasPrefix(fileURL, "/api/") {
		// Use BACKEND_URL env var for internal API calls
		backendURL := os.Getenv("BACKEND_URL")
		if backendURL == "" {
			backendURL = "http://localhost:3001"
		}
		actualURL = backendURL + fileURL
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(actualURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", "", fmt.Errorf("failed to fetch file: status %d - %s", resp.StatusCode, string(body))
	}

	// Read the file content
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read file: %w", err)
	}

	// Determine filename
	fileName := providedFileName
	if fileName == "" {
		// Try to get from Content-Disposition header
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			_, params, err := mime.ParseMediaType(cd)
			if err == nil {
				if fn, ok := params["filename"]; ok {
					fileName = fn
				}
			}
		}
		// Fallback: extract from URL
		if fileName == "" {
			urlPath := strings.Split(strings.Split(fileURL, "?")[0], "/")
			if len(urlPath) > 0 {
				lastPart := urlPath[len(urlPath)-1]
				if strings.Contains(lastPart, ".") {
					fileName = lastPart
				}
			}
		}
		// Final fallback
		if fileName == "" {
			fileName = "attachment"
		}
	}

	// Determine MIME type
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		// Try to infer from filename extension
		ext := strings.ToLower(filepath.Ext(fileName))
		switch ext {
		case ".pdf":
			mimeType = "application/pdf"
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".doc":
			mimeType = "application/msword"
		case ".docx":
			mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case ".xls":
			mimeType = "application/vnd.ms-excel"
		case ".xlsx":
			mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case ".csv":
			mimeType = "text/csv"
		case ".txt":
			mimeType = "text/plain"
		case ".html":
			mimeType = "text/html"
		case ".json":
			mimeType = "application/json"
		case ".zip":
			mimeType = "application/zip"
		default:
			mimeType = "application/octet-stream"
		}
	}

	return data, fileName, mimeType, nil
}

// parseEmailList parses a comma-separated list of email addresses
func parseEmailList(emailStr string) []SendGridEmail {
	var emails []SendGridEmail
	parts := strings.Split(emailStr, ",")
	for _, part := range parts {
		email := strings.TrimSpace(part)
		if email != "" && strings.Contains(email, "@") {
			emails = append(emails, SendGridEmail{Email: email})
		}
	}
	return emails
}
