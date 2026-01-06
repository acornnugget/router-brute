package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResumeStateSaveLoad(t *testing.T) {
	// Create temporary directory for resume files
	tmpDir, err := os.MkdirTemp("", "resume-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a resume state
	state := &ResumeState{
		Protocol:     "mikrotik-v6",
		Username:     "admin",
		PasswordFile: "/path/to/passwords.txt",
		TargetFile:   "/path/to/targets.txt",
		Workers:      5,
		RateLimit:    "100ms",
		Targets: []TargetProgress{
			{
				IP:             "192.168.1.1",
				Port:           8728,
				Username:       "admin",
				PasswordsTried: 150,
				Completed:      false,
				Success:        false,
			},
			{
				IP:             "192.168.1.2",
				Port:           8728,
				Username:       "admin",
				PasswordsTried: 300,
				Completed:      true,
				Success:        true,
				FoundPassword:  "admin123",
			},
		},
	}

	// Save the state
	filepath, err := SaveResumeState(state, tmpDir)
	if err != nil {
		t.Fatalf("Failed to save resume state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Fatalf("Resume file was not created: %s", filepath)
	}

	// Load the state
	loadedState, err := LoadResumeState(filepath)
	if err != nil {
		t.Fatalf("Failed to load resume state: %v", err)
	}

	// Verify loaded state matches original
	if loadedState.Protocol != state.Protocol {
		t.Errorf("Protocol mismatch: got %s, want %s", loadedState.Protocol, state.Protocol)
	}

	if loadedState.Username != state.Username {
		t.Errorf("Username mismatch: got %s, want %s", loadedState.Username, state.Username)
	}

	if len(loadedState.Targets) != len(state.Targets) {
		t.Errorf("Target count mismatch: got %d, want %d", len(loadedState.Targets), len(state.Targets))
	}

	// Verify first target
	if loadedState.Targets[0].IP != "192.168.1.1" {
		t.Errorf("Target 1 IP mismatch: got %s, want 192.168.1.1", loadedState.Targets[0].IP)
	}

	if loadedState.Targets[0].PasswordsTried != 150 {
		t.Errorf("Target 1 passwords tried mismatch: got %d, want 150", loadedState.Targets[0].PasswordsTried)
	}

	// Verify second target
	if loadedState.Targets[1].Success != true {
		t.Errorf("Target 2 success mismatch: got %v, want true", loadedState.Targets[1].Success)
	}

	if loadedState.Targets[1].FoundPassword != "admin123" {
		t.Errorf("Target 2 found password mismatch: got %s, want admin123", loadedState.Targets[1].FoundPassword)
	}
}

func TestResumeStateProgress(t *testing.T) {
	state := &ResumeState{
		Targets: []TargetProgress{
			{IP: "192.168.1.1", Completed: false, Success: false},
			{IP: "192.168.1.2", Completed: true, Success: true},
			{IP: "192.168.1.3", Completed: true, Success: false},
			{IP: "192.168.1.4", Completed: false, Success: false},
		},
	}

	completed, total, successful := state.GetProgress()

	if total != 4 {
		t.Errorf("Total mismatch: got %d, want 4", total)
	}

	if completed != 2 {
		t.Errorf("Completed mismatch: got %d, want 2", completed)
	}

	if successful != 1 {
		t.Errorf("Successful mismatch: got %d, want 1", successful)
	}
}

func TestResumeStateRemainingTargets(t *testing.T) {
	state := &ResumeState{
		Targets: []TargetProgress{
			{IP: "192.168.1.1", Completed: false},
			{IP: "192.168.1.2", Completed: true},
			{IP: "192.168.1.3", Completed: false},
			{IP: "192.168.1.4", Completed: true},
		},
	}

	remaining := state.GetRemainingTargets()

	if len(remaining) != 2 {
		t.Errorf("Remaining targets count mismatch: got %d, want 2", len(remaining))
	}

	if remaining[0].IP != "192.168.1.1" {
		t.Errorf("First remaining target mismatch: got %s, want 192.168.1.1", remaining[0].IP)
	}

	if remaining[1].IP != "192.168.1.3" {
		t.Errorf("Second remaining target mismatch: got %s, want 192.168.1.3", remaining[1].IP)
	}
}

func TestResumeStateUpdateTarget(t *testing.T) {
	state := &ResumeState{
		Targets: []TargetProgress{
			{IP: "192.168.1.1", Port: 8728, PasswordsTried: 0, Completed: false, Success: false},
		},
	}

	// Update progress
	state.UpdateTargetProgress("192.168.1.1", 8728, 100, false, false, "", 10000, false, 0)

	if state.Targets[0].PasswordsTried != 100 {
		t.Errorf("Passwords tried mismatch: got %d, want 100", state.Targets[0].PasswordsTried)
	}

	// Mark as completed with success
	state.UpdateTargetProgress("192.168.1.1", 8728, 150, true, true, "password123", 10000, false, 0)

	if !state.Targets[0].Completed {
		t.Error("Target should be marked as completed")
	}

	if !state.Targets[0].Success {
		t.Error("Target should be marked as successful")
	}

	if state.Targets[0].FoundPassword != "password123" {
		t.Errorf("Found password mismatch: got %s, want password123", state.Targets[0].FoundPassword)
	}
}

func TestProgressTrackerSaveLoad(t *testing.T) {
	// Create temporary directory for resume files
	tmpDir, err := os.MkdirTemp("", "progress-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create resume state
	state := &ResumeState{
		Protocol:     "mikrotik-v6",
		Username:     "admin",
		PasswordFile: "/path/to/passwords.txt",
		Workers:      5,
		RateLimit:    "100ms",
		Targets: []TargetProgress{
			{IP: "192.168.1.1", Port: 8728, Username: "admin"},
		},
	}

	// Create progress tracker with short interval for testing
	tracker := NewProgressTracker(state, tmpDir, 100*time.Millisecond, true)

	// Start the progress tracker to enable async update processing
	ctx := context.Background()
	tracker.Start(ctx)
	defer tracker.Stop()

	// Update progress
	tracker.UpdateTargetProgress("192.168.1.1", 8728, 50, false, false, "", 10000, false, 0)

	// Wait briefly for async update to be processed
	time.Sleep(50 * time.Millisecond)

	// Save immediately
	err = tracker.SaveNow()
	if err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	// Find the saved file
	files, err := filepath.Glob(filepath.Join(tmpDir, "resume_*.json"))
	if err != nil {
		t.Fatalf("Failed to list resume files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No resume files found")
	}

	// Load the state
	loadedState, err := LoadResumeState(files[0])
	if err != nil {
		t.Fatalf("Failed to load resume state: %v", err)
	}

	// Verify the loaded state
	if loadedState.Targets[0].PasswordsTried != 50 {
		t.Errorf("Passwords tried mismatch: got %d, want 50", loadedState.Targets[0].PasswordsTried)
	}
}
