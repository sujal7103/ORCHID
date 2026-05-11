package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NexusTaskStatus represents the execution state of a task
type NexusTaskStatus string

const (
	NexusTaskStatusDraft        NexusTaskStatus = "draft"
	NexusTaskStatusPending      NexusTaskStatus = "pending"
	NexusTaskStatusExecuting    NexusTaskStatus = "executing"
	NexusTaskStatusWaitingInput NexusTaskStatus = "waiting_input"
	NexusTaskStatusCompleted    NexusTaskStatus = "completed"
	NexusTaskStatusFailed       NexusTaskStatus = "failed"
	NexusTaskStatusCancelled    NexusTaskStatus = "cancelled"
)

// NexusTask represents a user task processed by Cortex
type NexusTask struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	SessionID    primitive.ObjectID  `bson:"sessionId" json:"session_id"`
	UserID       string              `bson:"userId" json:"user_id"`
	ParentTaskID *primitive.ObjectID `bson:"parentTaskId,omitempty" json:"parent_task_id,omitempty"`
	ProjectID    *primitive.ObjectID `bson:"projectId,omitempty" json:"project_id,omitempty"`

	// Definition
	Prompt   string `bson:"prompt" json:"prompt"`
	Goal     string `bson:"goal" json:"goal"`
	Priority int    `bson:"priority" json:"priority"` // 0=normal, 1=high, 2=urgent
	Source   string `bson:"source" json:"source"`     // "web","telegram","routine","decomposition"

	// Dispatch mode
	Mode     string              `bson:"mode" json:"mode"`                               // "quick" or "daemon"
	DaemonID *primitive.ObjectID `bson:"daemonId,omitempty" json:"daemon_id,omitempty"`

	// Execution state
	Status NexusTaskStatus  `bson:"status" json:"status"`
	Result *NexusTaskResult `bson:"result,omitempty" json:"result,omitempty"`
	Error  string           `bson:"error,omitempty" json:"error,omitempty"`

	// Sub-tasks (from decomposition)
	SubTaskIDs []primitive.ObjectID `bson:"subTaskIds,omitempty" json:"sub_task_ids,omitempty"`

	// Model
	ModelID string `bson:"modelId,omitempty" json:"model_id,omitempty"`

	// Routine link (set when task was created by a scheduled routine)
	RoutineID *primitive.ObjectID `bson:"routineId,omitempty" json:"routine_id,omitempty"`

	// Retry tracking
	RetryOfTaskID    *primitive.ObjectID `bson:"retryOfTaskId,omitempty" json:"retry_of_task_id,omitempty"`
	ManualRetryCount int                 `bson:"manualRetryCount" json:"manual_retry_count"`

	// Timestamps
	CreatedAt   time.Time  `bson:"createdAt" json:"created_at"`
	StartedAt   *time.Time `bson:"startedAt,omitempty" json:"started_at,omitempty"`
	CompletedAt *time.Time `bson:"completedAt,omitempty" json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `bson:"updatedAt" json:"updated_at"`
}

// NexusTaskResult holds the output of a completed task
type NexusTaskResult struct {
	Summary   string                 `bson:"summary" json:"summary"`
	Data      map[string]interface{} `bson:"data,omitempty" json:"data,omitempty"`
	Artifacts []NexusArtifact        `bson:"artifacts,omitempty" json:"artifacts,omitempty"`
}

// NexusArtifact represents a produced artifact (file, image, code, link)
type NexusArtifact struct {
	Type     string `bson:"type" json:"type"`         // "file","image","code","link"
	Name     string `bson:"name" json:"name"`
	Content  string `bson:"content" json:"content"`
	MimeType string `bson:"mimeType,omitempty" json:"mime_type,omitempty"`
}
