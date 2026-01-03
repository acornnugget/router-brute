package utils

import (
	"fmt"
)

// AuthenticationError represents authentication-specific errors
type AuthenticationError struct {
	Module     string
	Target     string
	Underlying error
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication error (%s@%s): %v", e.Module, e.Target, e.Underlying)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Underlying
}

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(module, target string, err error) *AuthenticationError {
	return &AuthenticationError{
		Module:     module,
		Target:     target,
		Underlying: err,
	}
}

// ConnectionError represents connection-related errors
type ConnectionError struct {
	Target     string
	Underlying error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error (%s): %v", e.Target, e.Underlying)
}

func (e *ConnectionError) Unwrap() error {
	return e.Underlying
}

// NewConnectionError creates a new connection error
func NewConnectionError(target string, err error) *ConnectionError {
	return &ConnectionError{
		Target:     target,
		Underlying: err,
	}
}
