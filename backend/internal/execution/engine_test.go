package execution

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// MockBlockExecutor is a configurable fake executor for testing the DAG engine.
type MockBlockExecutor struct {
	delay       time.Duration
	output      map[string]any
	err         error
	callCount   atomic.Int32
	failUntil   int // fail the first N calls, then succeed
	failErr     error
}

func (m *MockBlockExecutor) Execute(ctx context.Context, _ models.Block, _ map[string]any) (map[string]any, error) {
	count := int(m.callCount.Add(1))
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.failUntil > 0 && count <= m.failUntil {
		if m.failErr != nil {
			return nil, m.failErr
		}
		return nil, fmt.Errorf("mock failure %d", count)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// newTestRegistry creates a bare ExecutorRegistry for testing (no real services).
func newTestRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{executors: make(map[string]BlockExecutor)}
}

func newMockExecutor(output map[string]any) *MockBlockExecutor {
	return &MockBlockExecutor{output: output}
}

func newFailingExecutor(err error) *MockBlockExecutor {
	return &MockBlockExecutor{err: err}
}

func newDelayedExecutor(delay time.Duration, output map[string]any) *MockBlockExecutor {
	return &MockBlockExecutor{delay: delay, output: output}
}

// helper to build a simple workflow with blocks and connections
func buildWorkflow(blocks []models.Block, connections []models.Connection) *models.Workflow {
	return &models.Workflow{
		ID:     "test-workflow",
		Blocks: blocks,
		Connections: connections,
	}
}

// ---- Tests ----

func TestEngine_LinearChain(t *testing.T) {
	// A -> B -> C: outputs propagate through the chain
	registry := newTestRegistry()
	registry.Register("test", newMockExecutor(map[string]any{"response": "hello"}))

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "A", Type: "test"},
			{ID: "b", NormalizedID: "b", Name: "B", Type: "test"},
			{ID: "c", NormalizedID: "c", Name: "C", Type: "test"},
		},
		[]models.Connection{
			{SourceBlockID: "a", TargetBlockID: "b"},
			{SourceBlockID: "b", TargetBlockID: "c"},
		},
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}
	// Terminal block C should have output
	if _, ok := result.Output["C"]; !ok {
		t.Errorf("expected output from terminal block C, got keys: %v", getMapKeys(result.Output))
	}
}

func TestEngine_DiamondDAG(t *testing.T) {
	// A -> B, A -> C, B+C -> D (parallel then join)
	registry := newTestRegistry()
	registry.Register("test", newMockExecutor(map[string]any{"response": "ok"}))

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "A", Type: "test"},
			{ID: "b", NormalizedID: "b", Name: "B", Type: "test"},
			{ID: "c", NormalizedID: "c", Name: "C", Type: "test"},
			{ID: "d", NormalizedID: "d", Name: "D", Type: "test"},
		},
		[]models.Connection{
			{SourceBlockID: "a", TargetBlockID: "b"},
			{SourceBlockID: "a", TargetBlockID: "c"},
			{SourceBlockID: "b", TargetBlockID: "d"},
			{SourceBlockID: "c", TargetBlockID: "d"},
		},
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}
	if result.BlockStates["d"].Status != "completed" {
		t.Errorf("expected block D to be completed, got '%s'", result.BlockStates["d"].Status)
	}
}

func TestEngine_FailurePropagation(t *testing.T) {
	// A (fails) -> B should not run
	registry := newTestRegistry()
	failExec := newFailingExecutor(fmt.Errorf("boom"))
	successExec := newMockExecutor(map[string]any{"response": "ok"})
	registry.Register("fail", failExec)
	registry.Register("success", successExec)

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "A", Type: "fail"},
			{ID: "b", NormalizedID: "b", Name: "B", Type: "success"},
		},
		[]models.Connection{
			{SourceBlockID: "a", TargetBlockID: "b"},
		},
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
	if result.BlockStates["a"].Status != "failed" {
		t.Errorf("expected block A to be failed, got '%s'", result.BlockStates["a"].Status)
	}
	// Block B should NOT have been executed (still pending)
	if result.BlockStates["b"].Status == "completed" {
		t.Errorf("block B should not have completed when A failed")
	}
	// Verify the success executor was never called
	if successExec.callCount.Load() != 0 {
		t.Errorf("expected B executor to not be called, got %d calls", successExec.callCount.Load())
	}
}

func TestEngine_WorkflowTimeout(t *testing.T) {
	// Block takes 5s but workflow timeout is 100ms
	registry := newTestRegistry()
	registry.Register("slow", newDelayedExecutor(5*time.Second, map[string]any{"response": "late"}))

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "A", Type: "slow", Timeout: 1},
		},
		nil,
	)
	workflow.WorkflowTimeout = 1 // 1 second

	statusChan := make(chan models.ExecutionUpdate, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := engine.Execute(ctx, workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Block should have failed due to timeout
	if result.BlockStates["a"].Status != "failed" {
		t.Errorf("expected block A to fail from timeout, got '%s'", result.BlockStates["a"].Status)
	}
}

func TestEngine_TemplateResolution(t *testing.T) {
	// A produces output, B uses {{a.response}} in config
	capturedInputs := make(map[string]any)
	capturingExec := &InputCapturingExecutor{
		output:         map[string]any{"response": "captured"},
		capturedInputs: &capturedInputs,
	}

	registry := newTestRegistry()
	registry.Register("producer", newMockExecutor(map[string]any{"response": "hello from A"}))
	registry.Register("consumer", capturingExec)

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "block-a", Name: "BlockA", Type: "producer"},
			{ID: "b", NormalizedID: "block-b", Name: "BlockB", Type: "consumer"},
		},
		[]models.Connection{
			{SourceBlockID: "a", TargetBlockID: "b"},
		},
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}

	// Block B should have received block A's output via flattened keys
	if val, ok := capturedInputs["response"]; !ok || val != "hello from A" {
		t.Errorf("expected B to receive A's response 'hello from A', got %v", capturedInputs["response"])
	}

	// Block B should also have A's output under its normalized ID
	if blockAData, ok := capturedInputs["block-a"]; !ok {
		t.Errorf("expected B to receive A's output under 'block-a', got keys: %v", getMapKeys(capturedInputs))
	} else if m, ok := blockAData.(map[string]any); ok {
		if m["response"] != "hello from A" {
			t.Errorf("expected block-a.response='hello from A', got %v", m["response"])
		}
	}
}

func TestEngine_ParallelStartBlocks(t *testing.T) {
	// A and B are both start blocks (no deps), both should execute
	registry := newTestRegistry()
	exec := newMockExecutor(map[string]any{"response": "parallel"})
	registry.Register("test", exec)

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "A", Type: "test"},
			{ID: "b", NormalizedID: "b", Name: "B", Type: "test"},
		},
		nil, // no connections = both are start blocks
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if exec.callCount.Load() != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.callCount.Load())
	}
}

func TestEngine_PanicRecovery(t *testing.T) {
	// A panicking block should not crash the engine
	registry := newTestRegistry()
	registry.Register("panic", &PanickingExecutor{})
	registry.Register("test", newMockExecutor(map[string]any{"response": "ok"}))

	engine := NewWorkflowEngine(registry)

	workflow := buildWorkflow(
		[]models.Block{
			{ID: "a", NormalizedID: "a", Name: "PanicBlock", Type: "panic"},
			{ID: "b", NormalizedID: "b", Name: "SafeBlock", Type: "test"},
		},
		nil, // both are start blocks, independent
	)

	statusChan := make(chan models.ExecutionUpdate, 100)
	result, err := engine.Execute(context.Background(), workflow, nil, statusChan)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Panic block should be marked failed
	if result.BlockStates["a"].Status != "failed" {
		t.Errorf("expected panic block to be marked failed, got '%s'", result.BlockStates["a"].Status)
	}
	// Safe block should still have completed
	if result.BlockStates["b"].Status != "completed" {
		t.Errorf("expected safe block to complete, got '%s'", result.BlockStates["b"].Status)
	}
}

// ---- Helper Executors ----

// InputCapturingExecutor records the inputs it receives for test assertions.
type InputCapturingExecutor struct {
	output         map[string]any
	capturedInputs *map[string]any
}

func (e *InputCapturingExecutor) Execute(_ context.Context, _ models.Block, inputs map[string]any) (map[string]any, error) {
	for k, v := range inputs {
		(*e.capturedInputs)[k] = v
	}
	return e.output, nil
}

// PanickingExecutor always panics — used to test panic recovery.
type PanickingExecutor struct{}

func (e *PanickingExecutor) Execute(_ context.Context, _ models.Block, _ map[string]any) (map[string]any, error) {
	panic("intentional test panic")
}
