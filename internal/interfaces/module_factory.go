package interfaces

// ModuleFactory defines the interface for creating router modules
// This allows the multi-target engine to create modules without
// knowing the specific implementation details
type ModuleFactory interface {
	// CreateModule creates a new router module instance
	CreateModule() RouterModule

	// GetProtocolName returns the name of the protocol this factory creates
	GetProtocolName() string
}
