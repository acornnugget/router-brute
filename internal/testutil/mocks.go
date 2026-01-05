package testutil

import (
	"context"
	"errors"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/stretchr/testify/mock"
)

// MockModule is a comprehensive mock router module for testing
type MockModule struct {
	mock.Mock
	target          string
	username        string
	connected       bool
	successPassword string
	initError       error // Error to return from Initialize
}

// NewMockModule creates a new mock module with default success password
func NewMockModule() *MockModule {
	return &MockModule{
		successPassword: "correct-password",
	}
}

// NewMockModuleWithPassword creates a new mock module with custom success password
func NewMockModuleWithPassword(password string) *MockModule {
	return &MockModule{
		successPassword: password,
	}
}

// NewMockModuleWithInitError creates a mock module that fails on Initialize
func NewMockModuleWithInitError(err error) *MockModule {
	return &MockModule{
		initError: err,
	}
}

// Initialize initializes the mock module
func (m *MockModule) Initialize(target, username string, options map[string]interface{}) error {
	if m.initError != nil {
		return m.initError
	}
	m.target = target
	m.username = username
	return nil
}

// Connect establishes a mock connection
func (m *MockModule) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

// Authenticate attempts mock authentication
func (m *MockModule) Authenticate(ctx context.Context, password string) (bool, error) {
	if !m.connected {
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
	m.connected = false
	return nil
}

// GetProtocolName returns the protocol name
func (m *MockModule) GetProtocolName() string {
	return "mock"
}

// GetTarget returns the target
func (m *MockModule) GetTarget() string {
	return m.target
}

// GetUsername returns the username
func (m *MockModule) GetUsername() string {
	return m.username
}

// IsConnected returns connection status
func (m *MockModule) IsConnected() bool {
	return m.connected
}

// SetSuccessPassword sets the password that will succeed
func (m *MockModule) SetSuccessPassword(password string) {
	m.successPassword = password
}

// MockModuleFactory is a mock factory for creating mock modules
type MockModuleFactory struct {
	mock.Mock
}

// CreateModule creates a new mock module
func (f *MockModuleFactory) CreateModule() interfaces.RouterModule {
	args := f.Called()
	if len(args) == 0 {
		return NewMockModule()
	}
	return args.Get(0).(interfaces.RouterModule)
}

// GetProtocolName returns the protocol name
func (f *MockModuleFactory) GetProtocolName() string {
	args := f.Called()
	if len(args) == 0 {
		return "mock"
	}
	return args.String(0)
}

// SimpleMockModule is a lightweight mock without testify/mock dependency for core package
type SimpleMockModule struct {
	target      string
	username    string
	connected   bool
	successPass string
}

// NewSimpleMockModule creates a new simple mock module
func NewSimpleMockModule(target, username, successPass string) *SimpleMockModule {
	return &SimpleMockModule{
		target:      target,
		username:    username,
		successPass: successPass,
	}
}

// Initialize initializes the simple mock module
func (m *SimpleMockModule) Initialize(target, username string, options map[string]interface{}) error {
	m.target = target
	m.username = username
	return nil
}

// Connect establishes a mock connection
func (m *SimpleMockModule) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

// Authenticate attempts mock authentication
func (m *SimpleMockModule) Authenticate(ctx context.Context, password string) (bool, error) {
	if !m.connected {
		return false, errors.New("not connected")
	}
	return password == m.successPass, nil
}

// Close cleans up the mock connection
func (m *SimpleMockModule) Close() error {
	m.connected = false
	return nil
}

// GetProtocolName returns the protocol name
func (m *SimpleMockModule) GetProtocolName() string {
	return "test-mock"
}

// GetTarget returns the target
func (m *SimpleMockModule) GetTarget() string {
	return m.target
}

// GetUsername returns the username
func (m *SimpleMockModule) GetUsername() string {
	return m.username
}

// IsConnected returns connection status
func (m *SimpleMockModule) IsConnected() bool {
	return m.connected
}

// Ensure interfaces are implemented
var _ interfaces.RouterModule = (*MockModule)(nil)
var _ interfaces.ModuleFactory = (*MockModuleFactory)(nil)
var _ interfaces.RouterModule = (*SimpleMockModule)(nil)
