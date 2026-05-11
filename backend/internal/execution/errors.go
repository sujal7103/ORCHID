package execution

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrorCategory classifies errors for retry decisions
type ErrorCategory int

const (
	// ErrorCategoryUnknown - unclassified error, default to not retryable
	ErrorCategoryUnknown ErrorCategory = iota

	// ErrorCategoryTransient - temporary failures that may succeed on retry
	// Examples: timeout, rate limit (429), server error (5xx), network error
	ErrorCategoryTransient

	// ErrorCategoryPermanent - errors that will not succeed on retry
	// Examples: auth error (401/403), bad request (400), parse error
	ErrorCategoryPermanent

	// ErrorCategoryValidation - business logic validation failures
	// Examples: tool not called, required tool missing
	ErrorCategoryValidation
)

// String returns a human-readable category name
func (c ErrorCategory) String() string {
	switch c {
	case ErrorCategoryTransient:
		return "transient"
	case ErrorCategoryPermanent:
		return "permanent"
	case ErrorCategoryValidation:
		return "validation"
	default:
		return "unknown"
	}
}

// ExecutionError wraps errors with classification for retry logic
type ExecutionError struct {
	Category   ErrorCategory
	Message    string
	StatusCode int   // HTTP status code if applicable
	Retryable  bool  // Explicit retryable flag
	RetryAfter int   // Seconds to wait before retry (from Retry-After header)
	Cause      error // Original error
}

func (e *ExecutionError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%d] %s", e.StatusCode, e.Message)
	}
	return e.Message
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// IsRetryable determines if an error should be retried
func (e *ExecutionError) IsRetryable() bool {
	return e.Retryable
}

// ClassifyHTTPError classifies an HTTP response error
func ClassifyHTTPError(statusCode int, body string) *ExecutionError {
	err := &ExecutionError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf("HTTP %d: %s", statusCode, truncateString(body, 200)),
	}

	switch {
	// Rate limiting - always retryable
	case statusCode == http.StatusTooManyRequests:
		err.Category = ErrorCategoryTransient
		err.Retryable = true
		err.RetryAfter = 60 // Default 60 seconds for rate limiting

	// Server errors - retryable
	case statusCode >= 500 && statusCode < 600:
		err.Category = ErrorCategoryTransient
		err.Retryable = true

	// Request timeout - retryable
	case statusCode == http.StatusRequestTimeout:
		err.Category = ErrorCategoryTransient
		err.Retryable = true

	// Gateway errors - retryable
	case statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout:
		err.Category = ErrorCategoryTransient
		err.Retryable = true

	// Auth errors - NOT retryable
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		err.Category = ErrorCategoryPermanent
		err.Retryable = false

	// Bad request - NOT retryable
	case statusCode == http.StatusBadRequest:
		err.Category = ErrorCategoryPermanent
		err.Retryable = false

	// Not found - NOT retryable
	case statusCode == http.StatusNotFound:
		err.Category = ErrorCategoryPermanent
		err.Retryable = false

	// Unprocessable entity - NOT retryable
	case statusCode == http.StatusUnprocessableEntity:
		err.Category = ErrorCategoryPermanent
		err.Retryable = false

	default:
		err.Category = ErrorCategoryUnknown
		err.Retryable = false
	}

	return err
}

// ClassifyError classifies a general error
func ClassifyError(err error) *ExecutionError {
	if err == nil {
		return nil
	}

	// If already an ExecutionError, return as-is
	if execErr, ok := err.(*ExecutionError); ok {
		return execErr
	}

	errStr := err.Error()

	// Context timeout/cancellation
	if strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "context canceled") {
		return &ExecutionError{
			Category:  ErrorCategoryTransient,
			Message:   "Request timed out",
			Retryable: true,
			Cause:     err,
		}
	}

	// Network errors - connection issues
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "EOF") {
		return &ExecutionError{
			Category:  ErrorCategoryTransient,
			Message:   fmt.Sprintf("Network error: %s", truncateString(errStr, 100)),
			Retryable: true,
			Cause:     err,
		}
	}

	// TLS errors - usually permanent
	if strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "tls:") ||
		strings.Contains(errStr, "x509:") {
		return &ExecutionError{
			Category:  ErrorCategoryPermanent,
			Message:   "TLS/Certificate error",
			Retryable: false,
			Cause:     err,
		}
	}

	// DNS errors - may be transient
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") {
		return &ExecutionError{
			Category:  ErrorCategoryTransient,
			Message:   "DNS resolution error",
			Retryable: true,
			Cause:     err,
		}
	}

	// Default: unknown, not retryable
	return &ExecutionError{
		Category:  ErrorCategoryUnknown,
		Message:   truncateString(errStr, 200),
		Retryable: false,
		Cause:     err,
	}
}

// BackoffCalculator computes retry delays with exponential backoff and jitter
type BackoffCalculator struct {
	initialDelay  time.Duration
	maxDelay      time.Duration
	multiplier    float64
	jitterPercent int
}

// NewBackoffCalculator creates a calculator with specified parameters
func NewBackoffCalculator(initialDelayMs, maxDelayMs int, multiplier float64, jitterPercent int) *BackoffCalculator {
	// Apply defaults if not specified
	if initialDelayMs <= 0 {
		initialDelayMs = 1000 // 1 second default
	}
	if maxDelayMs <= 0 {
		maxDelayMs = 30000 // 30 seconds default
	}
	if multiplier <= 0 {
		multiplier = 2.0 // Double each time
	}
	if jitterPercent < 0 {
		jitterPercent = 20 // 20% jitter default
	}

	return &BackoffCalculator{
		initialDelay:  time.Duration(initialDelayMs) * time.Millisecond,
		maxDelay:      time.Duration(maxDelayMs) * time.Millisecond,
		multiplier:    multiplier,
		jitterPercent: jitterPercent,
	}
}

// NextDelay calculates the delay for the given attempt number (0-indexed)
func (b *BackoffCalculator) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential delay: initialDelay * (multiplier ^ attempt)
	delay := float64(b.initialDelay) * math.Pow(b.multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(b.maxDelay) {
		delay = float64(b.maxDelay)
	}

	// Add jitter to prevent thundering herd
	if b.jitterPercent > 0 {
		jitterRange := delay * float64(b.jitterPercent) / 100.0
		jitter := (rand.Float64()*2 - 1) * jitterRange // -jitterRange to +jitterRange
		delay += jitter
	}

	// Ensure non-negative
	if delay < 0 {
		delay = float64(b.initialDelay)
	}

	return time.Duration(delay)
}

// ShouldRetry determines if the error type should be retried based on policy
func ShouldRetry(err *ExecutionError, retryOn []string) bool {
	if err == nil || !err.Retryable {
		return false
	}

	// If no specific retry types configured, retry all retryable errors
	if len(retryOn) == 0 {
		return err.Retryable
	}

	// Map error to type string
	errorType := getErrorType(err)

	// Check if this error type is in the retry list
	for _, retryType := range retryOn {
		if retryType == errorType || retryType == "all_transient" {
			return true
		}
	}

	return false
}

// getErrorType maps an ExecutionError to a type string for retry matching
func getErrorType(err *ExecutionError) string {
	if err.StatusCode == 429 {
		return "rate_limit"
	}
	if err.StatusCode >= 500 {
		return "server_error"
	}
	if strings.Contains(strings.ToLower(err.Message), "timeout") ||
		strings.Contains(strings.ToLower(err.Message), "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Message), "network") ||
		strings.Contains(strings.ToLower(err.Message), "connection") {
		return "network_error"
	}
	return "unknown"
}

// CircuitBreaker tracks consecutive failures per error source (e.g., HTTP host, tool name)
// within a single workflow execution. After threshold consecutive failures from the same
// source, it trips and short-circuits retries for all blocks hitting that source.
type CircuitBreaker struct {
	mu               sync.Mutex
	consecutiveFails map[string]int // source → consecutive failure count
	tripped          map[string]bool
	threshold        int // consecutive failures before tripping
}

// NewCircuitBreaker creates a circuit breaker with the given threshold.
// A threshold of 5 means after 5 consecutive failures from the same source,
// all subsequent retries against that source are skipped.
func NewCircuitBreaker(threshold int) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	return &CircuitBreaker{
		consecutiveFails: make(map[string]int),
		tripped:          make(map[string]bool),
		threshold:        threshold,
	}
}

// RecordFailure records a failure from the given source. Returns true if the
// circuit has now tripped (or was already tripped).
func (cb *CircuitBreaker) RecordFailure(source string) bool {
	if source == "" {
		return false
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails[source]++
	if cb.consecutiveFails[source] >= cb.threshold {
		cb.tripped[source] = true
	}
	return cb.tripped[source]
}

// RecordSuccess resets the failure count for the given source.
func (cb *CircuitBreaker) RecordSuccess(source string) {
	if source == "" {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.consecutiveFails, source)
	delete(cb.tripped, source)
}

// IsTripped returns true if the circuit for the given source is open.
func (cb *CircuitBreaker) IsTripped(source string) bool {
	if source == "" {
		return false
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped[source]
}

// ErrorSource extracts a circuit breaker key from an error.
// For HTTP errors, uses the status code bucket (5xx, 429).
// For network errors, uses "network".
func ErrorSource(err *ExecutionError) string {
	if err == nil {
		return ""
	}
	if err.StatusCode == 429 {
		return "rate_limit"
	}
	if err.StatusCode >= 500 {
		return "server_5xx"
	}
	errType := getErrorType(err)
	if errType == "network_error" || errType == "timeout" {
		return errType
	}
	return ""
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
