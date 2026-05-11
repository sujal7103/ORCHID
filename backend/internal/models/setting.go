package models

import "time"

// Setting represents a system-wide configuration setting
type Setting struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// SystemModelAssignments holds model IDs for different system operations
type SystemModelAssignments struct {
	ToolSelector       string `json:"tool_selector"`        // Model for tool prediction
	MemoryExtractor    string `json:"memory_extractor"`     // Model for memory extraction
	TitleGenerator     string `json:"title_generator"`      // Model for conversation titles
	WorkflowValidator  string `json:"workflow_validator"`   // Model for block validation
	AgentDefault       string `json:"agent_default"`        // Default model for new agents
}

// Setting keys for system models
const (
	SettingKeyToolSelector      = "system_model.tool_selector"
	SettingKeyMemoryExtractor   = "system_model.memory_extractor"
	SettingKeyTitleGenerator    = "system_model.title_generator"
	SettingKeyWorkflowValidator = "system_model.workflow_validator"
	SettingKeyAgentDefault      = "system_model.agent_default"
)
