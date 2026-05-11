package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NexusProject groups related tasks into a named project with its own Kanban view
type NexusProject struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      string             `bson:"userId" json:"user_id"`
	Name        string             `bson:"name" json:"name"`
	Description       string             `bson:"description,omitempty" json:"description,omitempty"`
	SystemInstruction string             `bson:"systemInstruction,omitempty" json:"system_instruction,omitempty"`
	Icon              string             `bson:"icon" json:"icon"`   // Lucide icon name
	Color       string             `bson:"color" json:"color"` // Hex color
	IsArchived  bool               `bson:"isArchived" json:"is_archived"`
	SortOrder   int                `bson:"sortOrder" json:"sort_order"`
	CreatedAt   time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updated_at"`
}
