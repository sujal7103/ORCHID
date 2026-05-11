package models

import "time"

// Provider represents an AI API provider (OpenAI, Anthropic, etc.)
type Provider struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"base_url"`
	APIKey        string    `json:"api_key,omitempty"` // Omit from responses for security
	Enabled       bool      `json:"enabled"`
	AudioOnly     bool      `json:"audio_only,omitempty"`      // If true, provider is only used for audio transcription (not shown in model list)
	ImageOnly     bool      `json:"image_only,omitempty"`      // If true, provider is only used for image generation (not shown in model list)
	ImageEditOnly bool      `json:"image_edit_only,omitempty"` // If true, provider is only used for image editing (not shown in model list)
	Secure        bool      `json:"secure,omitempty"`          // If true, provider doesn't store user data
	DefaultModel  string    `json:"default_model,omitempty"`   // Default model for image generation
	SystemPrompt  string    `json:"system_prompt,omitempty"`
	Favicon       string    `json:"favicon,omitempty"` // Optional favicon URL for the provider
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ModelAlias represents a model alias with display name and description
type ModelAlias struct {
	ActualModel                 string `json:"actual_model"`                             // The actual model name to use with the provider API
	DisplayName                 string `json:"display_name"`                             // Human-readable name shown in the UI
	Description                 string `json:"description,omitempty"`                    // Optional description for the model
	SupportsVision              *bool  `json:"supports_vision,omitempty"`                // Optional override for vision/image support
	Agents                      *bool  `json:"agents,omitempty"`                         // If true, model is available for agent builder. If false/nil, hidden from agents
	SmartToolRouter             *bool  `json:"smart_tool_router,omitempty"`              // If true, model can be used as tool predictor for chat
	FreeTier                    *bool  `json:"free_tier,omitempty"`                      // If true, model is available for anonymous/free tier users
	StructuredOutputSupport     string `json:"structured_output_support,omitempty"`      // Structured output quality: "excellent", "good", "poor", "unknown"
	StructuredOutputCompliance  *int   `json:"structured_output_compliance,omitempty"`   // Compliance percentage (0-100)
	StructuredOutputWarning     string `json:"structured_output_warning,omitempty"`      // Warning message about structured output
	StructuredOutputSpeedMs     *int   `json:"structured_output_speed_ms,omitempty"`     // Average response time in milliseconds
	StructuredOutputBadge       string `json:"structured_output_badge,omitempty"`        // Badge label (e.g., "FASTEST")
	MemoryExtractor             *bool  `json:"memory_extractor,omitempty"`               // If true, model can extract memories from conversations
	MemorySelector              *bool  `json:"memory_selector,omitempty"`                // If true, model can select relevant memories for context
}

// ModelAliasView represents a model alias from the database (includes DB metadata)
type ModelAliasView struct {
	ID                          int       `json:"id"`
	AliasName                   string    `json:"alias_name"`
	ModelID                     string    `json:"model_id"`
	ProviderID                  int       `json:"provider_id"`
	DisplayName                 string    `json:"display_name"`
	Description                 *string   `json:"description,omitempty"`
	SupportsVision              *bool     `json:"supports_vision,omitempty"`
	AgentsEnabled               *bool     `json:"agents_enabled,omitempty"`
	SmartToolRouter             *bool     `json:"smart_tool_router,omitempty"`
	FreeTier                    *bool     `json:"free_tier,omitempty"`
	StructuredOutputSupport     *string   `json:"structured_output_support,omitempty"`
	StructuredOutputCompliance  *int      `json:"structured_output_compliance,omitempty"`
	StructuredOutputWarning     *string   `json:"structured_output_warning,omitempty"`
	StructuredOutputSpeedMs     *int      `json:"structured_output_speed_ms,omitempty"`
	StructuredOutputBadge       *string   `json:"structured_output_badge,omitempty"`
	MemoryExtractor             *bool     `json:"memory_extractor,omitempty"`
	MemorySelector              *bool     `json:"memory_selector,omitempty"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

// RecommendedModels represents recommended model tiers
type RecommendedModels struct {
	Top     string `json:"top"`     // Best/most capable model
	Medium  string `json:"medium"`  // Balanced model
	Fastest string `json:"fastest"` // Fastest/cheapest model
	New     string `json:"new"`     // Newly added model
}

// ProvidersConfig represents the providers.json file structure
type ProvidersConfig struct {
	Providers []ProviderConfig `json:"providers"`
}

// ProviderConfig represents a provider configuration from JSON
type ProviderConfig struct {
	Name              string                `json:"name"`
	BaseURL           string                `json:"base_url"`
	APIKey            string                `json:"api_key"`
	Enabled           bool                  `json:"enabled"`
	Secure            bool                  `json:"secure,omitempty"`             // Indicates provider doesn't store user data
	AudioOnly         bool                  `json:"audio_only,omitempty"`         // If true, provider is only used for audio transcription (not shown in model list)
	ImageOnly         bool                  `json:"image_only,omitempty"`         // If true, provider is only used for image generation (not shown in model list)
	ImageEditOnly     bool                  `json:"image_edit_only,omitempty"`    // If true, provider is only used for image editing (not shown in model list)
	DefaultModel      string                `json:"default_model,omitempty"`      // Default model for image generation
	SystemPrompt      string                `json:"system_prompt,omitempty"`
	Favicon           string                `json:"favicon,omitempty"`            // Optional favicon URL
	Filters           []FilterConfig        `json:"filters"`
	ModelAliases      map[string]ModelAlias `json:"model_aliases,omitempty"`      // Maps frontend model names to actual model names with descriptions
	RecommendedModels *RecommendedModels    `json:"recommended_models,omitempty"` // Recommended model tiers
}

// FilterConfig represents a filter configuration from JSON
type FilterConfig struct {
	Pattern  string `json:"pattern"`
	Action   string `json:"action"`   // "include" or "exclude"
	Priority int    `json:"priority"` // Higher priority = applied first
}
