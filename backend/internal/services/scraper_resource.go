package services

import (
	"context"
	"fmt"
	"io"
)

// ResourceManager handles resource limits for concurrent scraping
type ResourceManager struct {
	semaphore   chan struct{} // Limit concurrent requests
	maxBodySize int64         // Max response body size in bytes
}

// NewResourceManager creates a new resource manager
func NewResourceManager(maxConcurrent int, maxBodySize int64) *ResourceManager {
	return &ResourceManager{
		semaphore:   make(chan struct{}, maxConcurrent),
		maxBodySize: maxBodySize,
	}
}

// Acquire acquires a slot for a scraping operation
func (rm *ResourceManager) Acquire(ctx context.Context) error {
	select {
	case rm.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for resource: %w", ctx.Err())
	}
}

// Release releases a slot after scraping completes
func (rm *ResourceManager) Release() {
	<-rm.semaphore
}

// ReadBody reads the response body with size limit to prevent memory exhaustion
func (rm *ResourceManager) ReadBody(body io.Reader) ([]byte, error) {
	limitedReader := io.LimitReader(body, rm.maxBodySize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	// Check if we hit the limit
	if int64(len(data)) >= rm.maxBodySize {
		return nil, fmt.Errorf("response body too large (max %d bytes)", rm.maxBodySize)
	}

	return data, nil
}
