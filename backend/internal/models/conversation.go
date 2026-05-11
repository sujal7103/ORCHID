package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BuilderConversation represents a chat history for building an agent
type BuilderConversation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AgentID   string             `bson:"agentId" json:"agent_id"`   // String ID (timestamp-based from SQLite)
	UserID    string             `bson:"userId" json:"user_id"`     // Supabase user ID
	Messages  []BuilderMessage   `bson:"messages" json:"messages"`
	ModelID   string             `bson:"modelId" json:"model_id"`
	CreatedAt time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updated_at"`
	ExpiresAt *time.Time         `bson:"expiresAt,omitempty" json:"expires_at,omitempty"` // TTL for auto-deletion if user opts out
}

// BuilderMessage represents a single message in the builder conversation
type BuilderMessage struct {
	ID               string            `bson:"id" json:"id"`
	Role             string            `bson:"role" json:"role"` // "user" or "assistant"
	Content          string            `bson:"content" json:"content"`
	Timestamp        time.Time         `bson:"timestamp" json:"timestamp"`
	WorkflowSnapshot *WorkflowSnapshot `bson:"workflowSnapshot,omitempty" json:"workflow_snapshot,omitempty"`
}

// WorkflowSnapshot captures the state of the workflow at a message point
type WorkflowSnapshot struct {
	Version     int    `bson:"version" json:"version"`
	Action      string `bson:"action,omitempty" json:"action,omitempty"` // "create" or "modify" or null
	Explanation string `bson:"explanation,omitempty" json:"explanation,omitempty"`
}

// EncryptedBuilderConversation stores encrypted conversation data
// The Messages field is encrypted as a JSON blob
type EncryptedBuilderConversation struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AgentID           string             `bson:"agentId" json:"agent_id"`   // String ID (timestamp-based from SQLite)
	UserID            string             `bson:"userId" json:"user_id"`     // Supabase user ID
	EncryptedMessages string             `bson:"encryptedMessages" json:"-"` // Base64-encoded encrypted JSON
	ModelID           string             `bson:"modelId" json:"model_id"`
	MessageCount      int                `bson:"messageCount" json:"message_count"` // For display without decryption
	CreatedAt         time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt         time.Time          `bson:"updatedAt" json:"updated_at"`
	ExpiresAt         *time.Time         `bson:"expiresAt,omitempty" json:"expires_at,omitempty"`
}

// ConversationListItem is a summary for listing conversations
type ConversationListItem struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	ModelID      string    `json:"model_id"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AddMessageRequest is the request body for adding a message to a conversation
type AddMessageRequest struct {
	Role             string            `json:"role"`
	Content          string            `json:"content"`
	WorkflowSnapshot *WorkflowSnapshot `json:"workflow_snapshot,omitempty"`
}

// ConversationResponse is the full conversation response
type ConversationResponse struct {
	ID        string           `json:"id"`
	AgentID   string           `json:"agent_id"`
	ModelID   string           `json:"model_id"`
	Messages  []BuilderMessage `json:"messages"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// ToListItem converts an EncryptedBuilderConversation to ConversationListItem
func (c *EncryptedBuilderConversation) ToListItem() ConversationListItem {
	return ConversationListItem{
		ID:           c.ID.Hex(),
		AgentID:      c.AgentID, // AgentID is already a string
		ModelID:      c.ModelID,
		MessageCount: c.MessageCount,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}
