package health

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

const (
	defaultFailureThreshold = 3
	defaultCooldownDuration = 1 * time.Hour
)

// Service manages health tracking for all providers across all capability types
type Service struct {
	mu               sync.RWMutex
	healthCache      map[string]*ProviderHealth // key: "capability:providerID:modelName"
	strategies       map[CapabilityType]HealthCheckStrategy
	providerGetter   ProviderGetter
	failureThreshold int
	cooldownDuration time.Duration
}

// NewService creates a new health service
func NewService(providerGetter ProviderGetter, failureThreshold int, cooldownDuration time.Duration) *Service {
	if failureThreshold <= 0 {
		failureThreshold = defaultFailureThreshold
	}
	if cooldownDuration <= 0 {
		cooldownDuration = defaultCooldownDuration
	}

	return &Service{
		healthCache:      make(map[string]*ProviderHealth),
		strategies:       make(map[CapabilityType]HealthCheckStrategy),
		providerGetter:   providerGetter,
		failureThreshold: failureThreshold,
		cooldownDuration: cooldownDuration,
	}
}

func cacheKey(capability CapabilityType, providerID int, modelName string) string {
	return fmt.Sprintf("%s:%d:%s", capability, providerID, modelName)
}

// RegisterStrategy registers a health check strategy for a capability type
func (s *Service) RegisterStrategy(strategy HealthCheckStrategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategies[strategy.Capability()] = strategy
}

// RegisterProvider adds a provider to the health cache for a given capability
func (s *Service) RegisterProvider(capability CapabilityType, providerID int, providerName string, modelName string, priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cacheKey(capability, providerID, modelName)
	if _, exists := s.healthCache[key]; !exists {
		s.healthCache[key] = &ProviderHealth{
			ProviderID:   providerID,
			ProviderName: providerName,
			ModelName:    modelName,
			Capability:   capability,
			Status:       StatusUnknown,
			Priority:     priority,
		}
		log.Printf("[HEALTH] Registered %s provider %s (ID:%d) model=%s priority=%d",
			capability, providerName, providerID, modelName, priority)
	}
}

// GetHealthyProviders returns providers for a capability, ordered by priority, filtering out unhealthy/cooldown
func (s *Service) GetHealthyProviders(capability CapabilityType) []ProviderHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var healthy []ProviderHealth

	for _, h := range s.healthCache {
		if h.Capability != capability {
			continue
		}
		switch h.Status {
		case StatusCooldown:
			if now.After(h.CooldownUntil) {
				healthy = append(healthy, *h) // cooldown expired
			}
		case StatusUnhealthy:
			continue
		default:
			healthy = append(healthy, *h) // healthy or unknown
		}
	}

	sort.Slice(healthy, func(i, j int) bool {
		if healthy[i].Priority != healthy[j].Priority {
			return healthy[i].Priority > healthy[j].Priority
		}
		return healthy[i].LastSuccessAt.After(healthy[j].LastSuccessAt)
	})

	return healthy
}

// GetAllProviders returns all registered providers for a capability regardless of health
func (s *Service) GetAllProviders(capability CapabilityType) []ProviderHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ProviderHealth
	for _, h := range s.healthCache {
		if h.Capability == capability {
			result = append(result, *h)
		}
	}
	return result
}

// GetAllRegistered returns all registered providers across all capabilities
func (s *Service) GetAllRegistered() []ProviderHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ProviderHealth, 0, len(s.healthCache))
	for _, h := range s.healthCache {
		result = append(result, *h)
	}
	return result
}

// IsProviderHealthy checks if a specific provider is considered healthy for a capability
func (s *Service) IsProviderHealthy(capability CapabilityType, providerID int, modelName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := cacheKey(capability, providerID, modelName)
	h, exists := s.healthCache[key]
	if !exists {
		return true // unknown providers are assumed healthy (backward compat)
	}

	switch h.Status {
	case StatusUnhealthy:
		return false
	case StatusCooldown:
		return time.Now().After(h.CooldownUntil)
	default:
		return true
	}
}

// MarkHealthy marks a provider as healthy after a successful request
func (s *Service) MarkHealthy(capability CapabilityType, providerID int, modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cacheKey(capability, providerID, modelName)
	h, exists := s.healthCache[key]
	if !exists {
		return
	}

	wasUnhealthy := h.Status == StatusUnhealthy || h.Status == StatusCooldown
	h.Status = StatusHealthy
	h.FailureCount = 0
	h.LastError = ""
	h.LastSuccessAt = time.Now()
	h.LastChecked = time.Now()
	h.CooldownUntil = time.Time{}

	if wasUnhealthy {
		log.Printf("[HEALTH] %s provider %s/%s recovered - now healthy",
			capability, h.ProviderName, modelName)
	}
}

// MarkUnhealthy records a failure. After reaching the threshold, the provider is marked unhealthy.
func (s *Service) MarkUnhealthy(capability CapabilityType, providerID int, modelName string, errMsg string, httpCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cacheKey(capability, providerID, modelName)
	h, exists := s.healthCache[key]
	if !exists {
		return
	}

	h.FailureCount++
	h.LastError = errMsg
	h.LastChecked = time.Now()

	if h.FailureCount >= s.failureThreshold {
		h.Status = StatusUnhealthy
		log.Printf("[HEALTH] %s provider %s/%s marked UNHEALTHY after %d failures: %s",
			capability, h.ProviderName, modelName, h.FailureCount, truncateStr(errMsg, 200))
	} else {
		log.Printf("[HEALTH] %s provider %s/%s failure %d/%d: %s",
			capability, h.ProviderName, modelName, h.FailureCount, s.failureThreshold, truncateStr(errMsg, 200))
	}
}

// SetCooldown puts a provider into cooldown (typically after a quota error)
func (s *Service) SetCooldown(capability CapabilityType, providerID int, modelName string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cacheKey(capability, providerID, modelName)
	h, exists := s.healthCache[key]
	if !exists {
		return
	}

	h.Status = StatusCooldown
	h.CooldownUntil = time.Now().Add(duration)
	h.LastChecked = time.Now()

	log.Printf("[HEALTH] %s provider %s/%s in COOLDOWN until %s (reason: %s)",
		capability, h.ProviderName, modelName, h.CooldownUntil.Format(time.RFC3339), truncateStr(h.LastError, 100))
}

// IsInCooldown checks if a provider is currently in cooldown
func (s *Service) IsInCooldown(capability CapabilityType, providerID int, modelName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := cacheKey(capability, providerID, modelName)
	h, exists := s.healthCache[key]
	if !exists {
		return false
	}

	if h.Status != StatusCooldown {
		return false
	}

	return time.Now().Before(h.CooldownUntil)
}

// CheckProviderHealth performs an active health check using the registered strategy
func (s *Service) CheckProviderHealth(capability CapabilityType, providerID int, modelName string) error {
	s.mu.RLock()
	strategy, hasStrategy := s.strategies[capability]
	key := cacheKey(capability, providerID, modelName)
	entry, exists := s.healthCache[key]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("provider not registered: %s:%d:%s", capability, providerID, modelName)
	}

	if !hasStrategy {
		// No strategy registered - just check if provider is enabled
		provider, err := s.providerGetter(providerID)
		if err != nil {
			s.MarkUnhealthy(capability, providerID, modelName, fmt.Sprintf("provider lookup failed: %v", err), 0)
			return err
		}
		if !provider.Enabled {
			s.MarkUnhealthy(capability, providerID, modelName, "provider disabled", 0)
			return fmt.Errorf("provider %s is disabled", provider.Name)
		}
		s.MarkHealthy(capability, providerID, modelName)
		return nil
	}

	_, err := strategy.Check(entry, s.providerGetter)
	if err != nil {
		if IsQuotaError(0, err.Error()) {
			cooldown := ParseCooldownDuration(0, err.Error())
			s.SetCooldown(capability, providerID, modelName, cooldown)
		} else {
			s.MarkUnhealthy(capability, providerID, modelName, err.Error(), 0)
		}
		return err
	}

	s.MarkHealthy(capability, providerID, modelName)
	return nil
}

// GetStatus returns health status summary across all capabilities
func (s *Service) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	capStats := make(map[string]map[string]int)
	total := 0

	for _, h := range s.healthCache {
		cap := string(h.Capability)
		if capStats[cap] == nil {
			capStats[cap] = map[string]int{"healthy": 0, "unhealthy": 0, "cooldown": 0, "unknown": 0}
		}
		total++

		switch h.Status {
		case StatusHealthy:
			capStats[cap]["healthy"]++
		case StatusUnhealthy:
			capStats[cap]["unhealthy"]++
		case StatusCooldown:
			if time.Now().After(h.CooldownUntil) {
				capStats[cap]["unknown"]++
			} else {
				capStats[cap]["cooldown"]++
			}
		default:
			capStats[cap]["unknown"]++
		}
	}

	return map[string]interface{}{
		"total":        total,
		"capabilities": capStats,
	}
}

// GetCapabilityStatus returns health status for a specific capability
func (s *Service) GetCapabilityStatus(capability CapabilityType) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	healthy, unhealthy, cooldown, unknown := 0, 0, 0, 0
	for _, h := range s.healthCache {
		if h.Capability != capability {
			continue
		}
		switch h.Status {
		case StatusHealthy:
			healthy++
		case StatusUnhealthy:
			unhealthy++
		case StatusCooldown:
			if time.Now().After(h.CooldownUntil) {
				unknown++
			} else {
				cooldown++
			}
		default:
			unknown++
		}
	}

	return map[string]interface{}{
		"healthy":   healthy,
		"unhealthy": unhealthy,
		"cooldown":  cooldown,
		"unknown":   unknown,
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
