package services

import (
	"clara-agents/internal/health"
	"clara-agents/internal/models"
	"log"
	"sync"
)

// ImageEditProviderConfig holds the configuration for an image editing provider
type ImageEditProviderConfig struct {
	ID      int
	Name    string
	BaseURL string
	APIKey  string
	Favicon string
}

// ImageEditProviderService manages image editing providers
type ImageEditProviderService struct {
	providers []ImageEditProviderConfig
	mutex     sync.RWMutex
}

var (
	imageEditProviderInstance *ImageEditProviderService
	imageEditProviderOnce     sync.Once
)

// GetImageEditProviderService returns the singleton image edit provider service
func GetImageEditProviderService() *ImageEditProviderService {
	imageEditProviderOnce.Do(func() {
		imageEditProviderInstance = &ImageEditProviderService{
			providers: make([]ImageEditProviderConfig, 0),
		}
	})
	return imageEditProviderInstance
}

// LoadFromProviders loads image edit providers from the database provider models
// This is called during provider sync
func (s *ImageEditProviderService) LoadFromProviders(providers []models.Provider) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear existing providers
	s.providers = make([]ImageEditProviderConfig, 0)

	for _, p := range providers {
		// Only load enabled providers with image_edit_only flag
		if p.Enabled && p.ImageEditOnly {
			config := ImageEditProviderConfig{
				ID:      p.ID,
				Name:    p.Name,
				BaseURL: p.BaseURL,
				APIKey:  p.APIKey,
				Favicon: p.Favicon,
			}
			s.providers = append(s.providers, config)
			log.Printf("[IMAGE-EDIT-PROVIDER] Loaded image edit provider: %s (ID:%d)", p.Name, p.ID)
		}
	}

	log.Printf("[IMAGE-EDIT-PROVIDER] Total image edit providers loaded: %d", len(s.providers))
}

// GetProvider returns the first enabled image edit provider
// Returns nil if no image edit providers are configured
func (s *ImageEditProviderService) GetProvider() *ImageEditProviderConfig {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.providers) == 0 {
		return nil
	}

	// Return the first provider
	return &s.providers[0]
}

// GetAllProviders returns all configured image edit providers
func (s *ImageEditProviderService) GetAllProviders() []ImageEditProviderConfig {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make([]ImageEditProviderConfig, len(s.providers))
	copy(result, s.providers)
	return result
}

// GetHealthyProvider returns the first healthy image edit provider, falling back to the first provider
func (s *ImageEditProviderService) GetHealthyProvider(healthSvc *health.Service) *ImageEditProviderConfig {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.providers) == 0 {
		return nil
	}

	if healthSvc != nil {
		for i := range s.providers {
			if healthSvc.IsProviderHealthy(health.CapabilityImageEdit, s.providers[i].ID, "") {
				return &s.providers[i]
			}
		}
		log.Printf("[IMAGE-EDIT-PROVIDER] No healthy providers, falling back to first")
	}

	return &s.providers[0]
}

// HasProvider checks if any image edit provider is configured
func (s *ImageEditProviderService) HasProvider() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.providers) > 0
}
