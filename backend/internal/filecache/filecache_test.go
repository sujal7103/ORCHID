package filecache

import (
	"clara-agents/internal/security"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewService verifies service creation
func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService should return non-nil service")
	}
	if svc.cache == nil {
		t.Error("Service should have cache initialized")
	}
}

// TestGetServiceSingleton verifies singleton pattern
func TestGetServiceSingleton(t *testing.T) {
	svc1 := GetService()
	svc2 := GetService()

	if svc1 != svc2 {
		t.Error("GetService should return the same instance")
	}
}

// TestStoreAndGet tests basic store and retrieve
func TestStoreAndGet(t *testing.T) {
	svc := NewService()

	file := &CachedFile{
		FileID:         "test-file-123",
		UserID:         "user-456",
		ConversationID: "conv-789",
		Filename:       "test.pdf",
		MimeType:       "application/pdf",
		Size:           1024,
		UploadedAt:     time.Now(),
	}

	svc.Store(file)

	retrieved, found := svc.Get("test-file-123")
	if !found {
		t.Fatal("File should be found after store")
	}

	if retrieved.FileID != file.FileID {
		t.Errorf("Expected FileID %s, got %s", file.FileID, retrieved.FileID)
	}
	if retrieved.UserID != file.UserID {
		t.Errorf("Expected UserID %s, got %s", file.UserID, retrieved.UserID)
	}
	if retrieved.Filename != file.Filename {
		t.Errorf("Expected Filename %s, got %s", file.Filename, retrieved.Filename)
	}
}

// TestGetNotFound tests retrieval of non-existent file
func TestGetNotFound(t *testing.T) {
	svc := NewService()

	_, found := svc.Get("non-existent-file")
	if found {
		t.Error("Non-existent file should not be found")
	}
}

// TestGetByUser tests user-scoped retrieval
func TestGetByUser(t *testing.T) {
	svc := NewService()

	file := &CachedFile{
		FileID:   "file-for-user",
		UserID:   "user-123",
		Filename: "user-file.pdf",
	}
	svc.Store(file)

	// Same user should be able to retrieve
	retrieved, err := svc.GetByUser("file-for-user", "user-123")
	if err != nil {
		t.Errorf("Same user should retrieve file: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved file should not be nil")
	}

	// Different user should be denied
	_, err = svc.GetByUser("file-for-user", "different-user")
	if err == nil {
		t.Error("Different user should be denied access")
	}
}

// TestGetByUserAndConversation tests user+conversation scoped retrieval
func TestGetByUserAndConversation(t *testing.T) {
	svc := NewService()

	file := &CachedFile{
		FileID:         "conv-file",
		UserID:         "user-123",
		ConversationID: "conv-456",
		Filename:       "conv-file.pdf",
	}
	svc.Store(file)

	// Same user + conversation should work
	retrieved, err := svc.GetByUserAndConversation("conv-file", "user-123", "conv-456")
	if err != nil {
		t.Errorf("Same user+conversation should retrieve file: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved file should not be nil")
	}

	// Same user, different conversation should fail
	_, err = svc.GetByUserAndConversation("conv-file", "user-123", "different-conv")
	if err == nil {
		t.Error("Different conversation should be denied")
	}

	// Different user, same conversation should fail
	_, err = svc.GetByUserAndConversation("conv-file", "different-user", "conv-456")
	if err == nil {
		t.Error("Different user should be denied")
	}
}

// TestDelete tests file deletion
func TestDelete(t *testing.T) {
	svc := NewService()

	file := &CachedFile{
		FileID:   "delete-me",
		UserID:   "user-123",
		Filename: "to-delete.pdf",
	}
	svc.Store(file)

	// Verify it exists
	_, found := svc.Get("delete-me")
	if !found {
		t.Fatal("File should exist before deletion")
	}

	// Delete it
	svc.Delete("delete-me")

	// Verify it's gone
	_, found = svc.Get("delete-me")
	if found {
		t.Error("File should not exist after deletion")
	}
}

// TestDeleteConversationFiles tests conversation-level deletion
func TestDeleteConversationFiles(t *testing.T) {
	svc := NewService()

	// Store multiple files for same conversation
	for i := 0; i < 3; i++ {
		svc.Store(&CachedFile{
			FileID:         "conv-file-" + string(rune('a'+i)),
			UserID:         "user-123",
			ConversationID: "conv-to-delete",
			Filename:       "file.pdf",
		})
	}

	// Store file for different conversation
	svc.Store(&CachedFile{
		FileID:         "other-file",
		UserID:         "user-123",
		ConversationID: "other-conv",
		Filename:       "other.pdf",
	})

	// Delete conversation files
	svc.DeleteConversationFiles("conv-to-delete")

	// Conversation files should be gone
	files := svc.GetFilesForConversation("conv-to-delete")
	if len(files) != 0 {
		t.Errorf("Expected 0 files after deletion, got %d", len(files))
	}

	// Other conversation's file should remain
	_, found := svc.Get("other-file")
	if !found {
		t.Error("File from other conversation should still exist")
	}
}

// TestGetFilesForConversation tests conversation-level retrieval
func TestGetFilesForConversation(t *testing.T) {
	svc := NewService()

	targetConv := "target-conv"

	// Store files for target conversation
	for i := 0; i < 3; i++ {
		svc.Store(&CachedFile{
			FileID:         "target-file-" + string(rune('a'+i)),
			UserID:         "user-123",
			ConversationID: targetConv,
			Filename:       "file.pdf",
		})
	}

	// Store files for other conversation
	svc.Store(&CachedFile{
		FileID:         "other-file",
		UserID:         "user-123",
		ConversationID: "other-conv",
		Filename:       "other.pdf",
	})

	files := svc.GetFilesForConversation(targetConv)
	if len(files) != 3 {
		t.Errorf("Expected 3 files for conversation, got %d", len(files))
	}

	for _, file := range files {
		if file.ConversationID != targetConv {
			t.Errorf("File %s has wrong conversation %s", file.FileID, file.ConversationID)
		}
	}
}

// TestGetConversationFiles tests file ID retrieval
func TestGetConversationFiles(t *testing.T) {
	svc := NewService()

	conv := "my-conv"
	expectedIDs := []string{"file-1", "file-2", "file-3"}

	for _, id := range expectedIDs {
		svc.Store(&CachedFile{
			FileID:         id,
			ConversationID: conv,
		})
	}

	fileIDs := svc.GetConversationFiles(conv)
	if len(fileIDs) != len(expectedIDs) {
		t.Errorf("Expected %d file IDs, got %d", len(expectedIDs), len(fileIDs))
	}
}

// TestExtendTTL tests TTL extension
func TestExtendTTL(t *testing.T) {
	svc := NewService()

	file := &CachedFile{
		FileID:   "ttl-file",
		Filename: "ttl.pdf",
	}
	svc.Store(file)

	// Extend TTL (should not error)
	svc.ExtendTTL("ttl-file", 1*time.Hour)

	// Verify file still exists
	_, found := svc.Get("ttl-file")
	if !found {
		t.Error("File should still exist after TTL extension")
	}

	// Extend non-existent file (should not error)
	svc.ExtendTTL("non-existent", 1*time.Hour)
}

// TestSecureWipe tests secure wiping
func TestSecureWipe(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "wipe-test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	file := &CachedFile{
		FileID:        "wipe-file",
		UserID:        "user-123",
		Filename:      "wipe-test.txt",
		FilePath:      tmpFile,
		ExtractedText: security.NewSecureString("sensitive text"),
		FileHash:      security.Hash{1, 2, 3, 4, 5},
	}

	file.SecureWipe()

	// Verify memory is cleared
	if file.FileID != "" {
		t.Error("FileID should be cleared")
	}
	if file.UserID != "" {
		t.Error("UserID should be cleared")
	}
	if file.ExtractedText != nil {
		t.Error("ExtractedText should be nil")
	}

	// Verify hash is zeroed
	for i, b := range file.FileHash {
		if b != 0 {
			t.Errorf("FileHash[%d] should be 0, got %d", i, b)
		}
	}

	// Verify physical file is deleted
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Physical file should be deleted")
	}
}

// TestGetStats tests statistics retrieval
func TestGetStats(t *testing.T) {
	svc := NewService()

	// Store some files
	svc.Store(&CachedFile{
		FileID:    "stats-file-1",
		Size:      1000,
		WordCount: 100,
	})
	svc.Store(&CachedFile{
		FileID:    "stats-file-2",
		Size:      2000,
		WordCount: 200,
	})

	stats := svc.GetStats()

	if stats["total_files"].(int) != 2 {
		t.Errorf("Expected 2 files, got %v", stats["total_files"])
	}
	if stats["total_size"].(int64) != 3000 {
		t.Errorf("Expected total size 3000, got %v", stats["total_size"])
	}
	if stats["total_words"].(int) != 300 {
		t.Errorf("Expected 300 words, got %v", stats["total_words"])
	}
}

// TestGetAllFilesByUser tests user file listing
func TestGetAllFilesByUser(t *testing.T) {
	svc := NewService()

	targetUser := "target-user"

	// Store files for target user
	svc.Store(&CachedFile{
		FileID:     "user-file-1",
		UserID:     targetUser,
		Filename:   "file1.pdf",
		MimeType:   "application/pdf",
		Size:       1000,
		UploadedAt: time.Now(),
	})
	svc.Store(&CachedFile{
		FileID:     "user-file-2",
		UserID:     targetUser,
		Filename:   "file2.jpg",
		MimeType:   "image/jpeg",
		Size:       2000,
		UploadedAt: time.Now(),
	})

	// Store file for different user
	svc.Store(&CachedFile{
		FileID:   "other-file",
		UserID:   "other-user",
		Filename: "other.pdf",
	})

	files := svc.GetAllFilesByUser(targetUser)
	if len(files) != 2 {
		t.Errorf("Expected 2 files for user, got %d", len(files))
	}

	for _, metadata := range files {
		if metadata["file_id"] == "other-file" {
			t.Error("Should not return files from other users")
		}
	}
}

// TestDeleteAllFilesByUser tests GDPR-style user data deletion
func TestDeleteAllFilesByUser(t *testing.T) {
	svc := NewService()

	targetUser := "delete-user"

	// Store files for target user
	for i := 0; i < 5; i++ {
		svc.Store(&CachedFile{
			FileID:   "delete-file-" + string(rune('a'+i)),
			UserID:   targetUser,
			Filename: "file.pdf",
		})
	}

	// Store file for different user
	svc.Store(&CachedFile{
		FileID:   "keep-file",
		UserID:   "other-user",
		Filename: "keep.pdf",
	})

	deleted, err := svc.DeleteAllFilesByUser(targetUser)
	if err != nil {
		t.Errorf("DeleteAllFilesByUser should not error: %v", err)
	}
	if deleted != 5 {
		t.Errorf("Expected 5 files deleted, got %d", deleted)
	}

	// Verify target user's files are gone
	files := svc.GetAllFilesByUser(targetUser)
	if len(files) != 0 {
		t.Errorf("Expected 0 files for deleted user, got %d", len(files))
	}

	// Verify other user's file remains
	_, found := svc.Get("keep-file")
	if !found {
		t.Error("Other user's file should still exist")
	}
}

// TestCachedFileStructure tests CachedFile struct
func TestCachedFileStructure(t *testing.T) {
	file := &CachedFile{
		FileID:         "test-id",
		UserID:         "user-id",
		ConversationID: "conv-id",
		Filename:       "test.pdf",
		MimeType:       "application/pdf",
		Size:           12345,
		PageCount:      10,
		WordCount:      500,
		FilePath:       "/tmp/test.pdf",
		UploadedAt:     time.Now(),
	}

	if file.FileID == "" {
		t.Error("FileID should be set")
	}
	if file.MimeType != "application/pdf" {
		t.Errorf("MimeType should be application/pdf, got %s", file.MimeType)
	}
}

// Benchmark tests
func BenchmarkStore(b *testing.B) {
	svc := NewService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Store(&CachedFile{
			FileID:   "bench-file",
			UserID:   "user",
			Filename: "bench.pdf",
		})
	}
}

func BenchmarkGet(b *testing.B) {
	svc := NewService()
	svc.Store(&CachedFile{
		FileID:   "bench-get-file",
		UserID:   "user",
		Filename: "bench.pdf",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.Get("bench-get-file")
	}
}

func BenchmarkGetByUser(b *testing.B) {
	svc := NewService()
	svc.Store(&CachedFile{
		FileID:   "bench-user-file",
		UserID:   "target-user",
		Filename: "bench.pdf",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetByUser("bench-user-file", "target-user")
	}
}
