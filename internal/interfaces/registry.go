package interfaces

import (
	"fmt"
	"sort"
	"sync"
)

// ProtocolInfo holds information about a supported protocol.
type ProtocolInfo struct {
	// Name is the protocol identifier (e.g., "mikrotik-v6").
	Name string

	// Description is a human-readable description of the protocol.
	Description string

	// DefaultPort is the default port for this protocol.
	DefaultPort int

	// Factory creates a new RouterModule instance for this protocol.
	Factory func() RouterModule

	// MultiFactory creates modules for multi-target attacks.
	MultiFactory ModuleFactory
}

// ProtocolRegistry manages registered protocols.
type ProtocolRegistry struct {
	mu        sync.RWMutex
	protocols map[string]ProtocolInfo
}

// NewProtocolRegistry creates a new protocol registry.
func NewProtocolRegistry() *ProtocolRegistry {
	return &ProtocolRegistry{
		protocols: make(map[string]ProtocolInfo),
	}
}

// Register adds a protocol to the registry.
func (r *ProtocolRegistry) Register(info ProtocolInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info.Name == "" {
		return fmt.Errorf("protocol name cannot be empty")
	}
	if info.Factory == nil {
		return fmt.Errorf("protocol factory cannot be nil")
	}
	if _, exists := r.protocols[info.Name]; exists {
		return fmt.Errorf("protocol %q already registered", info.Name)
	}

	r.protocols[info.Name] = info
	return nil
}

// Get returns the protocol info for the given name.
func (r *ProtocolRegistry) Get(name string) (ProtocolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.protocols[name]
	return info, ok
}

// List returns all registered protocol names.
func (r *ProtocolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.protocols))
	for name := range r.protocols {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// All returns all registered protocols.
func (r *ProtocolRegistry) All() []ProtocolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	protocols := make([]ProtocolInfo, 0, len(r.protocols))
	for _, info := range r.protocols {
		protocols = append(protocols, info)
	}
	return protocols
}

// DefaultRegistry is the global protocol registry.
var DefaultRegistry = NewProtocolRegistry()

// Register is a convenience function to register a protocol with the default registry.
func Register(info ProtocolInfo) error {
	return DefaultRegistry.Register(info)
}
