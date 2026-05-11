package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DaemonTemplate represents a reusable daemon configuration preset.
// System templates have an empty UserID and IsDefault=true.
// User templates have a UserID and IsDefault=false.
type DaemonTemplate struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID string             `bson:"userId" json:"user_id"` // "" for system defaults

	// Identity
	Name        string `bson:"name" json:"name"`
	Slug        string `bson:"slug" json:"slug"` // LLM-friendly identifier for matching
	Description string `bson:"description" json:"description"`

	// Daemon configuration
	Role         string   `bson:"role" json:"role"`
	RoleLabel    string   `bson:"roleLabel" json:"role_label"`
	Persona      string   `bson:"persona" json:"persona"`             // Personality/tone
	Instructions string   `bson:"instructions" json:"instructions"`   // Step-by-step workflow
	Constraints  string   `bson:"constraints" json:"constraints"`     // Rules/guardrails
	OutputFormat string   `bson:"outputFormat" json:"output_format"`  // Expected output structure
	DefaultTools []string `bson:"defaultTools" json:"default_tools"`  // Tool categories: search, file, code, data, communication

	// Visual
	Icon  string `bson:"icon" json:"icon"`   // Lucide icon name
	Color string `bson:"color" json:"color"` // Hex color

	// Behavior
	MaxIterations int `bson:"maxIterations" json:"max_iterations"` // Default: 25
	MaxRetries    int `bson:"maxRetries" json:"max_retries"`       // Default: 3

	// Flags
	IsDefault bool `bson:"isDefault" json:"is_default"` // System-provided template
	IsActive  bool `bson:"isActive" json:"is_active"`   // User can disable without deleting

	// Learnings — accumulated from daemon executions using this template
	Learnings []TemplateLearning `bson:"learnings,omitempty" json:"learnings,omitempty"`

	// Stats — aggregated from all daemon runs using this template
	Stats TemplateStats `bson:"stats" json:"stats"`

	// Timestamps
	CreatedAt time.Time `bson:"createdAt" json:"created_at"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updated_at"`
}

// TemplateLearning represents an insight extracted from daemon execution
type TemplateLearning struct {
	Key             string    `bson:"key" json:"key"`                           // Dedup key
	Content         string    `bson:"content" json:"content"`                   // The learning itself
	Category        string    `bson:"category" json:"category"`                 // tool_usage, workflow, output, constraint
	Confidence      float64   `bson:"confidence" json:"confidence"`             // 0.0-1.0
	ReinforcedCount int       `bson:"reinforcedCount" json:"reinforced_count"`  // Times this was confirmed
	CreatedAt       time.Time `bson:"createdAt" json:"created_at"`
	LastSeenAt      time.Time `bson:"lastSeenAt" json:"last_seen_at"`
}

// TemplateStats tracks execution history for a template
type TemplateStats struct {
	TotalRuns      int     `bson:"totalRuns" json:"total_runs"`
	SuccessfulRuns int     `bson:"successfulRuns" json:"successful_runs"`
	FailedRuns     int     `bson:"failedRuns" json:"failed_runs"`
	AvgIterations  float64 `bson:"avgIterations" json:"avg_iterations"`
}

// BuildSystemPromptSection assembles the full daemon prompt from template fields
func (t *DaemonTemplate) BuildSystemPromptSection() string {
	var s string

	if t.Persona != "" {
		s += t.Persona + "\n\n"
	}
	if t.Instructions != "" {
		s += "## Workflow\n\n" + t.Instructions + "\n\n"
	}
	if t.Constraints != "" {
		s += "## Constraints\n\n" + t.Constraints + "\n\n"
	}
	if t.OutputFormat != "" {
		s += "## Output Format\n\n" + t.OutputFormat + "\n\n"
	}

	// Inject top learnings (max 10, sorted by reinforced count)
	if len(t.Learnings) > 0 {
		s += "## Learned Patterns (from previous runs)\n\n"
		count := 0
		for _, l := range t.Learnings {
			if l.Confidence < 0.5 {
				continue
			}
			s += "- " + l.Content + "\n"
			count++
			if count >= 10 {
				break
			}
		}
		s += "\n"
	}

	return s
}
