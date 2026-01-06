package v6

import (
	"github.com/nimda/router-brute/internal/interfaces"
)

// MikrotikV6Factory creates Mikrotik RouterOS v6 modules
type MikrotikV6Factory struct{}

// CreateModule creates a new MikrotikV6Module instance
func (f *MikrotikV6Factory) CreateModule() interfaces.RouterModule {
	return NewMikrotikV6Module()
}

// GetProtocolName returns the protocol name
func (f *MikrotikV6Factory) GetProtocolName() string {
	return "mikrotik-v6"
}
