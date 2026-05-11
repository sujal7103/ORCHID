package services

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements three-tier rate limiting for web scraping
type RateLimiter struct {
	globalLimiter     *rate.Limiter // Overall requests/second for the server
	perDomainLimiters *sync.Map     // map[string]*rate.Limiter - per domain limits
	perUserLimiters   *sync.Map     // map[string]*rate.Limiter - per user limits
	mu                sync.RWMutex
}

// NewRateLimiter creates a new three-tier rate limiter
func NewRateLimiter(globalRate, perUserRate float64) *RateLimiter {
	return &RateLimiter{
		globalLimiter:     rate.NewLimiter(rate.Limit(globalRate), int(globalRate*2)),      // 10 req/s, burst 20
		perDomainLimiters: &sync.Map{},
		perUserLimiters:   &sync.Map{},
	}
}

// Wait applies all three tiers of rate limiting
func (rl *RateLimiter) Wait(ctx context.Context, userID, domain string) error {
	// Tier 1: Global rate limit (protect server resources)
	if err := rl.globalLimiter.Wait(ctx); err != nil {
		return err
	}

	// Tier 2: Per-domain rate limit (respect target websites)
	domainLimiter := rl.getOrCreateDomainLimiter(domain)
	if err := domainLimiter.Wait(ctx); err != nil {
		return err
	}

	// Tier 3: Per-user rate limit (fair usage)
	userLimiter := rl.getOrCreateUserLimiter(userID)
	if err := userLimiter.Wait(ctx); err != nil {
		return err
	}

	return nil
}

// WaitWithCrawlDelay applies rate limiting with respect to robots.txt crawl-delay
func (rl *RateLimiter) WaitWithCrawlDelay(ctx context.Context, userID, domain string, crawlDelay time.Duration) error {
	// Tier 1: Global rate limit
	if err := rl.globalLimiter.Wait(ctx); err != nil {
		return err
	}

	// Tier 2: Per-domain rate limit with crawl-delay
	domainLimiter := rl.getOrCreateDomainLimiterWithDelay(domain, crawlDelay)
	if err := domainLimiter.Wait(ctx); err != nil {
		return err
	}

	// Tier 3: Per-user rate limit
	userLimiter := rl.getOrCreateUserLimiter(userID)
	if err := userLimiter.Wait(ctx); err != nil {
		return err
	}

	return nil
}

// getOrCreateDomainLimiter gets or creates a rate limiter for a domain (default 2 req/s)
func (rl *RateLimiter) getOrCreateDomainLimiter(domain string) *rate.Limiter {
	return rl.getOrCreateDomainLimiterWithDelay(domain, 500*time.Millisecond)
}

// getOrCreateDomainLimiterWithDelay gets or creates a rate limiter for a domain with custom delay
func (rl *RateLimiter) getOrCreateDomainLimiterWithDelay(domain string, crawlDelay time.Duration) *rate.Limiter {
	if limiter, ok := rl.perDomainLimiters.Load(domain); ok {
		return limiter.(*rate.Limiter)
	}

	// Create new limiter based on crawl delay
	requestsPerSecond := 1.0 / crawlDelay.Seconds()
	if requestsPerSecond > 5.0 {
		requestsPerSecond = 5.0 // Cap at 5 req/s
	}
	if requestsPerSecond < 0.2 {
		requestsPerSecond = 0.2 // Minimum 1 request per 5 seconds
	}

	newLimiter := rate.NewLimiter(rate.Limit(requestsPerSecond), 1)

	// Try to store, but use existing if another goroutine created it first
	actual, _ := rl.perDomainLimiters.LoadOrStore(domain, newLimiter)
	return actual.(*rate.Limiter)
}

// getOrCreateUserLimiter gets or creates a rate limiter for a user (5 req/s)
func (rl *RateLimiter) getOrCreateUserLimiter(userID string) *rate.Limiter {
	if limiter, ok := rl.perUserLimiters.Load(userID); ok {
		return limiter.(*rate.Limiter)
	}

	newLimiter := rate.NewLimiter(rate.Limit(5.0), 10) // 5 req/s, burst 10

	// Try to store, but use existing if another goroutine created it first
	actual, _ := rl.perUserLimiters.LoadOrStore(userID, newLimiter)
	return actual.(*rate.Limiter)
}

// SetGlobalRate updates the global rate limit
func (rl *RateLimiter) SetGlobalRate(requestsPerSecond float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.globalLimiter.SetLimit(rate.Limit(requestsPerSecond))
	rl.globalLimiter.SetBurst(int(requestsPerSecond * 2))
}
