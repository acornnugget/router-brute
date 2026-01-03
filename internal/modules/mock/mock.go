package mock

import (
	"context"
	"errors"
	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/nimda/router-brute/internal/modules"
)

// MockModule is a mock router module for testing
type MockModule struct {
	*modules.BaseRouterModule
	successPassword string
}

// NewMockModule creates a new mock module
func NewMockModule() *MockModule {
	return &MockModule{
		BaseRouterModule: modules.NewBaseRouterModule(),
		successPassword:  "correct-password",
	}
}

// GetProtocolName returns the protocol name
func (m *MockModule) GetProtocolName() string {
	return "mock"
}

// Connect establishes a mock connection
func (m *MockModule) Connect(ctx context.Context) error {
	m.SetConnected(true)
	return nil
}

// Authenticate attempts mock authentication
func (m *MockModule) Authenticate(ctx context.Context, password string) (bool, error) {
	if !m.IsConnected() {
		return false, errors.New("not connected")
	}

	// Simulate successful authentication if password matches
	if password == m.successPassword {
		return true, nil
	}

	return false, nil
}

// Close cleans up the mock connection
func (m *MockModule) Close() error {
	m.SetConnected(false)
	return nil
}

// SetSuccessPassword sets the password that will succeed
func (m *MockModule) SetSuccessPassword(password string) {
	m.successPassword = password
}

// Ensure MockModule implements the RouterModule interface
var _ interfaces.RouterModule = (*MockModule)(nil)
