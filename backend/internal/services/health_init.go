package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/health"
	"clara-agents/internal/vision"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	healthInitOnce    sync.Once
	healthProviderSvc *ProviderService
	healthDB          *database.DB
	healthSvc         *health.Service

	// These are also used by audio_init.go for backward compatibility
	visionProviderSvc *ProviderService
	visionDB          *database.DB
)

// SetHealthDependencies sets the dependencies needed for all health-monitored services.
// Must be called before InitHealthService.
func SetHealthDependencies(providerService *ProviderService, db *database.DB) {
	healthProviderSvc = providerService
	healthDB = db
	// Also set for backward compat with audio_init.go which uses visionProviderSvc
	visionProviderSvc = providerService
	visionDB = db
}

// GetHealthService returns the generalized health service singleton
func GetHealthService() *health.Service {
	return healthSvc
}

// InitHealthService initializes the health service for all capabilities and
// sets up vision and audio services with health-aware failover.
func InitHealthService() {
	if healthProviderSvc == nil {
		log.Println("[HEALTH-INIT] Provider service not set, health monitoring disabled")
		return
	}

	healthInitOnce.Do(func() {
		configService := GetConfigService()

		// Create health-package provider getter (converts models.Provider -> health.ProviderInfo)
		healthProviderGetter := func(id int) (*health.ProviderInfo, error) {
			p, err := healthProviderSvc.GetByID(id)
			if err != nil {
				return nil, err
			}
			return &health.ProviderInfo{
				ID:      p.ID,
				Name:    p.Name,
				BaseURL: p.BaseURL,
				APIKey:  p.APIKey,
				Enabled: p.Enabled,
			}, nil
		}

		// Create the generalized health service
		healthSvc = health.NewService(healthProviderGetter, 3, 1*time.Hour)

		// Register capability-specific health check strategies
		healthSvc.RegisterStrategy(&health.ChatHealthCheck{})
		healthSvc.RegisterStrategy(&health.VisionHealthCheck{})
		healthSvc.RegisterStrategy(&health.ImageGenHealthCheck{})
		healthSvc.RegisterStrategy(&health.AudioHealthCheck{})

		// Discover and register all providers by capability
		registerAllProviders(configService, healthProviderGetter)

		// Initialize vision service with health-aware failover
		initVisionWithHealth(configService, healthProviderGetter)

		status := healthSvc.GetStatus()
		log.Printf("[HEALTH-INIT] Health service initialized (%v providers registered across capabilities)", status["total"])
	})
}

// registerAllProviders discovers all providers and registers them in the health service by capability
func registerAllProviders(configService *ConfigService, providerGetter health.ProviderGetter) {
	priority := 100

	// --- Chat providers ---
	priority = registerChatProviders(configService, providerGetter, priority)

	// --- Vision providers ---
	priority = registerVisionProviders(configService, providerGetter, priority)

	// --- Image Generation providers ---
	priority = registerImageGenProviders(providerGetter, priority)

	// --- Image Edit providers ---
	priority = registerImageEditProviders(providerGetter, priority)

	// --- Audio providers ---
	registerAudioProviders(providerGetter, priority)
}

// registerChatProviders registers chat-capable providers from model aliases
func registerChatProviders(configService *ConfigService, providerGetter health.ProviderGetter, priority int) int {
	allAliases := configService.GetAllModelAliases()
	registered := 0

	for providerID, aliases := range allAliases {
		provider, err := providerGetter(providerID)
		if err != nil || !provider.Enabled {
			continue
		}

		// Pick one representative model per provider for chat health checks
		// Prefer the "fastest" recommended model, otherwise take first alias
		var chatModel string
		recommended := configService.GetRecommendedModels(providerID)
		if recommended != nil && recommended.Fastest != "" {
			chatModel = recommended.Fastest
		} else {
			for _, alias := range aliases {
				chatModel = alias.ActualModel
				break
			}
		}

		if chatModel != "" {
			healthSvc.RegisterProvider(health.CapabilityChat, providerID, provider.Name, chatModel, priority)
			priority -= 10
			registered++
		}
	}

	// Also check DB for providers with models that aren't covered by aliases
	if healthDB != nil {
		rows, err := healthDB.Query(`
			SELECT DISTINCT p.id, p.name, m.name
			FROM providers p
			JOIN models m ON m.provider_id = p.id
			WHERE p.enabled = 1
			AND p.audio_only = 0 AND p.image_only = 0 AND p.image_edit_only = 0
			AND m.is_visible = 1
			ORDER BY p.id ASC
			LIMIT 20
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var pID int
				var pName, mName string
				if err := rows.Scan(&pID, &pName, &mName); err != nil {
					continue
				}
				// RegisterProvider is idempotent
				healthSvc.RegisterProvider(health.CapabilityChat, pID, pName, mName, priority)
				priority -= 5
			}
		}
	}

	log.Printf("[HEALTH-INIT] Registered %d chat provider(s)", registered)
	return priority
}

// registerVisionProviders registers vision-capable providers
func registerVisionProviders(configService *ConfigService, providerGetter health.ProviderGetter, priority int) int {
	allAliases := configService.GetAllModelAliases()
	registered := 0

	for providerID, aliases := range allAliases {
		for _, aliasInfo := range aliases {
			if aliasInfo.SupportsVision != nil && *aliasInfo.SupportsVision {
				provider, err := providerGetter(providerID)
				if err != nil || !provider.Enabled {
					continue
				}
				healthSvc.RegisterProvider(health.CapabilityVision, providerID, provider.Name, aliasInfo.ActualModel, priority)
				priority -= 10
				registered++
			}
		}
	}

	// Also check database for vision models not covered by aliases
	if healthDB != nil {
		rows, err := healthDB.Query(`
			SELECT m.provider_id, p.name, m.name
			FROM models m
			JOIN providers p ON m.provider_id = p.id
			WHERE m.supports_vision = 1 AND m.is_visible = 1 AND p.enabled = 1
			ORDER BY m.provider_id ASC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var providerID int
				var providerName, modelName string
				if err := rows.Scan(&providerID, &providerName, &modelName); err != nil {
					continue
				}
				healthSvc.RegisterProvider(health.CapabilityVision, providerID, providerName, modelName, priority)
				priority -= 5
				registered++
			}
		}
	}

	log.Printf("[HEALTH-INIT] Registered %d vision provider(s)", registered)
	return priority
}

// registerImageGenProviders registers image generation providers from the database
func registerImageGenProviders(providerGetter health.ProviderGetter, priority int) int {
	registered := 0

	if healthDB != nil {
		rows, err := healthDB.Query(`
			SELECT id, name FROM providers
			WHERE enabled = 1 AND image_only = 1
			ORDER BY id ASC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var pID int
				var pName string
				if err := rows.Scan(&pID, &pName); err != nil {
					continue
				}
				// Image gen providers don't have a specific model name
				healthSvc.RegisterProvider(health.CapabilityImageGen, pID, pName, "", priority)
				priority -= 10
				registered++
			}
		}
	}

	log.Printf("[HEALTH-INIT] Registered %d image generation provider(s)", registered)
	return priority
}

// registerImageEditProviders registers image editing providers from the database
func registerImageEditProviders(providerGetter health.ProviderGetter, priority int) int {
	registered := 0

	if healthDB != nil {
		rows, err := healthDB.Query(`
			SELECT id, name FROM providers
			WHERE enabled = 1 AND image_edit_only = 1
			ORDER BY id ASC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var pID int
				var pName string
				if err := rows.Scan(&pID, &pName); err != nil {
					continue
				}
				healthSvc.RegisterProvider(health.CapabilityImageEdit, pID, pName, "", priority)
				priority -= 10
				registered++
			}
		}
	}

	log.Printf("[HEALTH-INIT] Registered %d image edit provider(s)", registered)
	return priority
}

// registerAudioProviders registers audio-capable providers (Groq and OpenAI)
func registerAudioProviders(providerGetter health.ProviderGetter, priority int) {
	registered := 0

	if healthDB != nil {
		rows, err := healthDB.Query(`
			SELECT id, name FROM providers
			WHERE enabled = 1 AND (audio_only = 1 OR LOWER(name) IN ('groq', 'openai'))
			ORDER BY CASE WHEN LOWER(name) = 'groq' THEN 0 ELSE 1 END, id ASC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var pID int
				var pName string
				if err := rows.Scan(&pID, &pName); err != nil {
					continue
				}
				healthSvc.RegisterProvider(health.CapabilityAudio, pID, pName, "", priority)
				priority -= 10
				registered++
			}
		}
	}

	log.Printf("[HEALTH-INIT] Registered %d audio provider(s)", registered)
}

// initVisionWithHealth sets up the vision service using the health-aware failover system
func initVisionWithHealth(configService *ConfigService, healthProviderGetter health.ProviderGetter) {
	// Vision-specific provider getter (converts health.ProviderInfo -> vision.Provider)
	visionProviderGetter := func(id int) (*vision.Provider, error) {
		p, err := healthProviderGetter(id)
		if err != nil {
			return nil, err
		}
		return &vision.Provider{
			ID:      p.ID,
			Name:    p.Name,
			BaseURL: p.BaseURL,
			APIKey:  p.APIKey,
			Enabled: p.Enabled,
		}, nil
	}

	// Legacy vision model finder callback (used as fallback when no healthy providers found)
	visionModelFinder := func() (int, string, error) {
		allAliases := configService.GetAllModelAliases()
		for providerID, aliases := range allAliases {
			for _, aliasInfo := range aliases {
				if aliasInfo.SupportsVision != nil && *aliasInfo.SupportsVision {
					provider, err := healthProviderSvc.GetByID(providerID)
					if err == nil && provider.Enabled {
						return providerID, aliasInfo.ActualModel, nil
					}
				}
			}
		}

		// Fallback: Check database
		if healthDB == nil {
			return 0, "", fmt.Errorf("database not available")
		}

		var providerID int
		var modelName string
		err := healthDB.QueryRow(`
			SELECT m.provider_id, m.name
			FROM models m
			JOIN providers p ON m.provider_id = p.id
			WHERE m.supports_vision = 1 AND m.is_visible = 1 AND p.enabled = 1
			ORDER BY m.provider_id ASC
			LIMIT 1
		`).Scan(&providerID, &modelName)

		if err != nil {
			return 0, "", fmt.Errorf("no vision model found: %w", err)
		}
		return providerID, modelName, nil
	}

	vision.InitService(visionProviderGetter, visionModelFinder, healthSvc)
	log.Println("[HEALTH-INIT] Vision service initialized with health-aware failover")
}
