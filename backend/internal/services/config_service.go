package services

import (
	"clara-agents/internal/models"
	"log"
	"sync"
)

// ConfigService handles configuration management
type ConfigService struct {
	mu                sync.RWMutex
	recommendedModels map[int]*models.RecommendedModels        // Provider ID -> Recommended Models
	modelAliases      map[int]map[string]models.ModelAlias     // Provider ID -> (Model Name -> Alias Info)
	providerSecurity  map[int]bool                             // Provider ID -> Secure flag
}

var (
	configServiceInstance *ConfigService
	configServiceOnce     sync.Once
)

// GetConfigService returns the singleton config service instance
func GetConfigService() *ConfigService {
	configServiceOnce.Do(func() {
		configServiceInstance = &ConfigService{
			recommendedModels: make(map[int]*models.RecommendedModels),
			modelAliases:      make(map[int]map[string]models.ModelAlias),
			providerSecurity:  make(map[int]bool),
		}
	})
	return configServiceInstance
}

// SetRecommendedModels stores recommended models for a provider
func (s *ConfigService) SetRecommendedModels(providerID int, recommended *models.RecommendedModels) {
	if recommended == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.recommendedModels[providerID] = recommended
	log.Printf("📌 [CONFIG] Set recommended models for provider %d: top=%s, medium=%s, fastest=%s",
		providerID, recommended.Top, recommended.Medium, recommended.Fastest)
}

// GetRecommendedModels retrieves recommended models for a provider
func (s *ConfigService) GetRecommendedModels(providerID int) *models.RecommendedModels {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.recommendedModels[providerID]
}

// GetAllRecommendedModels retrieves all recommended models across providers
func (s *ConfigService) GetAllRecommendedModels() map[int]*models.RecommendedModels {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[int]*models.RecommendedModels)
	for k, v := range s.recommendedModels {
		result[k] = v
	}

	return result
}

// SetModelAliases stores model aliases for a provider
func (s *ConfigService) SetModelAliases(providerID int, aliases map[string]models.ModelAlias) {
	if aliases == nil || len(aliases) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.modelAliases[providerID] = aliases
	log.Printf("📌 [CONFIG] Set %d model aliases for provider %d", len(aliases), providerID)
}

// GetModelAliases retrieves model aliases for a provider
func (s *ConfigService) GetModelAliases(providerID int) map[string]models.ModelAlias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.modelAliases[providerID]
}

// GetAllModelAliases retrieves all model aliases across providers
func (s *ConfigService) GetAllModelAliases() map[int]map[string]models.ModelAlias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[int]map[string]models.ModelAlias)
	for k, v := range s.modelAliases {
		result[k] = v
	}

	return result
}

// GetAliasForModel checks if a model has an alias configured
func (s *ConfigService) GetAliasForModel(providerID int, modelName string) *models.ModelAlias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if aliases, exists := s.modelAliases[providerID]; exists {
		if alias, found := aliases[modelName]; found {
			return &alias
		}
	}

	return nil
}

// GetModelAlias retrieves a specific alias by its key for a provider
func (s *ConfigService) GetModelAlias(providerID int, aliasKey string) *models.ModelAlias {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if aliases, exists := s.modelAliases[providerID]; exists {
		if alias, found := aliases[aliasKey]; found {
			return &alias
		}
	}

	return nil
}

// SetProviderSecure stores the secure flag for a provider
func (s *ConfigService) SetProviderSecure(providerID int, secure bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.providerSecurity[providerID] = secure
	if secure {
		log.Printf("🔒 [CONFIG] Provider %d marked as secure (doesn't store user data)", providerID)
	}
}

// IsProviderSecure checks if a provider is marked as secure
func (s *ConfigService) IsProviderSecure(providerID int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.providerSecurity[providerID]
}
