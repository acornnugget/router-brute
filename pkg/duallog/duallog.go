package duallog

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

var (
	stderrLogger zerolog.Logger
)

// Setup configures the dual logging system:
// - All logs go to STDOUT (complete log)
// - Progress messages go only to STDERR
// - Success messages go to both STDOUT and STDERR
func Setup(level zerolog.Level) {
	// Configure global logger to write to STDOUT
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zlog.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(level)

	// Create a separate logger for STDERR (progress and success messages)
	stderrLogger = zerolog.New(os.Stderr).With().Timestamp().Logger()
}

// Progress logs a progress message ONLY to STDERR
func Progress() *zerolog.Event {
	return stderrLogger.Info()
}

// Success logs a success message to BOTH STDOUT and STDERR
func Success() *DualEvent {
	return &DualEvent{
		stdout: zlog.Info(),
		stderr: stderrLogger.Info(),
	}
}

// DualEvent represents an event that writes to both STDOUT and STDERR
type DualEvent struct {
	stdout *zerolog.Event
	stderr *zerolog.Event
}

// Str adds a string field to both events
func (d *DualEvent) Str(key, val string) *DualEvent {
	d.stdout.Str(key, val)
	d.stderr.Str(key, val)
	return d
}

// Int adds an int field to both events
func (d *DualEvent) Int(key string, val int) *DualEvent {
	d.stdout.Int(key, val)
	d.stderr.Int(key, val)
	return d
}

// Dur adds a duration field to both events
func (d *DualEvent) Dur(key string, val interface{}) *DualEvent {
	// zerolog expects time.Duration for Dur()
	if duration, ok := val.(interface{ String() string }); ok {
		d.stdout.Str(key, duration.String())
		d.stderr.Str(key, duration.String())
	}
	return d
}

// Msg sends the message to both STDOUT and STDERR
func (d *DualEvent) Msg(msg string) {
	d.stdout.Msg(msg)
	d.stderr.Msg(msg)
}

// Msgf sends a formatted message to both STDOUT and STDERR
func (d *DualEvent) Msgf(format string, v ...interface{}) {
	d.stdout.Msgf(format, v...)
	d.stderr.Msgf(format, v...)
}

// GetStderrWriter returns a writer for STDERR (for non-structured logs)
func GetStderrWriter() io.Writer {
	return os.Stderr
}
