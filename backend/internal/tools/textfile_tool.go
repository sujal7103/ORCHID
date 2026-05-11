package tools

import (
	"clara-agents/internal/securefile"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// NewTextFileTool creates the create_text_file tool
func NewTextFileTool() *Tool {
	return &Tool{
		Name:        "create_text_file",
		DisplayName: "Create Text File",
		Description: `Creates a downloadable text-based file with specified content and extension. The file is stored for 30 days and requires an access code to download. Supports various formats like .txt, .json, .yaml, .xml, .csv, .md, .css, .js, .py, .go, .sh, .sql, .log, .ini, .toml, .env, and more.

IMPORTANT: Do NOT use this tool to create .html files unless the user explicitly asks to "create a file", "save as file", or "download as file". For HTML content, use artifacts instead - they render HTML directly in the chat without needing file downloads. Only create .html files when the user specifically wants a downloadable file.`,
		Icon: "FileCode",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The text content of the file",
				},
				"filename": map[string]interface{}{
					"type":        "string",
					"description": "Desired filename without extension (e.g., 'config', 'data', 'script')",
				},
				"extension": map[string]interface{}{
					"type":        "string",
					"description": "File extension without the dot (e.g., 'txt', 'json', 'yaml', 'csv', 'md', 'py', 'js')",
				},
			},
			"required": []string{"content", "extension"},
		},
		Execute:  executeCreateTextFile,
		Source:   ToolSourceBuiltin,
		Category: "output",
		Keywords: []string{"file", "text", "create", "generate", "save", "export", "json", "yaml", "csv", "code", "config", "write"},
	}
}

func executeCreateTextFile(args map[string]interface{}) (string, error) {
	// Extract parameters
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return "", fmt.Errorf("content is required")
	}

	extension, ok := args["extension"].(string)
	if !ok || extension == "" {
		return "", fmt.Errorf("extension is required")
	}

	// Clean extension (remove leading dot if present)
	extension = strings.TrimPrefix(extension, ".")

	filename, _ := args["filename"].(string)
	if filename == "" {
		filename = "file"
	}

	// Extract injected user context (set by ChatService)
	userID, _ := args["__user_id__"].(string)
	if userID == "" {
		userID = "system" // Fallback for tools executed outside user context
	}

	// Clean up internal parameters before logging
	delete(args, "__user_id__")
	delete(args, "__conversation_id__")

	log.Printf("📝 [TEXTFILE-TOOL] Generating text file: %s.%s (user: %s, length: %d chars)", filename, extension, userID, len(content))

	// Determine MIME type based on extension
	mimeType := getMimeTypeFromExtension(extension)

	// Full filename with extension
	fullFilename := fmt.Sprintf("%s.%s", filename, extension)

	// Store in secure file service with 30-day retention and access code
	secureFileService := securefile.GetService()
	secureResult, err := secureFileService.CreateFile(userID, []byte(content), fullFilename, mimeType)
	if err != nil {
		log.Printf("❌ [TEXTFILE-TOOL] Failed to store file securely: %v", err)
		return "", fmt.Errorf("failed to store file: %w", err)
	}

	// Format result for AI
	response := map[string]interface{}{
		"success":      true,
		"file_id":      secureResult.ID,
		"filename":     secureResult.Filename,
		"download_url": secureResult.DownloadURL,
		"access_code":  secureResult.AccessCode,
		"size":         secureResult.Size,
		"file_type":    "text",
		"extension":    extension,
		"expires_at":   secureResult.ExpiresAt.Format("2006-01-02"),
		"message":      fmt.Sprintf("Text file '%s' created successfully. Download link (valid for 30 days): %s", secureResult.Filename, secureResult.DownloadURL),
	}

	responseJSON, _ := json.Marshal(response)

	log.Printf("✅ [TEXTFILE-TOOL] Text file generated and stored securely: %s (%d bytes, expires: %s)",
		secureResult.Filename, secureResult.Size, secureResult.ExpiresAt.Format("2006-01-02"))

	return string(responseJSON), nil
}

// getMimeTypeFromExtension returns the MIME type for a given file extension
func getMimeTypeFromExtension(ext string) string {
	mimeTypes := map[string]string{
		"txt":  "text/plain",
		"json": "application/json",
		"yaml": "application/x-yaml",
		"yml":  "application/x-yaml",
		"xml":  "application/xml",
		"csv":  "text/csv",
		"md":   "text/markdown",
		"html": "text/html",
		"htm":  "text/html",
		"css":  "text/css",
		"js":   "text/javascript",
		"ts":   "text/typescript",
		"py":   "text/x-python",
		"go":   "text/x-go",
		"rs":   "text/x-rust",
		"java": "text/x-java",
		"c":    "text/x-c",
		"cpp":  "text/x-c++",
		"h":    "text/x-c",
		"hpp":  "text/x-c++",
		"sh":   "application/x-sh",
		"bash": "application/x-sh",
		"sql":  "application/sql",
		"log":  "text/plain",
		"ini":  "text/plain",
		"toml": "application/toml",
		"env":  "text/plain",
		"conf": "text/plain",
		"cfg":  "text/plain",
	}

	if mime, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mime
	}
	return "text/plain"
}
