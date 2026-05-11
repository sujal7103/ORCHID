package utils

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ImageUtils provides image processing utilities
type ImageUtils struct{}

// NewImageUtils creates a new ImageUtils instance
func NewImageUtils() *ImageUtils {
	return &ImageUtils{}
}

// EncodeToBase64 reads an image file and encodes it to base64
func (u *ImageUtils) EncodeToBase64(filePath string) (string, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read file contents
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// Get MIME type from extension
	mimeType := u.GetMimeTypeFromExtension(filepath.Ext(filePath))

	// Return data URL format
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

// GetMimeTypeFromExtension returns MIME type for a file extension
func (u *ImageUtils) GetMimeTypeFromExtension(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// IsValidImageExtension checks if the file extension is a valid image type
func (u *ImageUtils) IsValidImageExtension(ext string) bool {
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	return validExts[ext]
}

// GetFileSize returns the size of a file in bytes
func (u *ImageUtils) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// DeleteFile removes a file from the filesystem
func (u *ImageUtils) DeleteFile(filePath string) error {
	return os.Remove(filePath)
}

// FileExists checks if a file exists
func (u *ImageUtils) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
