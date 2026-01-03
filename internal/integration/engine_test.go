package integration

import (
	"testing"
	"time"

	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/modules/mock"
)

func TestEngineWithMockModule(t *testing.T) {
	// Create engine with 2 workers and 10ms rate limit
	engine := core.NewEngine(2, 10*time.Millisecond)

	// Create mock module
	mockModule := mock.NewMockModule()
	mockModule.Initialize("192.168.1.1", "admin", nil)
	mockModule.SetSuccessPassword("secret123")

	// Set the module
	engine.SetModule(mockModule)

	// Load some test passwords (include the success password)
	passwords := []string{"password1", "secret123", "password2", "wrongpass"}
	engine.LoadPasswords(passwords)

	// Start the engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Collect results
	successCount := 0
	totalResults := 0

	for result := range engine.Results() {
		totalResults++

		if result.Success {
			successCount++
			if result.Password != "secret123" {
				t.Errorf("Expected success password 'secret123', got '%s'", result.Password)
			}
		}

		// Stop after we get all results
		if totalResults >= len(passwords) {
			break
		}
	}

	// Stop the engine
	engine.Stop()

	// Verify we got the expected success
	if successCount != 1 {
		t.Errorf("Expected 1 successful authentication, got %d", successCount)
	}

	if totalResults != len(passwords) {
		t.Errorf("Expected %d total results, got %d", len(passwords), totalResults)
	}

	t.Logf("Integration test completed successfully")
	t.Logf("Successful authentications: %d", successCount)
	t.Logf("Total attempts: %d", totalResults)
}

func TestEngineContextCancellation(t *testing.T) {
	// Create engine with faster rate limit
	engine := core.NewEngine(1, 10*time.Millisecond)

	// Create mock module
	mockModule := mock.NewMockModule()
	mockModule.Initialize("192.168.1.1", "admin", nil)
	mockModule.SetSuccessPassword("never-found") // Set password that won't be found

	engine.SetModule(mockModule)

	// Load many passwords
	passwords := []string{"pass1", "pass2", "pass3", "pass4", "pass5"}
	engine.LoadPasswords(passwords)

	// Start the engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Let it run briefly then cancel
	time.Sleep(50 * time.Millisecond)
	engine.Stop()

	// Should have processed some but not all passwords
	resultCount := 0
	for range engine.Results() {
		resultCount++
	}

	if resultCount <= 0 {
		t.Error("Expected some results before cancellation")
	}

	if resultCount >= len(passwords) {
		t.Error("Expected cancellation to stop processing before completing all passwords")
	}

	t.Logf("Cancellation test completed successfully")
	t.Logf("Results before cancellation: %d", resultCount)
}
