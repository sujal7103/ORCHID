package services

import "encoding/json"

// EstimateTokens returns an approximate token count using the ~4 chars/token heuristic.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4
}

// EstimateMessagesTokens estimates the total token count for a set of chat messages.
// Accounts for role overhead (~4 tokens per message for role, separators).
func EstimateMessagesTokens(messages []map[string]interface{}) int {
	total := 0
	for _, msg := range messages {
		total += 4 // role + separators overhead per message
		if c, ok := msg["content"].(string); ok {
			total += EstimateTokens(c)
		}
		// tool_calls JSON adds tokens too
		if tc, ok := msg["tool_calls"]; ok {
			tcJSON, _ := json.Marshal(tc)
			total += EstimateTokens(string(tcJSON))
		}
	}
	return total
}

// EstimateToolDefTokens estimates the token overhead from tool definitions.
// Each tool def is roughly 100-200 tokens depending on description length.
func EstimateToolDefTokens(tools []map[string]interface{}) int {
	if len(tools) == 0 {
		return 0
	}
	toolsJSON, _ := json.Marshal(tools)
	return EstimateTokens(string(toolsJSON))
}

// TokenBreakdown provides a detailed token usage breakdown for debugging.
type TokenBreakdown struct {
	SystemTokens    int `json:"system_tokens"`
	UserTokens      int `json:"user_tokens"`
	AssistantTokens int `json:"assistant_tokens"`
	ToolTokens      int `json:"tool_tokens"`
	ToolDefTokens   int `json:"tool_def_tokens"`
	Total           int `json:"total"`
}

// BreakdownMessages computes a detailed token breakdown by message role.
func BreakdownMessages(messages []map[string]interface{}, tools []map[string]interface{}) TokenBreakdown {
	var bd TokenBreakdown

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		tokens := EstimateTokens(content) + 4 // +4 for role overhead

		switch role {
		case "system":
			bd.SystemTokens += tokens
		case "user":
			bd.UserTokens += tokens
		case "assistant":
			tokens += 0 // already counted content
			if tc, ok := msg["tool_calls"]; ok {
				tcJSON, _ := json.Marshal(tc)
				tokens += EstimateTokens(string(tcJSON))
			}
			bd.AssistantTokens += tokens
		case "tool":
			bd.ToolTokens += tokens
		}
	}

	bd.ToolDefTokens = EstimateToolDefTokens(tools)
	bd.Total = bd.SystemTokens + bd.UserTokens + bd.AssistantTokens + bd.ToolTokens + bd.ToolDefTokens
	return bd
}
