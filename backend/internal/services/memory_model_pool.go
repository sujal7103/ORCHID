package services

import (
	"clara-agents/internal/config"
	"clara-agents/internal/health"
	"clara-agents/internal/models"
	"database/sql"
	"fmt"
	"log"
	"sync"
)

// MemoryModelPool manages multiple models for memory operations with health tracking and failover
type MemoryModelPool struct {
	extractorModels []ModelCandidate
	selectorModels  []ModelCandidate
	extractorIndex  int
	selectorIndex   int
	mu              sync.Mutex
	chatService     *ChatService
	db              *sql.DB         // Database connection for querying model_aliases
	healthService   *health.Service // System-wide health service
}

// ModelCandidate represents a model eligible for memory operations
type ModelCandidate struct {
	ModelID      string
	ProviderID   int
	ProviderName string
	SpeedMs      int
	DisplayName  string
}

// NewMemoryModelPool creates a new model pool by discovering eligible models from providers
func NewMemoryModelPool(chatService *ChatService, db *sql.DB) (*MemoryModelPool, error) {
	pool := &MemoryModelPool{
		chatService: chatService,
		db:          db,
	}

	// Discover models from ChatService
	if err := pool.discoverModels(); err != nil {
		log.Printf("⚠️ [MODEL-POOL] Failed to discover models: %v", err)
		log.Printf("⚠️ [MODEL-POOL] Memory services will be disabled until models with memory flags are added")
	}

	if len(pool.extractorModels) == 0 {
		log.Printf("⚠️ [MODEL-POOL] No extractor models found - memory extraction disabled")
	}

	if len(pool.selectorModels) == 0 {
		log.Printf("⚠️ [MODEL-POOL] No selector models found - memory selection disabled")
	}

	if len(pool.extractorModels) > 0 || len(pool.selectorModels) > 0 {
		log.Printf("🎯 [MODEL-POOL] Initialized with %d extractors, %d selectors",
			len(pool.extractorModels), len(pool.selectorModels))
	}

	// Return pool even if empty - allows graceful degradation
	return pool, nil
}

// SetHealthService sets the system-wide health service for provider health tracking
func (p *MemoryModelPool) SetHealthService(hs *health.Service) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthService = hs
	log.Printf("✅ [MODEL-POOL] Health service set for provider health tracking")
}

// discoverModels scans database for models with memory flags
func (p *MemoryModelPool) discoverModels() error {
	// First try loading from database (MySQL-first approach)
	dbModels, err := p.discoverFromDatabase()
	if err == nil && len(dbModels) > 0 {
		log.Printf("✅ [MODEL-POOL] Discovered %d models from database", len(dbModels))
		return nil
	}

	// Fallback: Load providers configuration from providers.json
	providersConfig, err := config.LoadProviders("providers.json")
	if err != nil {
		// Gracefully handle missing providers.json (expected in admin UI workflow)
		log.Printf("⚠️ [MODEL-POOL] providers.json not found or invalid: %v", err)
		log.Printf("ℹ️ [MODEL-POOL] This is normal when starting with empty database")
		return nil // Not a fatal error - just means no models configured yet
	}

	for _, providerConfig := range providersConfig.Providers {
		if !providerConfig.Enabled {
			continue
		}

		for alias, modelAlias := range providerConfig.ModelAliases {
			// Get model configuration map (convert from ModelAlias)
			modelConfig := modelAliasToMap(modelAlias)

			// Check if model supports memory extraction
			if isExtractor, ok := modelConfig["memory_extractor"].(bool); ok && isExtractor {
				candidate := ModelCandidate{
					ModelID:      alias,
					ProviderName: providerConfig.Name,
					DisplayName:  getDisplayName(modelConfig),
					SpeedMs:      getSpeedMs(modelConfig),
				}
				p.extractorModels = append(p.extractorModels, candidate)

				log.Printf("✅ [MODEL-POOL] Found extractor: %s (%s) - %dms",
					alias, providerConfig.Name, candidate.SpeedMs)
			}

			// Check if model supports memory selection
			if isSelector, ok := modelConfig["memory_selector"].(bool); ok && isSelector {
				candidate := ModelCandidate{
					ModelID:      alias,
					ProviderName: providerConfig.Name,
					DisplayName:  getDisplayName(modelConfig),
					SpeedMs:      getSpeedMs(modelConfig),
				}
				p.selectorModels = append(p.selectorModels, candidate)

				log.Printf("✅ [MODEL-POOL] Found selector: %s (%s) - %dms",
					alias, providerConfig.Name, candidate.SpeedMs)
			}
		}
	}

	// Sort by speed (fastest first)
	p.sortModelsBySpeed(p.extractorModels)
	p.sortModelsBySpeed(p.selectorModels)

	return nil
}

// discoverFromDatabase loads memory models from database (model_aliases table)
func (p *MemoryModelPool) discoverFromDatabase() ([]ModelCandidate, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	// Query for models with memory_extractor or memory_selector flags, include provider_id
	rows, err := p.db.Query(`
		SELECT
			a.alias_name,
			a.provider_id,
			pr.name as provider_name,
			a.display_name,
			COALESCE(a.structured_output_speed_ms, 999999) as speed_ms,
			COALESCE(a.memory_extractor, 0) as memory_extractor,
			COALESCE(a.memory_selector, 0) as memory_selector
		FROM model_aliases a
		JOIN providers pr ON a.provider_id = pr.id
		WHERE (a.memory_extractor = 1 OR a.memory_selector = 1) AND pr.enabled = 1
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to query model_aliases: %w", err)
	}
	defer rows.Close()

	var candidates []ModelCandidate
	for rows.Next() {
		var aliasName, providerName, displayName string
		var providerID, speedMs int
		var isExtractor, isSelector int

		if err := rows.Scan(&aliasName, &providerID, &providerName, &displayName, &speedMs, &isExtractor, &isSelector); err != nil {
			log.Printf("⚠️ [MODEL-POOL] Failed to scan row: %v", err)
			continue
		}

		candidate := ModelCandidate{
			ModelID:      aliasName,
			ProviderID:   providerID,
			ProviderName: providerName,
			DisplayName:  displayName,
			SpeedMs:      speedMs,
		}

		if isExtractor == 1 {
			p.extractorModels = append(p.extractorModels, candidate)
			log.Printf("✅ [MODEL-POOL] Found extractor from DB: %s (ID:%d, %s) - %dms", aliasName, providerID, providerName, speedMs)
		}

		if isSelector == 1 {
			p.selectorModels = append(p.selectorModels, candidate)
			log.Printf("✅ [MODEL-POOL] Found selector from DB: %s (ID:%d, %s) - %dms", aliasName, providerID, providerName, speedMs)
		}

		candidates = append(candidates, candidate)
	}

	// Sort by speed (fastest first)
	p.sortModelsBySpeed(p.extractorModels)
	p.sortModelsBySpeed(p.selectorModels)

	return candidates, nil
}

// modelAliasToMap converts ModelAlias struct to map for easier access
func modelAliasToMap(alias models.ModelAlias) map[string]interface{} {
	m := make(map[string]interface{})

	// Set display_name
	m["display_name"] = alias.DisplayName

	// Set structured_output_speed_ms if available
	if alias.StructuredOutputSpeedMs != nil {
		m["structured_output_speed_ms"] = *alias.StructuredOutputSpeedMs
	}

	// Set memory flags if available
	if alias.MemoryExtractor != nil {
		m["memory_extractor"] = *alias.MemoryExtractor
	}
	if alias.MemorySelector != nil {
		m["memory_selector"] = *alias.MemorySelector
	}

	return m
}

// GetNextExtractor returns the next healthy extractor model using round-robin, fastest first
func (p *MemoryModelPool) GetNextExtractor() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.extractorModels) == 0 {
		return "", fmt.Errorf("no extractor models available")
	}

	// Try all models in round-robin fashion
	for attempts := 0; attempts < len(p.extractorModels); attempts++ {
		candidate := p.extractorModels[p.extractorIndex]
		p.extractorIndex = (p.extractorIndex + 1) % len(p.extractorModels)

		// Check health via system-wide health service
		if p.healthService != nil && !p.healthService.IsProviderHealthy(health.CapabilityChat, candidate.ProviderID, candidate.ModelID) {
			log.Printf("⏭️ [MODEL-POOL] Skipping unhealthy extractor: %s (provider ID:%d)", candidate.ModelID, candidate.ProviderID)
			continue
		}

		log.Printf("🔄 [MODEL-POOL] Selected extractor: %s (healthy, speed=%dms)", candidate.ModelID, candidate.SpeedMs)
		return candidate.ModelID, nil
	}

	// All models unhealthy - return fastest anyway as last resort
	log.Printf("⚠️ [MODEL-POOL] All extractors unhealthy, using fastest: %s", p.extractorModels[0].ModelID)
	return p.extractorModels[0].ModelID, nil
}

// GetNextSelector returns the next healthy selector model using round-robin, fastest first
func (p *MemoryModelPool) GetNextSelector() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.selectorModels) == 0 {
		return "", fmt.Errorf("no selector models available")
	}

	// Try all models in round-robin fashion
	for attempts := 0; attempts < len(p.selectorModels); attempts++ {
		candidate := p.selectorModels[p.selectorIndex]
		p.selectorIndex = (p.selectorIndex + 1) % len(p.selectorModels)

		// Check health via system-wide health service
		if p.healthService != nil && !p.healthService.IsProviderHealthy(health.CapabilityChat, candidate.ProviderID, candidate.ModelID) {
			log.Printf("⏭️ [MODEL-POOL] Skipping unhealthy selector: %s (provider ID:%d)", candidate.ModelID, candidate.ProviderID)
			continue
		}

		log.Printf("🔄 [MODEL-POOL] Selected selector: %s (healthy, speed=%dms)", candidate.ModelID, candidate.SpeedMs)
		return candidate.ModelID, nil
	}

	// All models unhealthy - return fastest anyway as last resort
	log.Printf("⚠️ [MODEL-POOL] All selectors unhealthy, using fastest: %s", p.selectorModels[0].ModelID)
	return p.selectorModels[0].ModelID, nil
}

// MarkSuccess records a successful model call via the system-wide health service
func (p *MemoryModelPool) MarkSuccess(modelID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.healthService == nil {
		return
	}

	providerID := p.findProviderID(modelID)
	if providerID > 0 {
		p.healthService.MarkHealthy(health.CapabilityChat, providerID, modelID)
	}
}

// MarkFailure records a failed model call via the system-wide health service
func (p *MemoryModelPool) MarkFailure(modelID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.healthService == nil {
		return
	}

	providerID := p.findProviderID(modelID)
	if providerID > 0 {
		p.healthService.MarkUnhealthy(health.CapabilityChat, providerID, modelID, "memory operation failed", 0)
	}
}

// findProviderID looks up the provider ID for a model (must be called with lock held)
func (p *MemoryModelPool) findProviderID(modelID string) int {
	for _, m := range p.extractorModels {
		if m.ModelID == modelID {
			return m.ProviderID
		}
	}
	for _, m := range p.selectorModels {
		if m.ModelID == modelID {
			return m.ProviderID
		}
	}
	return 0
}

// GetStats returns current pool statistics
func (p *MemoryModelPool) GetStats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	healthyExtractors := 0
	healthySelectors := 0

	for _, model := range p.extractorModels {
		if p.healthService == nil || p.healthService.IsProviderHealthy(health.CapabilityChat, model.ProviderID, model.ModelID) {
			healthyExtractors++
		}
	}

	for _, model := range p.selectorModels {
		if p.healthService == nil || p.healthService.IsProviderHealthy(health.CapabilityChat, model.ProviderID, model.ModelID) {
			healthySelectors++
		}
	}

	return map[string]interface{}{
		"total_extractors":   len(p.extractorModels),
		"healthy_extractors": healthyExtractors,
		"total_selectors":    len(p.selectorModels),
		"healthy_selectors":  healthySelectors,
	}
}

// Helper functions

func getDisplayName(modelConfig map[string]interface{}) string {
	if name, ok := modelConfig["display_name"].(string); ok {
		return name
	}
	return "Unknown"
}

func getSpeedMs(modelConfig map[string]interface{}) int {
	if speed, ok := modelConfig["structured_output_speed_ms"].(float64); ok {
		return int(speed)
	}
	if speed, ok := modelConfig["structured_output_speed_ms"].(int); ok {
		return speed
	}
	return 999999 // Default to slow if not specified
}

func (p *MemoryModelPool) sortModelsBySpeed(models []ModelCandidate) {
	// Simple bubble sort (fine for small arrays)
	n := len(models)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if models[j].SpeedMs > models[j+1].SpeedMs {
				models[j], models[j+1] = models[j+1], models[j]
			}
		}
	}
}
