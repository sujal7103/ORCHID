package execution

import (
	"clara-agents/internal/models"
	"context"
	"log"
	"time"
)

// WebhookTriggerExecutor is an MVP passthrough for webhook triggers.
// In production this will register an HTTP listener; for now it passes
// the workflow input through as if the webhook just fired.
type WebhookTriggerExecutor struct{}

func NewWebhookTriggerExecutor() *WebhookTriggerExecutor {
	return &WebhookTriggerExecutor{}
}

func (e *WebhookTriggerExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	log.Printf("🔗 [WEBHOOK-TRIGGER] Block '%s': passing through input data", block.Name)

	// Build response from the full inputs so that both
	// {{webhook.response.body.<field>}} and {{webhook.response.headers.<key>}}
	// continue to work. When an explicit "body" key exists, also promote its
	// fields to the top level of response for the common shorthand
	// {{webhook.response.<field>}}.
	response := make(map[string]any)
	// Copy all input keys (body, headers, method, etc.) into response
	for k, v := range inputs {
		response[k] = v
	}
	// Promote body fields to top level for backward compat shorthand
	if bodyMap, ok := inputs["body"].(map[string]any); ok {
		for k, v := range bodyMap {
			if _, exists := response[k]; !exists {
				response[k] = v
			}
		}
	}

	return map[string]any{
		"response":    response,
		"data":        inputs,
		"triggerType": "webhook",
		"path":        getString(block.Config, "path", "/"),
		"method":      getString(block.Config, "method", "POST"),
	}, nil
}

// ScheduleTriggerExecutor is an MVP passthrough for scheduled triggers.
// In production this will register a cron job; for now it passes
// the workflow input through as if the schedule just fired.
type ScheduleTriggerExecutor struct{}

func NewScheduleTriggerExecutor() *ScheduleTriggerExecutor {
	return &ScheduleTriggerExecutor{}
}

func (e *ScheduleTriggerExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	cronExpr := getString(block.Config, "cronExpression", "")
	timezone := getString(block.Config, "timezone", "UTC")
	log.Printf("⏰ [SCHEDULE-TRIGGER] Block '%s': cron=%s tz=%s, passing through input data", block.Name, cronExpr, timezone)

	return map[string]any{
		"response":       inputs,
		"data":           inputs,
		"triggerType":    "schedule",
		"cronExpression": cronExpr,
		"timezone":       timezone,
		"scheduledAt":    time.Now().Format(time.RFC3339),
	}, nil
}
