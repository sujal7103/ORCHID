package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NexusSave represents a user-saved output or document within Nexus.
// Saves can be captured from task results or created manually, and
// can be attached as context documents to future tasks.
type NexusSave struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID string             `bson:"userId" json:"user_id"`

	// Content
	Title   string   `bson:"title" json:"title"`
	Content string   `bson:"content" json:"content"` // Markdown
	Tags    []string `bson:"tags,omitempty" json:"tags,omitempty"`

	// Source reference (optional — links back to originating task)
	SourceTaskID    *primitive.ObjectID `bson:"sourceTaskId,omitempty" json:"source_task_id,omitempty"`
	SourceProjectID *primitive.ObjectID `bson:"sourceProjectId,omitempty" json:"source_project_id,omitempty"`

	// Timestamps
	CreatedAt time.Time `bson:"createdAt" json:"created_at"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updated_at"`
}
