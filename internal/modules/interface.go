package modules

import (
	"context"
	"errors"
	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/interfaces"
	"time"
)

// BaseRouterModule provides common functionality for router modules
type BaseRouterModule struct {
	target    string
	username  string
	options   map[string]interface{}
	connected bool
}

// NewBaseRouterModule creates a new base router module
func NewBaseRouterModule() *BaseRouterModule {
	return &BaseRouterModule{
		options: make(map[string]interface{}),
	}
}

// Initialize sets up the base module
func (b *BaseRouterModule) Initialize(target, username string, options map[string]interface{}) error {
	b.target = target
	b.username = username

	// Merge options
	if options != nil {
		for k, v := range options {
			b.options[k] = v
		}
	}

	return nil
}

// GetTarget returns the target
func (b *BaseRouterModule) GetTarget() string {
	return b.target
}

// GetUsername returns the username
func (b *BaseRouterModule) GetUsername() string {
	return b.username
}

// GetOption gets an option value
func (b *BaseRouterModule) GetOption(key string) (interface{}, bool) {
	val, ok := b.options[key]
	return val, ok
}

// SetConnected marks the module as connected
func (b *BaseRouterModule) SetConnected(connected bool) {
	b.connected = connected
}

// IsConnected returns whether the module is connected
func (b *BaseRouterModule) IsConnected() bool {
	return b.connected
}

// Connect is not implemented in BaseRouterModule - must be implemented by concrete modules
func (b *BaseRouterModule) Connect(ctx context.Context) error {
	return errors.New("Connect not implemented")
}

// Authenticate is not implemented in BaseRouterModule - must be implemented by concrete modules
func (b *BaseRouterModule) Authenticate(ctx context.Context, password string) (bool, error) {
	return false, errors.New("Authenticate not implemented")
}

// GetProtocolName is not implemented in BaseRouterModule - must be implemented by concrete modules
func (b *BaseRouterModule) GetProtocolName() string {
	return "base"
}

// Close is not implemented in BaseRouterModule - must be implemented by concrete modules
func (b *BaseRouterModule) Close() error {
	b.SetConnected(false)
	return nil
}

// Ensure BaseRouterModule implements the RouterModule interface
var _ interfaces.RouterModule = (*BaseRouterModule)(nil)

// CreateResult creates a standard result from this module
func (b *BaseRouterModule) CreateResult(password string, success bool, err error) core.Result {
	return core.Result{
		Username:    b.username,
		Password:    password,
		Success:     success,
		Error:       err,
		ModuleName:  b.GetProtocolName(),
		Target:      b.target,
		AttemptedAt: time.Now(),
	}
}
