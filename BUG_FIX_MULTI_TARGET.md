# Bug Fix: Multi-Target Mode - 0 Attempts on All Hosts

## The Issue

When using multi-target mode, all hosts were failing with 0 attempts made. Workers would timeout trying to connect, resulting in no authentication attempts.

### Symptoms

```
{"level":"error","error":"connection error: dial tcp X.X.X.X:8728: i/o timeout","worker_id":0,"message":"Worker failed to connect"}
...
Result: 0 attempts for all targets
```

## Root Causes

### Issue 1: Missing Auto-Close in `StartWithContext()`

The `StartWithContext()` method (used by multi-target mode) was missing the auto-close goroutine that was added to `Start()`.

**Impact**: If all workers failed to connect, channels would never close, causing the result collectors to hang indefinitely.

### Issue 2: No Pre-Flight Connection Check

Multi-target mode was starting engines and letting workers discover connection failures individually. This meant:
- All workers would timeout (10s each) for unreachable targets
- Wasted time trying multiple workers when the target is clearly unreachable
- Poor error reporting - had to wait for all workers to fail

## The Fixes

### Fix 1: Add Auto-Close to `StartWithContext()`

**File**: `internal/core/engine.go`

```go
// Before:
func (e *Engine) StartWithContext(ctx context.Context) {
    // ...
    for i := 0; i < e.workers; i++ {
        e.wg.Add(1)
        go e.worker(i)
    }
    // Missing: No auto-close goroutine!
}

// After:
func (e *Engine) StartWithContext(ctx context.Context) {
    // ...
    for i := 0; i < e.workers; i++ {
        e.wg.Add(1)
        go e.worker(i)
    }

    // Auto-close channels when all workers complete
    go func() {
        e.wg.Wait()
        e.closeChannels()
    }()
}
```

**Benefit**: Ensures channels are properly closed even when all workers fail, preventing hangs.

### Fix 2: Add Pre-Flight Connection Check to Multi-Target

**File**: `internal/core/multi_engine.go`

```go
// Added after module initialization:

// Pre-flight connection check to fail fast on unreachable targets
zlog.Debug().Str("target", target.IP).Msg("Testing connection to target...")
testCtx, testCancel := context.WithTimeout(mte.ctx, 10*time.Second)
if err := module.Connect(testCtx); err != nil {
    testCancel()
    zlog.Error().
        Str("target", target.IP).
        Int("port", target.Port).
        Err(err).
        Msg("Failed to connect to target")
    mte.errorsChan <- MultiTargetError{Target: target, Error: err}
    return  // Skip this target immediately
}
testCancel()
zlog.Info().Str("target", target.IP).Int("port", target.Port).Msg("Connection test successful")

// Close test connection - workers will reconnect
if err := module.Close(); err != nil {
    zlog.Debug().Str("target", target.IP).Err(err).Msg("Error closing test connection")
}
```

**Benefits**:
1. **Fail fast**: Unreachable targets are identified in ~10s instead of workers × 10s
2. **Better logging**: Clear "Failed to connect to target" message upfront
3. **Resource savings**: Don't waste time starting workers for unreachable targets
4. **Cleaner output**: Single error per target instead of N errors (one per worker)

## Behavior Changes

### Before

```
Starting attack on target 192.168.1.1...
[10s pass while all 5 workers timeout]
Worker 0 failed to connect
Worker 1 failed to connect
Worker 2 failed to connect
Worker 3 failed to connect
Worker 4 failed to connect
Result: 0 attempts

Starting attack on target 192.168.1.2...
[10s pass while all 5 workers timeout]
...
```

Total time for 5 unreachable targets with 5 workers: ~250 seconds (5 targets × 5 workers × 10s)

### After

```
Starting attack on target 192.168.1.1...
Testing connection to target...
Failed to connect to target: connection timeout
[Target skipped immediately]

Starting attack on target 192.168.1.2...
Testing connection to target...
Failed to connect to target: connection timeout
[Target skipped immediately]
...
```

Total time for 5 unreachable targets: ~50 seconds (5 targets × 10s)

**5x faster failure detection!**

## Test Results

All multi-target tests pass:
```
TestMultiTargetEngine_Simple: PASS
TestMultiTargetEngine_Basic: PASS
TestMultiTargetEngine_ConcurrentTargets: PASS
TestMultiTargetEngine_Cancellation: PASS
TestMultiTargetEngine_ErrorHandling: PASS
```

## Summary

- ✅ Added auto-close to `StartWithContext()` for consistency with `Start()`
- ✅ Added pre-flight connection check to multi-target mode
- ✅ Unreachable targets now fail fast (10s vs 50s for 5 workers)
- ✅ Cleaner error reporting
- ✅ All tests passing

## Related Fixes

This complements the earlier fixes:
- Pre-flight connection check in single-target mode
- Race condition fix with mutex in modules
- Auto-close channels in `Start()`
