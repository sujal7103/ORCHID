package services

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ImageEntry represents a registered image in a conversation
type ImageEntry struct {
	Handle       string    // "img-1", "img-2", etc.
	FileID       string    // UUID from filecache
	Filename     string    // Original or generated name
	Source       string    // "uploaded", "generated", "edited"
	SourceHandle string    // For edited images, which image was source
	Prompt       string    // For generated/edited images, the prompt used
	Width        int       // Image width (if known)
	Height       int       // Image height (if known)
	CreatedAt    time.Time // When this entry was created
}

// ImageRegistry holds the image entries for a single conversation
type ImageRegistry struct {
	entries  map[string]*ImageEntry // handle -> entry
	byFileID map[string]string      // fileID -> handle (reverse lookup)
	counter  int                    // for generating handles
	mutex    sync.RWMutex
}

// ImageRegistryService manages per-conversation image registries
type ImageRegistryService struct {
	registries map[string]*ImageRegistry // conversationID -> registry
	mutex      sync.RWMutex
}

var (
	imageRegistryInstance *ImageRegistryService
	imageRegistryOnce     sync.Once
)

// GetImageRegistryService returns the singleton image registry service
func GetImageRegistryService() *ImageRegistryService {
	imageRegistryOnce.Do(func() {
		imageRegistryInstance = &ImageRegistryService{
			registries: make(map[string]*ImageRegistry),
		}
		log.Printf("📸 [IMAGE-REGISTRY] Service initialized")
	})
	return imageRegistryInstance
}

// getOrCreateRegistry gets or creates a registry for a conversation
func (s *ImageRegistryService) getOrCreateRegistry(conversationID string) *ImageRegistry {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if registry, exists := s.registries[conversationID]; exists {
		return registry
	}

	registry := &ImageRegistry{
		entries:  make(map[string]*ImageEntry),
		byFileID: make(map[string]string),
		counter:  0,
	}
	s.registries[conversationID] = registry
	return registry
}

// generateHandle creates the next handle for a registry
func (r *ImageRegistry) generateHandle() string {
	r.counter++
	return fmt.Sprintf("img-%d", r.counter)
}

// RegisterUploadedImage registers an uploaded image and returns its handle
func (s *ImageRegistryService) RegisterUploadedImage(conversationID, fileID, filename string, width, height int) string {
	registry := s.getOrCreateRegistry(conversationID)

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	// Check if already registered
	if handle, exists := registry.byFileID[fileID]; exists {
		log.Printf("📸 [IMAGE-REGISTRY] Image already registered: %s -> %s", fileID, handle)
		return handle
	}

	handle := registry.generateHandle()
	entry := &ImageEntry{
		Handle:    handle,
		FileID:    fileID,
		Filename:  filename,
		Source:    "uploaded",
		Width:     width,
		Height:    height,
		CreatedAt: time.Now(),
	}

	registry.entries[handle] = entry
	registry.byFileID[fileID] = handle

	log.Printf("📸 [IMAGE-REGISTRY] Registered uploaded image: %s -> %s (%s)", handle, fileID, filename)
	return handle
}

// RegisterGeneratedImage registers a generated image and returns its handle
func (s *ImageRegistryService) RegisterGeneratedImage(conversationID, fileID, prompt string) string {
	registry := s.getOrCreateRegistry(conversationID)

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	// Check if already registered
	if handle, exists := registry.byFileID[fileID]; exists {
		return handle
	}

	handle := registry.generateHandle()

	// Create a short filename from prompt
	filename := truncatePromptForFilename(prompt) + ".png"

	entry := &ImageEntry{
		Handle:    handle,
		FileID:    fileID,
		Filename:  filename,
		Source:    "generated",
		Prompt:    prompt,
		Width:     1024, // Default generation size
		Height:    1024,
		CreatedAt: time.Now(),
	}

	registry.entries[handle] = entry
	registry.byFileID[fileID] = handle

	log.Printf("📸 [IMAGE-REGISTRY] Registered generated image: %s -> %s", handle, fileID)
	return handle
}

// RegisterEditedImage registers an edited image and returns its handle
func (s *ImageRegistryService) RegisterEditedImage(conversationID, fileID, sourceHandle, prompt string) string {
	registry := s.getOrCreateRegistry(conversationID)

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	// Check if already registered
	if handle, exists := registry.byFileID[fileID]; exists {
		return handle
	}

	handle := registry.generateHandle()

	// Create filename based on source and edit prompt
	filename := fmt.Sprintf("edited_%s_%s.png", sourceHandle, truncatePromptForFilename(prompt))

	entry := &ImageEntry{
		Handle:       handle,
		FileID:       fileID,
		Filename:     filename,
		Source:       "edited",
		SourceHandle: sourceHandle,
		Prompt:       prompt,
		Width:        1024, // Edited images typically same size
		Height:       1024,
		CreatedAt:    time.Now(),
	}

	registry.entries[handle] = entry
	registry.byFileID[fileID] = handle

	log.Printf("📸 [IMAGE-REGISTRY] Registered edited image: %s -> %s (from %s)", handle, fileID, sourceHandle)
	return handle
}

// GetByHandle returns an image entry by its handle
func (s *ImageRegistryService) GetByHandle(conversationID, handle string) *ImageEntry {
	s.mutex.RLock()
	registry, exists := s.registries[conversationID]
	s.mutex.RUnlock()

	if !exists {
		return nil
	}

	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	return registry.entries[handle]
}

// GetByFileID returns an image entry by its file ID
func (s *ImageRegistryService) GetByFileID(conversationID, fileID string) *ImageEntry {
	s.mutex.RLock()
	registry, exists := s.registries[conversationID]
	s.mutex.RUnlock()

	if !exists {
		return nil
	}

	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	handle, exists := registry.byFileID[fileID]
	if !exists {
		return nil
	}

	return registry.entries[handle]
}

// ListImages returns all images in a conversation
func (s *ImageRegistryService) ListImages(conversationID string) []*ImageEntry {
	s.mutex.RLock()
	registry, exists := s.registries[conversationID]
	s.mutex.RUnlock()

	if !exists {
		return nil
	}

	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	result := make([]*ImageEntry, 0, len(registry.entries))
	for _, entry := range registry.entries {
		result = append(result, entry)
	}

	return result
}

// ListHandles returns all available handles for a conversation (for error messages)
func (s *ImageRegistryService) ListHandles(conversationID string) []string {
	s.mutex.RLock()
	registry, exists := s.registries[conversationID]
	s.mutex.RUnlock()

	if !exists {
		return nil
	}

	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	handles := make([]string, 0, len(registry.entries))
	for handle := range registry.entries {
		handles = append(handles, handle)
	}

	return handles
}

// BuildSystemContext creates the system prompt injection for available images
func (s *ImageRegistryService) BuildSystemContext(conversationID string) string {
	images := s.ListImages(conversationID)
	if len(images) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Available Images]\n")
	sb.WriteString("You have access to the following images in this conversation. Use the image ID (e.g., 'img-1') when calling the edit_image tool:\n")

	for _, img := range images {
		sb.WriteString(fmt.Sprintf("- %s: \"%s\"", img.Handle, img.Filename))

		// Add source info
		switch img.Source {
		case "uploaded":
			sb.WriteString(" (uploaded by user")
		case "generated":
			sb.WriteString(" (generated")
		case "edited":
			sb.WriteString(fmt.Sprintf(" (edited from %s", img.SourceHandle))
		}

		// Add dimensions if known
		if img.Width > 0 && img.Height > 0 {
			sb.WriteString(fmt.Sprintf(", %dx%d", img.Width, img.Height))
		}

		sb.WriteString(")\n")
	}

	return sb.String()
}

// CleanupConversation removes all registry data for a conversation
func (s *ImageRegistryService) CleanupConversation(conversationID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.registries[conversationID]; exists {
		delete(s.registries, conversationID)
		log.Printf("📸 [IMAGE-REGISTRY] Cleaned up conversation: %s", conversationID)
	}
}

// HasImages checks if a conversation has any registered images
func (s *ImageRegistryService) HasImages(conversationID string) bool {
	s.mutex.RLock()
	registry, exists := s.registries[conversationID]
	s.mutex.RUnlock()

	if !exists {
		return false
	}

	registry.mutex.RLock()
	defer registry.mutex.RUnlock()

	return len(registry.entries) > 0
}

// truncatePromptForFilename creates a short filename-safe string from a prompt
func truncatePromptForFilename(prompt string) string {
	// Take first 30 chars, make filename-safe
	if len(prompt) > 30 {
		prompt = prompt[:30]
	}

	// Replace unsafe characters
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		if r == ' ' {
			return '_'
		}
		return -1 // Remove other characters
	}, prompt)

	if safe == "" {
		safe = "image"
	}

	return safe
}
