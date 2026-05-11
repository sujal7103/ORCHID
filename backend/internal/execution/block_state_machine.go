package execution

import "log"

// BlockStatus represents valid block execution states.
type BlockStatus string

const (
	BlockStatusPending  BlockStatus = "pending"
	BlockStatusRunning  BlockStatus = "running"
	BlockStatusRetrying BlockStatus = "retrying"
	BlockStatusCompleted BlockStatus = "completed"
	BlockStatusFailed   BlockStatus = "failed"
	BlockStatusSkipped  BlockStatus = "skipped"
)

// validTransitions defines the allowed state transitions for blocks.
// Any transition not listed here is invalid and will be rejected.
var validTransitions = map[BlockStatus]map[BlockStatus]bool{
	BlockStatusPending: {
		BlockStatusRunning: true,
		BlockStatusSkipped: true,
	},
	BlockStatusRunning: {
		BlockStatusCompleted: true,
		BlockStatusFailed:    true,
		BlockStatusRetrying:  true,
		BlockStatusSkipped:   true,
	},
	BlockStatusRetrying: {
		BlockStatusRunning:   true,
		BlockStatusCompleted: true,
		BlockStatusFailed:    true,
	},
	// Terminal states: completed/failed/skipped can only go back to pending (for for-each reset)
	BlockStatusCompleted: {
		BlockStatusPending: true,
	},
	BlockStatusFailed: {
		BlockStatusPending: true,
	},
	BlockStatusSkipped: {
		BlockStatusPending: true,
	},
}

// TransitionBlockStatus validates and performs a block status transition.
// Returns the new status if valid, or the current status if the transition is invalid.
func TransitionBlockStatus(current, desired BlockStatus) BlockStatus {
	allowed, exists := validTransitions[current]
	if !exists || !allowed[desired] {
		log.Printf("⚠️ [STATE] Invalid block transition: %s → %s (rejected)", current, desired)
		return current
	}
	return desired
}

// IsTerminal returns true if the status is a final state.
func IsTerminal(status BlockStatus) bool {
	return status == BlockStatusCompleted || status == BlockStatusFailed || status == BlockStatusSkipped
}
