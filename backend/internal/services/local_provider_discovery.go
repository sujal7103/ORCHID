package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// LocalProviderDiscovery probes for Ollama and LM Studio on the host machine
// and auto-registers them as providers with all models visible.
type LocalProviderDiscovery struct {
	db              *database.DB
	providerService *ProviderService
	modelService    *ModelService
	chatService     *ChatService
	mu              sync.Mutex
}

// localProviderDef defines a local provider to probe for
type localProviderDef struct {
	Name    string
	EnvVar  string   // env var override for base URL
	Probes  []string // URLs to try in order
	APIPath string   // path to models endpoint (appended to base URL)
	Favicon string
}

var localProviders = []localProviderDef{
	{
		Name:   "Ollama",
		EnvVar: "OLLAMA_BASE_URL",
		Probes: []string{
			"http://host.docker.internal:11434",
			"http://localhost:11434",
			"http://127.0.0.1:11434",
			"http://ollama:11434",
		},
		APIPath: "/v1", // Ollama exposes OpenAI-compatible API at /v1
		Favicon: "https://ollama.com/public/ollama.png",
	},
	{
		Name:   "LM Studio",
		EnvVar: "LMSTUDIO_BASE_URL",
		Probes: []string{
			"http://host.docker.internal:1234",
			"http://localhost:1234",
			"http://127.0.0.1:1234",
			"http://lmstudio:1234",
		},
		APIPath: "/v1",
		Favicon: "https://lmstudio.ai/favicon.ico",
	},
}

// NewLocalProviderDiscovery creates a new discovery service
func NewLocalProviderDiscovery(db *database.DB, providerService *ProviderService, modelService *ModelService, chatService *ChatService) *LocalProviderDiscovery {
	return &LocalProviderDiscovery{
		db:              db,
		providerService: providerService,
		modelService:    modelService,
		chatService:     chatService,
	}
}

// DiscoverAndSync probes for local providers, registers them, fetches models,
// and makes all models visible. Safe to call repeatedly (idempotent).
func (d *LocalProviderDiscovery) DiscoverAndSync() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, def := range localProviders {
		baseURL := d.probeProvider(def)
		if baseURL == "" {
			// Provider not reachable — disable if it was previously auto-added
			d.disableIfAutoAdded(def.Name)
			continue
		}

		apiBase := baseURL + def.APIPath
		d.ensureProvider(def, apiBase)
	}
}

// probeProvider tries to reach a local provider, returns the working base URL or ""
func (d *LocalProviderDiscovery) probeProvider(def localProviderDef) string {
	// Check env var override first
	if envURL := os.Getenv(def.EnvVar); envURL != "" {
		envURL = strings.TrimRight(envURL, "/")
		if d.isReachable(envURL) {
			return envURL
		}
		log.Printf("⚠️  [DISCOVERY] %s env %s=%s not reachable", def.Name, def.EnvVar, envURL)
	}

	// Try each probe URL
	for _, url := range def.Probes {
		if d.isReachable(url) {
			return url
		}
	}

	return ""
}

// isReachable checks if a URL responds (tries /api/tags for Ollama, /v1/models for OpenAI-compat)
func (d *LocalProviderDiscovery) isReachable(baseURL string) bool {
	client := &http.Client{Timeout: 3 * time.Second}

	// Try the root or a known endpoint
	endpoints := []string{
		baseURL + "/v1/models",
		baseURL + "/api/tags",
		baseURL,
	}

	for _, url := range endpoints {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return true
			}
		}
	}

	return false
}

// ensureProvider creates or updates a local provider and syncs its models
func (d *LocalProviderDiscovery) ensureProvider(def localProviderDef, apiBaseURL string) {
	existing, err := d.providerService.GetByName(def.Name)
	if err != nil {
		log.Printf("⚠️  [DISCOVERY] Error checking provider %s: %v", def.Name, err)
		return
	}

	providerConfig := models.ProviderConfig{
		Name:    def.Name,
		BaseURL: apiBaseURL,
		APIKey:  "not-needed", // Local providers don't need API keys
		Enabled: true,
		Favicon: def.Favicon,
	}

	var provider *models.Provider
	if existing == nil {
		// Create new provider
		log.Printf("🔍 [DISCOVERY] Found %s at %s — auto-registering", def.Name, apiBaseURL)
		provider, err = d.providerService.Create(providerConfig)
		if err != nil {
			log.Printf("⚠️  [DISCOVERY] Failed to create %s provider: %v", def.Name, err)
			return
		}
	} else {
		// Update URL if it changed (e.g., host.docker.internal vs localhost)
		if existing.BaseURL != apiBaseURL || !existing.Enabled {
			log.Printf("🔄 [DISCOVERY] Updating %s URL: %s → %s", def.Name, existing.BaseURL, apiBaseURL)
			providerConfig.Favicon = def.Favicon
			if err := d.providerService.Update(existing.ID, providerConfig); err != nil {
				log.Printf("⚠️  [DISCOVERY] Failed to update %s provider: %v", def.Name, err)
			}
		}
		provider = existing
	}

	// Fetch and sync models
	d.syncModels(provider, def)
}

// syncModels fetches models from a local provider and makes them all visible
func (d *LocalProviderDiscovery) syncModels(provider *models.Provider, def localProviderDef) {
	// Use the standard OpenAI-compatible model fetch
	if err := d.modelService.FetchFromProvider(provider); err != nil {
		// Ollama might need the /api/tags endpoint instead
		if def.Name == "Ollama" {
			d.syncOllamaModels(provider)
			return
		}
		log.Printf("⚠️  [DISCOVERY] Failed to fetch models from %s: %v", provider.Name, err)
		return
	}

	// Make ALL models from local providers visible (no filters needed)
	if _, err := d.db.Exec("UPDATE models SET is_visible = 1 WHERE provider_id = ?", provider.ID); err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to set model visibility for %s: %v", provider.Name, err)
	}

	// Remove stale models that are no longer served
	d.cleanStaleModels(provider)

	// Count visible models
	var count int
	d.db.QueryRow("SELECT COUNT(*) FROM models WHERE provider_id = ? AND is_visible = 1", provider.ID).Scan(&count)
	log.Printf("✅ [DISCOVERY] %s: %d models available", provider.Name, count)
}

// syncOllamaModels uses Ollama's native /api/tags endpoint
func (d *LocalProviderDiscovery) syncOllamaModels(provider *models.Provider) {
	// Strip /v1 to get the Ollama base URL
	ollamaBase := strings.TrimSuffix(provider.BaseURL, "/v1")
	url := ollamaBase + "/api/tags"

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to reach Ollama /api/tags: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️  [DISCOVERY] Ollama /api/tags returned %d: %s", resp.StatusCode, string(body))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to read Ollama response: %v", err)
		return
	}

	var tagsResp struct {
		Models []struct {
			Name       string `json:"name"`
			Model      string `json:"model"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
			Details    struct {
				Family            string `json:"family"`
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &tagsResp); err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to parse Ollama tags: %v", err)
		return
	}

	log.Printf("🔄 [DISCOVERY] Ollama has %d models via /api/tags", len(tagsResp.Models))

	for _, m := range tagsResp.Models {
		modelID := m.Name
		displayName := m.Name
		description := ""
		if m.Details.ParameterSize != "" {
			description = fmt.Sprintf("%s (%s)", m.Details.Family, m.Details.ParameterSize)
		}

		_, err := d.db.Exec(`
			INSERT INTO models (id, provider_id, name, display_name, description, is_visible, fetched_at)
			VALUES (?, ?, ?, ?, ?, 1, ?)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				display_name = VALUES(display_name),
				description = VALUES(description),
				is_visible = 1,
				fetched_at = VALUES(fetched_at)
		`, modelID, provider.ID, modelID, displayName, description, time.Now())

		if err != nil {
			log.Printf("⚠️  [DISCOVERY] Failed to store Ollama model %s: %v", modelID, err)
		}
	}

	// Clean stale models
	d.cleanStaleModels(provider)

	var count int
	d.db.QueryRow("SELECT COUNT(*) FROM models WHERE provider_id = ? AND is_visible = 1", provider.ID).Scan(&count)
	log.Printf("✅ [DISCOVERY] Ollama: %d models available", count)
}

// cleanStaleModels removes models that haven't been seen in the latest fetch
// (for local providers, models come and go as users pull/remove them)
func (d *LocalProviderDiscovery) cleanStaleModels(provider *models.Provider) {
	// Delete models that weren't updated in this sync (fetched_at is old)
	cutoff := time.Now().Add(-5 * time.Minute)
	result, err := d.db.Exec(`
		DELETE FROM models
		WHERE provider_id = ? AND fetched_at < ?
	`, provider.ID, cutoff)

	if err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to clean stale models for %s: %v", provider.Name, err)
		return
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("🧹 [DISCOVERY] Removed %d stale models from %s", rows, provider.Name)
	}
}

// disableIfAutoAdded disables a local provider if it exists but is no longer reachable
func (d *LocalProviderDiscovery) disableIfAutoAdded(name string) {
	existing, err := d.providerService.GetByName(name)
	if err != nil || existing == nil {
		return // Not registered, nothing to do
	}

	if !existing.Enabled {
		return // Already disabled
	}

	// Only disable if it was auto-added (no API key or "not-needed" key)
	if existing.APIKey != "" && existing.APIKey != "not-needed" {
		return // Manually configured, don't touch
	}

	log.Printf("⚠️  [DISCOVERY] %s is no longer reachable — disabling provider", name)
	config := models.ProviderConfig{
		Name:    existing.Name,
		BaseURL: existing.BaseURL,
		APIKey:  existing.APIKey,
		Enabled: false,
		Favicon: existing.Favicon,
	}
	if err := d.providerService.Update(existing.ID, config); err != nil {
		log.Printf("⚠️  [DISCOVERY] Failed to disable %s: %v", name, err)
	}

	// Hide its models
	d.db.Exec("UPDATE models SET is_visible = 0 WHERE provider_id = ?", existing.ID)
}

// StartBackgroundDiscovery runs discovery periodically to pick up newly started
// local providers and model changes (pulls/deletes)
func (d *LocalProviderDiscovery) StartBackgroundDiscovery(interval time.Duration) {
	// Run once immediately at startup
	d.DiscoverAndSync()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("⏰ [DISCOVERY] Local provider discovery running every %v", interval)

	for range ticker.C {
		d.DiscoverAndSync()
	}
}
