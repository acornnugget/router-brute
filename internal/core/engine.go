package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
	zlog "github.com/rs/zerolog/log"
)

// Engine represents the core brute-forcing engine
type Engine struct {
	workers       int
	rateLimit     time.Duration
	passwordQueue *PasswordQueue
	results       chan Result
	errors        chan error
	ctx           context.Context
	cancelFunc    context.CancelFunc
	wg            sync.WaitGroup
	startTime     time.Time
	module        interfaces.RouterModule
	closed        bool
	closeMu       sync.Mutex
}

// GetResults returns the channel for receiving authentication results
func (e *Engine) GetResults() chan Result {
	return e.results
}

// GetErrors returns the channel for receiving errors
func (e *Engine) GetErrors() chan error {
	return e.errors
}

// WaitForCompletion blocks until all workers have completed
func (e *Engine) WaitForCompletion() {
	e.wg.Wait()
}

// Result represents the outcome of a single authentication attempt
type Result struct {
	Username     string
	Password     string
	Success      bool
	Error        error
	ModuleName   string
	Target       string
	TimeConsumed time.Duration
	AttemptedAt  time.Time
}

// NewEngine creates a new brute-forcing engine
func NewEngine(workers int, rateLimit time.Duration) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		workers:    workers,
		rateLimit:  rateLimit,
		results:    make(chan Result, workers*2),
		errors:     make(chan error, workers),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// SetModule sets the router module to use
func (e *Engine) SetModule(module interfaces.RouterModule) {
	e.module = module
}

// LoadPasswords loads a list of passwords to try
func (e *Engine) LoadPasswords(passwords []string) {
	e.passwordQueue = NewPasswordQueue(passwords)
}

// Start begins the brute-forcing process (backward compatible)
func (e *Engine) Start() error {
	if e.passwordQueue == nil || e.passwordQueue.Total() == 0 {
		return errors.New("no passwords loaded")
	}

	if e.module == nil {
		return errors.New("no router module set")
	}

	if e.ctx == nil {
		var cancel context.CancelFunc
		e.ctx, cancel = context.WithCancel(context.Background())
		e.cancelFunc = cancel
	}

	e.startTime = time.Now()

	// Start worker pool
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// Auto-close channels when all workers complete
	go func() {
		e.wg.Wait()
		e.closeChannels()
	}()

	return nil
}

// StartWithContext begins the brute-forcing process with explicit context
func (e *Engine) StartWithContext(ctx context.Context) {
	if e.passwordQueue == nil || e.passwordQueue.Total() == 0 {
		zlog.Error().Msg("No passwords loaded")
		return
	}

	if e.module == nil {
		zlog.Error().Msg("No router module set")
		return
	}

	e.ctx = ctx
	e.startTime = time.Now()

	// Start worker pool
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
}

// worker handles individual authentication attempts
func (e *Engine) worker(id int) {
	defer e.wg.Done()

	// Connect the module if not already connected
	if e.module != nil && !e.module.IsConnected() {
		if err := e.module.Connect(e.ctx); err != nil {
			zlog.Error().Err(err).Int("worker_id", id).Msg("Worker failed to connect")
			select {
			case e.errors <- err:
			case <-e.ctx.Done():
			}
			return
		}
	}

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			// Get next password to try
			password := e.passwordQueue.Next()
			if password == "" {
				// No more passwords, exit worker
				return
			}

			// Rate limiting
			time.Sleep(e.rateLimit)

			// Perform authentication
			success := false
			var err error
			var elapsed time.Duration

			if e.module != nil {
				start := time.Now()
				success, err = e.module.Authenticate(e.ctx, password)
				elapsed = time.Since(start)
			} else {
				// Fallback to mock behavior if no module set
				success = false
				err = nil
				elapsed = time.Second
			}

			// Create result
			result := Result{
				Username:     e.module.GetUsername(),
				Password:     password,
				Success:      success,
				Error:        err,
				ModuleName:   e.module.GetProtocolName(),
				Target:       e.module.GetTarget(),
				TimeConsumed: elapsed,
				AttemptedAt:  time.Now(),
			}

			// Send result
			select {
			case e.results <- result:
				// Result sent successfully
			case <-e.ctx.Done():
				return
			}
		}
	}
}

// closeChannels safely closes channels only once
func (e *Engine) closeChannels() {
	e.closeMu.Lock()
	defer e.closeMu.Unlock()
	if !e.closed {
		e.closed = true
		close(e.results)
		close(e.errors)
	}
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	e.wg.Wait()

	// Close the module connection
	if e.module != nil {
		if err := e.module.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing module connection")
		}
	}

	e.closeChannels()
}

// Close cleans up the engine resources
func (e *Engine) Close() {
	// Cancel the context to signal workers to stop
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	e.wg.Wait()
	if e.module != nil {
		if err := e.module.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing module connection")
		}
	}
	e.closeChannels()
}

// Results returns the channel for receiving results
func (e *Engine) Results() <-chan Result {
	return e.results
}

// Errors returns the channel for receiving errors
func (e *Engine) Errors() <-chan error {
	return e.errors
}

// Progress returns the current progress (0.0 to 1.0)
func (e *Engine) Progress() float64 {
	if e.passwordQueue == nil {
		return 0.0
	}
	return e.passwordQueue.Progress()
}

// Stats returns statistics about the current run
func (e *Engine) Stats() map[string]interface{} {
	if e.passwordQueue == nil {
		return nil
	}

	return map[string]interface{}{
		"started_at":      e.startTime,
		"total_passwords": e.passwordQueue.Total(),
		"remaining":       e.passwordQueue.Remaining(),
		"progress":        e.Progress(),
		"workers":         e.workers,
		"rate_limit":      e.rateLimit.String(),
	}
}
