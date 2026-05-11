package execution

import (
	"log"
	"sync"
	"time"
)

// ExecutionTracker tracks active workflow executions for graceful shutdown.
// On shutdown, it stops accepting new executions and waits for running ones to complete.
type ExecutionTracker struct {
	wg       sync.WaitGroup
	mu       sync.RWMutex
	draining bool
}

// NewExecutionTracker creates a new execution tracker.
func NewExecutionTracker() *ExecutionTracker {
	return &ExecutionTracker{}
}

// Acquire registers a new active execution. Returns false if the server is
// draining (shutting down) and new executions should be rejected.
func (t *ExecutionTracker) Acquire() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.draining {
		return false
	}
	t.wg.Add(1)
	return true
}

// Release marks an execution as finished.
func (t *ExecutionTracker) Release() {
	t.wg.Done()
}

// Drain stops accepting new executions and waits up to timeout for active ones
// to complete. Returns true if all executions finished, false if timed out.
func (t *ExecutionTracker) Drain(timeout time.Duration) bool {
	t.mu.Lock()
	t.draining = true
	t.mu.Unlock()

	log.Printf("🔄 [TRACKER] Draining active executions (timeout: %s)...", timeout)

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ [TRACKER] All active executions completed")
		return true
	case <-time.After(timeout):
		log.Println("⚠️ [TRACKER] Drain timeout reached, some executions may be interrupted")
		return false
	}
}

// IsDraining returns true if the tracker is in drain mode (shutting down).
func (t *ExecutionTracker) IsDraining() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.draining
}
