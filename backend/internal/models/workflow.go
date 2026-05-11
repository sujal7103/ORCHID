package models

// ConversationMessage represents a message in the conversation history
type ConversationMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message content
}

// WorkflowGenerateRequest represents a request to generate or modify a workflow
type WorkflowGenerateRequest struct {
	AgentID             string                `json:"agent_id"`
	UserMessage         string                `json:"user_message"`
	CurrentWorkflow     *Workflow             `json:"current_workflow,omitempty"`      // For modifications
	ModelID             string                `json:"model_id,omitempty"`              // Optional model override
	ConversationID      string                `json:"conversation_id,omitempty"`       // For conversation persistence
	ConversationHistory []ConversationMessage `json:"conversation_history,omitempty"`  // Recent conversation context for better tool selection
}

// WorkflowGenerateResponse represents the structured output from workflow generation
type WorkflowGenerateResponse struct {
	Success              bool              `json:"success"`
	Workflow             *Workflow         `json:"workflow,omitempty"`
	Explanation          string            `json:"explanation"`
	Action               string            `json:"action"` // "create" or "modify"
	Error                string            `json:"error,omitempty"`
	Version              int               `json:"version"`
	Errors               []ValidationError `json:"errors,omitempty"`
	SuggestedName        string            `json:"suggested_name,omitempty"`        // AI-generated agent name suggestion
	SuggestedDescription string            `json:"suggested_description,omitempty"` // AI-generated agent description
}

// ValidationError represents a workflow validation error
type ValidationError struct {
	Type         string `json:"type"` // "schema", "cycle", "type_mismatch", "missing_input"
	Message      string `json:"message"`
	BlockID      string `json:"blockId,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
}

// WorkflowJSONSchema returns the JSON schema for structured output
func WorkflowJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"blocks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Unique block ID in kebab-case format matching the block name",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"llm_inference", "variable", "code_block"},
							"description": "Block type - llm_inference for AI agents, variable for inputs, code_block for direct tool execution",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Human-readable block name",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "What this block does",
						},
						"config": map[string]interface{}{
							"type":        "object",
							"description": "Block-specific configuration",
						},
						"position": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"x": map[string]interface{}{"type": "integer"},
								"y": map[string]interface{}{"type": "integer"},
							},
							"required": []string{"x", "y"},
						},
						"timeout": map[string]interface{}{
							"type":        "integer",
							"description": "Timeout in seconds (default 30, max 120 for LLM blocks)",
						},
					},
					"required": []string{"id", "type", "name", "description", "config", "position", "timeout"},
				},
			},
			"connections": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Unique connection ID",
						},
						"sourceBlockId": map[string]interface{}{
							"type":        "string",
							"description": "ID of the source block",
						},
						"sourceOutput": map[string]interface{}{
							"type":        "string",
							"description": "Output port name (usually 'output')",
						},
						"targetBlockId": map[string]interface{}{
							"type":        "string",
							"description": "ID of the target block",
						},
						"targetInput": map[string]interface{}{
							"type":        "string",
							"description": "Input port name (usually 'input')",
						},
					},
					"required": []string{"id", "sourceBlockId", "sourceOutput", "targetBlockId", "targetInput"},
				},
			},
			"variables": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "object"},
			},
			"explanation": map[string]interface{}{
				"type":        "string",
				"description": "Brief explanation of the workflow or changes made",
			},
		},
		"required": []string{"blocks", "connections", "variables", "explanation"},
	}
}

// WorkflowExecuteResult contains the result of a workflow execution
type WorkflowExecuteResult struct {
	Status      string
	Output      map[string]interface{}
	BlockStates map[string]*BlockState
	Error       string
}

// WorkflowExecuteFunc is a function type for executing workflows
// This allows scheduler to call workflow engine without import cycle
type WorkflowExecuteFunc func(workflow *Workflow, inputs map[string]interface{}) (*WorkflowExecuteResult, error)
