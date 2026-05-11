package models

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestScheduleToResponse(t *testing.T) {
	now := time.Now()
	nextRun := now.Add(1 * time.Hour)
	lastRun := now.Add(-1 * time.Hour)

	schedule := &Schedule{
		ID:             primitive.NewObjectID(),
		AgentID:        "agent-123",
		UserID:         "user-456",
		CronExpression: "0 9 * * *",
		Timezone:       "America/New_York",
		Enabled:        true,
		InputTemplate:  map[string]interface{}{"topic": "AI news"},
		NextRunAt:      &nextRun,
		LastRunAt:      &lastRun,
		TotalRuns:      10,
		SuccessfulRuns: 8,
		FailedRuns:     2,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	resp := schedule.ToResponse()

	if resp.ID != schedule.ID.Hex() {
		t.Errorf("Expected ID %s, got %s", schedule.ID.Hex(), resp.ID)
	}

	if resp.AgentID != schedule.AgentID {
		t.Errorf("Expected AgentID %s, got %s", schedule.AgentID, resp.AgentID)
	}

	if resp.CronExpression != schedule.CronExpression {
		t.Errorf("Expected CronExpression %s, got %s", schedule.CronExpression, resp.CronExpression)
	}

	if resp.Timezone != schedule.Timezone {
		t.Errorf("Expected Timezone %s, got %s", schedule.Timezone, resp.Timezone)
	}

	if resp.Enabled != schedule.Enabled {
		t.Errorf("Expected Enabled %v, got %v", schedule.Enabled, resp.Enabled)
	}

	if resp.TotalRuns != schedule.TotalRuns {
		t.Errorf("Expected TotalRuns %d, got %d", schedule.TotalRuns, resp.TotalRuns)
	}
}

func TestGetTierLimits(t *testing.T) {
	tests := []struct {
		tier         string
		maxSchedules int
		maxAPIKeys   int
	}{
		{"free", 5, 3},
		{"pro", 50, 50},
		{"max", 100, 100},
		{"enterprise", -1, -1},       // unlimited
		{"legacy_unlimited", -1, -1}, // unlimited
		{"unknown", 5, 3},            // defaults to free
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			limits := GetTierLimits(tt.tier)

			if limits.MaxSchedules != tt.maxSchedules {
				t.Errorf("Expected MaxSchedules %d for tier %s, got %d", tt.maxSchedules, tt.tier, limits.MaxSchedules)
			}

			if limits.MaxAPIKeys != tt.maxAPIKeys {
				t.Errorf("Expected MaxAPIKeys %d for tier %s, got %d", tt.maxAPIKeys, tt.tier, limits.MaxAPIKeys)
			}
		})
	}
}

func TestDefaultTierLimits(t *testing.T) {
	// Verify all expected tiers exist
	expectedTiers := []string{"free", "pro", "max", "enterprise", "legacy_unlimited"}

	for _, tier := range expectedTiers {
		if _, ok := DefaultTierLimits[tier]; !ok {
			t.Errorf("Expected tier %s in DefaultTierLimits", tier)
		}
	}

	// Verify free tier has reasonable defaults
	freeLimits := DefaultTierLimits["free"]
	if freeLimits.MaxSchedules <= 0 {
		t.Error("Free tier should have positive MaxSchedules limit")
	}
	if freeLimits.RequestsPerMinute <= 0 {
		t.Error("Free tier should have positive RequestsPerMinute limit")
	}
	if freeLimits.RetentionDays <= 0 {
		t.Error("Free tier should have positive RetentionDays")
	}

	// Verify enterprise tier has unlimited (-1) for schedules and API keys
	enterpriseLimits := DefaultTierLimits["enterprise"]
	if enterpriseLimits.MaxSchedules != -1 {
		t.Error("Enterprise tier should have unlimited MaxSchedules (-1)")
	}
	if enterpriseLimits.MaxAPIKeys != -1 {
		t.Error("Enterprise tier should have unlimited MaxAPIKeys (-1)")
	}
}

func TestCreateScheduleRequest(t *testing.T) {
	enabled := true
	req := CreateScheduleRequest{
		CronExpression: "0 9 * * *",
		Timezone:       "UTC",
		InputTemplate:  map[string]interface{}{"key": "value"},
		Enabled:        &enabled,
	}

	if req.CronExpression != "0 9 * * *" {
		t.Errorf("Expected CronExpression '0 9 * * *', got %s", req.CronExpression)
	}

	if req.Timezone != "UTC" {
		t.Errorf("Expected Timezone 'UTC', got %s", req.Timezone)
	}

	if req.Enabled == nil || *req.Enabled != true {
		t.Error("Expected Enabled to be true")
	}
}
