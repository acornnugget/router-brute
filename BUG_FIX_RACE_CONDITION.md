# Critical Bug Fix: Race Condition in Module Authentication

## The Bug

**Severity**: CRITICAL
**Type**: Race Condition / Nil Pointer Dereference
**Impact**: Panic crash with multiple workers

### Error Message
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x50 pc=0x67655a]

goroutine 35 [running]:
github.com/nimda/router-brute/internal/modules/mikrotik/v6.(*MikrotikV6Module).sendLogin(...)
    internal/modules/mikrotik/v6/client.go:157
```

## Root Cause

Multiple worker goroutines shared a **single module instance** without synchronization. The `Authenticate()` method:

1. Closes the connection (`m.conn = nil`)
2. Reconnects (`m.conn = new connection`)
3. Sends login command

When multiple workers call `Authenticate()` concurrently:

```
Timeline:
T0: Worker A: Close() → m.conn = nil
T1: Worker B: Close() → m.conn = nil (redundant)
T2: Worker A: Connect() → m.conn = new conn A
T3: Worker B: Connect() → m.conn = new conn B (overwrites A's connection!)
T4: Worker A: sendLogin() uses m.conn → might be nil or wrong connection
T5: Worker B: sendLogin() uses m.conn → PANIC or wrong behavior
```

## The Fix

Added `sync.Mutex` to serialize authentication operations in all three modules:

### Files Modified

1. **internal/modules/mikrotik/v6/client.go**
   - Added `mu sync.Mutex` field
   - Wrapped `Authenticate()` with `mu.Lock()/defer mu.Unlock()`

2. **internal/modules/mikrotik/v7/client.go**
   - Added `mu sync.Mutex` field
   - Wrapped `Authenticate()` with `mu.Lock()/defer mu.Unlock()`

3. **internal/modules/mikrotik/v7/rest/client.go**
   - Added `mu sync.Mutex` field
   - Wrapped `Authenticate()` with `mu.Lock()/defer mu.Unlock()`

### Code Changes

```go
// Before (UNSAFE):
type MikrotikV6Module struct {
    *modules.BaseRouterModule
    conn    net.Conn
    port    int
    timeout time.Duration
}

func (m *MikrotikV6Module) Authenticate(ctx context.Context, password string) (bool, error) {
    // Multiple goroutines can race here!
    if m.IsConnected() {
        m.Close() // Sets m.conn = nil
    }
    m.Connect(ctx) // Sets m.conn = new connection
    m.sendLogin(...)  // Uses m.conn - might be nil!
}

// After (SAFE):
type MikrotikV6Module struct {
    *modules.BaseRouterModule
    mu      sync.Mutex // Protects conn and authentication operations
    conn    net.Conn
    port    int
    timeout time.Duration
}

func (m *MikrotikV6Module) Authenticate(ctx context.Context, password string) (bool, error) {
    // Lock to prevent concurrent authentication attempts
    m.mu.Lock()
    defer m.mu.Unlock()

    // Now only one goroutine at a time can access this critical section
    if m.IsConnected() {
        m.Close()
    }
    m.Connect(ctx)
    m.sendLogin(...)
}
```

## Why This Happened

The engine design shares a single module instance across all workers:

```go
// internal/core/engine.go
type Engine struct {
    module  interfaces.RouterModule  // SHARED by all workers!
    workers int
}

func (e *Engine) worker(id int) {
    // All workers use the same e.module
    success, err := e.module.Authenticate(e.ctx, password)
}
```

This design assumes modules are thread-safe, but they weren't.

## Alternative Solutions Considered

1. **Create module per worker** ❌
   - Would waste resources (multiple connections per target)
   - Current design is intentional for efficiency

2. **Connection pooling** ❌
   - Over-engineering for this use case
   - Adds complexity

3. **Mutex protection** ✅
   - Simple, effective
   - Minimal performance impact (authentication is I/O bound)
   - Maintains current architecture

## Performance Impact

**Minimal**. Authentication is I/O bound (network latency >> mutex overhead).

Before: Multiple workers attempted concurrent auth → race condition → crash
After: Workers serialize authentication → no race → stable operation

The serialization actually matches the protocol's design - you can only have one active authentication session per connection anyway.

## Testing

All tests pass:
```
internal/core: 15/15 tests PASS
internal/integration: 2/2 tests PASS
internal/modules/mikrotik/v6: 2/2 tests PASS
internal/modules/mikrotik/v7: 2/2 tests PASS
Build: ✅ SUCCESS
```

## Lessons Learned

1. **Shared mutable state requires synchronization**
2. **Always protect connection lifecycle operations with locks**
3. **Document thread-safety assumptions in interfaces**
4. **Run with `-race` flag to catch these issues early**

## Recommendation

Add to future improvements:
- Document thread-safety requirements in `RouterModule` interface
- Add race detector to CI/CD pipeline (`go test -race`)
- Consider adding thread-safety tests

## Related Issues

- This fixes the panic reported in production
- May also fix intermittent authentication failures that were hard to reproduce
