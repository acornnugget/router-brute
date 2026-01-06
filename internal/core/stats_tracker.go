package core

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// StatsTracker tracks attack statistics and outputs progress reports
type StatsTracker struct {
	mu                  sync.RWMutex
	startTime           time.Time
	totalPasswords      int
	totalTargets        int
	passwordsTried      int
	targetsCompleted    int
	targetsAlive        int // Non-dead targets
	outputInterval      time.Duration
	stopChan            chan struct{}
	wg                  sync.WaitGroup
	progressTracker     *ProgressTracker
	lastPasswordsTried  int
	lastReportTime      time.Time
	currentSpeed        float64 // passwords per second
}

// NewStatsTracker creates a new statistics tracker
func NewStatsTracker(totalPasswords, totalTargets int, outputInterval time.Duration, progressTracker *ProgressTracker) *StatsTracker {
	return &StatsTracker{
		startTime:       time.Now(),
		totalPasswords:  totalPasswords,
		totalTargets:    totalTargets,
		targetsAlive:    totalTargets,
		outputInterval:  outputInterval,
		stopChan:        make(chan struct{}),
		progressTracker: progressTracker,
		lastReportTime:  time.Now(),
	}
}

// Start begins the statistics output loop
func (st *StatsTracker) Start(ctx context.Context) {
	if st.outputInterval == 0 {
		return
	}

	st.wg.Add(1)
	go st.outputLoop(ctx)
}

// Stop stops the statistics output loop
func (st *StatsTracker) Stop() {
	close(st.stopChan)
	st.wg.Wait()
}

// UpdateProgress updates the current progress
func (st *StatsTracker) UpdateProgress(passwordsTried, targetsCompleted, targetsAlive int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.passwordsTried = passwordsTried
	st.targetsCompleted = targetsCompleted
	st.targetsAlive = targetsAlive
}

// outputLoop periodically outputs progress statistics to STDERR
func (st *StatsTracker) outputLoop(ctx context.Context) {
	defer st.wg.Done()

	ticker := time.NewTicker(st.outputInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			st.outputProgress()

		case <-st.stopChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// outputProgress calculates and outputs progress statistics
func (st *StatsTracker) outputProgress() {
	st.mu.RLock()
	elapsed := time.Since(st.startTime)
	passwordsTried := st.passwordsTried
	targetsCompleted := st.targetsCompleted
	targetsAlive := st.targetsAlive
	totalPasswords := st.totalPasswords
	totalTargets := st.totalTargets
	st.mu.RUnlock()

	// Calculate speed (passwords per minute)
	var speedPerMinute float64
	if elapsed.Seconds() > 0 {
		speedPerSecond := float64(passwordsTried) / elapsed.Seconds()
		speedPerMinute = speedPerSecond * 60
		st.mu.Lock()
		st.currentSpeed = speedPerSecond
		st.mu.Unlock()
	}

	// Calculate remaining work
	remainingTargets := targetsAlive - targetsCompleted
	if remainingTargets < 0 {
		remainingTargets = 0
	}

	// Calculate total estimated time (if we had all targets alive)
	var totalEstimatedTime time.Duration
	if st.currentSpeed > 0 {
		totalAttempts := totalTargets * totalPasswords
		totalEstimatedTime = time.Duration(float64(totalAttempts)/st.currentSpeed) * time.Second
	}

	// Calculate estimated time left
	var timeLeft time.Duration
	if st.currentSpeed > 0 && remainingTargets > 0 {
		// For remaining targets, assume average of half the passwords need to be tried
		// (unless we know better from progress tracker)
		remainingAttempts := remainingTargets * totalPasswords

		// If we have progress tracker, get more accurate remaining count
		if st.progressTracker != nil {
			state := st.progressTracker.GetState()
			if state != nil {
				remainingAttempts = 0
				for _, target := range state.Targets {
					if !target.Completed && !target.Dead {
						remainingPasswords := totalPasswords - target.PasswordsTried
						if remainingPasswords < 0 {
							remainingPasswords = 0
						}
						remainingAttempts += remainingPasswords
					}
				}
			}
		}

		timeLeft = time.Duration(float64(remainingAttempts)/st.currentSpeed) * time.Second
	}

	// Output to STDERR
	fmt.Fprintf(os.Stderr, "\n=== Progress Report ===\n")
	fmt.Fprintf(os.Stderr, "Speed:                %.1f passwords/minute (%.2f passwords/second)\n", speedPerMinute, st.currentSpeed)
	fmt.Fprintf(os.Stderr, "Targets:              %d/%d completed (%d alive, %d dead)\n",
		targetsCompleted, totalTargets, targetsAlive, totalTargets-targetsAlive)
	fmt.Fprintf(os.Stderr, "Passwords tried:      %d\n", passwordsTried)
	fmt.Fprintf(os.Stderr, "Elapsed:              %s\n", formatDuration(elapsed))

	if totalEstimatedTime > 0 {
		fmt.Fprintf(os.Stderr, "Total estimated:      %s (all targets, all passwords)\n", formatDuration(totalEstimatedTime))
	}

	if timeLeft > 0 {
		fmt.Fprintf(os.Stderr, "Estimated time left:  %s (remaining targets)\n", formatDuration(timeLeft))
	}

	fmt.Fprintf(os.Stderr, "======================\n\n")
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// GetCurrentSpeed returns the current speed in passwords per second
func (st *StatsTracker) GetCurrentSpeed() float64 {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.currentSpeed
}
