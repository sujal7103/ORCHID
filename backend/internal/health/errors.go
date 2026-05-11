package health

import (
	"net/http"
	"strings"
	"time"
)

// IsQuotaError detects if an error is related to quota exhaustion or rate limiting
func IsQuotaError(statusCode int, responseBody string) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	lowerBody := strings.ToLower(responseBody)
	quotaPatterns := []string{
		"quota exceeded",
		"rate limit",
		"too many requests",
		"request limit",
		"tokens per minute",
		"requests per minute",
		"daily limit",
		"insufficient_quota",
		"billing",
		"rate_limit_exceeded",
		"quota_exceeded",
	}

	for _, pattern := range quotaPatterns {
		if strings.Contains(lowerBody, pattern) {
			return true
		}
	}

	return false
}

// ParseCooldownDuration determines the appropriate cooldown based on the error type
func ParseCooldownDuration(statusCode int, responseBody string) time.Duration {
	lowerBody := strings.ToLower(responseBody)

	// Daily limit or billing issues - cool down for a long time
	if strings.Contains(lowerBody, "daily limit") ||
		strings.Contains(lowerBody, "billing") ||
		strings.Contains(lowerBody, "insufficient_quota") {
		return 24 * time.Hour
	}

	// Rate limit (per-minute) - short cooldown
	if statusCode == http.StatusTooManyRequests ||
		strings.Contains(lowerBody, "tokens per minute") ||
		strings.Contains(lowerBody, "requests per minute") {
		return 5 * time.Minute
	}

	// Default cooldown
	return 1 * time.Hour
}
