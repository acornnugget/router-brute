package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockModuleFactory for testing
type MockModuleFactory struct {
	mock.Mock
}

func (m *MockModuleFactory) CreateModule() interfaces.RouterModule {
	args := m.Called()
	return args.Get(0).(interfaces.RouterModule)
}

func (m *MockModuleFactory) GetProtocolName() string {
	args := m.Called()
	return args.String(0)
}

// MockRouterModule for testing
type MockRouterModule struct {
	mock.Mock
}

func (m *MockRouterModule) Initialize(target, username string, options map[string]interface{}) error {
	args := m.Called(target, username, options)
	return args.Error(0)
}

func (m *MockRouterModule) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRouterModule) Authenticate(ctx context.Context, password string) (bool, error) {
	args := m.Called(ctx, password)
	return args.Bool(0), args.Error(1)
}

func (m *MockRouterModule) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRouterModule) GetProtocolName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockRouterModule) GetTarget() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockRouterModule) GetUsername() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockRouterModule) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestMultiTargetEngine_Basic(t *testing.T) {
	// Create mock module factory
	mockFactory := new(MockModuleFactory)
	mockModule := new(MockRouterModule)

	// Setup mock expectations
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")
	mockModule.On("Initialize", "192.168.1.1", "admin", mock.Anything).Return(nil)
	mockModule.On("Connect", mock.Anything).Return(nil)
	mockModule.On("Authenticate", mock.Anything, "password1").Return(false, nil)
	mockModule.On("Authenticate", mock.Anything, "password2").Return(true, nil)
	mockModule.On("Close").Return(nil)
	mockModule.On("GetProtocolName").Return("mock")
	mockModule.On("GetTarget").Return("192.168.1.1")
	mockModule.On("GetUsername").Return("admin")
	mockModule.On("IsConnected").Return(false)

	// Create engine with 1 worker to simplify debugging
	engine := NewMultiTargetEngine(mockFactory, 1, 1, 100*time.Millisecond)
	targets := []*Target{
		{Username: "admin", IP: "192.168.1.1", Port: 8728},
	}
	passwords := []string{"password1", "password2"}
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engine.Start(ctx)

	// Collect results with timeout
	var results []MultiTargetResult
	resultsCollected := make(chan bool)
	go func() {
		for result := range engine.GetResults() {
			results = append(results, result)
		}
		resultsCollected <- true
	}()

	select {
	case <-resultsCollected:
		// Results collected successfully
	case <-time.After(6 * time.Second):
		t.Fatal("Test timed out waiting for results")
	}

	// Verify results
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "password2", results[0].SuccessPassword)
	assert.Equal(t, 2, results[0].Attempts)

	// Verify no errors with timeout
	var errors []MultiTargetError
	errorsCollected := make(chan bool)
	go func() {
		for err := range engine.GetErrors() {
			errors = append(errors, err)
		}
		errorsCollected <- true
	}()

	select {
	case <-errorsCollected:
		// Errors collected successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out waiting for errors")
	}

	assert.Len(t, errors, 0)

	mockFactory.AssertExpectations(t)
	mockModule.AssertExpectations(t)
}

func TestMultiTargetEngine_ConcurrentTargets(t *testing.T) {
	// Create mock module factory
	mockFactory := new(MockModuleFactory)
	mockModule1 := new(MockRouterModule)
	mockModule2 := new(MockRouterModule)

	// Setup mock expectations for first target
	mockFactory.On("CreateModule").Return(mockModule1).Once()
	mockFactory.On("CreateModule").Return(mockModule2).Once()
	mockFactory.On("GetProtocolName").Return("mock")

	mockModule1.On("Initialize", "192.168.1.1", "admin", mock.Anything).Return(nil)
	mockModule1.On("Connect", mock.Anything).Return(nil)
	mockModule1.On("Authenticate", mock.Anything, "password1").Return(false, nil)
	mockModule1.On("Authenticate", mock.Anything, "password2").Return(false, nil)
	mockModule1.On("Close").Return(nil)
	mockModule1.On("GetProtocolName").Return("mock")
	mockModule1.On("GetTarget").Return("192.168.1.1")
	mockModule1.On("GetUsername").Return("admin")
	mockModule1.On("IsConnected").Return(false) // Allow unlimited calls

	// Setup mock expectations for second target
	mockModule2.On("Initialize", "192.168.1.2", "admin", mock.Anything).Return(nil)
	mockModule2.On("Connect", mock.Anything).Return(nil)
	mockModule2.On("Authenticate", mock.Anything, "password1").Return(false, nil)
	mockModule2.On("Authenticate", mock.Anything, "password2").Return(true, nil)
	mockModule2.On("Close").Return(nil)
	mockModule2.On("GetProtocolName").Return("mock")
	mockModule2.On("GetTarget").Return("192.168.1.2")
	mockModule2.On("GetUsername").Return("admin")
	mockModule2.On("IsConnected").Return(false) // Allow unlimited calls

	// Create engine with 2 concurrent targets
	engine := NewMultiTargetEngine(mockFactory, 2, 2, 100*time.Millisecond)
	targets := []*Target{
		{Username: "admin", IP: "192.168.1.1", Port: 8728},
		{Username: "admin", IP: "192.168.1.2", Port: 8728},
	}
	passwords := []string{"password1", "password2"}
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engine.Start(ctx)

	// Collect results
	var results []MultiTargetResult
	for result := range engine.GetResults() {
		results = append(results, result)
	}

	// Verify results (order may vary due to concurrency)
	assert.Len(t, results, 2)
	
	// Find the results for each target
	var result1, result2 *MultiTargetResult
	for _, result := range results {
		if result.Target.IP == "192.168.1.1" {
			result1 = &result
		} else if result.Target.IP == "192.168.1.2" {
			result2 = &result
		}
	}
	
	// First target should fail
	assert.NotNil(t, result1)
	assert.False(t, result1.Success)
	assert.Equal(t, 2, result1.Attempts)
	
	// Second target should succeed
	assert.NotNil(t, result2)
	assert.True(t, result2.Success)
	assert.Equal(t, "password2", result2.SuccessPassword)
	assert.Equal(t, 2, result2.Attempts)

	// Verify no errors
	var errors []MultiTargetError
	for err := range engine.GetErrors() {
		errors = append(errors, err)
	}
	assert.Len(t, errors, 0)

	mockFactory.AssertExpectations(t)
	mockModule1.AssertExpectations(t)
	mockModule2.AssertExpectations(t)
}

func TestMultiTargetEngine_Cancellation(t *testing.T) {
	// Create mock module factory
	mockFactory := new(MockModuleFactory)
	mockModule := new(MockRouterModule)

	// Setup mock expectations
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")
	mockModule.On("Initialize", "192.168.1.1", "admin", mock.Anything).Return(nil)
	mockModule.On("Connect", mock.Anything).Return(nil)
	// Mock Authenticate to handle any calls that might happen before cancellation
	mockModule.On("Authenticate", mock.Anything, mock.Anything).Return(false, nil)
	mockModule.On("Close").Return(nil)
	mockModule.On("GetProtocolName").Return("mock")
	mockModule.On("GetTarget").Return("192.168.1.1")
	mockModule.On("GetUsername").Return("admin")
	mockModule.On("IsConnected").Return(false)

	// Create engine
	engine := NewMultiTargetEngine(mockFactory, 2, 1, 100*time.Millisecond)
	targets := []*Target{
		{Username: "admin", IP: "192.168.1.1", Port: 8728},
	}
	passwords := []string{"password1", "password2"}
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack with very short timeout (shorter than rate limit)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	engine.Start(ctx)

	// Collect results (should have one result with 0 attempts due to cancellation)
	var results []MultiTargetResult
	for result := range engine.GetResults() {
		results = append(results, result)
	}

	// Should have one result with 0-2 attempts due to race condition in cancellation timing
	// The exact number depends on when the context cancels relative to worker processing
	assert.Len(t, results, 1)
	assert.True(t, results[0].Attempts >= 0 && results[0].Attempts <= 2, "Expected 0-2 attempts, got %d", results[0].Attempts)
	assert.False(t, results[0].Success)
	assert.Equal(t, "", results[0].SuccessPassword)

	mockFactory.AssertExpectations(t)
	mockModule.AssertExpectations(t)
}

func TestMultiTargetEngine_ErrorHandling(t *testing.T) {
	errExpected := errors.New("initialization failed")
	
	// Create mock module factory
	mockFactory := new(MockModuleFactory)
	mockModule := new(MockRouterModule)

	// Setup mock expectations - initialization will fail
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")
	mockModule.On("Initialize", "192.168.1.1", "admin", mock.Anything).Return(errExpected)

	// Create engine
	engine := NewMultiTargetEngine(mockFactory, 2, 1, 100*time.Millisecond)
	targets := []*Target{
		{Username: "admin", IP: "192.168.1.1", Port: 8728},
	}
	passwords := []string{"password1", "password2"}
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engine.Start(ctx)

	// Collect results (should be empty due to error)
	var results []MultiTargetResult
	for result := range engine.GetResults() {
		results = append(results, result)
	}
	assert.Len(t, results, 0)

	// Collect errors
	var errors []MultiTargetError
	for err := range engine.GetErrors() {
		errors = append(errors, err)
	}
	assert.Len(t, errors, 1)
	assert.Equal(t, errExpected, errors[0].Error)
	assert.Equal(t, "192.168.1.1", errors[0].Target.IP)

	mockFactory.AssertExpectations(t)
	mockModule.AssertExpectations(t)
}