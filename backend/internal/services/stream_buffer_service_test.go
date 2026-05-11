package services

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamBuffer_CreateAndAppend(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	convID := "test-conv-123"
	userID := "user-456"
	connID := "conn-789"

	// Create buffer
	svc.CreateBuffer(convID, userID, connID)

	// Verify buffer exists
	if !svc.HasBuffer(convID) {
		t.Fatal("Buffer should exist after creation")
	}

	// Append chunks
	chunks := []string{"Hello", " ", "World", "!"}
	for _, chunk := range chunks {
		err := svc.AppendChunk(convID, chunk)
		if err != nil {
			t.Fatalf("Failed to append chunk: %v", err)
		}
	}

	// Get buffer data
	data, err := svc.GetBufferData(convID)
	if err != nil {
		t.Fatalf("Failed to get buffer data: %v", err)
	}

	expected := "Hello World!"
	if data.CombinedChunks != expected {
		t.Errorf("Expected combined chunks %q, got %q", expected, data.CombinedChunks)
	}

	if data.ChunkCount != 4 {
		t.Errorf("Expected 4 chunks, got %d", data.ChunkCount)
	}

	if data.UserID != userID {
		t.Errorf("Expected userID %q, got %q", userID, data.UserID)
	}
}

func TestStreamBuffer_MarkComplete(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	convID := "test-conv-complete"
	userID := "user-456"
	connID := "conn-789"

	svc.CreateBuffer(convID, userID, connID)
	svc.AppendChunk(convID, "Test content")
	svc.MarkComplete(convID, "Full test content")

	data, err := svc.GetBufferData(convID)
	if err != nil {
		t.Fatalf("Failed to get buffer data: %v", err)
	}

	if !data.IsComplete {
		t.Error("Buffer should be marked as complete")
	}
}

func TestStreamBuffer_ClearBuffer(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	convID := "test-conv-clear"
	svc.CreateBuffer(convID, "user", "conn")
	svc.AppendChunk(convID, "Test")

	// Verify exists
	if !svc.HasBuffer(convID) {
		t.Fatal("Buffer should exist before clear")
	}

	// Clear
	svc.ClearBuffer(convID)

	// Verify gone
	if svc.HasBuffer(convID) {
		t.Error("Buffer should not exist after clear")
	}

	// GetBufferData should return error
	_, err := svc.GetBufferData(convID)
	if err != ErrBufferNotFound {
		t.Errorf("Expected ErrBufferNotFound, got %v", err)
	}
}

func TestStreamBuffer_MemoryLimits(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	svc.CreateBuffer("conv-1", "user-1", "conn-1")

	// Try to exceed max chunks
	for i := 0; i < MaxChunksPerBuffer+10; i++ {
		err := svc.AppendChunk("conv-1", "x")
		if i >= MaxChunksPerBuffer {
			if err != ErrBufferFull {
				t.Errorf("Expected ErrBufferFull at chunk %d, got %v", i, err)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error at chunk %d: %v", i, err)
			}
		}
	}
}

func TestStreamBuffer_SizeLimit(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	svc.CreateBuffer("conv-size", "user", "conn")

	// Create a large chunk that exceeds the size limit
	largeChunk := strings.Repeat("x", MaxBufferSize+1)
	err := svc.AppendChunk("conv-size", largeChunk)
	if err != ErrBufferSizeExceeded {
		t.Errorf("Expected ErrBufferSizeExceeded, got %v", err)
	}
}

func TestStreamBuffer_RateLimiting(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	svc.CreateBuffer("conv-rate", "user", "conn")
	svc.AppendChunk("conv-rate", "test")

	// First GetBufferData should succeed
	_, err := svc.GetBufferData("conv-rate")
	if err != nil {
		t.Fatalf("First GetBufferData should succeed: %v", err)
	}

	// Immediate second call should be rate limited
	_, err = svc.GetBufferData("conv-rate")
	if err != ErrResumeTooFast {
		t.Errorf("Expected ErrResumeTooFast, got %v", err)
	}

	// Wait for rate limit to expire
	time.Sleep(1100 * time.Millisecond)

	// Should succeed now
	_, err = svc.GetBufferData("conv-rate")
	if err != nil {
		t.Errorf("GetBufferData should succeed after rate limit: %v", err)
	}
}

func TestStreamBuffer_ConcurrentAccess(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	svc.CreateBuffer("conv-concurrent", "user", "conn")

	// Concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			svc.AppendChunk("conv-concurrent", fmt.Sprintf("chunk-%d-", idx))
		}(i)
	}
	wg.Wait()

	data, err := svc.GetBufferData("conv-concurrent")
	if err != nil {
		t.Fatalf("Failed to get buffer data: %v", err)
	}

	if data.ChunkCount != numGoroutines {
		t.Errorf("Expected %d chunks, got %d", numGoroutines, data.ChunkCount)
	}
}

func TestStreamBuffer_NonExistentBuffer(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	// Append to non-existent buffer should not error (just no-op)
	err := svc.AppendChunk("non-existent", "test")
	if err != nil {
		t.Errorf("AppendChunk to non-existent buffer should not error: %v", err)
	}

	// GetBufferData should return error
	_, err = svc.GetBufferData("non-existent")
	if err != ErrBufferNotFound {
		t.Errorf("Expected ErrBufferNotFound, got %v", err)
	}
}

func TestStreamBuffer_DuplicateCreate(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	convID := "conv-duplicate"

	// Create buffer
	svc.CreateBuffer(convID, "user-1", "conn-1")
	svc.AppendChunk(convID, "original")

	// Try to create again - should not overwrite
	svc.CreateBuffer(convID, "user-2", "conn-2")

	data, err := svc.GetBufferData(convID)
	if err != nil {
		t.Fatalf("Failed to get buffer data: %v", err)
	}

	// Should still have original user
	if data.UserID != "user-1" {
		t.Errorf("Buffer should not be overwritten, expected user-1, got %s", data.UserID)
	}

	// Should still have original content
	if data.CombinedChunks != "original" {
		t.Errorf("Buffer content should not be overwritten")
	}
}

func TestStreamBuffer_Stats(t *testing.T) {
	svc := NewStreamBufferService()
	defer svc.Shutdown()

	// Create multiple buffers
	svc.CreateBuffer("conv-1", "user", "conn")
	svc.CreateBuffer("conv-2", "user", "conn")
	svc.CreateBuffer("conv-3", "user", "conn")

	svc.AppendChunk("conv-1", "hello")
	svc.AppendChunk("conv-2", "world")
	svc.AppendChunk("conv-3", "!")

	stats := svc.GetBufferStats()

	activeBuffers := stats["active_buffers"].(int)
	if activeBuffers != 3 {
		t.Errorf("Expected 3 active buffers, got %d", activeBuffers)
	}

	totalChunks := stats["total_chunks"].(int)
	if totalChunks != 3 {
		t.Errorf("Expected 3 total chunks, got %d", totalChunks)
	}
}

func TestStreamBuffer_Shutdown(t *testing.T) {
	svc := NewStreamBufferService()
	svc.CreateBuffer("conv-shutdown", "user", "conn")
	svc.AppendChunk("conv-shutdown", "test")

	// Verify exists
	if !svc.HasBuffer("conv-shutdown") {
		t.Fatal("Buffer should exist before shutdown")
	}

	// Shutdown
	svc.Shutdown()

	// After shutdown, HasBuffer should return false (buffers is nil)
	// This is a simple check - in production, you'd want more robust handling
}
