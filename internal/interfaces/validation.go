package interfaces

import (
	"fmt"
	"os"
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// ValidatePort checks if the port number is valid.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return &ValidationError{
			Field:   "port",
			Message: fmt.Sprintf("invalid port %d (must be 1-65535)", port),
		}
	}
	return nil
}

// ValidateFile checks if a file exists and is readable.
func ValidateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationError{
				Field:   "file",
				Message: fmt.Sprintf("file does not exist: %s", path),
			}
		}
		return &ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("cannot access file: %s", err),
		}
	}
	if info.IsDir() {
		return &ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("path is a directory, not a file: %s", path),
		}
	}
	return nil
}

// ValidateWorkers checks if the number of workers is valid.
func ValidateWorkers(workers int) error {
	if workers < 1 {
		return &ValidationError{
			Field:   "workers",
			Message: fmt.Sprintf("invalid workers count %d (must be >= 1)", workers),
		}
	}
	if workers > 100 {
		return &ValidationError{
			Field:   "workers",
			Message: fmt.Sprintf("workers count %d is too high (max 100)", workers),
		}
	}
	return nil
}

// ValidateTarget checks if a target is specified.
func ValidateTarget(target string) error {
	if target == "" {
		return &ValidationError{
			Field:   "target",
			Message: "target cannot be empty",
		}
	}
	return nil
}
