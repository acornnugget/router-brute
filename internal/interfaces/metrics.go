package interfaces

import "time"

// Metrics defines the interface for collecting attack metrics.
// Implementations can write to Prometheus, StatsD, or any other metrics backend.
type Metrics interface {
	// IncAttempts increments the number of authentication attempts.
	IncAttempts(target, protocol string)

	// IncSuccess increments the number of successful authentications.
	IncSuccess(target, protocol string)

	// IncFailure increments the number of failed authentications.
	IncFailure(target, protocol string)

	// IncError increments the number of errors.
	IncError(target, protocol string)

	// ObserveLatency records the latency of an authentication attempt.
	ObserveLatency(target, protocol string, duration time.Duration)
}

// NoopMetrics is a no-op implementation of Metrics.
// Use this when metrics collection is not needed.
type NoopMetrics struct{}

func (n *NoopMetrics) IncAttempts(target, protocol string)                            {}
func (n *NoopMetrics) IncSuccess(target, protocol string)                             {}
func (n *NoopMetrics) IncFailure(target, protocol string)                             {}
func (n *NoopMetrics) IncError(target, protocol string)                               {}
func (n *NoopMetrics) ObserveLatency(target, protocol string, duration time.Duration) {}

// AttackStats holds statistics from an attack.
type AttackStats struct {
	TotalAttempts int
	SuccessCount  int
	ErrorCount    int
	Duration      time.Duration
}
