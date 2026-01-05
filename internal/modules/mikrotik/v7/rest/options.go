package rest

import (
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
)

// Option is a functional option for configuring MikrotikV7RestModule.
type Option func(*MikrotikV7RestModule)

// WithPort sets the port number.
func WithPort(port int) Option {
	return func(m *MikrotikV7RestModule) {
		m.port = port
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(m *MikrotikV7RestModule) {
		m.timeout = timeout
	}
}

// WithHTTPS enables HTTPS mode.
func WithHTTPS(enable bool) Option {
	return func(m *MikrotikV7RestModule) {
		m.useHTTPS = enable
		if enable && m.port == 80 {
			m.port = 443
		}
	}
}

// WithConfig applies a ModuleConfig to the module.
func WithConfig(cfg *interfaces.ModuleConfig) Option {
	return func(m *MikrotikV7RestModule) {
		if cfg.Port > 0 {
			m.port = cfg.Port
		}
		if cfg.Timeout > 0 {
			m.timeout = cfg.Timeout
		}
		if https, ok := cfg.Extra["https"].(bool); ok {
			m.useHTTPS = https
		}
	}
}

// NewMikrotikV7RestModuleWithOptions creates a new module with functional options.
func NewMikrotikV7RestModuleWithOptions(opts ...Option) *MikrotikV7RestModule {
	m := NewMikrotikV7RestModule()
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// DefaultPort returns the default port for MikroTik v7 REST API.
const DefaultPort = 80

func init() {
	// Register this protocol with the default registry
	_ = interfaces.Register(interfaces.ProtocolInfo{
		Name:         "mikrotik-v7-rest",
		Description:  "MikroTik RouterOS v7 REST API",
		DefaultPort:  DefaultPort,
		Factory:      func() interfaces.RouterModule { return NewMikrotikV7RestModule() },
		MultiFactory: &MikrotikV7RestFactory{},
	})
}
