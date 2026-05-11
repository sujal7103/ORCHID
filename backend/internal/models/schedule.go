package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Schedule represents a cron-based schedule for agent execution
type Schedule struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AgentID        string             `bson:"agentId" json:"agentId"`
	UserID         string             `bson:"userId" json:"userId"`
	CronExpression string             `bson:"cronExpression" json:"cronExpression"`
	Timezone       string             `bson:"timezone" json:"timezone"`
	Enabled        bool               `bson:"enabled" json:"enabled"`
	InputTemplate  map[string]any     `bson:"inputTemplate,omitempty" json:"inputTemplate,omitempty"`

	// Tracking
	NextRunAt *time.Time `bson:"nextRunAt,omitempty" json:"nextRunAt,omitempty"`
	LastRunAt *time.Time `bson:"lastRunAt,omitempty" json:"lastRunAt,omitempty"`

	// Statistics
	TotalRuns      int64 `bson:"totalRuns" json:"totalRuns"`
	SuccessfulRuns int64 `bson:"successfulRuns" json:"successfulRuns"`
	FailedRuns     int64 `bson:"failedRuns" json:"failedRuns"`

	// Timestamps
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// CreateScheduleRequest represents a request to create a schedule
type CreateScheduleRequest struct {
	CronExpression string         `json:"cronExpression" validate:"required"`
	Timezone       string         `json:"timezone" validate:"required"`
	InputTemplate  map[string]any `json:"inputTemplate,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"` // Defaults to true
}

// UpdateScheduleRequest represents a request to update a schedule
type UpdateScheduleRequest struct {
	CronExpression *string        `json:"cronExpression,omitempty"`
	Timezone       *string        `json:"timezone,omitempty"`
	InputTemplate  map[string]any `json:"inputTemplate,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
}

// ScheduleResponse represents the API response for a schedule
type ScheduleResponse struct {
	ID             string         `json:"id"`
	AgentID        string         `json:"agentId"`
	CronExpression string         `json:"cronExpression"`
	Timezone       string         `json:"timezone"`
	Enabled        bool           `json:"enabled"`
	InputTemplate  map[string]any `json:"inputTemplate,omitempty"`
	NextRunAt      *time.Time     `json:"nextRunAt,omitempty"`
	LastRunAt      *time.Time     `json:"lastRunAt,omitempty"`
	TotalRuns      int64          `json:"totalRuns"`
	SuccessfulRuns int64          `json:"successfulRuns"`
	FailedRuns     int64          `json:"failedRuns"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// ToResponse converts a Schedule to ScheduleResponse
func (s *Schedule) ToResponse() *ScheduleResponse {
	return &ScheduleResponse{
		ID:             s.ID.Hex(),
		AgentID:        s.AgentID,
		CronExpression: s.CronExpression,
		Timezone:       s.Timezone,
		Enabled:        s.Enabled,
		InputTemplate:  s.InputTemplate,
		NextRunAt:      s.NextRunAt,
		LastRunAt:      s.LastRunAt,
		TotalRuns:      s.TotalRuns,
		SuccessfulRuns: s.SuccessfulRuns,
		FailedRuns:     s.FailedRuns,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// TierLimits defines rate limits and quotas per subscription tier
type TierLimits struct {
	MaxSchedules        int   `json:"maxSchedules"`
	MaxAPIKeys          int   `json:"maxApiKeys"`
	RequestsPerMinute   int64 `json:"requestsPerMinute"`
	RequestsPerHour     int64 `json:"requestsPerHour"`
	RetentionDays       int   `json:"retentionDays"`
	MaxExecutionsPerDay int64 `json:"maxExecutionsPerDay"`

	// Usage limits
	MaxMessagesPerMonth       int64 `json:"maxMessagesPerMonth"`       // Monthly message count limit
	MaxFileUploadsPerDay      int64 `json:"maxFileUploadsPerDay"`      // Daily file upload limit
	MaxImageGensPerDay        int64 `json:"maxImageGensPerDay"`        // Daily image generation limit
	MaxMemoryExtractionsPerDay int64 `json:"maxMemoryExtractionsPerDay"` // Daily memory extraction limit
}

// DefaultTierLimits provides tier configurations
var DefaultTierLimits = map[string]TierLimits{
	"free": {
		MaxSchedules:               5,
		MaxAPIKeys:                 3,
		RequestsPerMinute:          60,
		RequestsPerHour:            1000,
		RetentionDays:              30,
		MaxExecutionsPerDay:        100,
		MaxMessagesPerMonth:        300,
		MaxFileUploadsPerDay:       10,
		MaxImageGensPerDay:         10,
		MaxMemoryExtractionsPerDay: 15, // ~15 extractions/day for free tier
	},
	"pro": {
		MaxSchedules:               50,
		MaxAPIKeys:                 50,
		RequestsPerMinute:          300,
		RequestsPerHour:            5000,
		RetentionDays:              30,
		MaxExecutionsPerDay:        1000,
		MaxMessagesPerMonth:        10000,
		MaxFileUploadsPerDay:       50,
		MaxImageGensPerDay:         50,
		MaxMemoryExtractionsPerDay: 100, // ~100 extractions/day for pro
	},
	"max": {
		MaxSchedules:               100,
		MaxAPIKeys:                 100,
		RequestsPerMinute:          500,
		RequestsPerHour:            10000,
		RetentionDays:              30,
		MaxExecutionsPerDay:        2000,
		MaxMessagesPerMonth:        -1,  // unlimited
		MaxFileUploadsPerDay:       -1,  // unlimited
		MaxImageGensPerDay:         -1,  // unlimited
		MaxMemoryExtractionsPerDay: -1,  // unlimited
	},
	"enterprise": {
		MaxSchedules:               -1,  // unlimited
		MaxAPIKeys:                 -1,
		RequestsPerMinute:          1000,
		RequestsPerHour:            -1,  // unlimited
		RetentionDays:              365,
		MaxExecutionsPerDay:        -1,  // unlimited
		MaxMessagesPerMonth:        -1,  // unlimited
		MaxFileUploadsPerDay:       -1,  // unlimited
		MaxImageGensPerDay:         -1,  // unlimited
		MaxMemoryExtractionsPerDay: -1,  // unlimited
	},
	"legacy_unlimited": {
		MaxSchedules:               -1,  // unlimited
		MaxAPIKeys:                 -1,  // unlimited
		RequestsPerMinute:          -1,  // unlimited
		RequestsPerHour:            -1,  // unlimited
		RetentionDays:              365,
		MaxExecutionsPerDay:        -1,  // unlimited
		MaxMessagesPerMonth:        -1,  // unlimited
		MaxFileUploadsPerDay:       -1,  // unlimited
		MaxImageGensPerDay:         -1,  // unlimited
		MaxMemoryExtractionsPerDay: -1,  // unlimited
	},
}

// GetTierLimits returns the limits for a given tier
func GetTierLimits(tier string) TierLimits {
	if limits, ok := DefaultTierLimits[tier]; ok {
		return limits
	}
	return DefaultTierLimits["free"]
}
