package interfaces

import "context"

// RouterModule defines the interface that all router modules must implement
type RouterModule interface {
	// Initialize sets up the module with target information
	Initialize(target, username string, options map[string]interface{}) error

	// Connect establishes a connection to the router
	Connect(ctx context.Context) error

	// Authenticate attempts to authenticate with the given password
	Authenticate(ctx context.Context, password string) (bool, error)

	// Close cleans up any resources
	Close() error

	// GetProtocolName returns the name of the protocol/module
	GetProtocolName() string

	// GetTarget returns the target being attacked
	GetTarget() string

	// GetUsername returns the username being used
	GetUsername() string

	// IsConnected returns whether the module is currently connected
	IsConnected() bool
}
