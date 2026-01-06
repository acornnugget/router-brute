package v7

import (
	"github.com/nimda/router-brute/internal/interfaces"
)

// MikrotikV7Factory creates Mikrotik RouterOS v7 modules
type MikrotikV7Factory struct{}

// CreateModule creates a new MikrotikV7Module instance
func (f *MikrotikV7Factory) CreateModule() interfaces.RouterModule {
	return NewMikrotikV7Module()
}

// GetProtocolName returns the protocol name
func (f *MikrotikV7Factory) GetProtocolName() string {
	return "mikrotik-v7"
}
