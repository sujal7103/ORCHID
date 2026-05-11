package securefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewService tests service creation
func TestNewService(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	if svc == nil {
		t.Fatal("NewService should return non-nil service")
	}

	if svc.storageDir != tempDir {
		t.Errorf("Expected storage dir %s, got %s", tempDir, svc.storageDir)
	}
}

// TestCreateFile tests file creation with access code
func TestCreateFile(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	content := []byte("test file content")
	userID := "user-123"
	filename := "test.txt"
	mimeType := "text/plain"

	result, err := svc.CreateFile(userID, content, filename, mimeType)
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// Verify result fields
	if result.ID == "" {
		t.Error("File ID should not be empty")
	}
	if result.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, result.Filename)
	}
	if result.AccessCode == "" {
		t.Error("Access code should not be empty")
	}
	if len(result.AccessCode) != 32 {
		t.Errorf("Access code should be 32 chars, got %d", len(result.AccessCode))
	}
	if result.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.Size)
	}
	if result.MimeType != mimeType {
		t.Errorf("Expected mime type %s, got %s", mimeType, result.MimeType)
	}
	if result.ExpiresAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Error("Expiration should be ~30 days from now")
	}

	// Verify download URL format (should contain the API path and code)
	expectedURLSuffix := "/api/files/" + result.ID + "?code=" + result.AccessCode
	if len(result.DownloadURL) < len(expectedURLSuffix) {
		t.Errorf("Download URL too short: %s", result.DownloadURL)
	}
	// Check that URL ends with the expected path (ignoring the host prefix)
	if result.DownloadURL[len(result.DownloadURL)-len(expectedURLSuffix):] != expectedURLSuffix {
		t.Errorf("Download URL format incorrect: %s (expected to end with %s)", result.DownloadURL, expectedURLSuffix)
	}
}

// TestGetFile tests retrieving file with valid access code
func TestGetFile(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	content := []byte("secret content")
	userID := "user-456"
	filename := "secret.txt"

	result, err := svc.CreateFile(userID, content, filename, "text/plain")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// Retrieve with valid access code
	file, retrievedContent, err := svc.GetFile(result.ID, result.AccessCode)
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}

	if file.ID != result.ID {
		t.Errorf("Expected file ID %s, got %s", result.ID, file.ID)
	}
	if file.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, file.Filename)
	}
	if string(retrievedContent) != string(content) {
		t.Errorf("Content mismatch: expected %s, got %s", content, retrievedContent)
	}
}

// TestGetFileInvalidAccessCode tests access code validation
func TestGetFileInvalidAccessCode(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	content := []byte("protected content")
	result, _ := svc.CreateFile("user-789", content, "protected.txt", "text/plain")

	// Try with wrong access code
	_, _, err := svc.GetFile(result.ID, "wrong-access-code")
	if err == nil {
		t.Error("Expected error for invalid access code")
	}
	if err.Error() != "invalid access code" {
		t.Errorf("Expected 'invalid access code' error, got: %v", err)
	}
}

// TestGetFileNotFound tests file not found error
func TestGetFileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	_, _, err := svc.GetFile("non-existent-id", "some-code")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if err.Error() != "file not found or expired" {
		t.Errorf("Expected 'file not found or expired' error, got: %v", err)
	}
}

// TestGetFileInfo tests retrieving file metadata
func TestGetFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	content := []byte("info test content")
	filename := "info-test.json"
	mimeType := "application/json"

	result, _ := svc.CreateFile("user-info", content, filename, mimeType)

	// Get info with valid access code
	file, err := svc.GetFileInfo(result.ID, result.AccessCode)
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}

	if file.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, file.Filename)
	}
	if file.MimeType != mimeType {
		t.Errorf("Expected mime type %s, got %s", mimeType, file.MimeType)
	}
	if file.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), file.Size)
	}
}

// TestGetFileInfoInvalidCode tests GetFileInfo with invalid access code
func TestGetFileInfoInvalidCode(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	result, _ := svc.CreateFile("user-x", []byte("data"), "file.txt", "text/plain")

	_, err := svc.GetFileInfo(result.ID, "bad-code")
	if err == nil {
		t.Error("Expected error for invalid access code")
	}
}

// TestDeleteFile tests file deletion with ownership check
func TestDeleteFile(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	userID := "owner-user"
	content := []byte("deletable content")

	result, _ := svc.CreateFile(userID, content, "delete-me.txt", "text/plain")

	// Verify file exists on disk
	files, _ := os.ReadDir(tempDir)
	initialCount := len(files)

	// Delete with correct owner
	err := svc.DeleteFile(result.ID, userID)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file removed from disk
	files, _ = os.ReadDir(tempDir)
	if len(files) != initialCount-1 {
		t.Error("File should be deleted from disk")
	}

	// Verify file removed from cache
	_, _, err = svc.GetFile(result.ID, result.AccessCode)
	if err == nil {
		t.Error("File should not be accessible after deletion")
	}
}

// TestDeleteFileWrongOwner tests deletion by non-owner
func TestDeleteFileWrongOwner(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	result, _ := svc.CreateFile("owner-a", []byte("data"), "owned.txt", "text/plain")

	// Try to delete with wrong owner
	err := svc.DeleteFile(result.ID, "owner-b")
	if err == nil {
		t.Error("Expected error for non-owner deletion")
	}
	if err.Error() != "access denied" {
		t.Errorf("Expected 'access denied' error, got: %v", err)
	}
}

// TestDeleteFileNotFound tests deleting non-existent file
func TestDeleteFileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	err := svc.DeleteFile("non-existent", "user-1")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// TestListUserFiles tests listing files for a user
func TestListUserFiles(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	userA := "user-a"
	userB := "user-b"

	// Create files for user A
	svc.CreateFile(userA, []byte("file1"), "file1.txt", "text/plain")
	svc.CreateFile(userA, []byte("file2"), "file2.txt", "text/plain")

	// Create file for user B
	svc.CreateFile(userB, []byte("file3"), "file3.txt", "text/plain")

	// List user A's files
	filesA := svc.ListUserFiles(userA)
	if len(filesA) != 2 {
		t.Errorf("Expected 2 files for user A, got %d", len(filesA))
	}

	// List user B's files
	filesB := svc.ListUserFiles(userB)
	if len(filesB) != 1 {
		t.Errorf("Expected 1 file for user B, got %d", len(filesB))
	}

	// List non-existent user's files
	filesC := svc.ListUserFiles("user-c")
	if len(filesC) != 0 {
		t.Errorf("Expected 0 files for user C, got %d", len(filesC))
	}
}

// TestGetStats tests service statistics
func TestGetStats(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	// Create some files
	svc.CreateFile("user-1", []byte("content 1"), "file1.txt", "text/plain")
	svc.CreateFile("user-2", []byte("content 2 longer"), "file2.txt", "text/plain")

	stats := svc.GetStats()

	totalFiles, ok := stats["total_files"].(int)
	if !ok || totalFiles != 2 {
		t.Errorf("Expected 2 total files, got %v", stats["total_files"])
	}

	totalSize, ok := stats["total_size"].(int64)
	if !ok || totalSize != int64(len("content 1")+len("content 2 longer")) {
		t.Errorf("Expected total size %d, got %v", len("content 1")+len("content 2 longer"), stats["total_size"])
	}

	storageDir, ok := stats["storage_dir"].(string)
	if !ok || storageDir != tempDir {
		t.Errorf("Expected storage dir %s, got %v", tempDir, stats["storage_dir"])
	}
}

// TestAccessCodeGeneration tests access code is cryptographically random
func TestAccessCodeGeneration(t *testing.T) {
	codes := make(map[string]bool)

	// Generate many codes and check uniqueness
	for i := 0; i < 100; i++ {
		code, err := generateAccessCode()
		if err != nil {
			t.Fatalf("generateAccessCode failed: %v", err)
		}

		if len(code) != 32 {
			t.Errorf("Access code length should be 32, got %d", len(code))
		}

		if codes[code] {
			t.Error("Duplicate access code generated")
		}
		codes[code] = true
	}
}

// TestAccessCodeHashing tests SHA256 hashing
func TestAccessCodeHashing(t *testing.T) {
	code := "test-access-code-12345678"
	hash := hashAccessCode(code)

	// SHA256 produces 64-char hex string
	if len(hash) != 64 {
		t.Errorf("Hash length should be 64, got %d", len(hash))
	}

	// Same input should produce same hash
	hash2 := hashAccessCode(code)
	if hash != hash2 {
		t.Error("Same input should produce same hash")
	}

	// Different input should produce different hash
	hash3 := hashAccessCode("different-code")
	if hash == hash3 {
		t.Error("Different inputs should produce different hashes")
	}
}

// TestMimeTypeExtensionMapping tests extension detection from MIME type
func TestMimeTypeExtensionMapping(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected string
	}{
		{"application/pdf", ".pdf"},
		{"application/json", ".json"},
		{"text/plain", ".txt"},
		{"text/csv", ".csv"},
		{"text/html", ".html"},
		{"unknown/type", ".bin"},
	}

	for _, tc := range testCases {
		result := getExtensionFromMimeType(tc.mimeType)
		if result != tc.expected {
			t.Errorf("getExtensionFromMimeType(%s) = %s, expected %s", tc.mimeType, result, tc.expected)
		}
	}
}

// TestFileExtensionFromFilename tests extension detection from filename
func TestFileExtensionFromFilename(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	// Create file with extension in filename
	result, _ := svc.CreateFile("user", []byte("data"), "document.pdf", "application/pdf")

	// Verify file was created with correct extension
	files, _ := filepath.Glob(filepath.Join(tempDir, result.ID+"*"))
	if len(files) != 1 {
		t.Fatal("Expected exactly one file")
	}

	if filepath.Ext(files[0]) != ".pdf" {
		t.Errorf("Expected .pdf extension, got %s", filepath.Ext(files[0]))
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewService(tempDir)

	done := make(chan bool, 10)

	// Create files concurrently
	for i := 0; i < 10; i++ {
		go func(idx int) {
			_, err := svc.CreateFile("concurrent-user", []byte("data"), "concurrent.txt", "text/plain")
			if err != nil {
				t.Errorf("Concurrent CreateFile failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all files created
	files := svc.ListUserFiles("concurrent-user")
	if len(files) != 10 {
		t.Errorf("Expected 10 files, got %d", len(files))
	}
}

// Benchmark tests
func BenchmarkCreateFile(b *testing.B) {
	tempDir := b.TempDir()
	svc := NewService(tempDir)
	content := []byte("benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.CreateFile("bench-user", content, "bench.txt", "text/plain")
	}
}

func BenchmarkGetFile(b *testing.B) {
	tempDir := b.TempDir()
	svc := NewService(tempDir)
	content := []byte("benchmark content")

	result, _ := svc.CreateFile("bench-user", content, "bench.txt", "text/plain")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetFile(result.ID, result.AccessCode)
	}
}

func BenchmarkAccessCodeGeneration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateAccessCode()
	}
}

func BenchmarkAccessCodeHashing(b *testing.B) {
	code := "test-access-code-12345678"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashAccessCode(code)
	}
}
