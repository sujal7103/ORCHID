package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/temoto/robotstxt"
)

// RobotsChecker handles robots.txt fetching and compliance checking
type RobotsChecker struct {
	cache     *cache.Cache
	userAgent string
	client    *http.Client
}

// NewRobotsChecker creates a new robots.txt checker
func NewRobotsChecker(userAgent string) *RobotsChecker {
	return &RobotsChecker{
		cache:     cache.New(24*time.Hour, 1*time.Hour), // Cache robots.txt for 24 hours
		userAgent: userAgent,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CanFetch checks if the URL can be fetched according to robots.txt
// Returns (allowed bool, crawlDelay time.Duration, error)
func (rc *RobotsChecker) CanFetch(ctx context.Context, urlStr string) (bool, time.Duration, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false, 0, fmt.Errorf("invalid URL: %w", err)
	}

	domain := parsedURL.Scheme + "://" + parsedURL.Host
	robotsURL := domain + "/robots.txt"

	// Check cache first
	if cached, found := rc.cache.Get(domain); found {
		robotsData := cached.(*robotstxt.RobotsData)
		group := robotsData.FindGroup(rc.userAgent)
		allowed := group.Test(parsedURL.Path)
		crawlDelay := rc.getCrawlDelay(group)
		return allowed, crawlDelay, nil
	}

	// Fetch robots.txt
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", rc.userAgent)

	resp, err := rc.client.Do(req)
	if err != nil {
		// If robots.txt doesn't exist or network error, allow by default
		return true, 1 * time.Second, nil
	}
	defer resp.Body.Close()

	// If robots.txt returns 404 or other error, allow by default
	if resp.StatusCode != http.StatusOK {
		return true, 1 * time.Second, nil
	}

	// Read and parse robots.txt
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // Max 1MB
	if err != nil {
		return true, 1 * time.Second, nil
	}

	robotsData, err := robotstxt.FromBytes(body)
	if err != nil {
		// If parsing fails, be conservative and allow
		return true, 1 * time.Second, nil
	}

	// Cache the robots.txt data
	rc.cache.Set(domain, robotsData, cache.DefaultExpiration)

	// Check if path is allowed
	group := robotsData.FindGroup(rc.userAgent)
	allowed := group.Test(parsedURL.Path)
	crawlDelay := rc.getCrawlDelay(group)

	return allowed, crawlDelay, nil
}

// getCrawlDelay extracts crawl delay from robots.txt group
func (rc *RobotsChecker) getCrawlDelay(group *robotstxt.Group) time.Duration {
	if group.CrawlDelay > 0 {
		delay := time.Duration(group.CrawlDelay) * time.Second
		// Cap at maximum 10 seconds
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
		return delay
	}

	// Default to 1 second if no crawl delay specified
	return 1 * time.Second
}

// SetUserAgent updates the user agent string
func (rc *RobotsChecker) SetUserAgent(userAgent string) {
	rc.userAgent = userAgent
}
