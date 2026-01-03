# Step 3: Router Module Interface Definition

## Objective
Define the standard interface that all router-specific modules must implement.

## Tasks
1. Create `internal/modules/interface.go`
2. Define interface methods:
   - `Connect() error`
   - `Authenticate(username, password string) (bool, error)`
   - `Close() error`
   - `GetProtocolName() string`
3. Implement common error types
4. Create base struct with common fields
5. Add documentation for module developers

## Expected Duration: 1 hour