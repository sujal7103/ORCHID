package execution

import (
	"fmt"
	"strings"
)

// UserFriendlyError converts an ExecutionError into a message that makes sense
// to non-technical users, with actionable guidance.
func UserFriendlyError(err *ExecutionError) string {
	if err == nil {
		return ""
	}

	// Rate limiting
	if err.StatusCode == 429 {
		if err.RetryAfter > 0 {
			return fmt.Sprintf("The AI provider is rate-limiting requests. Retrying automatically in %d seconds.", err.RetryAfter)
		}
		return "The AI provider is rate-limiting requests. The system will retry automatically."
	}

	// Server errors
	if err.StatusCode >= 500 && err.StatusCode < 600 {
		return "The external service is temporarily unavailable. The system will retry automatically."
	}

	// Auth errors
	if err.StatusCode == 401 {
		return "Your API key is invalid or expired. Please update it in Settings > Providers."
	}
	if err.StatusCode == 403 {
		return "Access denied by the provider. Check that your API key has the required permissions."
	}

	// Bad request
	if err.StatusCode == 400 {
		return "The request was rejected by the provider. This usually means the prompt is too long or contains unsupported content."
	}

	// Not found
	if err.StatusCode == 404 {
		return "The requested model or endpoint was not found. Check that the model name is correct."
	}

	// Quota/billing
	if err.StatusCode == 402 || containsAny(err.Message, "quota", "billing", "insufficient_quota", "payment") {
		return "Your provider account has reached its spending limit. Add credits or upgrade your plan at the provider's dashboard."
	}

	// Timeout
	if containsAny(err.Message, "timeout", "deadline exceeded", "timed out") {
		return "The request took too long. Try simplifying the prompt or using a faster model."
	}

	// Network errors
	if containsAny(err.Message, "connection refused", "connection reset", "network", "no such host") {
		return "Could not reach the external service. Check your network connection and provider status."
	}

	// TLS/certificate
	if containsAny(err.Message, "certificate", "tls:", "x509:") {
		return "SSL/TLS certificate error connecting to the provider. This may be a network configuration issue."
	}

	// Panic recovery
	if strings.HasPrefix(err.Message, "internal panic") {
		return "An unexpected internal error occurred. This has been logged for investigation."
	}

	// Fallback: return the original message truncated
	msg := err.Message
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return msg
}

func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}
