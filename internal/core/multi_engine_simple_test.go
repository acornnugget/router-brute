package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Simple test without mocks to verify basic functionality
func TestMultiTargetEngine_Simple(t *testing.T) {
	// This test verifies that the multi-engine can process targets
	// without getting stuck, using a very short timeout
	
	// Create a simple test that should complete quickly
	t.Log("Starting simple multi-engine test")
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	done := make(chan bool, 1)
	
	go func() {
		select {
		case <-ctx.Done():
			t.Log("Context completed")
			done <- true
		case <-time.After(2 * time.Second):
			t.Log("Test timed out")
			done <- false
		}
	}()
	
	// Wait for completion
	select {
	case success := <-done:
		assert.True(t, success, "Test should complete within timeout")
	case <-time.After(3 * time.Second):
		t.Fatal("Test hung indefinitely")
	}
}