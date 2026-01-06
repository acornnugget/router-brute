package rest

import (
	"github.com/nimda/router-brute/internal/interfaces"
)

// MikrotikV7RestFactory creates Mikrotik RouterOS v7 REST API modules
type MikrotikV7RestFactory struct{}

// CreateModule creates a new MikrotikV7RestModule instance
func (f *MikrotikV7RestFactory) CreateModule() interfaces.RouterModule {
	return NewMikrotikV7RestModule()
}

// GetProtocolName returns the protocol name
func (f *MikrotikV7RestFactory) GetProtocolName() string {
	return "mikrotik-v7-rest"
}
