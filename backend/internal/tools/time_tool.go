package tools

import (
	"fmt"
	"time"
)

// NewTimeTool creates the get_current_time tool
func NewTimeTool() *Tool {
	return &Tool{
		Name:        "get_current_time",
		DisplayName: "Get Current Time",
		Description: "Get the current time in a specific timezone",
		Icon:        "Clock",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "Timezone name (e.g., 'America/New_York', 'Asia/Tokyo', 'UTC'). Defaults to UTC.",
					"default":     "UTC",
				},
			},
			"required": []string{}, // Timezone is now optional
		},
		Execute:  executeGetCurrentTime,
		Source:   ToolSourceBuiltin,
		Category: "time",
		Keywords: []string{"time", "date", "clock", "now", "current", "timezone", "datetime", "timestamp"},
	}
}

func executeGetCurrentTime(args map[string]interface{}) (string, error) {
	// Default to UTC if timezone not provided
	timezone := "UTC"
	if tz, ok := args["timezone"].(string); ok && tz != "" {
		timezone = tz
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone '%s', use format like 'America/New_York' or 'UTC'", timezone)
	}

	currentTime := time.Now().In(loc)
	return currentTime.Format("2006-01-02 15:04:05 MST"), nil
}
