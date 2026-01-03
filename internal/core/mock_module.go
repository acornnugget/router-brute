package core

import (
	"context"
	"errors"
)

// mockModule is a simple mock module for testing that doesn't create import cycles
type mockModule struct {
	target      string
	username    string
	connected   bool
	successPass string
}

func newMockModule(target, username, successPass string) *mockModule {
	return &mockModule{
		target:      target,
		username:    username,
		successPass: successPass,
	}
}

func (m *mockModule) Initialize(target, username string, options map[string]interface{}) error {
	m.target = target
	m.username = username
	return nil
}

func (m *mockModule) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockModule) Authenticate(ctx context.Context, password string) (bool, error) {
	if !m.connected {
		return false, errors.New("not connected")
	}
	return password == m.successPass, nil
}

func (m *mockModule) Close() error {
	m.connected = false
	return nil
}

func (m *mockModule) GetProtocolName() string {
	return "test-mock"
}

func (m *mockModule) GetTarget() string {
	return m.target
}

func (m *mockModule) GetUsername() string {
	return m.username
}

func (m *mockModule) IsConnected() bool {
	return m.connected
}
