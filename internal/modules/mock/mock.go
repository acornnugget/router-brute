package mock

import (
	"github.com/nimda/router-brute/internal/testutil"
)

// MockModule is a mock router module for testing
// This is now a type alias to the testutil.MockModule for consistency
type MockModule = testutil.MockModule

// NewMockModule creates a new mock module
func NewMockModule() *MockModule {
	return testutil.NewMockModule()
}

// NewMockModuleWithPassword creates a new mock module with custom success password
func NewMockModuleWithPassword(password string) *MockModule {
	return testutil.NewMockModuleWithPassword(password)
}

// MockFactory is a mock factory for creating mock modules
type MockFactory = testutil.MockModuleFactory
