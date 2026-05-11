package tools

import (
	"clara-agents/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// UserConnectionKey is the key for injecting user connection into tool args
const UserConnectionKey = "__user_connection__"

// PromptWaiterKey is the key for injecting the prompt response waiter function
const PromptWaiterKey = "__prompt_waiter__"

// NewAskUserTool creates a tool that allows the AI to ask clarifying questions via modal prompts
func NewAskUserTool() *Tool {
	return &Tool{
		Name:        "ask_user",
		DisplayName: "Ask User Questions",
		Description: "Ask the user clarifying questions via an interactive modal dialog. Use this when you need additional information from the user to complete a task (e.g., preferences, choices, confirmation). This tool WAITS for the user to respond (blocks execution) and returns their answers, so you can use the responses immediately in your next step. Maximum wait time is 5 minutes.",
		Icon:        "MessageCircleQuestion",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title of the prompt dialog (e.g., 'Need More Information', 'Create Project')",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Optional description explaining why you're asking these questions",
				},
				"questions": map[string]interface{}{
					"type":        "array",
					"description": "Array of questions to ask the user (minimum 1, maximum 5 questions recommended)",
					"minItems":    1,
					"maxItems":    10,
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Unique identifier for this question (e.g., 'language', 'framework', 'email')",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "Question type: 'text', 'number', 'checkbox', 'select' (radio), or 'multi-select' (checkboxes)",
								"enum":        []string{"text", "number", "checkbox", "select", "multi-select"},
							},
							"label": map[string]interface{}{
								"type":        "string",
								"description": "The question text to display to the user",
							},
							"placeholder": map[string]interface{}{
								"type":        "string",
								"description": "Placeholder text for text/number inputs (optional)",
							},
							"required": map[string]interface{}{
								"type":        "boolean",
								"description": "Whether the user must answer this question (default: false)",
								"default":     false,
							},
							"options": map[string]interface{}{
								"type":        "array",
								"description": "Options for 'select' or 'multi-select' questions (required for those types)",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
							"allow_other": map[string]interface{}{
								"type":        "boolean",
								"description": "For select/multi-select: allow 'Other' option with custom text input (default: false)",
								"default":     false,
							},
							"validation": map[string]interface{}{
								"type":        "object",
								"description": "Validation rules for the question (optional)",
								"properties": map[string]interface{}{
									"min": map[string]interface{}{
										"type":        "number",
										"description": "Minimum value for number type",
									},
									"max": map[string]interface{}{
										"type":        "number",
										"description": "Maximum value for number type",
									},
									"pattern": map[string]interface{}{
										"type":        "string",
										"description": "Regex pattern for text validation (e.g., email pattern)",
									},
									"min_length": map[string]interface{}{
										"type":        "integer",
										"description": "Minimum length for text input",
									},
									"max_length": map[string]interface{}{
										"type":        "integer",
										"description": "Maximum length for text input",
									},
								},
							},
						},
						"required": []string{"id", "type", "label"},
					},
				},
				"allow_skip": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the user can skip/cancel the prompt (default: true). Set to false for critical questions.",
					"default":     true,
				},
			},
			"required": []string{"title", "questions"},
		},
		Execute:  executeAskUser,
		Source:   ToolSourceBuiltin,
		Category: "interaction",
		Keywords: []string{"ask", "question", "prompt", "user", "input", "clarify", "modal"},
	}
}

func executeAskUser(args map[string]interface{}) (string, error) {
	// Extract user connection (injected by chat service)
	userConn, ok := args[UserConnectionKey].(*models.UserConnection)
	if !ok || userConn == nil {
		return "", fmt.Errorf("interactive prompts are not available in this context (user connection not found)")
	}

	// Extract title
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return "", fmt.Errorf("title is required")
	}

	// Extract description (optional)
	description, _ := args["description"].(string)

	// Extract questions array
	questionsRaw, ok := args["questions"].([]interface{})
	if !ok || len(questionsRaw) == 0 {
		return "", fmt.Errorf("questions array is required and must not be empty")
	}

	// Convert questions to InteractiveQuestion structs
	questions := make([]models.InteractiveQuestion, 0, len(questionsRaw))
	for i, qRaw := range questionsRaw {
		qMap, ok := qRaw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("question at index %d is not a valid object", i)
		}

		// Extract required fields
		id, _ := qMap["id"].(string)
		qType, _ := qMap["type"].(string)
		label, _ := qMap["label"].(string)

		if id == "" || qType == "" || label == "" {
			return "", fmt.Errorf("question at index %d is missing required fields (id, type, or label)", i)
		}

		// Validate question type
		validTypes := map[string]bool{
			"text":         true,
			"number":       true,
			"checkbox":     true,
			"select":       true,
			"multi-select": true,
		}
		if !validTypes[qType] {
			return "", fmt.Errorf("invalid question type '%s' at index %d. Must be: text, number, checkbox, select, or multi-select", qType, i)
		}

		question := models.InteractiveQuestion{
			ID:    id,
			Type:  qType,
			Label: label,
		}

		// Optional: placeholder
		if placeholder, ok := qMap["placeholder"].(string); ok {
			question.Placeholder = placeholder
		}

		// Optional: required
		if required, ok := qMap["required"].(bool); ok {
			question.Required = required
		}

		// Optional: options (required for select/multi-select)
		if optionsRaw, ok := qMap["options"].([]interface{}); ok {
			options := make([]string, 0, len(optionsRaw))
			for _, opt := range optionsRaw {
				if optStr, ok := opt.(string); ok {
					options = append(options, optStr)
				}
			}
			question.Options = options
		} else if qType == "select" || qType == "multi-select" {
			return "", fmt.Errorf("question '%s' (type: %s) requires an 'options' array", id, qType)
		}

		// Optional: allow_other
		if allowOther, ok := qMap["allow_other"].(bool); ok {
			question.AllowOther = allowOther
		}

		// Optional: validation
		if validationRaw, ok := qMap["validation"].(map[string]interface{}); ok {
			validation := &models.QuestionValidation{}

			if min, ok := validationRaw["min"].(float64); ok {
				validation.Min = &min
			}
			if max, ok := validationRaw["max"].(float64); ok {
				validation.Max = &max
			}
			if pattern, ok := validationRaw["pattern"].(string); ok {
				validation.Pattern = pattern
			}
			if minLength, ok := validationRaw["min_length"].(float64); ok {
				minLenInt := int(minLength)
				validation.MinLength = &minLenInt
			}
			if maxLength, ok := validationRaw["max_length"].(float64); ok {
				maxLenInt := int(maxLength)
				validation.MaxLength = &maxLenInt
			}

			question.Validation = validation
		}

		questions = append(questions, question)
	}

	// Extract allow_skip (default: true)
	allowSkip := true
	if skipRaw, ok := args["allow_skip"].(bool); ok {
		allowSkip = skipRaw
	}

	// Generate prompt ID
	promptID := uuid.New().String()

	// Create the prompt message
	prompt := models.ServerMessage{
		Type:           "interactive_prompt",
		PromptID:       promptID,
		ConversationID: userConn.ConversationID,
		Title:          title,
		Description:    description,
		Questions:      questions,
		AllowSkip:      &allowSkip,
	}

	// Extract prompt waiter function (injected by chat service)
	waiterFunc, ok := args[PromptWaiterKey].(models.PromptWaiterFunc)
	if !ok || waiterFunc == nil {
		return "", fmt.Errorf("prompt waiter not available (internal error)")
	}

	// Send the prompt to the user
	success := userConn.SafeSend(prompt)
	if !success {
		log.Printf("❌ [ASK_USER] Failed to send interactive prompt (connection closed)")
		return "", fmt.Errorf("failed to send prompt: connection closed")
	}

	log.Printf("✅ [ASK_USER] Sent interactive prompt: %s (id: %s, questions: %d, allow_skip: %v)",
		title, promptID, len(questions), allowSkip)
	log.Printf("⏳ [ASK_USER] Waiting for user response...")

	// Wait for user response (5 minute timeout)
	answers, skipped, err := waiterFunc(promptID, 5*time.Minute)
	if err != nil {
		log.Printf("❌ [ASK_USER] Error waiting for response: %v", err)
		return "", fmt.Errorf("failed to receive user response: %w", err)
	}

	// Check if user skipped
	if skipped {
		log.Printf("📋 [ASK_USER] User skipped the prompt")
		return "User skipped the prompt without providing answers.", nil
	}

	// Format the answers for the LLM
	result := map[string]interface{}{
		"status":  "completed",
		"message": "User answered the prompt. Here are their responses:",
		"answers": make(map[string]interface{}),
	}

	// Convert answers to a format the LLM can understand
	for questionID, answer := range answers {
		// Find the question to get its label
		var questionLabel string
		for _, q := range questions {
			if q.ID == questionID {
				questionLabel = q.Label
				break
			}
		}

		answerData := map[string]interface{}{
			"question": questionLabel,
			"answer":   answer.Value,
		}

		if answer.IsOther {
			answerData["is_custom_answer"] = true
		}

		result["answers"].(map[string]interface{})[questionID] = answerData
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	log.Printf("✅ [ASK_USER] Returning user's answers to LLM")
	return string(resultJSON), nil
}
