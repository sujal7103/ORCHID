package models

import (
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// ClientMessage represents a message from the client
type ClientMessage struct {
	Type               string                   `json:"type"` // "chat_message", "new_conversation", "stop_generation", "resume_stream", or "interactive_prompt_response"
	ConversationID     string                   `json:"conversation_id"`
	Content            string                   `json:"content,omitempty"`
	History            []map[string]interface{} `json:"history,omitempty"`             // Optional: Client provides conversation history
	ModelID            string                   `json:"model_id,omitempty"`            // Option 1: Select from platform models
	CustomConfig       *CustomAPIConfig         `json:"custom_config,omitempty"`       // Option 2: Bring your own API key
	SystemInstructions string                   `json:"system_instructions,omitempty"` // Optional: Custom system prompt override
	Attachments        []MessageAttachment      `json:"attachments,omitempty"`         // File attachments (images, documents)
	DisableTools       bool                     `json:"disable_tools,omitempty"`       // Disable tools for this message (e.g., agent builder)
	SelectedTools      []string                 `json:"selected_tools,omitempty"`      // If set, only use these tools (by name)

	// Interactive prompt response fields
	PromptID string                      `json:"prompt_id,omitempty"` // ID of the prompt being responded to
	Answers  map[string]InteractiveAnswer `json:"answers,omitempty"`   // Map of question_id -> answer
	Skipped  bool                        `json:"skipped,omitempty"`   // True if user skipped/cancelled
}

// MessageAttachment represents a file attachment in a message
type MessageAttachment struct {
	Type     string `json:"type"`      // "image", "document", "audio"
	FileID   string `json:"file_id"`   // UUID from upload endpoint
	URL      string `json:"url"`       // File URL (e.g., "/uploads/uuid.jpg")
	MimeType string `json:"mime_type"` // MIME type (e.g., "image/jpeg")
	Size     int64  `json:"size"`      // File size in bytes
	Filename string `json:"filename"`  // Original filename
}

// CustomAPIConfig allows users to bring their own API keys (BYOK)
type CustomAPIConfig struct {
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model,omitempty"`
}

// InteractiveQuestion represents a question in an interactive prompt
type InteractiveQuestion struct {
	ID          string                 `json:"id"`                     // Unique question ID
	Type        string                 `json:"type"`                   // "text", "select", "multi-select", "number", "checkbox"
	Label       string                 `json:"label"`                  // Question text
	Placeholder string                 `json:"placeholder,omitempty"`  // Placeholder for text inputs
	Required    bool                   `json:"required,omitempty"`     // Whether answer is required
	Options     []string               `json:"options,omitempty"`      // Options for select/multi-select
	AllowOther  bool                   `json:"allow_other,omitempty"`  // Enable "Other" option
	DefaultValue interface{}            `json:"default_value,omitempty"` // Default value
	Validation  *QuestionValidation    `json:"validation,omitempty"`   // Validation rules
}

// QuestionValidation represents validation rules for a question
type QuestionValidation struct {
	Min       *float64 `json:"min,omitempty"`        // Minimum value for number type
	Max       *float64 `json:"max,omitempty"`        // Maximum value for number type
	Pattern   string   `json:"pattern,omitempty"`    // Regex pattern for text type
	MinLength *int     `json:"min_length,omitempty"` // Minimum length for text
	MaxLength *int     `json:"max_length,omitempty"` // Maximum length for text
}

// InteractiveAnswer represents a user's answer to a question
type InteractiveAnswer struct {
	QuestionID string      `json:"question_id"`      // ID of the question
	Value      interface{} `json:"value"`            // Answer value (string, number, bool, or []string)
	IsOther    bool        `json:"is_other,omitempty"` // True if "Other" option selected
}

// ServerMessage represents a message sent to the client
type ServerMessage struct {
	Type            string                 `json:"type"` // "stream_chunk", "reasoning_chunk", "tool_call", "tool_result", "stream_end", "stream_resume", "stream_missed", "conversation_reset", "conversation_title", "context_optimizing", "status_update", "interactive_prompt", "prompt_timeout", "prompt_validation_error", "error"
	Content         string                 `json:"content,omitempty"`
	Title           string                 `json:"title,omitempty"` // Auto-generated conversation title OR interactive prompt title
	ToolName        string                 `json:"tool_name,omitempty"`
	ToolDisplayName string                 `json:"tool_display_name,omitempty"` // User-friendly tool name (e.g., "Search Web")
	ToolIcon        string                 `json:"tool_icon,omitempty"`         // Lucide React icon name (e.g., "Calculator", "Search", "Clock")
	ToolDescription string                 `json:"tool_description,omitempty"`  // Human-readable tool description
	Status          string                 `json:"status,omitempty"`            // "executing", "completed", "started"
	Arguments       map[string]interface{} `json:"arguments,omitempty"`
	Result          string                 `json:"result,omitempty"`
	Plots           []PlotData             `json:"plots,omitempty"`           // Visualization plots from E2B tools
	ConversationID  string                 `json:"conversation_id,omitempty"`
	Tokens          *TokenUsage            `json:"tokens,omitempty"`
	ErrorCode       string                 `json:"code,omitempty"`
	ErrorMessage    string                 `json:"message,omitempty"`
	IsComplete      bool                   `json:"is_complete,omitempty"`      // For stream_resume: whether generation completed
	Reason          string                 `json:"reason,omitempty"`           // For stream_missed: "expired" or "not_found"
	Progress        int                    `json:"progress,omitempty"`         // For context_optimizing: progress percentage (0-100)

	// Interactive prompt fields
	PromptID    string                 `json:"prompt_id,omitempty"`    // Unique prompt ID
	Description string                 `json:"description,omitempty"`  // Optional prompt description
	Questions   []InteractiveQuestion  `json:"questions,omitempty"`    // Array of questions
	AllowSkip   *bool                  `json:"allow_skip,omitempty"`   // Whether user can skip (pointer to distinguish false from unset)
	Errors      map[string]string      `json:"errors,omitempty"`       // Validation errors (question_id -> error message)
}

// PlotData represents a visualization plot (chart/graph) from E2B tools
type PlotData struct {
	Format string `json:"format"` // "png", "jpg", "svg"
	Data   string `json:"data"`   // Base64-encoded image data
}

// TokenUsage represents token consumption statistics
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// PromptWaiterFunc is a function type that waits for a prompt response
// Returns (answers, skipped, error)
type PromptWaiterFunc func(promptID string, timeout time.Duration) (map[string]InteractiveAnswer, bool, error)

// UserConnection represents a single WebSocket connection
type UserConnection struct {
	ConnID             string
	UserID             string // User ID from authentication
	ClientIP           string // Client IP address (for anonymous rate limiting)
	Conn               *websocket.Conn
	ConversationID     string
	Messages           []map[string]interface{}
	MessageCount       int              // Track number of messages for title generation
	ModelID            string           // Selected model ID from platform
	CustomConfig       *CustomAPIConfig // OR user's custom API configuration (BYOK)
	SystemInstructions string           // Optional: User-provided system prompt override
	DisableTools       bool             // Disable tools for this connection (e.g., agent builder)
	SelectedTools      []string         // If set, only use these tools (by name)
	CreatedAt          time.Time
	WriteChan          chan ServerMessage
	StopChan           chan bool
	Mutex              sync.Mutex
	closed             bool           // Track if connection is closed
	PromptWaiter       PromptWaiterFunc // Function to wait for interactive prompt responses
}

// SafeSend sends a message to WriteChan safely, returning false if the channel is closed
func (uc *UserConnection) SafeSend(msg ServerMessage) bool {
	uc.Mutex.Lock()
	if uc.closed {
		uc.Mutex.Unlock()
		return false
	}
	uc.Mutex.Unlock()

	// Use defer/recover to handle panic from send on closed channel
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed, mark connection as closed
			uc.Mutex.Lock()
			uc.closed = true
			uc.Mutex.Unlock()
		}
	}()

	uc.WriteChan <- msg
	return true
}

// MarkClosed marks the connection as closed
func (uc *UserConnection) MarkClosed() {
	uc.Mutex.Lock()
	uc.closed = true
	uc.Mutex.Unlock()
}

// IsClosed returns true if the connection has been marked as closed
func (uc *UserConnection) IsClosed() bool {
	uc.Mutex.Lock()
	defer uc.Mutex.Unlock()
	return uc.closed
}

// ChatRequest represents a request to OpenAI-compatible chat completion API
type ChatRequest struct {
	Model       string                   `json:"model"`
	Messages    []map[string]interface{} `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	Stream      bool                     `json:"stream"`
	Temperature float64                  `json:"temperature,omitempty"`
}
