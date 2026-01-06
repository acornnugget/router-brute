package core

import (
	"context"
	"sync"
	"time"

	zlog "github.com/rs/zerolog/log"
)

// progressUpdate represents a progress update message
type progressUpdate struct {
	ip                string
	port              int
	passwordsTried    int
	completed         bool
	success           bool
	foundPassword     string
	timeoutMs         int
	dead              bool
	consecutiveErrors int
}

// ProgressTracker tracks attack progress and periodically saves state
type ProgressTracker struct {
	mu              sync.RWMutex
	resumeState     *ResumeState
	saveInterval    time.Duration
	saveDirectory   string
	autoSave        bool
	stopChan        chan struct{}
	updateChan      chan progressUpdate // Non-blocking progress updates
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
		updateChan:    make(chan progressUpdate, 1000), // Buffered channel to prevent blocking
		lastSaveTime:  time.Now(),
	}
}

// Start begins the update processor and auto-save goroutines
func (pt *ProgressTracker) Start(ctx context.Context) {
	// Always start the update processor to handle non-blocking updates
	pt.wg.Add(1)
	go pt.updateProcessor(ctx)

	// Start auto-save if enabled
	if pt.autoSave && pt.saveInterval > 0 {
		pt.wg.Add(1)
		go pt.autoSaveLoop(ctx)
		zlog.Info().
			Dur("interval", pt.saveInterval).
			Str("directory", pt.saveDirectory).
			Msg("Auto-save enabled")
	}
}

// Stop stops all goroutines and performs a final save
func (pt *ProgressTracker) Stop() {
	// Close channels to signal goroutines to stop
	close(pt.stopChan)
	close(pt.updateChan)

	// Wait for all goroutines to finish
	pt.wg.Wait()

	// Final save
	if err := pt.SaveNow(); err != nil {
		zlog.Error().Err(err).Msg("Failed to save final progress state")
	}
}

// updateProcessor processes progress updates from the channel (non-blocking for callers)
func (pt *ProgressTracker) updateProcessor(ctx context.Context) {
	defer pt.wg.Done()

	for {
		select {
		case update, ok := <-pt.updateChan:
			if !ok {
				// Channel closed, stop processing
				zlog.Debug().Msg("Update processor stopped")
				return
			}

			// Process the update with mutex protection
			pt.mu.Lock()
			if pt.resumeState != nil {
				pt.resumeState.UpdateTargetProgress(
					update.ip,
					update.port,
					update.passwordsTried,
					update.completed,
					update.success,
					update.foundPassword,
					update.timeoutMs,
					update.dead,
					update.consecutiveErrors,
				)
				pt.changesDetected = true

				// Log significant progress changes
				if update.completed {
					if update.success {
						zlog.Info().
							Str("target", update.ip).
							Int("port", update.port).
							Int("passwords_tried", update.passwordsTried).
							Str("password", update.foundPassword).
							Msg("Target completed successfully")
					} else {
						zlog.Info().
							Str("target", update.ip).
							Int("port", update.port).
							Int("passwords_tried", update.passwordsTried).
							Msg("Target completed (no success)")
					}
				}
			}
			pt.mu.Unlock()

		case <-ctx.Done():
			zlog.Debug().Msg("Update processor cancelled by context")
			return
		}
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

// UpdateTargetProgress updates progress for a target (non-blocking)
func (pt *ProgressTracker) UpdateTargetProgress(ip string, port int, passwordsTried int, completed bool, success bool, foundPassword string, timeoutMs int, dead bool, consecutiveErrors int) {
	// Send update to channel without blocking
	// Use select with default to make it completely non-blocking
	select {
	case pt.updateChan <- progressUpdate{
		ip:                ip,
		port:              port,
		passwordsTried:    passwordsTried,
		completed:         completed,
		success:           success,
		foundPassword:     foundPassword,
		timeoutMs:         timeoutMs,
		dead:              dead,
		consecutiveErrors: consecutiveErrors,
	}:
		// Update sent successfully
	default:
		// Channel full, log warning but don't block
		zlog.Warn().
			Str("target", ip).
			Int("port", port).
			Msg("Progress update channel full, dropping update")
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
