package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
)

// SwitchExecutor routes to N branches based on value matching.
// First match wins; unmatched goes to "default".
// The engine uses output["branch"] to determine which downstream blocks run.
type SwitchExecutor struct{}

func NewSwitchExecutor() *SwitchExecutor {
	return &SwitchExecutor{}
}

func (e *SwitchExecutor) Execute(ctx context.Context, block models.Block, inputs map[string]any) (map[string]any, error) {
	config := block.Config

	field := StripTemplateBraces(getString(config, "field", "response"))
	fieldValue := ResolvePath(inputs, field)

	rawCases, _ := config["cases"].([]interface{})

	log.Printf("🔀 [SWITCH] Block '%s': evaluating field=%s value=%v against %d cases",
		block.Name, field, fieldValue, len(rawCases))

	branch := "default"

	for _, rc := range rawCases {
		caseMap, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}

		label := fmt.Sprintf("%v", caseMap["label"])
		operator := fmt.Sprintf("%v", caseMap["operator"])
		compareValue := fmt.Sprintf("%v", caseMap["value"])

		if evaluateCondition(fieldValue, operator, compareValue) {
			branch = label
			log.Printf("🔀 [SWITCH] Block '%s': matched case '%s'", block.Name, label)
			break
		}
	}

	if branch == "default" {
		log.Printf("🔀 [SWITCH] Block '%s': no case matched, using default", block.Name)
	}

	return map[string]any{
		"response": fieldValue,
		"data":     fieldValue,
		"branch":   branch,
	}, nil
}
