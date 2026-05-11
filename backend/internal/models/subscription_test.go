package models

import (
	"testing"
	"time"
)

func TestPlanComparison(t *testing.T) {
	tests := []struct {
		name     string
		fromTier string
		toTier   string
		expected int // -1 = upgrade, 0 = same, 1 = downgrade
	}{
		{"free to pro is upgrade", TierFree, TierPro, -1},
		{"free to max is upgrade", TierFree, TierMax, -1},
		{"pro to max is upgrade", TierPro, TierMax, -1},
		{"max to pro is downgrade", TierMax, TierPro, 1},
		{"pro to free is downgrade", TierPro, TierFree, 1},
		{"max to free is downgrade", TierMax, TierFree, 1},
		{"same tier", TierPro, TierPro, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareTiers(tt.fromTier, tt.toTier)
			if result != tt.expected {
				t.Errorf("CompareTiers(%s, %s) = %d, want %d",
					tt.fromTier, tt.toTier, result, tt.expected)
			}
		})
	}
}

func TestSubscriptionStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{SubStatusActive, true},
		{SubStatusOnHold, true},        // Still active during grace
		{SubStatusPendingCancel, true}, // Active until period ends
		{SubStatusCancelled, false},
		{SubStatusPaused, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			sub := &Subscription{Status: tt.status}
			if sub.IsActive() != tt.expected {
				t.Errorf("IsActive() for status %s = %v, want %v",
					tt.status, sub.IsActive(), tt.expected)
			}
		})
	}
}

func TestSubscription_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		periodEnd time.Time
		expected  bool
	}{
		{"future date", now.Add(24 * time.Hour), false},
		{"past date", now.Add(-24 * time.Hour), true},
		{"just now", now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{CurrentPeriodEnd: tt.periodEnd}
			if sub.IsExpired() != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", sub.IsExpired(), tt.expected)
			}
		})
	}
}

func TestSubscription_HasScheduledChange(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name          string
		scheduledTier string
		scheduledAt   *time.Time
		expected      bool
	}{
		{"no scheduled change", "", nil, false},
		{"has scheduled downgrade", TierPro, &future, true},
		{"empty tier with date", "", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				ScheduledTier:     tt.scheduledTier,
				ScheduledChangeAt: tt.scheduledAt,
			}
			if sub.HasScheduledChange() != tt.expected {
				t.Errorf("HasScheduledChange() = %v, want %v",
					sub.HasScheduledChange(), tt.expected)
			}
		})
	}
}

func TestPlan_GetByID(t *testing.T) {
	plans := GetAvailablePlans()

	// Test finding each plan
	for _, plan := range plans {
		found := GetPlanByID(plan.ID)
		if found == nil {
			t.Errorf("GetPlanByID(%s) returned nil", plan.ID)
		}
		if found != nil && found.ID != plan.ID {
			t.Errorf("GetPlanByID(%s) returned plan with ID %s", plan.ID, found.ID)
		}
	}

	// Test non-existent plan
	if GetPlanByID("nonexistent") != nil {
		t.Error("Expected nil for non-existent plan")
	}
}

func TestTierLimits_Max(t *testing.T) {
	limits := GetTierLimits(TierMax)

	if limits.MaxSchedules != 100 {
		t.Errorf("Expected MaxSchedules 100, got %d", limits.MaxSchedules)
	}
	if limits.MaxAPIKeys != 100 {
		t.Errorf("Expected MaxAPIKeys 100, got %d", limits.MaxAPIKeys)
	}
}

func TestGetPlanByTier(t *testing.T) {
	tests := []struct {
		tier     string
		expected string
	}{
		{TierFree, "free"},
		{TierPro, "pro"},
		{TierMax, "max"},
		{TierEnterprise, "enterprise"},
		{"invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			plan := GetPlanByTier(tt.tier)
			if tt.expected == "" {
				if plan != nil {
					t.Errorf("Expected nil for invalid tier, got %v", plan)
				}
			} else {
				if plan == nil {
					t.Errorf("Expected plan for tier %s, got nil", tt.tier)
				} else if plan.ID != tt.expected {
					t.Errorf("Expected plan ID %s, got %s", tt.expected, plan.ID)
				}
			}
		})
	}
}
