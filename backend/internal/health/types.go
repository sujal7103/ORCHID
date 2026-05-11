package health

import "time"

// CapabilityType identifies what kind of provider/model interaction a health entry covers
type CapabilityType string

const (
	CapabilityChat      CapabilityType = "chat"
	CapabilityVision    CapabilityType = "vision"
	CapabilityImageGen  CapabilityType = "image_gen"
	CapabilityImageEdit CapabilityType = "image_edit"
	CapabilityAudio     CapabilityType = "audio"
)

// HealthStatus represents the health state of a provider
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusCooldown  HealthStatus = "cooldown"
	StatusUnknown   HealthStatus = "unknown"
)

// ProviderHealth tracks the health of a single provider+model+capability combination
type ProviderHealth struct {
	ProviderID    int
	ProviderName  string
	ModelName     string // empty for capability-only providers (image_gen, audio)
	Capability    CapabilityType
	Status        HealthStatus
	LastChecked   time.Time
	LastSuccessAt time.Time
	FailureCount  int
	LastError     string
	CooldownUntil time.Time
	Priority      int // Higher = preferred
}

// ProviderInfo is a minimal provider representation used by health checks
// to avoid importing the models package
type ProviderInfo struct {
	ID      int
	Name    string
	BaseURL string
	APIKey  string
	Enabled bool
}

// ProviderGetter retrieves provider info by ID
type ProviderGetter func(id int) (*ProviderInfo, error)

// HealthCheckStrategy is the interface for capability-specific health checks
type HealthCheckStrategy interface {
	// Check performs a lightweight health check for a registered provider entry.
	// Returns latency in milliseconds and any error encountered.
	Check(entry *ProviderHealth, getter ProviderGetter) (latencyMs int, err error)
	// Capability returns which capability type this strategy checks.
	Capability() CapabilityType
}
