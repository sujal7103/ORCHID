package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Skill represents a reusable AI skill definition
type Skill struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name             string             `bson:"name" json:"name"`
	Description      string             `bson:"description" json:"description"`
	Icon             string             `bson:"icon" json:"icon"`                             // lucide icon name
	Category         string             `bson:"category" json:"category"`                     // "research", "communication", etc.
	SystemPrompt     string             `bson:"system_prompt" json:"system_prompt"`            // injected before the LLM call
	RequiredTools    []string           `bson:"required_tools" json:"required_tools"`          // tool names from registry
	PreferredServers []string           `bson:"preferred_servers" json:"preferred_servers"`    // MCP server names
	Keywords         []string           `bson:"keywords" json:"keywords"`                     // for auto-routing
	TriggerPatterns  []string           `bson:"trigger_patterns" json:"trigger_patterns"`      // prefix patterns
	Mode             string             `bson:"mode" json:"mode"`                             // "auto" | "manual"
	IsBuiltin        bool               `bson:"is_builtin" json:"is_builtin"`
	AuthorID         string             `bson:"author_id,omitempty" json:"author_id,omitempty"`
	Version          string             `bson:"version" json:"version"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

// UserSkill represents a user's enablement of a skill
type UserSkill struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"user_id" json:"user_id"`
	SkillID   primitive.ObjectID `bson:"skill_id" json:"skill_id"`
	Enabled   bool               `bson:"enabled" json:"enabled"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// UserSkillWithDetails combines a user_skill record with its skill details
type UserSkillWithDetails struct {
	UserSkill `bson:",inline"`
	Skill     Skill `bson:"skill" json:"skill"`
}

// CreateSkillRequest represents the request body for creating a skill
type CreateSkillRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Icon             string   `json:"icon"`
	Category         string   `json:"category"`
	SystemPrompt     string   `json:"system_prompt"`
	RequiredTools    []string `json:"required_tools"`
	PreferredServers []string `json:"preferred_servers"`
	Keywords         []string `json:"keywords"`
	TriggerPatterns  []string `json:"trigger_patterns"`
	Mode             string   `json:"mode"`
}

// BulkEnableRequest represents the request body for bulk-enabling skills
type BulkEnableRequest struct {
	SkillIDs []string `json:"skill_ids"`
}

// ImportSkillMDRequest represents the request body for importing from SKILL.md text
type ImportSkillMDRequest struct {
	Content string `json:"content"`
}

// ImportGitHubURLRequest represents the request body for importing from a GitHub URL
type ImportGitHubURLRequest struct {
	URL string `json:"url"`
}
