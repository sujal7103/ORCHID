package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// CacheEntry stores cached tier with expiration info for TTL-based invalidation
type CacheEntry struct {
	Tier      string
	ExpiresAt *time.Time // For promo users, this is subscriptionExpiresAt; nil for regular users
	CachedAt  time.Time
}

// TierService manages subscription tier limits and lookups
type TierService struct {
	mongoDB    *database.MongoDB
	cache      map[string]CacheEntry // userID -> CacheEntry with TTL info
	mu         sync.RWMutex
	defaultTTL time.Duration // Default cache TTL for non-promo users
}

// NewTierService creates a new tier service
func NewTierService(mongoDB *database.MongoDB) *TierService {
	return &TierService{
		mongoDB:    mongoDB,
		cache:      make(map[string]CacheEntry),
		defaultTTL: 5 * time.Minute,
	}
}

// GetUserTier returns the subscription tier for a user
func (s *TierService) GetUserTier(ctx context.Context, userID string) string {
	now := time.Now()

	// Check cache first
	s.mu.RLock()
	if entry, ok := s.cache[userID]; ok {
		// Check if cache entry is still valid
		// For promo users: check if promo has expired
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			s.mu.RUnlock()
			// Promo expired - invalidate cache and re-fetch
			s.InvalidateCache(userID)
			log.Printf("🔄 [TIER] Promo expired for user %s, re-fetching tier", userID)
			return s.fetchAndCacheTier(ctx, userID)
		}

		// For non-promo users: check default TTL
		if entry.ExpiresAt == nil && now.Sub(entry.CachedAt) > s.defaultTTL {
			s.mu.RUnlock()
			// Cache TTL exceeded - re-fetch
			s.InvalidateCache(userID)
			return s.fetchAndCacheTier(ctx, userID)
		}

		s.mu.RUnlock()
		return entry.Tier
	}
	s.mu.RUnlock()

	return s.fetchAndCacheTier(ctx, userID)
}

// fetchAndCacheTier fetches the tier from database and caches it
func (s *TierService) fetchAndCacheTier(ctx context.Context, userID string) string {
	// Default to free tier
	tier := models.TierFree
	var expiresAt *time.Time

	// Look up from MongoDB if available
	if s.mongoDB != nil {
		collection := s.mongoDB.Database().Collection("users")

		var user struct {
			SubscriptionTier      string     `bson:"subscriptionTier"`
			SubscriptionStatus    string     `bson:"subscriptionStatus"`
			SubscriptionExpiresAt *time.Time `bson:"subscriptionExpiresAt"`
		}

		err := collection.FindOne(ctx, bson.M{"supabaseUserId": userID}).Decode(&user)
		if err == nil {
			// Check if subscription is still valid
			if user.SubscriptionStatus == models.SubStatusOnHold {
				// Grace period - keep features for now
				if user.SubscriptionTier != "" {
					tier = user.SubscriptionTier
					expiresAt = user.SubscriptionExpiresAt
				}
			} else if user.SubscriptionStatus == models.SubStatusCancelled {
				// Only downgrade if explicitly cancelled (not just expired)
				// Pro tier is now permanent for all users - expiration dates are ignored
				tier = models.TierFree
				expiresAt = nil
			} else if user.SubscriptionTier != "" {
				// Active subscription - ignore expiration (Pro is permanent)
				tier = user.SubscriptionTier
				// Don't track expiration since Pro is now permanent
				expiresAt = nil
			}
		}
	}

	// Cache the result with expiration info
	s.mu.Lock()
	s.cache[userID] = CacheEntry{
		Tier:      tier,
		ExpiresAt: expiresAt,
		CachedAt:  time.Now(),
	}
	s.mu.Unlock()

	return tier
}

// GetLimits returns the limits for a user based on their tier
func (s *TierService) GetLimits(ctx context.Context, userID string) models.TierLimits {
	// Get base tier
	tier := s.GetUserTier(ctx, userID)
	baseLimits := models.GetTierLimits(tier)

	// Check for granular limit overrides
	if s.mongoDB != nil {
		collection := s.mongoDB.Database().Collection("users")

		var user struct {
			LimitOverrides *models.TierLimits `bson:"limitOverrides"`
		}

		err := collection.FindOne(ctx, bson.M{"supabaseUserId": userID}).Decode(&user)
		if err == nil && user.LimitOverrides != nil {
			// Merge overrides with base limits
			return s.mergeLimits(baseLimits, *user.LimitOverrides)
		}
	}

	return baseLimits
}

// mergeLimits merges override limits with base limits
// Non-zero override values replace base limits
// Zero override values are ignored (use base limit)
func (s *TierService) mergeLimits(base, override models.TierLimits) models.TierLimits {
	result := base // Start with base limits

	// Override each field if non-zero
	if override.MaxSchedules != 0 {
		result.MaxSchedules = override.MaxSchedules
	}
	if override.MaxAPIKeys != 0 {
		result.MaxAPIKeys = override.MaxAPIKeys
	}
	if override.RequestsPerMinute != 0 {
		result.RequestsPerMinute = override.RequestsPerMinute
	}
	if override.RequestsPerHour != 0 {
		result.RequestsPerHour = override.RequestsPerHour
	}
	if override.RetentionDays != 0 {
		result.RetentionDays = override.RetentionDays
	}
	if override.MaxExecutionsPerDay != 0 {
		result.MaxExecutionsPerDay = override.MaxExecutionsPerDay
	}
	if override.MaxMessagesPerMonth != 0 {
		result.MaxMessagesPerMonth = override.MaxMessagesPerMonth
	}
	if override.MaxFileUploadsPerDay != 0 {
		result.MaxFileUploadsPerDay = override.MaxFileUploadsPerDay
	}
	if override.MaxImageGensPerDay != 0 {
		result.MaxImageGensPerDay = override.MaxImageGensPerDay
	}

	return result
}

// InvalidateCache removes a user from the cache (call when tier changes)
func (s *TierService) InvalidateCache(userID string) {
	s.mu.Lock()
	delete(s.cache, userID)
	s.mu.Unlock()
	log.Printf("🔄 [TIER] Invalidated cache for user %s", userID)
}

// CheckScheduleLimit checks if user can create another schedule
func (s *TierService) CheckScheduleLimit(ctx context.Context, userID string, currentCount int64) bool {
	limits := s.GetLimits(ctx, userID)
	if limits.MaxSchedules < 0 {
		return true // Unlimited
	}
	return currentCount < int64(limits.MaxSchedules)
}

// CheckAPIKeyLimit checks if user can create another API key
func (s *TierService) CheckAPIKeyLimit(ctx context.Context, userID string, currentCount int64) bool {
	limits := s.GetLimits(ctx, userID)
	if limits.MaxAPIKeys < 0 {
		return true // Unlimited
	}
	return currentCount < int64(limits.MaxAPIKeys)
}

// RateLimitConfig holds rate limit values
type RateLimitConfig struct {
	RequestsPerMinute int64
	RequestsPerHour   int64
}

// GetRateLimits returns the rate limit configuration for a user
func (s *TierService) GetRateLimits(ctx context.Context, userID string) RateLimitConfig {
	limits := s.GetLimits(ctx, userID)
	return RateLimitConfig{
		RequestsPerMinute: limits.RequestsPerMinute,
		RequestsPerHour:   limits.RequestsPerHour,
	}
}

// GetExecutionRetentionDays returns how long to keep execution history
func (s *TierService) GetExecutionRetentionDays(ctx context.Context, userID string) int {
	limits := s.GetLimits(ctx, userID)
	return limits.RetentionDays
}

// CheckMessageLimit checks if user can send another message this month
func (s *TierService) CheckMessageLimit(ctx context.Context, userID string, currentCount int64) bool {
	limits := s.GetLimits(ctx, userID)
	if limits.MaxMessagesPerMonth < 0 {
		return true // Unlimited
	}
	return currentCount < limits.MaxMessagesPerMonth
}

// CheckFileUploadLimit checks if user can upload another file today
func (s *TierService) CheckFileUploadLimit(ctx context.Context, userID string, currentCount int64) bool {
	limits := s.GetLimits(ctx, userID)
	if limits.MaxFileUploadsPerDay < 0 {
		return true // Unlimited
	}
	return currentCount < limits.MaxFileUploadsPerDay
}

// CheckImageGenLimit checks if user can generate another image today
func (s *TierService) CheckImageGenLimit(ctx context.Context, userID string, currentCount int64) bool {
	limits := s.GetLimits(ctx, userID)
	if limits.MaxImageGensPerDay < 0 {
		return true // Unlimited
	}
	return currentCount < limits.MaxImageGensPerDay
}
