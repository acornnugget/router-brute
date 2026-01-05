package v7

import (
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
)

// Option is a functional option for configuring MikrotikV7Module.
type Option func(*MikrotikV7Module)

// WithPort sets the port number.
func WithPort(port int) Option {
	return func(m *MikrotikV7Module) {
		m.port = port
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(m *MikrotikV7Module) {
		m.timeout = timeout
	}
}

// WithConfig applies a ModuleConfig to the module.
func WithConfig(cfg *interfaces.ModuleConfig) Option {
	return func(m *MikrotikV7Module) {
		if cfg.Port > 0 {
			m.port = cfg.Port
		}
		if cfg.Timeout > 0 {
			m.timeout = cfg.Timeout
		}
	}
}

// NewMikrotikV7ModuleWithOptions creates a new module with functional options.
func NewMikrotikV7ModuleWithOptions(opts ...Option) *MikrotikV7Module {
	m := NewMikrotikV7Module()
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// DefaultPort returns the default port for MikroTik v7 WebFig API.
const DefaultPort = 8729

func init() {
	// Register this protocol with the default registry
	_ = interfaces.Register(interfaces.ProtocolInfo{
		Name:         "mikrotik-v7",
		Description:  "MikroTik RouterOS v7 WebFig binary API",
		DefaultPort:  DefaultPort,
		Factory:      func() interfaces.RouterModule { return NewMikrotikV7Module() },
		MultiFactory: &MikrotikV7Factory{},
	})
}
