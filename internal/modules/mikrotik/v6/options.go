package v6

import (
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
)

// Option is a functional option for configuring MikrotikV6Module.
type Option func(*MikrotikV6Module)

// WithPort sets the port number.
func WithPort(port int) Option {
	return func(m *MikrotikV6Module) {
		m.port = port
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(m *MikrotikV6Module) {
		m.timeout = timeout
	}
}

// WithConfig applies a ModuleConfig to the module.
func WithConfig(cfg *interfaces.ModuleConfig) Option {
	return func(m *MikrotikV6Module) {
		if cfg.Port > 0 {
			m.port = cfg.Port
		}
		if cfg.Timeout > 0 {
			m.timeout = cfg.Timeout
		}
	}
}

// NewMikrotikV6ModuleWithOptions creates a new module with functional options.
func NewMikrotikV6ModuleWithOptions(opts ...Option) *MikrotikV6Module {
	m := NewMikrotikV6Module()
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// DefaultPort returns the default port for MikroTik v6 API.
const DefaultPort = 8728

func init() {
	// Register this protocol with the default registry
	_ = interfaces.Register(interfaces.ProtocolInfo{
		Name:         "mikrotik-v6",
		Description:  "MikroTik RouterOS v6 binary API",
		DefaultPort:  DefaultPort,
		Factory:      func() interfaces.RouterModule { return NewMikrotikV6Module() },
		MultiFactory: &MikrotikV6Factory{},
	})
}
