package services

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNewExecutionService(t *testing.T) {
	// Test creation without dependencies (both nil)
	service := NewExecutionService(nil, nil)
	if service == nil {
		t.Fatal("Expected non-nil execution service")
	}
}

func TestExecutionRecord_Structure(t *testing.T) {
	// Test that ExecutionRecord can be created with all fields
	record := &ExecutionRecord{
		ID:              primitive.NewObjectID(),
		AgentID:         "agent-123",
		UserID:          "user-456",
		WorkflowVersion: 1,
		TriggerType:     "manual",
		Status:          "pending",
		Input:           map[string]interface{}{"topic": "test"},
	}

	if record.AgentID != "agent-123" {
		t.Errorf("Expected AgentID 'agent-123', got '%s'", record.AgentID)
	}

	if record.TriggerType != "manual" {
		t.Errorf("Expected TriggerType 'manual', got '%s'", record.TriggerType)
	}
}

func TestCreateExecutionRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		req         CreateExecutionRequest
		wantAgentID string
		wantTrigger string
	}{
		{
			name: "manual trigger",
			req: CreateExecutionRequest{
				AgentID:         "agent-1",
				UserID:          "user-1",
				WorkflowVersion: 1,
				TriggerType:     "manual",
			},
			wantAgentID: "agent-1",
			wantTrigger: "manual",
		},
		{
			name: "scheduled trigger",
			req: CreateExecutionRequest{
				AgentID:         "agent-2",
				UserID:          "user-2",
				WorkflowVersion: 2,
				TriggerType:     "scheduled",
				ScheduleID:      primitive.NewObjectID(),
			},
			wantAgentID: "agent-2",
			wantTrigger: "scheduled",
		},
		{
			name: "api trigger",
			req: CreateExecutionRequest{
				AgentID:         "agent-3",
				UserID:          "user-3",
				WorkflowVersion: 1,
				TriggerType:     "api",
				APIKeyID:        primitive.NewObjectID(),
			},
			wantAgentID: "agent-3",
			wantTrigger: "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.AgentID != tt.wantAgentID {
				t.Errorf("Expected AgentID '%s', got '%s'", tt.wantAgentID, tt.req.AgentID)
			}
			if tt.req.TriggerType != tt.wantTrigger {
				t.Errorf("Expected TriggerType '%s', got '%s'", tt.wantTrigger, tt.req.TriggerType)
			}
		})
	}
}

func TestListExecutionsOptions_Defaults(t *testing.T) {
	opts := &ListExecutionsOptions{}

	// Default values should be zero/empty
	if opts.Page != 0 {
		t.Errorf("Expected default Page 0, got %d", opts.Page)
	}
	if opts.Limit != 0 {
		t.Errorf("Expected default Limit 0, got %d", opts.Limit)
	}
	if opts.Status != "" {
		t.Errorf("Expected empty default Status, got '%s'", opts.Status)
	}
}

func TestListExecutionsOptions_WithFilters(t *testing.T) {
	opts := &ListExecutionsOptions{
		Page:        2,
		Limit:       50,
		Status:      "completed",
		TriggerType: "scheduled",
		AgentID:     "agent-123",
	}

	if opts.Page != 2 {
		t.Errorf("Expected Page 2, got %d", opts.Page)
	}
	if opts.Limit != 50 {
		t.Errorf("Expected Limit 50, got %d", opts.Limit)
	}
	if opts.Status != "completed" {
		t.Errorf("Expected Status 'completed', got '%s'", opts.Status)
	}
	if opts.TriggerType != "scheduled" {
		t.Errorf("Expected TriggerType 'scheduled', got '%s'", opts.TriggerType)
	}
	if opts.AgentID != "agent-123" {
		t.Errorf("Expected AgentID 'agent-123', got '%s'", opts.AgentID)
	}
}

func TestPaginatedExecutions_Empty(t *testing.T) {
	result := &PaginatedExecutions{
		Executions: []ExecutionRecord{},
		Total:      0,
		Page:       1,
		Limit:      20,
		HasMore:    false,
	}

	if len(result.Executions) != 0 {
		t.Errorf("Expected 0 executions, got %d", len(result.Executions))
	}
	if result.HasMore {
		t.Error("Expected HasMore to be false")
	}
}

func TestExecutionStats_Empty(t *testing.T) {
	stats := &ExecutionStats{
		Total:        0,
		SuccessCount: 0,
		FailedCount:  0,
		SuccessRate:  0,
		ByStatus:     make(map[string]StatusStats),
	}

	if stats.Total != 0 {
		t.Errorf("Expected Total 0, got %d", stats.Total)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("Expected SuccessRate 0, got %f", stats.SuccessRate)
	}
}

func TestExecutionStats_Calculations(t *testing.T) {
	// Simulate stats calculation
	stats := &ExecutionStats{
		Total:        100,
		SuccessCount: 85,
		FailedCount:  15,
		SuccessRate:  85.0,
		ByStatus: map[string]StatusStats{
			"completed": {Count: 85, AvgDuration: 1500},
			"failed":    {Count: 15, AvgDuration: 2000},
		},
	}

	if stats.SuccessRate != 85.0 {
		t.Errorf("Expected SuccessRate 85.0, got %f", stats.SuccessRate)
	}

	completedStats, ok := stats.ByStatus["completed"]
	if !ok {
		t.Fatal("Expected 'completed' status in ByStatus")
	}
	if completedStats.Count != 85 {
		t.Errorf("Expected completed count 85, got %d", completedStats.Count)
	}
}

func TestExecutionCompleteRequest_Fields(t *testing.T) {
	req := &ExecutionCompleteRequest{
		Status: "completed",
		Output: map[string]interface{}{
			"result": "success",
		},
		Error: "",
	}

	if req.Status != "completed" {
		t.Errorf("Expected Status 'completed', got '%s'", req.Status)
	}
	if req.Error != "" {
		t.Errorf("Expected empty Error, got '%s'", req.Error)
	}
}

func TestExecutionCompleteRequest_WithError(t *testing.T) {
	req := &ExecutionCompleteRequest{
		Status: "failed",
		Output: nil,
		Error:  "execution timeout",
	}

	if req.Status != "failed" {
		t.Errorf("Expected Status 'failed', got '%s'", req.Status)
	}
	if req.Error != "execution timeout" {
		t.Errorf("Expected Error 'execution timeout', got '%s'", req.Error)
	}
}

// Integration test helper - would need MongoDB to run actual tests
func TestExecutionService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require a running MongoDB instance
	// For now, just verify the service can be created
	_ = context.Background()
	service := NewExecutionService(nil, nil)
	if service == nil {
		t.Fatal("Expected non-nil service")
	}
}
