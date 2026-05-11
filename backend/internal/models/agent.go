package models

import "time"

// Agent represents a workflow automation agent
type Agent struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"` // draft, active, deployed
	Workflow    *Workflow `json:"workflow,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Workflow represents a DAG of blocks for an agent
type Workflow struct {
	ID              string       `json:"id"`
	AgentID         string       `json:"agent_id"`
	Blocks          []Block      `json:"blocks"`
	Connections     []Connection `json:"connections"`
	Variables       []Variable   `json:"variables"`
	Version         int          `json:"version"`
	WorkflowModelID   string       `json:"workflowModelId,omitempty" bson:"workflowModelId,omitempty"`
	WorkflowTimeout   int          `json:"workflowTimeout,omitempty" bson:"workflowTimeout,omitempty"`     // Max execution time in seconds (default 600)
	MaxParallelBlocks int          `json:"maxParallelBlocks,omitempty" bson:"maxParallelBlocks,omitempty"` // Max concurrent block goroutines (default 20)
	CreatedAt         time.Time    `json:"created_at,omitempty"`
	UpdatedAt         time.Time    `json:"updated_at,omitempty"`
}

// Block represents a single node in the workflow DAG
type Block struct {
	ID           string         `json:"id"`
	NormalizedID string         `json:"normalizedId"` // Normalized name for variable interpolation (e.g., "search-latest-news")
	Type         string         `json:"type"`         // llm_inference, tool_execution, webhook, variable, python_tool
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Config       map[string]any `json:"config"`
	Position     Position       `json:"position"`
	Timeout      int            `json:"timeout"` // seconds, default 30
	RetryConfig  *RetryConfig   `json:"retryConfig,omitempty"`
}

// RetryConfig specifies automatic retry behavior for a block on transient failures
type RetryConfig struct {
	MaxRetries   int      `json:"maxRetries"`              // 0 = no retry (default)
	RetryOn      []string `json:"retryOn,omitempty"`       // ["rate_limit", "timeout", "server_error", "network_error", "all_transient"]
	BackoffMs    int      `json:"backoffMs,omitempty"`     // Initial backoff in ms (default 1000)
	MaxBackoffMs int      `json:"maxBackoffMs,omitempty"`  // Max backoff in ms (default 30000)
}

// Position represents x,y coordinates for canvas layout
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Connection represents an edge between two blocks
type Connection struct {
	ID            string `json:"id"`
	SourceBlockID string `json:"sourceBlockId"`
	SourceOutput  string `json:"sourceOutput,omitempty"`
	TargetBlockID string `json:"targetBlockId"`
	TargetInput   string `json:"targetInput,omitempty"`
}

// Variable represents a workflow-level variable
type Variable struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // string, number, boolean, array, object
	DefaultValue any    `json:"defaultValue,omitempty"`
}

// Execution represents a single workflow execution run
type Execution struct {
	ID              string                  `json:"id"`
	AgentID         string                  `json:"agent_id"`
	WorkflowVersion int                     `json:"workflow_version"`
	Status          string                  `json:"status"` // pending, running, completed, failed, partial_failure
	Input           map[string]any          `json:"input,omitempty"`
	Output          map[string]any          `json:"output,omitempty"`
	BlockStates     map[string]*BlockState  `json:"block_states,omitempty"`
	StartedAt       *time.Time              `json:"started_at,omitempty"`
	CompletedAt     *time.Time              `json:"completed_at,omitempty"`
}

// BlockState represents the execution state of a single block
type BlockState struct {
	Status      string         `json:"status"` // pending, running, completed, failed, skipped
	Inputs      map[string]any `json:"inputs,omitempty"`
	Outputs     map[string]any `json:"outputs,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`

	// Retry tracking
	RetryCount   int            `json:"retry_count,omitempty"`   // Number of retries attempted
	RetryHistory []RetryAttempt `json:"retry_history,omitempty"` // Detailed retry history
}

// RetryAttempt records a single retry attempt for debugging and monitoring
type RetryAttempt struct {
	Attempt   int       `json:"attempt"`     // 0-indexed attempt number
	Error     string    `json:"error"`       // Error message from this attempt
	ErrorType string    `json:"error_type"`  // "timeout", "rate_limit", "server_error", etc.
	Timestamp time.Time `json:"timestamp"`   // When the attempt occurred
	Duration  int64     `json:"duration_ms"` // How long the attempt took
}

// ExecutionUpdate is sent via WebSocket to stream execution progress
type ExecutionUpdate struct {
	Type        string         `json:"type"` // execution_update
	ExecutionID string         `json:"execution_id"`
	BlockID     string         `json:"block_id"`
	Status      string         `json:"status"`
	Inputs      map[string]any `json:"inputs,omitempty"`  // Available inputs for debugging
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// ExecutionComplete is sent when workflow execution finishes
type ExecutionComplete struct {
	Type        string         `json:"type"` // execution_complete
	ExecutionID string         `json:"execution_id"`
	Status      string         `json:"status"` // completed, failed, partial_failure
	FinalOutput map[string]any `json:"final_output,omitempty"`
	Duration    int64          `json:"duration_ms"`
}

// ============================================================================
// Standardized API Response Types
// Clean, well-structured output for API consumers
// ============================================================================

// ExecutionAPIResponse is the standardized response for workflow execution
// This provides a clean, predictable structure for API consumers
type ExecutionAPIResponse struct {
	// Status of the execution: completed, failed, partial
	Status string `json:"status"`

	// Result contains the primary output from the workflow
	// This is the "answer" - extracted from the final block's response
	Result string `json:"result"`

	// Data contains the structured JSON data from the final block (if it was a structured output block)
	// This is populated when the terminal block has outputFormat="json" and valid parsed data
	Data any `json:"data,omitempty"`

	// Artifacts contains all generated charts, images, visualizations
	// Each artifact has type, format, and base64 data
	Artifacts []APIArtifact `json:"artifacts,omitempty"`

	// Files contains all generated files with download URLs
	Files []APIFile `json:"files,omitempty"`

	// Blocks contains detailed output from each block (for debugging/advanced use)
	Blocks map[string]APIBlockOutput `json:"blocks,omitempty"`

	// Metadata contains execution statistics
	Metadata ExecutionMetadata `json:"metadata"`

	// Error contains error message if status is failed
	Error string `json:"error,omitempty"`
}

// APIArtifact represents a generated artifact (chart, image, etc.)
type APIArtifact struct {
	Type       string `json:"type"`                  // "chart", "image", "plot"
	Format     string `json:"format"`                // "png", "jpeg", "svg"
	Data       string `json:"data"`                  // Base64 encoded data
	Title      string `json:"title,omitempty"`       // Description/title
	SourceBlock string `json:"source_block,omitempty"` // Which block generated this
}

// APIFile represents a generated file
type APIFile struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	MimeType    string `json:"mime_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	SourceBlock string `json:"source_block,omitempty"`
}

// APIBlockOutput is a clean representation of a block's output
type APIBlockOutput struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Response   string         `json:"response,omitempty"`   // Primary text output
	Data       map[string]any `json:"data,omitempty"`       // Structured data (if JSON output)
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
}

// ExecutionMetadata contains execution statistics
type ExecutionMetadata struct {
	ExecutionID     string `json:"execution_id"`
	AgentID         string `json:"agent_id,omitempty"`
	WorkflowVersion int    `json:"workflow_version,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
	TotalTokens     int    `json:"total_tokens,omitempty"`
	BlocksExecuted  int    `json:"blocks_executed"`
	BlocksFailed    int    `json:"blocks_failed"`
}

// ExecuteWorkflowRequest is received from the client to start execution
type ExecuteWorkflowRequest struct {
	Type    string         `json:"type"` // execute_workflow
	AgentID string         `json:"agent_id"`
	Input   map[string]any `json:"input,omitempty"`
}

// CreateAgentRequest is the request body for creating an agent
type CreateAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateAgentRequest is the request body for updating an agent
type UpdateAgentRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// SaveWorkflowRequest is the request body for saving a workflow
type SaveWorkflowRequest struct {
	Blocks             []Block      `json:"blocks"`
	Connections        []Connection `json:"connections"`
	Variables          []Variable   `json:"variables,omitempty"`
	WorkflowModelID    string       `json:"workflowModelId,omitempty"`
	CreateVersion      bool         `json:"createVersion,omitempty"`      // Only create version snapshot if true
	VersionDescription string       `json:"versionDescription,omitempty"` // Description for the version (if created)
}

// ============================================================================
// Agent-Per-Block Architecture Types (Sprint 4)
// Each LLM block can act as a mini-agent with tool access and structured output
// ============================================================================

// AgentBlockConfig defines the configuration for an LLM block with agent capabilities
type AgentBlockConfig struct {
	// Model Configuration
	Model       string  `json:"model,omitempty"`       // Default: "sonnet-4.5" (resolves to glm-4.6)
	Temperature float64 `json:"temperature,omitempty"` // Default: 0.7

	// Prompts
	SystemPrompt string `json:"systemPrompt,omitempty"`
	UserPrompt   string `json:"userPrompt,omitempty"` // Supports {{variable}} interpolation

	// Tool Configuration
	EnabledTools []string `json:"enabledTools,omitempty"` // e.g., ["search_web", "calculate_math"]
	MaxToolCalls int      `json:"maxToolCalls,omitempty"` // Default: 15
	Credentials  []string `json:"credentials,omitempty"`  // Credential IDs for tool authentication

	// Execution Mode Configuration (for deterministic block execution)
	RequireToolUsage bool     `json:"requireToolUsage,omitempty"` // Default: true when tools exist - forces tool calls
	MaxRetries       int      `json:"maxRetries,omitempty"`       // Default: 2 - retry attempts if tool not called
	RequiredTools    []string `json:"requiredTools,omitempty"`    // Specific tools that MUST be called

	// Retry Policy for LLM API calls (transient error handling)
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"` // Optional retry configuration for API failures

	// Output Configuration
	OutputSchema *JSONSchema `json:"outputSchema,omitempty"` // JSON schema for validation
	StrictOutput bool        `json:"strictOutput,omitempty"` // Require exact schema match
}

// RetryPolicy defines retry behavior for block execution (LLM API calls)
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts (default: 1)
	MaxRetries int `json:"maxRetries,omitempty"`

	// InitialDelay is the initial delay before first retry in milliseconds (default: 1000)
	InitialDelay int `json:"initialDelay,omitempty"`

	// MaxDelay is the maximum delay between retries in milliseconds (default: 30000)
	MaxDelay int `json:"maxDelay,omitempty"`

	// BackoffMultiplier is the exponential backoff multiplier (default: 2.0)
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`

	// RetryOn specifies which error types to retry (default: ["timeout", "rate_limit", "server_error"])
	RetryOn []string `json:"retryOn,omitempty"`

	// JitterPercent adds randomness to delay to prevent thundering herd (0-100, default: 20)
	JitterPercent int `json:"jitterPercent,omitempty"`
}

// DefaultRetryPolicy returns sensible production defaults for retry behavior
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        1,
		InitialDelay:      1000,  // 1 second
		MaxDelay:          30000, // 30 seconds
		BackoffMultiplier: 2.0,
		RetryOn:           []string{"timeout", "rate_limit", "server_error"},
		JitterPercent:     20,
	}
}

// JSONSchema represents a JSON Schema for output validation
type JSONSchema struct {
	Type                 string                 `json:"type"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
	Default              any                    `json:"default,omitempty"`
}

// AgentBlockResult represents the result of an agent block execution
type AgentBlockResult struct {
	// The validated output (matches OutputSchema if provided)
	Output map[string]any `json:"output"`

	// Raw LLM response (before parsing)
	RawResponse string `json:"rawResponse,omitempty"`

	// Model used for execution
	Model string `json:"model"`

	// Token usage
	Tokens TokenUsage `json:"tokens"`

	// Tool calls made during execution
	ToolCalls []ToolCallRecord `json:"toolCalls,omitempty"`

	// Number of iterations in the agent loop
	Iterations int `json:"iterations"`
}

// ToolCallRecord records a tool call made during execution
type ToolCallRecord struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Result    string         `json:"result"`
	Error     string         `json:"error,omitempty"`
	Duration  int64          `json:"durationMs"`
}

// ============================================================================
// Pagination Types
// ============================================================================

// AgentListItem is a lightweight agent representation for list views
type AgentListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	HasWorkflow bool      `json:"has_workflow"`
	BlockCount  int       `json:"block_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PaginatedAgentsResponse is the response for paginated agent list
type PaginatedAgentsResponse struct {
	Agents  []AgentListItem `json:"agents"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"has_more"`
}

// RecentAgentsResponse is the response for recent agents (landing page)
type RecentAgentsResponse struct {
	Agents []AgentListItem `json:"agents"`
}

// ============================================================================
// Sync Types (for first-message persistence)
// ============================================================================

// SyncAgentRequest is the request body for syncing a local agent to backend
type SyncAgentRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Workflow    SaveWorkflowRequest `json:"workflow"`
	ModelID     string           `json:"model_id,omitempty"`
}

// SyncAgentResponse is the response after syncing an agent
type SyncAgentResponse struct {
	Agent          *Agent    `json:"agent"`
	Workflow       *Workflow `json:"workflow"`
	ConversationID string    `json:"conversation_id"`
}
