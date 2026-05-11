package models

import "time"

// MCPConnection represents an active MCP client connection
type MCPConnection struct {
	ID             string                       `json:"id"`
	UserID         string                       `json:"user_id"`
	ClientID       string                       `json:"client_id"`
	ClientVersion  string                       `json:"client_version"`
	Platform       string                       `json:"platform"`
	ConnectedAt    time.Time                    `json:"connected_at"`
	LastHeartbeat  time.Time                    `json:"last_heartbeat"`
	IsActive       bool                         `json:"is_active"`
	Tools          []MCPTool                    `json:"tools"`
	Servers        []MCPServerConfig            `json:"servers,omitempty"`
	WriteChan      chan MCPServerMessage        `json:"-"`
	StopChan       chan bool                    `json:"-"`
	PendingResults  map[string]chan MCPToolResult          `json:"-"` // call_id -> result channel
	PendingCommands map[string]chan MCPServerCommandResult `json:"-"` // request_id -> command result channel
}

// MCPTool represents a tool registered by an MCP client
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`            // JSON Schema
	Source      string                 `json:"source"`                // "mcp_local"
	UserID      string                 `json:"user_id"`
	ServerName  string                 `json:"server_name,omitempty"` // MCP server that provides this tool
}

// MCPClientMessage represents messages from MCP client to backend
type MCPClientMessage struct {
	Type    string                 `json:"type"` // "register_tools", "tool_result", "heartbeat", "disconnect"
	Payload map[string]interface{} `json:"payload"`
}

// MCPServerMessage represents messages from backend to MCP client
type MCPServerMessage struct {
	Type    string                 `json:"type"` // "tool_call", "ack", "error"
	Payload map[string]interface{} `json:"payload"`
}

// MCPToolRegistration represents the registration payload from client
type MCPToolRegistration struct {
	ClientID      string          `json:"client_id"`
	ClientVersion string          `json:"client_version"`
	Platform      string          `json:"platform"`
	Tools         []MCPTool       `json:"tools"`
	Servers       []MCPServerConfig `json:"servers,omitempty"`
}

// MCPToolCall represents a tool execution request to client
type MCPToolCall struct {
	CallID    string                 `json:"call_id"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Timeout   int                    `json:"timeout"` // seconds
}

// MCPToolResult represents a tool execution result from client
type MCPToolResult struct {
	CallID  string `json:"call_id"`
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

// MCPHeartbeat represents a heartbeat message
type MCPHeartbeat struct {
	Timestamp time.Time `json:"timestamp"`
}

// MCPServerCommandResult represents the ack from the bridge client after a server management command
type MCPServerCommandResult struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// MCPServerConfig represents an MCP server configuration for REST API
type MCPServerConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Type        string   `json:"type"` // "stdio"
	Enabled     bool     `json:"enabled"`
}
