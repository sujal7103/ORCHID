package execution

import (
	"testing"
)

func TestCircuitBreaker_TripsAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3)

	// First 2 failures: not tripped
	if cb.RecordFailure("server_5xx") {
		t.Error("should not trip after 1 failure")
	}
	if cb.RecordFailure("server_5xx") {
		t.Error("should not trip after 2 failures")
	}
	if cb.IsTripped("server_5xx") {
		t.Error("should not be tripped before threshold")
	}

	// 3rd failure: trips
	if !cb.RecordFailure("server_5xx") {
		t.Error("should trip after 3 consecutive failures")
	}
	if !cb.IsTripped("server_5xx") {
		t.Error("should be tripped after threshold reached")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := NewCircuitBreaker(3)

	cb.RecordFailure("server_5xx")
	cb.RecordFailure("server_5xx")
	// 2 failures, then success resets
	cb.RecordSuccess("server_5xx")

	if cb.IsTripped("server_5xx") {
		t.Error("success should reset the circuit")
	}

	// Need 3 more failures to trip again
	cb.RecordFailure("server_5xx")
	cb.RecordFailure("server_5xx")
	if cb.IsTripped("server_5xx") {
		t.Error("should need full threshold after reset")
	}
	cb.RecordFailure("server_5xx")
	if !cb.IsTripped("server_5xx") {
		t.Error("should trip again after threshold")
	}
}

func TestCircuitBreaker_IndependentSources(t *testing.T) {
	cb := NewCircuitBreaker(2)

	cb.RecordFailure("server_5xx")
	cb.RecordFailure("rate_limit")
	cb.RecordFailure("server_5xx")

	if !cb.IsTripped("server_5xx") {
		t.Error("server_5xx should be tripped (2 failures)")
	}
	if cb.IsTripped("rate_limit") {
		t.Error("rate_limit should NOT be tripped (only 1 failure)")
	}
}

func TestCircuitBreaker_EmptySource(t *testing.T) {
	cb := NewCircuitBreaker(1)

	// Empty source should never trip (permanent errors have no source)
	if cb.RecordFailure("") {
		t.Error("empty source should never trip")
	}
	if cb.IsTripped("") {
		t.Error("empty source should never be tripped")
	}
}

func TestErrorSource(t *testing.T) {
	tests := []struct {
		name     string
		err      *ExecutionError
		expected string
	}{
		{"nil error", nil, ""},
		{"429 rate limit", &ExecutionError{StatusCode: 429}, "rate_limit"},
		{"500 server error", &ExecutionError{StatusCode: 500}, "server_5xx"},
		{"502 bad gateway", &ExecutionError{StatusCode: 502}, "server_5xx"},
		{"timeout keyword", &ExecutionError{Message: "connection timeout"}, "timeout"},
		{"deadline exceeded", &ExecutionError{Message: "context deadline exceeded"}, "timeout"},
		{"network error", &ExecutionError{Message: "Network error: connection refused"}, "network_error"},
		{"400 bad request", &ExecutionError{StatusCode: 400, Message: "bad request"}, ""},
		{"auth error", &ExecutionError{StatusCode: 401, Message: "unauthorized"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorSource(tt.err)
			if got != tt.expected {
				t.Errorf("ErrorSource() = %q, want %q", got, tt.expected)
			}
		})
	}
}
