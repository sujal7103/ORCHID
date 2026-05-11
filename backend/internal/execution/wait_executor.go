package execution

import (
	"clara-agents/internal/models"
	"context"
	"log"
	"strings"
	"time"
)

// WaitExecutor pauses execution for a configured duration, then passes data through.
type WaitExecutor struct{}

func NewWaitExecutor() *WaitExecutor {
	return &WaitExecutor{}
}

func (e *WaitExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	duration := getFloat(config, "duration", 1)
	unit := getString(config, "unit", "seconds")

	var waitDuration time.Duration
	switch unit {
	case "ms":
		waitDuration = time.Duration(duration) * time.Millisecond
	case "minutes":
		waitDuration = time.Duration(duration) * time.Minute
	default: // "seconds"
		waitDuration = time.Duration(duration) * time.Second
	}

	// Cap at 5 minutes to prevent abuse
	if waitDuration > 5*time.Minute {
		waitDuration = 5 * time.Minute
	}

	log.Printf("⏳ [WAIT] Block '%s': waiting %v", block.Name, waitDuration)

	select {
	case <-time.After(waitDuration):
		// Done waiting
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	log.Printf("⏳ [WAIT] Block '%s': done waiting", block.Name)

	// Pass through all inputs (strip internal keys)
	output := make(map[string]any)
	for k, v := range inputs {
		if !strings.HasPrefix(k, "_") {
			output[k] = v
		}
	}

	// Ensure response and data keys exist
	if _, ok := output["response"]; !ok {
		output["response"] = true
	}
	if _, ok := output["data"]; !ok {
		output["data"] = output["response"]
	}

	return output, nil
}
