package filecache

import (
	"clara-agents/internal/security"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// CachedFile represents a file stored in memory cache
type CachedFile struct {
	FileID         string
	UserID         string
	ConversationID string
	ExtractedText  *security.SecureString // For PDFs
	FileHash       security.Hash
	Filename       string
	MimeType       string
	Size           int64
	PageCount      int    // For PDFs
	WordCount      int    // For PDFs
	FilePath       string // For images (disk location)
	UploadedAt     time.Time
}

// Service manages uploaded files in memory
type Service struct {
	cache *cache.Cache
	mu    sync.RWMutex
}

var (
	instance *Service
	once     sync.Once
)

// GetService returns the singleton file cache service
func GetService() *Service {
	once.Do(func() {
		instance = NewService()
	})
	return instance
}

// NewService creates a new file cache service
func NewService() *Service {
	c := cache.New(30*time.Minute, 10*time.Minute)

	// Set eviction handler for secure wiping
	c.OnEvicted(func(key string, value interface{}) {
		if file, ok := value.(*CachedFile); ok {
			log.Printf("🗑️  [FILE-CACHE] Evicting file %s (%s) - secure wiping memory", file.FileID, file.Filename)
			file.SecureWipe()
		}
	})

	return &Service{
		cache: c,
	}
}

// Store stores a file in the cache
func (s *Service) Store(file *CachedFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache.Set(file.FileID, file, cache.DefaultExpiration)
	log.Printf("📦 [FILE-CACHE] Stored file %s (%s) - %d bytes, %d words",
		file.FileID, file.Filename, file.Size, file.WordCount)
}

// Get retrieves a file from the cache
func (s *Service) Get(fileID string) (*CachedFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, found := s.cache.Get(fileID)
	if !found {
		return nil, false
	}

	file, ok := value.(*CachedFile)
	if !ok {
		return nil, false
	}

	return file, true
}

// GetByUserAndConversation retrieves a file if it belongs to the user and conversation
func (s *Service) GetByUserAndConversation(fileID, userID, conversationID string) (*CachedFile, error) {
	file, found := s.Get(fileID)
	if !found {
		return nil, fmt.Errorf("file not found or expired")
	}

	// Verify ownership
	if file.UserID != userID {
		return nil, fmt.Errorf("access denied: file belongs to different user")
	}

	// Verify conversation
	if file.ConversationID != conversationID {
		return nil, fmt.Errorf("file belongs to different conversation")
	}

	return file, nil
}

// GetByUser retrieves a file if it belongs to the user (ignores conversation)
func (s *Service) GetByUser(fileID, userID string) (*CachedFile, error) {
	file, found := s.Get(fileID)
	if !found {
		return nil, fmt.Errorf("file not found or expired")
	}

	// Verify ownership
	if file.UserID != userID {
		return nil, fmt.Errorf("access denied: file belongs to different user")
	}

	return file, nil
}

// GetFilesForConversation returns all files for a conversation
func (s *Service) GetFilesForConversation(conversationID string) []*CachedFile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var files []*CachedFile
	for _, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.ConversationID == conversationID {
				files = append(files, file)
			}
		}
	}

	return files
}

// GetConversationFiles returns all file IDs for a conversation
func (s *Service) GetConversationFiles(conversationID string) []string {
	files := s.GetFilesForConversation(conversationID)
	fileIDs := make([]string, 0, len(files))
	for _, file := range files {
		fileIDs = append(fileIDs, file.FileID)
	}
	return fileIDs
}

// Delete removes a file from the cache and securely wipes it
func (s *Service) Delete(fileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get the file first to wipe it
	if value, found := s.cache.Get(fileID); found {
		if file, ok := value.(*CachedFile); ok {
			log.Printf("🗑️  [FILE-CACHE] Deleting file %s (%s)", file.FileID, file.Filename)
			file.SecureWipe()
		}
	}

	s.cache.Delete(fileID)
}

// DeleteConversationFiles deletes all files for a conversation
func (s *Service) DeleteConversationFiles(conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🗑️  [FILE-CACHE] Deleting all files for conversation %s", conversationID)

	for key, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.ConversationID == conversationID {
				file.SecureWipe()
				s.cache.Delete(key)
			}
		}
	}
}

// ExtendTTL extends the TTL of a file to match conversation lifetime
func (s *Service) ExtendTTL(fileID string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if value, found := s.cache.Get(fileID); found {
		s.cache.Set(fileID, value, duration)
		log.Printf("⏰ [FILE-CACHE] Extended TTL for file %s to %v", fileID, duration)
	}
}

// SecureWipe securely wipes the file's sensitive data
func (f *CachedFile) SecureWipe() {
	if f.ExtractedText != nil {
		f.ExtractedText.Wipe()
		f.ExtractedText = nil
	}

	// Delete physical file if it exists (for images)
	if f.FilePath != "" {
		if err := os.Remove(f.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("⚠️  Failed to delete file %s: %v", f.FilePath, err)
		} else {
			log.Printf("🗑️  Deleted file from disk: %s", f.FilePath)
		}
	}

	// Wipe hash
	for i := range f.FileHash {
		f.FileHash[i] = 0
	}

	// Clear other fields
	f.FileID = ""
	f.UserID = ""
	f.ConversationID = ""
	f.Filename = ""
	f.FilePath = ""
}

// CleanupExpiredFiles deletes files older than 1 hour
func (s *Service) CleanupExpiredFiles() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for key, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.FilePath != "" {
				if now.Sub(file.UploadedAt) > 1*time.Hour {
					log.Printf("🗑️  [FILE-CACHE] Deleting expired file: %s (uploaded %v ago)",
						file.Filename, now.Sub(file.UploadedAt))

					if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
						log.Printf("⚠️  Failed to delete expired file %s: %v", file.FilePath, err)
					}

					s.cache.Delete(key)
					expiredCount++
				}
			}
		}
	}

	if expiredCount > 0 {
		log.Printf("✅ [FILE-CACHE] Cleaned up %d expired files", expiredCount)
	}
}

// CleanupOrphanedFiles scans the uploads directory and deletes orphaned files
func (s *Service) CleanupOrphanedFiles(uploadDir string, maxAge time.Duration) {
	s.mu.RLock()
	trackedFiles := make(map[string]bool)
	for _, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.FilePath != "" {
				trackedFiles[file.FilePath] = true
			}
		}
	}
	s.mu.RUnlock()

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		log.Printf("⚠️  [CLEANUP] Failed to read uploads directory: %v", err)
		return
	}

	now := time.Now()
	orphanedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := fmt.Sprintf("%s/%s", uploadDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileAge := now.Sub(info.ModTime())

		if !trackedFiles[filePath] {
			if fileAge > 5*time.Minute {
				log.Printf("🗑️  [CLEANUP] Deleting orphaned file: %s (age: %v)", entry.Name(), fileAge)
				if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
					log.Printf("⚠️  [CLEANUP] Failed to delete orphaned file %s: %v", entry.Name(), err)
				} else {
					orphanedCount++
				}
			}
		}
	}

	if orphanedCount > 0 {
		log.Printf("✅ [CLEANUP] Deleted %d orphaned files", orphanedCount)
	}
}

// RunStartupCleanup performs initial cleanup when server starts
func (s *Service) RunStartupCleanup(uploadDir string) {
	log.Printf("🧹 [STARTUP] Running startup file cleanup in %s...", uploadDir)

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		log.Printf("⚠️  [STARTUP] Failed to read uploads directory: %v", err)
		return
	}

	now := time.Now()
	deletedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := fmt.Sprintf("%s/%s", uploadDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > 1*time.Hour {
			log.Printf("🗑️  [STARTUP] Deleting stale file: %s (modified: %v ago)",
				entry.Name(), now.Sub(info.ModTime()))
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Printf("⚠️  [STARTUP] Failed to delete file %s: %v", entry.Name(), err)
			} else {
				deletedCount++
			}
		}
	}

	log.Printf("✅ [STARTUP] Startup cleanup complete: deleted %d stale files", deletedCount)
}

// GetStats returns cache statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := s.cache.Items()
	totalSize := int64(0)
	totalWords := 0

	for _, item := range items {
		if file, ok := item.Object.(*CachedFile); ok {
			totalSize += file.Size
			totalWords += file.WordCount
		}
	}

	return map[string]interface{}{
		"total_files": len(items),
		"total_size":  totalSize,
		"total_words": totalWords,
	}
}

// GetAllFilesByUser returns metadata for all files owned by a user
func (s *Service) GetAllFilesByUser(userID string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var fileMetadata []map[string]interface{}

	for _, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.UserID == userID {
				metadata := map[string]interface{}{
					"file_id":         file.FileID,
					"filename":        file.Filename,
					"mime_type":       file.MimeType,
					"size":            file.Size,
					"uploaded_at":     file.UploadedAt.Format(time.RFC3339),
					"conversation_id": file.ConversationID,
				}

				if file.MimeType == "application/pdf" {
					metadata["page_count"] = file.PageCount
					metadata["word_count"] = file.WordCount
				}

				fileMetadata = append(fileMetadata, metadata)
			}
		}
	}

	return fileMetadata
}

// DeleteAllFilesByUser deletes all files owned by a user
func (s *Service) DeleteAllFilesByUser(userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deletedCount := 0

	for key, item := range s.cache.Items() {
		if file, ok := item.Object.(*CachedFile); ok {
			if file.UserID == userID {
				log.Printf("🗑️  [GDPR] Deleting file %s (%s) for user %s", file.FileID, file.Filename, userID)
				file.SecureWipe()
				s.cache.Delete(key)
				deletedCount++
			}
		}
	}

	log.Printf("✅ [GDPR] Deleted %d files for user %s", deletedCount, userID)
	return deletedCount, nil
}
