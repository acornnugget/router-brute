package interfaces

import "time"

// ModuleConfig holds common configuration for all router modules.
// This provides a type-safe way to pass configuration instead of map[string]interface{}.
type ModuleConfig struct {
	Port    int
	Timeout time.Duration
	// Extra holds module-specific options that don't fit the common config
	Extra map[string]interface{}
}

// NewModuleConfig creates a new ModuleConfig with sensible defaults.
func NewModuleConfig() *ModuleConfig {
	return &ModuleConfig{
		Port:    0, // Must be set by caller
		Timeout: 10 * time.Second,
		Extra:   make(map[string]interface{}),
	}
}

// ToOptions converts ModuleConfig to the legacy map format for backward compatibility.
func (c *ModuleConfig) ToOptions() map[string]interface{} {
	opts := map[string]interface{}{
		"port":    c.Port,
		"timeout": c.Timeout,
	}
	for k, v := range c.Extra {
		opts[k] = v
	}
	return opts
}

// Validate checks if the configuration is valid.
func (c *ModuleConfig) Validate() error {
	if err := ValidatePort(c.Port); err != nil {
		return err
	}
	if c.Timeout <= 0 {
		return &ValidationError{Field: "timeout", Message: "timeout must be positive"}
	}
	return nil
}
