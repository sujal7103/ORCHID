package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EngramEntry represents a single entry in Cortex's central knowledge store
type EngramEntry struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID primitive.ObjectID `bson:"sessionId" json:"session_id"`
	UserID    string             `bson:"userId" json:"user_id"`

	Type    string `bson:"type" json:"type"`       // "task_result","daemon_output","user_fact","status_log"
	Key     string `bson:"key" json:"key"`         // e.g. "task_abc_result","daemon_1_final"
	Value   string `bson:"value" json:"value"`     // JSON content
	Summary string `bson:"summary" json:"summary"` // One-line summary for quick context loading
	Source  string `bson:"source" json:"source"`   // "cortex","daemon_xyz","user","routine"

	// TTL for status entries (auto-cleanup)
	ExpiresAt *time.Time `bson:"expiresAt,omitempty" json:"expires_at,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"created_at"`
}
