package core

import (
	"testing"
	"time"
)

func TestEngineBasic(t *testing.T) {
	// Create engine with 2 workers and 10ms rate limit
	engine := NewEngine(2, 10*time.Millisecond)

	// Create and set a local mock module
	mockModule := newMockModule("127.0.0.1", "test-user", "secret123")
	engine.SetModule(mockModule)

	// Load some test passwords
	passwords := []string{"password1", "secret123", "password2", "password3"}
	engine.LoadPasswords(passwords)

	// Start the engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Let it run for a short time
	time.Sleep(100 * time.Millisecond)

	// Check progress
	progress := engine.Progress()
	if progress <= 0.0 {
		t.Error("Expected some progress, got 0.0")
	}

	// Get stats
	stats := engine.Stats()
	if stats == nil {
		t.Error("Expected stats, got nil")
	}

	// Check results channel
	resultCount := 0
	successCount := 0
	timeout := time.After(200 * time.Millisecond)
collecting:
	for {
		select {
		case result, ok := <-engine.Results():
			if !ok {
				break collecting
			}
			resultCount++
			if result.Success {
				successCount++
			}
			if resultCount >= len(passwords) {
				break collecting
			}
		case <-timeout:
			break collecting
		}
	}

	if resultCount == 0 {
		t.Error("Expected some results, got none")
	}

	if successCount != 1 {
		t.Errorf("Expected 1 successful authentication, got %d", successCount)
	}

	// Stop the engine
	engine.Stop()

	t.Logf("Engine test completed successfully")
	t.Logf("Progress: %.2f%%", progress*100)
	t.Logf("Results received: %d", resultCount)
	t.Logf("Successful authentications: %d", successCount)
}
