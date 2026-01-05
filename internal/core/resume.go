package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	zlog "github.com/rs/zerolog/log"
)

// ResumeState represents the complete state of an attack that can be saved and resumed
type ResumeState struct {
	Timestamp    time.Time          `json:"timestamp"`
	Protocol     string             `json:"protocol"`
	Username     string             `json:"username"`
	PasswordFile string             `json:"password_file"`
	TargetFile   string             `json:"target_file,omitempty"` // For multi-target mode
	Workers      int                `json:"workers"`
	RateLimit    string             `json:"rate_limit"`
	Targets      []TargetProgress   `json:"targets"`
	Options      map[string]interface{} `json:"options,omitempty"`
}

// TargetProgress tracks progress for a single target
type TargetProgress struct {
	IP             string `json:"ip"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PasswordsTried int    `json:"passwords_tried"` // Number of passwords attempted
	Completed      bool   `json:"completed"`       // Target finished (success or all passwords tried)
	Success        bool   `json:"success"`         // Found valid credentials
	FoundPassword  string `json:"found_password,omitempty"` // The successful password (if any)
}

// SaveResumeState saves the current attack state to a timestamped file
func SaveResumeState(state *ResumeState, directory string) (string, error) {
	// Ensure directory exists
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("failed to create resume directory: %w", err)
	}

	// Generate timestamped filename
	filename := fmt.Sprintf("resume_%s.json", time.Now().Format("20060102_150405"))
	filepath := filepath.Join(directory, filename)

	// Update timestamp
	state.Timestamp = time.Now()

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal resume state: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write resume file: %w", err)
	}

	zlog.Info().Str("file", filepath).Msg("Saved resume state")
	return filepath, nil
}

// LoadResumeState loads attack state from a resume file
func LoadResumeState(filepath string) (*ResumeState, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read resume file: %w", err)
	}

	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse resume file: %w", err)
	}

	zlog.Info().
		Str("file", filepath).
		Time("original_timestamp", state.Timestamp).
		Int("targets", len(state.Targets)).
		Msg("Loaded resume state")

	return &state, nil
}

// GetProgress returns the overall progress statistics
func (rs *ResumeState) GetProgress() (completed, total, successful int) {
	total = len(rs.Targets)
	for _, target := range rs.Targets {
		if target.Completed {
			completed++
			if target.Success {
				successful++
			}
		}
	}
	return completed, total, successful
}

// GetRemainingTargets returns targets that haven't been completed yet
func (rs *ResumeState) GetRemainingTargets() []TargetProgress {
	var remaining []TargetProgress
	for _, target := range rs.Targets {
		if !target.Completed {
			remaining = append(remaining, target)
		}
	}
	return remaining
}

// UpdateTargetProgress updates the progress for a specific target
func (rs *ResumeState) UpdateTargetProgress(ip string, port int, passwordsTried int, completed bool, success bool, foundPassword string) {
	for i := range rs.Targets {
		if rs.Targets[i].IP == ip && rs.Targets[i].Port == port {
			rs.Targets[i].PasswordsTried = passwordsTried
			rs.Targets[i].Completed = completed
			rs.Targets[i].Success = success
			if success {
				rs.Targets[i].FoundPassword = foundPassword
			}
			return
		}
	}
}

// AddTarget adds a new target to the resume state
func (rs *ResumeState) AddTarget(target TargetProgress) {
	rs.Targets = append(rs.Targets, target)
}

// GetTargetProgress returns the progress for a specific target
func (rs *ResumeState) GetTargetProgress(ip string, port int) *TargetProgress {
	for i := range rs.Targets {
		if rs.Targets[i].IP == ip && rs.Targets[i].Port == port {
			return &rs.Targets[i]
		}
	}
	return nil
}

// PrintSummary prints a human-readable summary of the resume state
func (rs *ResumeState) PrintSummary() {
	completed, total, successful := rs.GetProgress()

	fmt.Printf("\n=== Resume State Summary ===\n")
	fmt.Printf("Timestamp:     %s\n", rs.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Protocol:      %s\n", rs.Protocol)
	fmt.Printf("Username:      %s\n", rs.Username)
	fmt.Printf("Password File: %s\n", rs.PasswordFile)
	if rs.TargetFile != "" {
		fmt.Printf("Target File:   %s\n", rs.TargetFile)
	}
	fmt.Printf("Workers:       %d\n", rs.Workers)
	fmt.Printf("Rate Limit:    %s\n", rs.RateLimit)
	fmt.Printf("\nProgress:      %d/%d targets completed (%d successful)\n", completed, total, successful)

	if successful > 0 {
		fmt.Printf("\nSuccessful credentials:\n")
		for _, target := range rs.Targets {
			if target.Success {
				fmt.Printf("  %s:%d - %s:%s\n", target.IP, target.Port, target.Username, target.FoundPassword)
			}
		}
	}

	remaining := rs.GetRemainingTargets()
	if len(remaining) > 0 {
		fmt.Printf("\nRemaining targets: %d\n", len(remaining))
		for i, target := range remaining {
			if i < 5 { // Show first 5
				fmt.Printf("  %s:%d (%d passwords tried)\n", target.IP, target.Port, target.PasswordsTried)
			}
		}
		if len(remaining) > 5 {
			fmt.Printf("  ... and %d more\n", len(remaining)-5)
		}
	}
	fmt.Printf("=============================\n\n")
}
