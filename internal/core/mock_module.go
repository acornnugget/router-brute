package core

import (
	"github.com/nimda/router-brute/internal/testutil"
)

// newMockModule creates a new simple mock module for testing
func newMockModule(target, username, successPass string) *testutil.SimpleMockModule {
	return testutil.NewSimpleMockModule(target, username, successPass)
}
