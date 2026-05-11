package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NexusSession represents a persistent user session for the Nexus multi-agent system
type NexusSession struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID string             `bson:"userId" json:"user_id"`

	// Rolling context
	RecentTaskIDs  []primitive.ObjectID `bson:"recentTaskIds,omitempty" json:"recent_task_ids,omitempty"` // Last 20 completed
	ContextSummary string               `bson:"contextSummary,omitempty" json:"context_summary,omitempty"`

	// Active state
	ActiveDaemonIDs []primitive.ObjectID `bson:"activeDaemonIds,omitempty" json:"active_daemon_ids,omitempty"`
	ActiveTaskIDs   []primitive.ObjectID `bson:"activeTaskIds,omitempty" json:"active_task_ids,omitempty"`

	// Pinned skills
	PinnedSkillIDs []primitive.ObjectID `bson:"pinnedSkillIds,omitempty" json:"pinned_skill_ids,omitempty"`

	// Preferences
	ModelID string `bson:"modelId,omitempty" json:"model_id,omitempty"`

	// Stats
	TotalTasks     int64 `bson:"totalTasks" json:"total_tasks"`
	CompletedTasks int64 `bson:"completedTasks" json:"completed_tasks"`
	FailedTasks    int64 `bson:"failedTasks" json:"failed_tasks"`

	// Timestamps
	CreatedAt      time.Time `bson:"createdAt" json:"created_at"`
	UpdatedAt      time.Time `bson:"updatedAt" json:"updated_at"`
	LastActivityAt time.Time `bson:"lastActivityAt" json:"last_activity_at"`
}
