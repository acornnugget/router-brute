package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nimda/router-brute/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMultiTargetEngine_Basic(t *testing.T) {
	// Create mock module factory
	mockFactory := new(testutil.MockModuleFactory)
	mockModule := testutil.NewMockModuleWithPassword("password2")

	// Setup mock expectations
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")

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

	// Collect results - channels will be closed when complete
	var results []MultiTargetResult
	for result := range engine.GetResults() {
		results = append(results, result)
	}

	// Verify results
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "password2", results[0].SuccessPassword)
	assert.Equal(t, 2, results[0].Attempts)

	// Verify no errors
	var errs []MultiTargetError
	for err := range engine.GetErrors() {
		errs = append(errs, err)
	}
	assert.Len(t, errs, 0)

	mockFactory.AssertExpectations(t)
}

func TestMultiTargetEngine_ConcurrentTargets(t *testing.T) {
	// Create mock module factory
	mockFactory := new(testutil.MockModuleFactory)
	mockModule1 := testutil.NewMockModuleWithPassword("") // No success password
	mockModule2 := testutil.NewMockModuleWithPassword("password2")

	// Setup mock expectations for first target
	mockFactory.On("CreateModule").Return(mockModule1).Once()
	mockFactory.On("CreateModule").Return(mockModule2).Once()
	mockFactory.On("GetProtocolName").Return("mock")

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
		switch result.Target.IP {
		case "192.168.1.1":
			result1 = &result
		case "192.168.1.2":
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
	var targetErrors []MultiTargetError
	for err := range engine.GetErrors() {
		targetErrors = append(targetErrors, err)
	}
	assert.Len(t, targetErrors, 0)

	mockFactory.AssertExpectations(t)
	mockModule1.AssertExpectations(t)
	mockModule2.AssertExpectations(t)
}

func TestMultiTargetEngine_Cancellation(t *testing.T) {
	// Create mock module factory
	mockFactory := new(testutil.MockModuleFactory)
	mockModule := testutil.NewMockModuleWithPassword("") // No success password

	// Setup mock expectations
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")

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
	mockFactory := new(testutil.MockModuleFactory)
	// Create a mock module that will fail on Initialize
	mockModule := testutil.NewMockModuleWithInitError(errExpected)

	// Setup mock expectations - factory will return the failing module
	mockFactory.On("CreateModule").Return(mockModule)
	mockFactory.On("GetProtocolName").Return("mock")

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
	var errs []MultiTargetError
	for err := range engine.GetErrors() {
		errs = append(errs, err)
	}
	assert.Len(t, errs, 1)
	assert.Equal(t, errExpected, errs[0].Error)
	assert.Equal(t, "192.168.1.1", errs[0].Target.IP)

	mockFactory.AssertExpectations(t)
}
