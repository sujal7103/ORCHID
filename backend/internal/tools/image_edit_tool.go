package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"clara-agents/internal/filecache"

	"github.com/google/uuid"
)

// ImageEditConfigKey is the key for injecting image edit provider config into tool args
const ImageEditConfigKey = "__image_edit_config__"

// ImageRegistryKey is the key for injecting image registry into tool args
const ImageRegistryKey = "__image_registry__"

// ImageEditConfig holds the configuration for image editing
type ImageEditConfig struct {
	BaseURL string
	APIKey  string
}

// ImageRegistryEntry represents a registered image (interface to avoid import cycle)
type ImageRegistryEntry struct {
	Handle   string
	FileID   string
	Filename string
	Source   string
}

// ImageRegistryInterface defines the methods needed from the image registry (to avoid import cycle)
type ImageRegistryInterface interface {
	GetByHandle(conversationID, handle string) *ImageRegistryEntry
	ListHandles(conversationID string) []string
	RegisterGeneratedImage(conversationID, fileID, prompt string) string
	RegisterEditedImage(conversationID, fileID, sourceHandle, prompt string) string
}

// ChutesImageEditRequest represents the Chutes image edit API request format
type ChutesImageEditRequest struct {
	Seed              *int     `json:"seed"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	Prompt            string   `json:"prompt"`
	ImageB64s         []string `json:"image_b64s"`
	TrueCFGScale      float64  `json:"true_cfg_scale"`
	NegativePrompt    string   `json:"negative_prompt"`
	NumInferenceSteps int      `json:"num_inference_steps"`
}

// NewImageEditTool creates a new image editing tool
func NewImageEditTool() *Tool {
	return &Tool{
		Name:        "edit_image",
		DisplayName: "Edit Image",
		Description: `Edits an existing image based on a text prompt using AI image editing.

Use this tool when the user asks you to:
- Modify, edit, or change an existing image
- Add or remove elements from an image
- Change colors, style, or composition of an image
- Apply effects or transformations to an image
- Enhance or retouch a photo

Parameters:
- image_id: The image handle (e.g., "img-1") from the available images list
- prompt: Detailed description of the changes to make
- size: Output size preset (use this to change dimensions)
- negative_prompt: (optional) What to avoid in the edited result

Use 'size' parameter with a preset value. Available presets:
- square (default), square_hd, square_small - Square formats
- landscape, landscape_wide - Horizontal formats
- banner - Wide banner for headers
- portrait, portrait_tall - Vertical formats
- phone_wallpaper, instagram_story - Mobile formats (9:16)
- pc_wallpaper, pc_wallpaper_2k, pc_wallpaper_4k - Desktop wallpaper (16:9)
- ultrawide - Ultrawide monitor
- youtube_thumb, twitter_header, facebook_cover - Social media formats
- poster, a4_portrait - Print formats

The edited image will be displayed directly in the chat and assigned a new image ID for further editing.

Tips for good edit prompts:
- Be specific about what changes you want
- Describe the desired outcome, not just what to remove
- Include style details if you want to maintain or change the aesthetic`,
		Icon:     "Paintbrush",
		Source:   ToolSourceBuiltin,
		Category: "generation",
		Keywords: []string{"image", "edit", "modify", "change", "transform", "enhance", "retouch", "alter", "adjust"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_id": map[string]interface{}{
					"type":        "string",
					"description": "The image handle to edit (e.g., 'img-1'). Check the available images list in the system context.",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "A detailed description of the changes to make to the image. Be specific about what you want to modify, add, or change.",
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": "Size preset for output: square, square_hd, landscape, portrait, banner, phone_wallpaper, pc_wallpaper, pc_wallpaper_2k, pc_wallpaper_4k, ultrawide, youtube_thumb, twitter_header, facebook_cover, instagram_story, poster. Default is 'square' (1024x1024).",
					"enum":        []string{"square", "square_hd", "square_small", "landscape", "landscape_wide", "banner", "portrait", "portrait_tall", "phone_wallpaper", "instagram_story", "pc_wallpaper", "pc_wallpaper_2k", "pc_wallpaper_4k", "ultrawide", "youtube_thumb", "twitter_header", "facebook_cover", "poster", "a4_portrait"},
				},
				"width": map[string]interface{}{
					"type":        "integer",
					"description": "Custom width in pixels (64-4096). Use this for custom dimensions instead of size presets. Must be used with height.",
				},
				"height": map[string]interface{}{
					"type":        "integer",
					"description": "Custom height in pixels (64-4096). Use this for custom dimensions instead of size presets. Must be used with width.",
				},
				"negative_prompt": map[string]interface{}{
					"type":        "string",
					"description": "Optional: What to avoid in the edited result (e.g., 'blurry, distorted, low quality').",
				},
			},
			"required": []string{"image_id", "prompt"},
		},
		Execute: executeImageEdit,
	}
}

func executeImageEdit(args map[string]interface{}) (string, error) {
	// Extract parameters
	imageID, ok := args["image_id"].(string)
	if !ok || imageID == "" {
		return "", fmt.Errorf("image_id is required")
	}

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	negativePrompt, _ := args["negative_prompt"].(string)

	// Determine image dimensions (same logic as generate_image)
	width, height := 1024, 1024 // Default square

	// Check for size preset first
	if sizePreset, ok := args["size"].(string); ok && sizePreset != "" {
		if preset, exists := ImageSizePresets[strings.ToLower(sizePreset)]; exists {
			width = preset.Width
			height = preset.Height
			log.Printf("🎨 [IMAGE-EDIT] Using size preset '%s': %dx%d", sizePreset, width, height)
		} else {
			log.Printf("⚠️ [IMAGE-EDIT] Unknown size preset '%s', using default 1024x1024", sizePreset)
		}
	}

	// Custom dimensions override presets
	if customWidth, ok := args["width"].(float64); ok && customWidth > 0 {
		if customHeight, ok := args["height"].(float64); ok && customHeight > 0 {
			width = int(customWidth)
			height = int(customHeight)
			// Clamp to reasonable limits
			if width < 64 {
				width = 64
			} else if width > 4096 {
				width = 4096
			}
			if height < 64 {
				height = 64
			} else if height > 4096 {
				height = 4096
			}
			log.Printf("🎨 [IMAGE-EDIT] Using custom dimensions: %dx%d", width, height)
		}
	}

	// Get injected context
	convID, _ := args["__conversation_id__"].(string)
	userID, _ := args["__user_id__"].(string)

	if convID == "" || userID == "" {
		return "", fmt.Errorf("missing conversation or user context")
	}

	// Get image edit config (injected by chat_service)
	editConfig, ok := args[ImageEditConfigKey].(*ImageEditConfig)
	if !ok || editConfig == nil {
		return "", fmt.Errorf("no image edit provider configured. Please add an image edit provider in providers.json")
	}

	// Get image registry (injected by chat_service)
	registry, ok := args[ImageRegistryKey].(ImageRegistryInterface)
	if !ok || registry == nil {
		return "", fmt.Errorf("image registry not available")
	}

	log.Printf("🎨 [IMAGE-EDIT] Editing image %s with prompt: %s (size: %dx%d)", imageID, truncateString(prompt, 100), width, height)

	// Look up the source image by handle
	entry := registry.GetByHandle(convID, imageID)
	if entry == nil {
		// Provide helpful error message with available handles
		handles := registry.ListHandles(convID)
		if len(handles) == 0 {
			return "", fmt.Errorf("image '%s' not found. No images are available in this conversation. Please upload an image first", imageID)
		}
		return "", fmt.Errorf("image '%s' not found. Available images: %s", imageID, strings.Join(handles, ", "))
	}

	// Get the file from cache
	fileCache := filecache.GetService()
	file, err := fileCache.GetByUserAndConversation(entry.FileID, userID, convID)
	if err != nil {
		return "", fmt.Errorf("source image has expired or is no longer available. Please upload the image again")
	}

	// Read the source image
	imageData, err := os.ReadFile(file.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read source image: %w", err)
	}

	// Encode to base64
	b64Image := base64.StdEncoding.EncodeToString(imageData)

	log.Printf("🎨 [IMAGE-EDIT] Source image loaded: %d bytes, calling Chutes API", len(imageData))

	// Call the Chutes edit API
	editedImageB64, err := callChutesEditAPI(editConfig, b64Image, prompt, negativePrompt, width, height)
	if err != nil {
		return "", fmt.Errorf("image editing failed: %w", err)
	}

	// Save the edited image to filecache
	newFileID, err := saveEditedImageToCache(editedImageB64, userID, convID)
	if err != nil {
		return "", fmt.Errorf("failed to save edited image: %w", err)
	}

	// Register the edited image in the registry
	newHandle := registry.RegisterEditedImage(convID, newFileID, imageID, prompt)

	log.Printf("✅ [IMAGE-EDIT] Successfully edited %s -> %s", imageID, newHandle)

	// Return result with plots for frontend rendering
	// Note: The base64 in plots is extracted by chat_service for image capture,
	// then replaced with "[N plot(s) generated]" before sending to LLM
	result := map[string]interface{}{
		"success":      true,
		"image_id":     newHandle,
		"file_id":      newFileID,
		"source_image": imageID,
		"prompt":       prompt,
		"plots": []map[string]interface{}{
			{
				"format": "png",
				"data":   editedImageB64,
			},
		},
		"message": fmt.Sprintf("Successfully edited image '%s'. The result is now available as '%s'. Original image '%s' is unchanged and can still be referenced.", imageID, newHandle, imageID),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// callChutesEditAPI calls the Chutes image edit API
func callChutesEditAPI(config *ImageEditConfig, sourceImageB64, prompt, negativePrompt string, width, height int) (string, error) {
	reqBody := ChutesImageEditRequest{
		Seed:              nil, // Random seed
		Width:             width,
		Height:            height,
		Prompt:            prompt,
		ImageB64s:         []string{sourceImageB64},
		TrueCFGScale:      4.0,
		NegativePrompt:    negativePrompt,
		NumInferenceSteps: 40,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := strings.TrimSuffix(config.BaseURL, "/") + "/generate"

	log.Printf("🎨 [IMAGE-EDIT] Calling Chutes API: %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	// Execute request with timeout (image editing can take a while)
	client := &http.Client{Timeout: 300 * time.Second} // 5 minutes for editing
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("image edit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [IMAGE-EDIT] Chutes API returned error (status %d): %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("image edit failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Chutes returns raw binary PNG, convert to base64
	if len(body) == 0 {
		return "", fmt.Errorf("no image data in response")
	}

	// Verify it looks like a PNG (starts with PNG magic bytes)
	if len(body) < 8 || string(body[1:4]) != "PNG" {
		log.Printf("⚠️ [IMAGE-EDIT] Response may not be PNG, first bytes: %v", body[:min(16, len(body))])
	}

	// Convert binary PNG to base64
	imageData := base64.StdEncoding.EncodeToString(body)
	log.Printf("✅ [IMAGE-EDIT] Converted %d bytes to base64 (%d chars)", len(body), len(imageData))

	return imageData, nil
}

// saveEditedImageToCache saves the edited image and returns the new file ID
func saveEditedImageToCache(imageB64, userID, conversationID string) (string, error) {
	// Decode base64
	imageData, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Generate unique file ID
	fileID := uuid.New().String()
	filename := fmt.Sprintf("%s.png", fileID)

	// Ensure uploads directory exists
	uploadsDir := "./uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create uploads directory: %w", err)
	}

	// Write to disk
	filePath := fmt.Sprintf("%s/%s", uploadsDir, filename)
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to write image: %w", err)
	}

	// Register with filecache using Store method
	fileCacheService := filecache.GetService()
	cachedFile := &filecache.CachedFile{
		FileID:         fileID,
		UserID:         userID,
		ConversationID: conversationID,
		FilePath:       filePath,
		Filename:       filename,
		MimeType:       "image/png",
		Size:           int64(len(imageData)),
		UploadedAt:     time.Now(),
	}
	fileCacheService.Store(cachedFile)

	log.Printf("💾 [IMAGE-EDIT] Saved edited image: %s (%d bytes)", fileID, len(imageData))

	return fileID, nil
}
