package tools

import (
	"clara-agents/internal/security"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetUploadedFile retrieves an uploaded file by its ID
// Returns: file content, filename, error
func GetUploadedFile(fileID string) ([]byte, string, error) {
	// SECURITY: Validate fileID to prevent path traversal attacks
	if err := security.ValidateFileID(fileID); err != nil {
		return nil, "", err
	}
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	// Try common extensions for uploaded files
	extensions := []string{
		".csv",
		".xlsx",
		".xls",
		".json",
		".txt",
		"", // No extension
	}

	for _, ext := range extensions {
		filePath := filepath.Join(uploadDir, fileID+ext)

		// Check if file exists
		if _, err := os.Stat(filePath); err == nil {
			// File found, read it
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read file: %w", err)
			}

			filename := fileID + ext
			if ext == "" {
				filename = fileID
			}

			return content, filename, nil
		}
	}

	// File not found with any extension
	return nil, "", fmt.Errorf("file not found: %s", fileID)
}

// ValidateCSVFile checks if a file ID points to a valid CSV file
func ValidateCSVFile(fileID string) error {
	// SECURITY: Validate fileID to prevent path traversal attacks
	if err := security.ValidateFileID(fileID); err != nil {
		return err
	}

	// Check if file exists
	_, _, err := GetUploadedFile(fileID)
	if err != nil {
		return fmt.Errorf("invalid file_id: %w", err)
	}

	return nil
}

// SupportedFileType checks if a filename has a supported extension for data analysis
func SupportedFileType(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{
		".csv":  true,
		".xlsx": true,
		".xls":  true,
		".json": true,
		".txt":  true,
	}
	return supported[ext]
}
