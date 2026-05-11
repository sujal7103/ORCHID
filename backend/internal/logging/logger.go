package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures the global slog logger.
// In production (ENVIRONMENT=production) it uses JSON output for log aggregation.
// Otherwise it uses the human-readable text handler.
func Init() {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	slog.SetDefault(slog.New(handler))
}

// WithExecution returns a logger with execution context fields attached.
// Use this for all logging within a workflow execution.
func WithExecution(executionID, workflowID, userID string) *slog.Logger {
	return slog.With(
		"execution_id", executionID,
		"workflow_id", workflowID,
		"user_id", userID,
	)
}

// WithBlock returns a logger scoped to a specific block within an execution.
func WithBlock(logger *slog.Logger, blockID, blockName, blockType string) *slog.Logger {
	return logger.With(
		"block_id", blockID,
		"block_name", blockName,
		"block_type", blockType,
	)
}
