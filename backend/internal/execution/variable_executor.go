package execution

import (
	"clara-agents/internal/filecache"
	"clara-agents/internal/models"
	"clara-agents/internal/security"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileReference represents a file that can be passed between workflow blocks
type FileReference struct {
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Type     string `json:"type"` // "image", "document", "audio", "data"
}

// isFileReference checks if a value is a file reference (map with file_id)
func isFileReference(value any) bool {
	if m, ok := value.(map[string]any); ok {
		_, hasFileID := m["file_id"]
		return hasFileID
	}
	return false
}

// getFileType determines the file type category from MIME type
func getFileType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case mimeType == "application/pdf",
		mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		mimeType == "application/msword":
		return "document"
	case mimeType == "application/json",
		mimeType == "text/csv",
		mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		strings.HasPrefix(mimeType, "text/"):
		return "data"
	default:
		return "data"
	}
}

// validateFileReference validates a file reference and enriches it with metadata
func validateFileReference(value map[string]any, userID string) (*FileReference, error) {
	fileID, ok := value["file_id"].(string)
	if !ok || fileID == "" {
		return nil, fmt.Errorf("invalid file reference: missing file_id")
	}

	// SECURITY: Validate fileID to prevent path traversal attacks
	if err := security.ValidateFileID(fileID); err != nil {
		return nil, fmt.Errorf("invalid file reference: %w", err)
	}

	// Get file from cache service
	fileCacheService := filecache.GetService()
	file, found := fileCacheService.Get(fileID)

	// If not in cache, try to find on disk and restore cache entry
	if !found {
		log.Printf("⚠️ [VAR-EXEC] File %s not in cache, attempting disk recovery...", fileID)

		// Try to find the file on disk
		uploadDir := os.Getenv("UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = "./uploads"
		}

		// Try common extensions for data files
		extensions := []string{".csv", ".xlsx", ".xls", ".json", ".txt", ".png", ".jpg", ".jpeg", ""}
		var foundPath string
		var foundFilename string

		for _, ext := range extensions {
			testPath := filepath.Join(uploadDir, fileID+ext)
			if info, err := os.Stat(testPath); err == nil {
				foundPath = testPath
				foundFilename = fileID + ext
				log.Printf("✅ [VAR-EXEC] Found file on disk: %s (size: %d bytes)", testPath, info.Size())

				// Restore cache entry
				mimeType := getMimeTypeFromExtension(ext)
				cachedFile := &filecache.CachedFile{
					FileID:     fileID,
					UserID:     userID, // Use current user since original is unknown
					Filename:   foundFilename,
					MimeType:   mimeType,
					Size:       info.Size(),
					FilePath:   foundPath,
					UploadedAt: time.Now(),
				}
				fileCacheService.Store(cachedFile)
				file = cachedFile
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("file not found or has expired: %s (checked disk at %s)", fileID, uploadDir)
		}
	}

	// For workflow context, we allow access if userID matches or if no userID check is needed
	if userID != "" && file.UserID != "" && file.UserID != userID {
		return nil, fmt.Errorf("access denied: you don't have permission to access this file")
	}

	return &FileReference{
		FileID:   file.FileID,
		Filename: file.Filename,
		MimeType: file.MimeType,
		Size:     file.Size,
		Type:     getFileType(file.MimeType),
	}, nil
}

// getMimeTypeFromExtension returns MIME type based on file extension
func getMimeTypeFromExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".csv":
		return "text/csv"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// VariableExecutor executes variable blocks (read/set workflow variables)
type VariableExecutor struct{}

// NewVariableExecutor creates a new variable executor
func NewVariableExecutor() *VariableExecutor {
	return &VariableExecutor{}
}

// Execute runs a variable block
func (e *VariableExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	operation := getString(config, "operation", "read")
	variableName := getString(config, "variableName", "")

	// Extract user context for file validation
	userID, _ := inputs["__user_id__"].(string)

	if variableName == "" {
		return nil, fmt.Errorf("variableName is required for variable block")
	}

	log.Printf("📦 [VAR-EXEC] Block '%s': %s variable '%s'", block.Name, operation, variableName)

	switch operation {
	case "read":
		// Read from inputs (workflow variables are passed as inputs)
		value, ok := inputs[variableName]
		if !ok || value == nil || value == "" {
			// Check inputType in config to determine if we should use defaultValue (text) or fileValue (file)
			inputType := getString(config, "inputType", "text")

			if inputType == "file" {
				// Check for fileValue in config (used for Start block file input)
				if fileValue, hasFile := config["fileValue"]; hasFile && fileValue != nil {
					if fileMap, isMap := fileValue.(map[string]any); isMap {
						// Validate and use file reference
						fileRef, err := validateFileReference(fileMap, userID)
						if err != nil {
							log.Printf("⚠️ [VAR-EXEC] File reference validation failed: %v", err)
						} else {
							value = map[string]any{
								"file_id":   fileRef.FileID,
								"filename":  fileRef.Filename,
								"mime_type": fileRef.MimeType,
								"size":      fileRef.Size,
								"type":      fileRef.Type,
							}
							log.Printf("📁 [VAR-EXEC] Using fileValue for '%s': %s (%s)", variableName, fileRef.Filename, fileRef.Type)
							output := map[string]any{
								"value":      value,
								variableName: value,
							}
							log.Printf("🔍 [VAR-EXEC] Output keys: %v", getKeys(output))
							return output, nil
						}
					}
				}
			}

			// Check for defaultValue in config (used for Start block text input)
			defaultValue := getString(config, "defaultValue", "")
			if defaultValue != "" {
				log.Printf("📦 [VAR-EXEC] Using defaultValue for '%s': %s", variableName, defaultValue)
				output := map[string]any{
					"value":      defaultValue,
					variableName: defaultValue,
				}
				log.Printf("🔍 [VAR-EXEC] Output keys: %v", getKeys(output))
				return output, nil
			}
			log.Printf("⚠️ [VAR-EXEC] Variable '%s' not found and no defaultValue/fileValue, returning nil", variableName)
			output := map[string]any{
				"value":      nil,
				variableName: nil,
			}
			log.Printf("🔍 [VAR-EXEC] Output keys: %v", getKeys(output))
			return output, nil
		}

		// Handle file references - validate and enrich with metadata
		if isFileReference(value) {
			fileRef, err := validateFileReference(value.(map[string]any), userID)
			if err != nil {
				log.Printf("⚠️ [VAR-EXEC] File reference validation failed: %v", err)
				// Return the original value but log the warning
			} else {
				// Convert FileReference to map for downstream use
				value = map[string]any{
					"file_id":   fileRef.FileID,
					"filename":  fileRef.Filename,
					"mime_type": fileRef.MimeType,
					"size":      fileRef.Size,
					"type":      fileRef.Type,
				}
				log.Printf("📁 [VAR-EXEC] Validated file reference: %s (%s)", fileRef.Filename, fileRef.Type)
			}
		}

		log.Printf("✅ [VAR-EXEC] Read variable '%s': %v", variableName, value)
		output := map[string]any{
			"value":      value,
			variableName: value,
		}
		log.Printf("🔍 [VAR-EXEC] Output keys: %v", getKeys(output))
		return output, nil

	case "set":
		// Set/transform a value
		valueExpr := getString(config, "valueExpression", "")
		var value any

		if valueExpr != "" {
			// Resolve value from expression (path in inputs)
			value = ResolvePath(inputs, valueExpr)
		} else {
			// Check for a direct value in config
			if v, ok := config["value"]; ok {
				value = v
			}
		}

		// Handle file references - validate and enrich with metadata
		if isFileReference(value) {
			fileRef, err := validateFileReference(value.(map[string]any), userID)
			if err != nil {
				log.Printf("⚠️ [VAR-EXEC] File reference validation failed: %v", err)
				// Return the original value but log the warning
			} else {
				// Convert FileReference to map for downstream use
				value = map[string]any{
					"file_id":   fileRef.FileID,
					"filename":  fileRef.Filename,
					"mime_type": fileRef.MimeType,
					"size":      fileRef.Size,
					"type":      fileRef.Type,
				}
				log.Printf("📁 [VAR-EXEC] Validated file reference: %s (%s)", fileRef.Filename, fileRef.Type)
			}
		}

		log.Printf("✅ [VAR-EXEC] Set variable '%s' = %v", variableName, value)
		output := map[string]any{
			"value":      value,
			variableName: value,
		}
		log.Printf("🔍 [VAR-EXEC] Output keys: %v", getKeys(output))
		return output, nil

	default:
		return nil, fmt.Errorf("unknown variable operation: %s", operation)
	}
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
