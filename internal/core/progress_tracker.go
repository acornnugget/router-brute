package core

import (
	"context"
	"sync"
	"time"

	zlog "github.com/rs/zerolog/log"
)

// ProgressTracker tracks attack progress and periodically saves state
type ProgressTracker struct {
	mu              sync.RWMutex
	resumeState     *ResumeState
	saveInterval    time.Duration
	saveDirectory   string
	autoSave        bool
	stopChan        chan struct{}
	wg              sync.WaitGroup
	lastSaveTime    time.Time
	changesDetected bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(resumeState *ResumeState, saveDirectory string, saveInterval time.Duration, autoSave bool) *ProgressTracker {
	return &ProgressTracker{
		resumeState:   resumeState,
		saveInterval:  saveInterval,
		saveDirectory: saveDirectory,
		autoSave:      autoSave,
		stopChan:      make(chan struct{}),
		lastSaveTime:  time.Now(),
	}
}

// Start begins the auto-save goroutine if enabled
func (pt *ProgressTracker) Start(ctx context.Context) {
	if !pt.autoSave || pt.saveInterval == 0 {
		return
	}

	pt.wg.Add(1)
	go pt.autoSaveLoop(ctx)
	zlog.Info().
		Dur("interval", pt.saveInterval).
		Str("directory", pt.saveDirectory).
		Msg("Auto-save enabled")
}

// Stop stops the auto-save goroutine and performs a final save
func (pt *ProgressTracker) Stop() {
	if !pt.autoSave {
		return
	}

	close(pt.stopChan)
	pt.wg.Wait()

	// Final save
	if err := pt.SaveNow(); err != nil {
		zlog.Error().Err(err).Msg("Failed to save final progress state")
	}
}

// autoSaveLoop periodically saves the resume state
func (pt *ProgressTracker) autoSaveLoop(ctx context.Context) {
	defer pt.wg.Done()

	ticker := time.NewTicker(pt.saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pt.mu.RLock()
			shouldSave := pt.changesDetected
			pt.mu.RUnlock()

			if shouldSave {
				if err := pt.SaveNow(); err != nil {
					zlog.Error().Err(err).Msg("Failed to save progress during auto-save")
				}
			}

		case <-pt.stopChan:
			zlog.Debug().Msg("Auto-save stopped")
			return

		case <-ctx.Done():
			zlog.Debug().Msg("Auto-save cancelled by context")
			return
		}
	}
}

// SaveNow immediately saves the current state
func (pt *ProgressTracker) SaveNow() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.resumeState == nil {
		return nil
	}

	filepath, err := SaveResumeState(pt.resumeState, pt.saveDirectory)
	if err != nil {
		zlog.Error().Err(err).Msg("Failed to save resume state")
		return err
	}

	pt.lastSaveTime = time.Now()
	pt.changesDetected = false

	completed, total, successful := pt.resumeState.GetProgress()
	zlog.Debug().
		Str("file", filepath).
		Int("completed", completed).
		Int("total", total).
		Int("successful", successful).
		Msg("Progress saved")

	return nil
}

// UpdateTargetProgress updates progress for a target and marks changes detected
func (pt *ProgressTracker) UpdateTargetProgress(ip string, port int, passwordsTried int, completed bool, success bool, foundPassword string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.resumeState == nil {
		return
	}

	pt.resumeState.UpdateTargetProgress(ip, port, passwordsTried, completed, success, foundPassword)
	pt.changesDetected = true

	// Log significant progress changes
	if completed {
		if success {
			zlog.Info().
				Str("target", ip).
				Int("port", port).
				Int("passwords_tried", passwordsTried).
				Str("password", foundPassword).
				Msg("Target completed successfully")
		} else {
			zlog.Info().
				Str("target", ip).
				Int("port", port).
				Int("passwords_tried", passwordsTried).
				Msg("Target completed (no success)")
		}
	}
}

// GetState returns a copy of the current resume state
func (pt *ProgressTracker) GetState() *ResumeState {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	// Return a deep copy to avoid race conditions
	if pt.resumeState == nil {
		return nil
	}

	// Simple copy (good enough for our use case)
	stateCopy := *pt.resumeState
	stateCopy.Targets = make([]TargetProgress, len(pt.resumeState.Targets))
	copy(stateCopy.Targets, pt.resumeState.Targets)

	return &stateCopy
}

// GetTargetProgress returns the progress for a specific target
func (pt *ProgressTracker) GetTargetProgress(ip string, port int) *TargetProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.resumeState == nil {
		return nil
	}

	return pt.resumeState.GetTargetProgress(ip, port)
}

// GetTimeSinceLastSave returns the duration since the last save
func (pt *ProgressTracker) GetTimeSinceLastSave() time.Duration {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return time.Since(pt.lastSaveTime)
}
