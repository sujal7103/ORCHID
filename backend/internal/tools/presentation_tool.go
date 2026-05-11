package tools

import (
	"clara-agents/internal/presentation"
	"clara-agents/internal/securefile"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// NewPresentationTool creates the create_presentation tool
func NewPresentationTool() *Tool {
	return &Tool{
		Name:        "create_presentation",
		DisplayName: "Create Presentation",
		Description: `Creates a professional multi-page PDF presentation with custom HTML pages in 16:9 landscape format.

**CRITICAL: Each page MUST be a COMPLETE standalone HTML document with <html>, <head>, and <body> tags!**

## How to Structure Your Call

Each page must be a COMPLETE HTML document (starting with <html> and ending with </html>):

{
  "title": "My Presentation",
  "pages": [
    {"html": "<!DOCTYPE html><html><head><style>body{display:flex;align-items:center;justify-content:center;height:100vh;background:#667eea;color:white;margin:0}</style></head><body><h1>Title Slide</h1></body></html>"},
    {"html": "<!DOCTYPE html><html><head><style>body{background:#764ba2;color:white;padding:40px;margin:0}</style></head><body><h2>Topic 1</h2><p>Content here</p></body></html>"},
    {"html": "<!DOCTYPE html><html><head><style>body{background:#f093fb;color:white;padding:40px;margin:0}</style></head><body><h2>Topic 2</h2><p>More content</p></body></html>"},
    {"html": "<!DOCTYPE html><html><head><style>body{background:#4facfe;color:white;padding:40px;margin:0}</style></head><body><h2>Topic 3</h2></body></html>"},
    {"html": "<!DOCTYPE html><html><head><style>body{display:flex;align-items:center;justify-content:center;height:100vh;background:#667eea;color:white;margin:0}</style></head><body><h1>Thank You</h1></body></html>"}
  ]
}

**REQUIREMENTS:**
- MUST create 5-15 pages (NOT just 1-2!)
- Each page MUST be a complete HTML document starting with <html> and ending with </html>
- Include <head> with <style> tags for custom CSS
- Put your content in <body> tags
- Each slide is rendered as ONE PDF page (16:9 landscape, 10.67" x 6")

The final PDF is stored for 30 days with an access code for download.`,
		Icon: "Presentation",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "The presentation title (used for the filename and metadata)",
				},
				"pages": map[string]interface{}{
					"type":        "array",
					"description": "ARRAY OF PAGE OBJECTS - MUST contain 5-15 complete HTML documents! Each object must have an 'html' field containing a COMPLETE HTML document. Example: [{\"html\": \"<!DOCTYPE html><html><head><style>...</style></head><body><h1>Slide 1</h1></body></html>\"}, {\"html\": \"<!DOCTYPE html><html><head>...</head><body><h2>Slide 2</h2></body></html>\"}, ...]",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"html": map[string]interface{}{
								"type":        "string",
								"description": "COMPLETE HTML DOCUMENT for this slide. MUST start with <html> and end with </html>. MUST include <head> and <body> tags. Put styles in <head><style>...</style></head>. Example: \"<!DOCTYPE html><html><head><style>body{background:#667eea;color:white;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}</style></head><body><h1>My Title</h1></body></html>\"",
							},
						},
						"required": []string{"html"},
					},
				},
			},
			"required": []string{"title", "pages"},
		},
		Execute:  executeCreatePresentation,
		Source:   ToolSourceBuiltin,
		Category: "output",
		Keywords: []string{"presentation", "slides", "powerpoint", "ppt", "slideshow", "pdf", "create", "generate", "export", "custom", "design"},
	}
}

func executeCreatePresentation(args map[string]interface{}) (string, error) {
	// Extract title (required)
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return "", fmt.Errorf("title is required")
	}

	// Extract pages (required)
	pagesRaw, ok := args["pages"].([]interface{})
	if !ok || len(pagesRaw) == 0 {
		return "", fmt.Errorf("pages array is required and must not be empty")
	}

	// Validate minimum page count (HARD REQUIREMENT: at least 3 pages)
	if len(pagesRaw) < 3 {
		return "", fmt.Errorf("presentations must have at least 3 pages (you provided %d). A complete presentation needs: (1) a title/cover page, (2) one or more content pages, and (3) a conclusion/thank you page. Please provide at least 3 page objects in the pages array, each with an 'html' field. Example: pages: [{\"html\": \"<div>Cover</div>\"}, {\"html\": \"<div>Content 1</div>\"}, {\"html\": \"<div>Content 2</div>\"}, {\"html\": \"<div>Thank You</div>\"}]", len(pagesRaw))
	}

	// Warn about short presentations (RECOMMENDATION: 5-15 pages)
	if len(pagesRaw) < 5 {
		log.Printf("⚠️  [PRESENTATION-TOOL] Short presentation detected (%d pages). Typical presentations should have 5-15 pages for better content coverage.", len(pagesRaw))
	}

	// Parse pages
	slides := make([]presentation.Slide, 0, len(pagesRaw))
	for i, pageRaw := range pagesRaw {
		pageMap, ok := pageRaw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("page %d is not a valid object", i)
		}

		// HTML content (required)
		htmlContent, ok := pageMap["html"].(string)
		if !ok || htmlContent == "" {
			return "", fmt.Errorf("page %d missing required 'html' field. Each page object must have an 'html' field containing the HTML content for that slide. Example: {\"html\": \"<div style='display:flex;align-items:center;justify-content:center;height:100%%;background:#667eea;color:white'><h1>Slide Title</h1></div>\"}", i+1)
		}

		// Validate HTML content is not just whitespace
		trimmed := strings.TrimSpace(htmlContent)
		if trimmed == "" {
			return "", fmt.Errorf("page %d has empty HTML content (only whitespace). Please provide actual HTML content for this slide. Example: {\"html\": \"<div style='...'><h1>Content Here</h1></div>\"}", i+1)
		}

		// Validate HTML is a complete document (must have <html> and </html> tags)
		lowerHTML := strings.ToLower(trimmed)
		if !strings.Contains(lowerHTML, "<html") {
			return "", fmt.Errorf("page %d must be a COMPLETE HTML document starting with <html> tag. You provided: %q. Each slide MUST be a standalone HTML document. Example: {\"html\": \"<!DOCTYPE html><html><head><style>body{...}</style></head><body><h1>Title</h1></body></html>\"}", i+1, trimmed)
		}
		if !strings.Contains(lowerHTML, "</html>") {
			return "", fmt.Errorf("page %d must be a COMPLETE HTML document ending with </html> tag. Your HTML is missing the closing </html> tag. Example: {\"html\": \"<!DOCTYPE html><html><head><style>...</style></head><body>Content</body></html>\"}", i+1)
		}
		if !strings.Contains(lowerHTML, "<body") || !strings.Contains(lowerHTML, "</body>") {
			return "", fmt.Errorf("page %d must include <body></body> tags. Complete HTML structure required: <html><head>...</head><body>Your content here</body></html>", i+1)
		}

		slides = append(slides, presentation.Slide{
			HTML: htmlContent,
		})
	}

	// Extract injected user context
	userID, _ := args["__user_id__"].(string)
	if userID == "" {
		userID = "system"
	}
	conversationID, _ := args["__conversation_id__"].(string)

	// Clean up internal parameters
	delete(args, "__user_id__")
	delete(args, "__conversation_id__")

	log.Printf("🎯 [PRESENTATION-TOOL] Creating custom HTML presentation: %s (%d pages, 16:9 landscape PDF)",
		title, len(slides))

	// Build config
	config := presentation.PresentationConfig{
		Title:  title,
		Slides: slides,
	}

	// Get presentation service and generate PDF
	presService := presentation.GetService()
	tempResult, err := presService.GeneratePresentation(config, userID, conversationID)
	if err != nil {
		log.Printf("❌ [PRESENTATION-TOOL] Failed to generate presentation: %v", err)
		return "", fmt.Errorf("failed to generate presentation: %w", err)
	}

	// Read the generated file
	fileContent, err := os.ReadFile(tempResult.FilePath)
	if err != nil {
		log.Printf("❌ [PRESENTATION-TOOL] Failed to read generated file: %v", err)
		return "", fmt.Errorf("failed to read generated file: %w", err)
	}

	// Store in secure file service
	secureFileService := securefile.GetService()
	secureResult, err := secureFileService.CreateFile(userID, fileContent, tempResult.Filename, "application/pdf")
	if err != nil {
		log.Printf("❌ [PRESENTATION-TOOL] Failed to store presentation securely: %v", err)
		return "", fmt.Errorf("failed to store presentation: %w", err)
	}

	// Cleanup temporary file
	if err := os.Remove(tempResult.FilePath); err != nil {
		log.Printf("⚠️ [PRESENTATION-TOOL] Failed to cleanup temp file: %v", err)
	}

	// Format response
	response := map[string]interface{}{
		"success":      true,
		"file_id":      secureResult.ID,
		"filename":     secureResult.Filename,
		"download_url": secureResult.DownloadURL,
		"access_code":  secureResult.AccessCode,
		"size":         secureResult.Size,
		"file_type":    "pdf",
		"page_count":   len(slides),
		"aspect_ratio": "16:9 landscape",
		"page_size":    "10.67\" x 6\" (widescreen)",
		"expires_at":   secureResult.ExpiresAt.Format("2006-01-02"),
		"message":      fmt.Sprintf("Custom HTML presentation '%s' created successfully with %d pages in 16:9 landscape format. Download link (valid for 30 days): %s", secureResult.Filename, len(slides), secureResult.DownloadURL),
	}

	responseJSON, _ := json.Marshal(response)

	log.Printf("✅ [PRESENTATION-TOOL] Custom HTML presentation generated: %s (%d bytes, %d pages, expires: %s)",
		secureResult.Filename, secureResult.Size, len(slides), secureResult.ExpiresAt.Format("2006-01-02"))

	return string(responseJSON), nil
}

