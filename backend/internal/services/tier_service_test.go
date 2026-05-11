package services

import (
	"context"
	"testing"
)

func TestNewTierService(t *testing.T) {
	// Test without MongoDB (nil)
	service := NewTierService(nil)
	if service == nil {
		t.Fatal("Expected non-nil tier service")
	}
}

func TestTierService_GetUserTier_DefaultsToFree(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	tier := service.GetUserTier(ctx, "user-123")
	if tier != "free" {
		t.Errorf("Expected 'free' tier, got %s", tier)
	}
}

func TestTierService_GetLimits(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	limits := service.GetLimits(ctx, "user-123")

	// Default to free tier limits
	if limits.MaxSchedules != 5 {
		t.Errorf("Expected MaxSchedules 5, got %d", limits.MaxSchedules)
	}

	if limits.MaxAPIKeys != 3 {
		t.Errorf("Expected MaxAPIKeys 3, got %d", limits.MaxAPIKeys)
	}
}

func TestTierService_CheckScheduleLimit(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	tests := []struct {
		name         string
		currentCount int64
		expected     bool
	}{
		{"under limit", 3, true},
		{"at limit", 5, false},
		{"over limit", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.CheckScheduleLimit(ctx, "user-123", tt.currentCount)
			if result != tt.expected {
				t.Errorf("Expected %v for currentCount %d, got %v", tt.expected, tt.currentCount, result)
			}
		})
	}
}

func TestTierService_CheckAPIKeyLimit(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	tests := []struct {
		name         string
		currentCount int64
		expected     bool
	}{
		{"under limit", 1, true},
		{"at limit", 3, false},
		{"over limit", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.CheckAPIKeyLimit(ctx, "user-123", tt.currentCount)
			if result != tt.expected {
				t.Errorf("Expected %v for currentCount %d, got %v", tt.expected, tt.currentCount, result)
			}
		})
	}
}

func TestTierService_GetRateLimits(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	rateLimits := service.GetRateLimits(ctx, "user-123")

	// Default to free tier rate limits
	if rateLimits.RequestsPerMinute != 60 {
		t.Errorf("Expected RequestsPerMinute 60, got %d", rateLimits.RequestsPerMinute)
	}

	if rateLimits.RequestsPerHour != 1000 {
		t.Errorf("Expected RequestsPerHour 1000, got %d", rateLimits.RequestsPerHour)
	}
}

func TestTierService_GetExecutionRetentionDays(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	days := service.GetExecutionRetentionDays(ctx, "user-123")

	// Default to free tier retention
	if days != 30 {
		t.Errorf("Expected 30 days retention, got %d", days)
	}
}

func TestTierService_InvalidateCache(t *testing.T) {
	service := NewTierService(nil)
	ctx := context.Background()

	// Get tier to populate cache
	_ = service.GetUserTier(ctx, "user-123")

	// Invalidate cache
	service.InvalidateCache("user-123")

	// Should still return free (default) but cache should be empty
	tier := service.GetUserTier(ctx, "user-123")
	if tier != "free" {
		t.Errorf("Expected 'free' tier after cache invalidation, got %s", tier)
	}
}
