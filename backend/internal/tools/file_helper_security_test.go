package tools

import (
	"clara-agents/internal/security"
	"strings"
	"testing"
)

// TestValidateFileID_PathTraversalProtection tests that path traversal attempts are blocked
func TestValidateFileID_PathTraversalProtection(t *testing.T) {
	maliciousIDs := []struct {
		fileID      string
		description string
	}{
		{"../../../etc/passwd", "path traversal with .."},
		{"..\\..\\..\\windows\\system32\\config\\sam", "windows path traversal"},
		{"/etc/passwd", "absolute path with /"},
		{"C:\\Windows\\System.ini", "windows absolute path"},
		{"file.txt/../../../etc/passwd", "path traversal in middle"},
		{"aaaa-bbbb-cccc-dddd/../etc/passwd", "UUID-like with path traversal"},
		{".", "current directory"},
		{"..", "parent directory"},
		{"./uploads/file.csv", "relative path"},
		{"uploads/file.csv", "subdirectory traversal"},
	}

	for _, tc := range maliciousIDs {
		t.Run(tc.description, func(t *testing.T) {
			err := security.ValidateFileID(tc.fileID)
			if err == nil {
				t.Errorf("Expected error for malicious file_id %q (%s), but got none", tc.fileID, tc.description)
			}
			if !strings.Contains(err.Error(), "invalid file_id") && !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("Expected 'invalid file_id' or 'path traversal' in error message, got: %v", err)
			}
		})
	}
}

// TestValidateFileID_ValidUUIDs tests that valid UUIDs are accepted
func TestValidateFileID_ValidUUIDs(t *testing.T) {
	validUUIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000", // UUID v4
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // UUID v1
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890", // lowercase hex
		"A1B2C3D4-E5F6-7890-ABCD-EF1234567890", // uppercase hex
		"12345678-1234-5678-1234-567890abcdef", // mixed case
	}

	for _, uuid := range validUUIDs {
		t.Run(uuid, func(t *testing.T) {
			err := security.ValidateFileID(uuid)
			if err != nil {
				t.Errorf("Expected valid UUID %q to be accepted, but got error: %v", uuid, err)
			}
		})
	}
}

// TestValidateFileID_InvalidFormats tests that invalid UUID formats are rejected
func TestValidateFileID_InvalidFormats(t *testing.T) {
	invalidFormats := []struct {
		fileID      string
		description string
	}{
		{"", "empty string"},
		{"not-a-uuid", "random string"},
		{"550e8400-e29b-41d4-a716", "too short (missing segment)"},
		{"550e8400-e29b-41d4-a716-446655440000-extra", "too long"},
		{"550e8400e29b41d4a716446655440000", "no hyphens"},
		{"550e8400-e29b-41d4-a716-44665544000g", "invalid hex character"},
		{"550e8400_e29b_41d4_a716_446655440000", "underscores instead of hyphens"},
	}

	for _, tc := range invalidFormats {
		t.Run(tc.description, func(t *testing.T) {
			err := security.ValidateFileID(tc.fileID)
			if err == nil {
				t.Errorf("Expected error for invalid format %q (%s), but got none", tc.fileID, tc.description)
			}
		})
	}
}

// TestGetUploadedFile_PathTraversalBlocked tests that GetUploadedFile blocks path traversal
func TestGetUploadedFile_PathTraversalBlocked(t *testing.T) {
	// Test that path traversal attempts are blocked before file system access
	maliciousID := "../../../etc/passwd"

	content, filename, err := GetUploadedFile(maliciousID)

	if err == nil {
		t.Errorf("Expected error for path traversal attempt, but got content: %v, filename: %s", content, filename)
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected 'path traversal' in error message, got: %v", err)
	}

	if content != nil {
		t.Errorf("Expected nil content for blocked request, got: %v", content)
	}

	if filename != "" {
		t.Errorf("Expected empty filename for blocked request, got: %s", filename)
	}
}

// TestValidateCSVFile_PathTraversalBlocked tests that ValidateCSVFile blocks path traversal
func TestValidateCSVFile_PathTraversalBlocked(t *testing.T) {
	// Test that path traversal attempts are blocked in CSV validation
	maliciousID := "../../database/users.db"

	err := ValidateCSVFile(maliciousID)

	if err == nil {
		t.Error("Expected error for path traversal attempt in ValidateCSVFile")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected 'path traversal' in error message, got: %v", err)
	}
}

// TestValidateFileID_EmptyString tests that empty string is rejected
func TestValidateFileID_EmptyString(t *testing.T) {
	err := security.ValidateFileID("")
	if err == nil {
		t.Error("Expected error for empty file_id")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' in error message, got: %v", err)
	}
}
