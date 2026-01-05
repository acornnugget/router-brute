# Optimization: Connection Reuse

## Overview

Implemented connection reuse for MikroTik RouterOS authentication to maximize performance while respecting protocol limits.

## Performance Impact

**~5x faster authentication**
- **Before**: 10-15ms per attempt (reconnecting each time)
- **After**: 2-3ms per attempt (connection reuse)
- **Speedup**: 4-6x improvement

## Implementation

### RouterOS v6 Binary API

Based on empirical testing (see `debugging/CONNECTION_LIMIT_FINDINGS.md`), RouterOS v6 has a **5 failed authentication attempts per connection** limit.

**Strategy**: Reuse connection for up to **4 attempts**, then reconnect.

**Files Modified**:
- `internal/modules/mikrotik/v6/client.go`

**Changes**:
```go
type MikrotikV6Module struct {
    // ...
    attemptsOnConn int // Track attempts on current connection
}

func (m *MikrotikV6Module) Authenticate(ctx context.Context, password string) (bool, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Reconnect after 4 attempts (stay under 5-attempt limit)
    if m.IsConnected() && m.attemptsOnConn >= 4 {
        m.Close()
    }

    if !m.IsConnected() {
        m.Connect(ctx)
        m.attemptsOnConn = 0
    }

    m.attemptsOnConn++

    // Authenticate...
}
```

### RouterOS v7 Binary API

Applied same optimization for consistency, assuming similar limits.

**Files Modified**:
- `internal/modules/mikrotik/v7/client.go`

**Note**: v7 WebFig mode uses HTTP and doesn't need explicit connection reuse (HTTP client handles it).

### RouterOS v7 REST API

HTTP-based protocol - connection reuse is automatically handled by Go's HTTP client via keep-alive.

**Files Modified**:
- `internal/modules/mikrotik/v7/rest/client.go`

**Changes**: Removed unnecessary Close/Connect cycle, relying on HTTP client's connection pooling.

## Test Results

### Connection Limit Test (`test_connection_limit.go`)

Tested against 192.168.1.74:8728 to determine RouterOS limits:

```
Attempt #1-4: !trap =message=invalid user name or password (6)
Attempt #5:   !fatal too many commands before login
Attempt #6:   EOF (connection closed)
```

**Result**: Exactly 5 attempts before disconnection.

### Connection Reuse Test (`test_connection_reuse.go`)

Validated connection reuse implementation:

```
12 attempts in 27.9ms = 2.3ms per attempt

Attempts 1-4:  Same connection (~2ms each)
Reconnection:  "Reconnecting after reaching attempt limit"
Attempts 5-8:  Same connection (~2ms each)
Reconnection:  "Reconnecting after reaching attempt limit"
Attempts 9-12: Same connection (~2ms each)
```

**Result**: ✅ Connection reuse working as designed.

## Thread Safety

All modules use `sync.Mutex` to serialize authentication operations:

```go
type MikrotikV6Module struct {
    mu             sync.Mutex  // Protects conn and authentication operations
    attemptsOnConn int         // Protected by mu
    // ...
}

func (m *MikrotikV6Module) Authenticate(...) {
    m.mu.Lock()
    defer m.mu.Unlock()
    // All state modifications protected
}
```

**Guarantee**: Only one goroutine can authenticate at a time per module instance, preventing race conditions.

## Error Handling

The implementation gracefully handles the connection limit:

1. **Normal failures** (attempts 1-4): `!trap =message=invalid user name or password (6)`
   - Counted towards attempt limit
   - Connection stays open

2. **Limit warning** (5th attempt): `!fatal too many commands before login`
   - Treated as authentication failure (not fatal error)
   - Connection closes after this

3. **Reconnection**: Automatically triggered before reaching limit
   - Debug log: "Reconnecting after reaching attempt limit"
   - Counter reset on new connection

## Benefits

1. **Performance**: 4-6x faster authentication attempts
2. **Reliability**: Never hits RouterOS connection limit
3. **Simplicity**: Automatic reconnection handling
4. **Thread-safe**: Mutex protection prevents race conditions
5. **Transparent**: No changes required to calling code

## Testing

All tests passing:
```bash
go test ./...
# All modules: PASS
```

Integration test results confirm connection reuse works correctly in multi-worker scenarios.

## Monitoring

Connection reuse can be monitored via debug logs:

```bash
# Look for reconnection messages:
{"level":"debug","target":"X.X.X.X","attempts":4,"message":"Reconnecting after reaching attempt limit"}

# Each authentication attempt:
{"level":"debug","target":"X.X.X.X","username":"admin","password":"...","message":"Trying:"}
```

Reconnections should occur every 4 attempts, confirming the optimization is working.

## Future Considerations

- **Adaptive limits**: Could detect connection limit errors and adjust dynamically
- **Protocol-specific tuning**: Different protocols might have different optimal reuse counts
- **Metrics**: Track connection reuse rate and performance gains

## References

- `debugging/CONNECTION_LIMIT_FINDINGS.md` - Detailed limit analysis
- `debugging/test_connection_limit.go` - Limit discovery test
- `debugging/test_connection_reuse.go` - Reuse validation test
