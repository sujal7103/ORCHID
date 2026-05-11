package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// NewDiscordTool creates a Discord webhook messaging tool
func NewDiscordTool() *Tool {
	return &Tool{
		Name:        "send_discord_message",
		DisplayName: "Send Discord Message",
		Description: "Send a message to Discord via webhook. Message content is limited to 2000 characters max (Discord API limit). Just provide the message content - webhook authentication is handled automatically via configured credentials. Do NOT ask the user for webhook URLs. Supports embeds for rich formatting.",
		Icon:        "MessageCircle",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"discord", "message", "chat", "notify", "webhook", "channel", "bot", "notification"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"webhook_url": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Resolved from credentials. Do not ask user for this.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Message content (max 2000 characters). This is the main text message.",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Override the default webhook username (optional)",
				},
				"avatar_url": map[string]interface{}{
					"type":        "string",
					"description": "Override the default webhook avatar URL (optional)",
				},
				"embed_title": map[string]interface{}{
					"type":        "string",
					"description": "Title for an embed (optional, for rich formatting)",
				},
				"embed_description": map[string]interface{}{
					"type":        "string",
					"description": "Description for an embed (optional, max 4096 characters)",
				},
				"embed_color": map[string]interface{}{
					"type":        "number",
					"description": "Embed color as decimal (optional, e.g., 5814783 for blue)",
				},
				"image_data": map[string]interface{}{
					"type":        "string",
					"description": "Base64 encoded image data to attach (optional). Can include data URI prefix or raw base64.",
				},
				"image_filename": map[string]interface{}{
					"type":        "string",
					"description": "Filename for the attached image (optional, defaults to 'chart.png')",
				},
				"file_url": map[string]interface{}{
					"type":        "string",
					"description": "URL to download a file to attach (optional). Supports both absolute URLs and relative paths starting with /api/files/. For relative paths, the backend URL will be automatically resolved.",
				},
				"file_name": map[string]interface{}{
					"type":        "string",
					"description": "Filename for the attached file from URL (optional, will be inferred from URL if not provided)",
				},
			},
			"required": []string{},
		},
		Execute: executeDiscordMessage,
	}
}

func executeDiscordMessage(args map[string]interface{}) (string, error) {
	// Resolve webhook URL from credential or direct parameter
	webhookURL, err := ResolveWebhookURL(args, "discord")
	if err != nil {
		// Fallback: check for direct webhook_url if credential resolution failed
		if url, ok := args["webhook_url"].(string); ok && url != "" {
			webhookURL = url
		} else {
			return "", fmt.Errorf("failed to get webhook URL: %w", err)
		}
	}

	// Validate Discord webhook URL
	if !strings.Contains(webhookURL, "discord.com/api/webhooks/") && !strings.Contains(webhookURL, "discordapp.com/api/webhooks/") {
		return "", fmt.Errorf("invalid Discord webhook URL")
	}

	// Extract content (optional now since we might just send an image)
	content, _ := args["content"].(string)

	// Truncate content if too long (Discord limit is 2000)
	if len(content) > 2000 {
		content = content[:1997] + "..."
	}

	// Check for image data
	imageData, hasImage := args["image_data"].(string)
	imageFilename := "chart.png"
	if fn, ok := args["image_filename"].(string); ok && fn != "" {
		imageFilename = fn
	}

	// Check for file URL
	fileURL, hasFileURL := args["file_url"].(string)
	var fileData []byte
	fileName := ""
	if fn, ok := args["file_name"].(string); ok && fn != "" {
		fileName = fn
	}

	// Fetch file from URL if provided
	if hasFileURL && fileURL != "" {
		var fetchErr error
		fileData, fileName, fetchErr = fetchFileFromURL(fileURL, fileName)
		if fetchErr != nil {
			return "", fmt.Errorf("failed to fetch file from URL: %w", fetchErr)
		}
	}

	// Build Discord webhook payload
	payload := map[string]interface{}{}
	if content != "" {
		payload["content"] = content
	}

	// Optional username override
	if username, ok := args["username"].(string); ok && username != "" {
		payload["username"] = username
	}

	// Optional avatar override
	if avatarURL, ok := args["avatar_url"].(string); ok && avatarURL != "" {
		payload["avatar_url"] = avatarURL
	}

	// Build embed if any embed fields provided
	embed := make(map[string]interface{})
	hasEmbed := false

	if embedTitle, ok := args["embed_title"].(string); ok && embedTitle != "" {
		embed["title"] = embedTitle
		hasEmbed = true
	}

	if embedDesc, ok := args["embed_description"].(string); ok && embedDesc != "" {
		// Truncate embed description if too long (Discord limit is 4096)
		if len(embedDesc) > 4096 {
			embedDesc = embedDesc[:4093] + "..."
		}
		embed["description"] = embedDesc
		hasEmbed = true
	}

	if embedColor, ok := args["embed_color"].(float64); ok {
		embed["color"] = int(embedColor)
		hasEmbed = true
	}

	// If we have an image, reference it in the embed
	if hasImage && imageData != "" {
		if !hasEmbed {
			embed["title"] = "Generated Chart"
			hasEmbed = true
		}
		// Reference the attached image in the embed
		embed["image"] = map[string]interface{}{
			"url": "attachment://" + imageFilename,
		}
	}

	if hasEmbed {
		embed["timestamp"] = time.Now().UTC().Format(time.RFC3339)
		payload["embeds"] = []map[string]interface{}{embed}
	}

	// Require at least content, image, file, or embed
	hasFile := len(fileData) > 0
	if content == "" && !hasImage && !hasFile && !hasEmbed {
		return "", fmt.Errorf("either content, image_data, file_url, or embed is required")
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	var req *http.Request

	if hasFile {
		// Send with multipart/form-data for file attachment
		req, err = createMultipartRequestWithFile(webhookURL, payload, fileData, fileName)
	} else if hasImage && imageData != "" {
		// Send with multipart/form-data for image attachment
		req, err = createMultipartRequest(webhookURL, payload, imageData, imageFilename)
	} else {
		// Send as JSON (no image)
		jsonPayload, jsonErr := json.Marshal(payload)
		if jsonErr != nil {
			return "", fmt.Errorf("failed to serialize payload: %w", jsonErr)
		}
		req, err = http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
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

	// Build result
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := map[string]interface{}{
		"success":      success,
		"status_code":  resp.StatusCode,
		"status":       resp.Status,
		"message_sent": success,
	}

	if content != "" {
		result["content_length"] = len(content)
	}
	if hasFile {
		result["file_attached"] = true
		result["file_name"] = fileName
		result["file_size"] = len(fileData)
	}
	if hasImage {
		result["image_attached"] = true
		result["image_filename"] = imageFilename
	}

	// Include response body if there's an error
	if !success && len(respBody) > 0 {
		result["error"] = string(respBody)
	}

	// Add success message
	if success {
		if hasFile {
			result["message"] = fmt.Sprintf("Discord message with file '%s' sent successfully", fileName)
		} else if hasImage {
			result["message"] = "Discord message with image sent successfully"
		} else {
			result["message"] = "Discord message sent successfully"
		}
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// fetchFileFromURL fetches a file from a URL (supports relative paths for internal files)
func fetchFileFromURL(fileURL, providedFileName string) ([]byte, string, error) {
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
		return nil, "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch file: status %d", resp.StatusCode)
	}

	// Read the file content
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	// Determine filename
	fileName := providedFileName
	if fileName == "" {
		// Try to get from Content-Disposition header
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if strings.Contains(cd, "filename=") {
				parts := strings.Split(cd, "filename=")
				if len(parts) > 1 {
					fileName = strings.Trim(parts[1], "\"' ")
				}
			}
		}
		// Fallback: extract from URL
		if fileName == "" {
			parts := strings.Split(strings.Split(fileURL, "?")[0], "/")
			if len(parts) > 0 {
				fileName = parts[len(parts)-1]
			}
		}
		// Final fallback
		if fileName == "" {
			fileName = "attachment"
		}
	}

	return data, fileName, nil
}

// createMultipartRequestWithFile creates a multipart request with JSON payload and raw file data
func createMultipartRequestWithFile(webhookURL string, payload map[string]interface{}, fileData []byte, filename string) (*http.Request, error) {
	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add payload_json field
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return nil, fmt.Errorf("failed to write payload field: %w", err)
	}

	// Add file attachment
	part, err := writer.CreateFormFile("files[0]", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", webhookURL, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// createMultipartRequest creates a multipart request with JSON payload and image attachment
func createMultipartRequest(webhookURL string, payload map[string]interface{}, imageData, filename string) (*http.Request, error) {
	// Decode base64 image data
	// Remove data URI prefix if present
	imageData = strings.TrimPrefix(imageData, "data:image/png;base64,")
	imageData = strings.TrimPrefix(imageData, "data:image/jpeg;base64,")
	imageData = strings.TrimPrefix(imageData, "data:image/jpg;base64,")
	imageData = strings.TrimPrefix(imageData, "data:image/gif;base64,")

	imageBytes, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add payload_json field
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return nil, fmt.Errorf("failed to write payload field: %w", err)
	}

	// Add file attachment
	part, err := writer.CreateFormFile("files[0]", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(imageBytes); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", webhookURL, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
