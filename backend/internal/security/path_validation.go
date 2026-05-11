package security

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateFileID validates that a file ID is a valid UUID and contains no path traversal sequences.
// This prevents path traversal attacks like "../../../etc/passwd" or absolute paths.
//
// Returns an error if the fileID:
//   - Is empty
//   - Contains path traversal sequences (.., /, \)
//   - Is not a valid UUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
//
// This function should be called before using any user-provided file ID in file system operations.
func ValidateFileID(fileID string) error {
	if fileID == "" {
		return fmt.Errorf("file_id cannot be empty")
	}

	// Check for path traversal sequences
	if strings.Contains(fileID, "..") {
		return fmt.Errorf("invalid file_id: path traversal attempt detected (..)")
	}
	if strings.Contains(fileID, "/") {
		return fmt.Errorf("invalid file_id: path traversal attempt detected (/)")
	}
	if strings.Contains(fileID, "\\") {
		return fmt.Errorf("invalid file_id: path traversal attempt detected (\\)")
	}

	// Validate UUID format (standard UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	// UUIDs are always 36 characters with hyphens at positions 8, 13, 18, 23
	// This regex matches UUID v1, v4, and other valid UUID formats
	uuidPattern := regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	if !uuidPattern.MatchString(fileID) {
		return fmt.Errorf("invalid file_id format: expected UUID (got %q)", fileID)
	}

	return nil
}
