package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DaemonStatus represents the execution state of a daemon
type DaemonStatus string

const (
	DaemonStatusIdle         DaemonStatus = "idle"
	DaemonStatusExecuting    DaemonStatus = "executing"
	DaemonStatusWaitingInput DaemonStatus = "waiting_input"
	DaemonStatusCompleted    DaemonStatus = "completed"
	DaemonStatusFailed       DaemonStatus = "failed"
)

// Daemon represents an autonomous sub-agent spawned by Cortex
type Daemon struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID primitive.ObjectID `bson:"sessionId" json:"session_id"`
	UserID    string             `bson:"userId" json:"user_id"`
	TaskID    primitive.ObjectID `bson:"taskId" json:"task_id"`

	// Dynamic specialization (assigned by Cortex)
	Role             string               `bson:"role" json:"role"`
	RoleLabel        string               `bson:"roleLabel" json:"role_label"`
	TemplateSlug     string               `bson:"templateSlug,omitempty" json:"template_slug,omitempty"` // Which template was used
	Persona          string               `bson:"persona" json:"persona"`
	AssignedTools    []string             `bson:"assignedTools" json:"assigned_tools"`
	AssignedSkillIDs []primitive.ObjectID `bson:"assignedSkillIds,omitempty" json:"assigned_skill_ids,omitempty"`

	// Dependency coordination
	PlanIndex         int               `bson:"planIndex" json:"plan_index"`
	DependsOn         []int             `bson:"dependsOn,omitempty" json:"depends_on,omitempty"`
	DependencyResults map[string]string `bson:"dependencyResults,omitempty" json:"dependency_results,omitempty"`

	// Status
	Status        DaemonStatus `bson:"status" json:"status"`
	CurrentAction string       `bson:"currentAction,omitempty" json:"current_action,omitempty"`
	Progress      float64      `bson:"progress" json:"progress"` // 0.0-1.0

	// Daemon memory (working state)
	WorkingMemory []DaemonMemoryEntry `bson:"workingMemory,omitempty" json:"working_memory,omitempty"`

	// LLM conversation log
	Messages []DaemonMessage `bson:"messages,omitempty" json:"messages,omitempty"`

	// Model
	ModelID string `bson:"modelId,omitempty" json:"model_id,omitempty"`

	// Execution tracking
	Iterations    int `bson:"iterations" json:"iterations"`
	MaxIterations int `bson:"maxIterations" json:"max_iterations"` // Default: 25
	RetryCount    int `bson:"retryCount" json:"retry_count"`
	MaxRetries    int `bson:"maxRetries" json:"max_retries"` // Default: 3

	// Timestamps
	CreatedAt   time.Time  `bson:"createdAt" json:"created_at"`
	StartedAt   *time.Time `bson:"startedAt,omitempty" json:"started_at,omitempty"`
	CompletedAt *time.Time `bson:"completedAt,omitempty" json:"completed_at,omitempty"`
}

// DaemonMemoryEntry represents an intermediate result in daemon working memory
type DaemonMemoryEntry struct {
	Key       string    `bson:"key" json:"key"`
	Value     string    `bson:"value" json:"value"`     // JSON-serialized
	Summary   string    `bson:"summary" json:"summary"` // One-line summary for context injection
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}

// DaemonMessage represents a message in the daemon's LLM conversation
type DaemonMessage struct {
	Role       string            `bson:"role" json:"role"` // "system","user","assistant","tool"
	Content    string            `bson:"content" json:"content"`
	ToolCall   *DaemonToolCall   `bson:"toolCall,omitempty" json:"tool_call,omitempty"`
	ToolResult *DaemonToolResult `bson:"toolResult,omitempty" json:"tool_result,omitempty"`
	Timestamp  time.Time         `bson:"timestamp" json:"timestamp"`
}

// DaemonToolCall represents a tool invocation by a daemon
type DaemonToolCall struct {
	ID        string `bson:"id" json:"id"`
	Name      string `bson:"name" json:"name"`
	Arguments string `bson:"arguments" json:"arguments"` // JSON string
}

// DaemonToolResult represents the result of a tool invocation
type DaemonToolResult struct {
	ToolCallID string `bson:"toolCallId" json:"tool_call_id"`
	Content    string `bson:"content" json:"content"`
	IsError    bool   `bson:"isError,omitempty" json:"is_error,omitempty"`
}
