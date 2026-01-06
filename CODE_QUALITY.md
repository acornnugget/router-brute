# Code Quality Review

This document contains a comprehensive code quality analysis of the router-brute project, identifying issues, violations of clean code principles, and suggestions for improvement.

---

## Table of Contents

1. [Critical Issues](#1-critical-issues)
2. [Code Duplication](#2-code-duplication)
3. [Clean Code Violations](#3-clean-code-violations)
4. [Potential Bugs](#4-potential-bugs)
5. [Extensibility Issues](#5-extensibility-issues)
6. [Error Handling](#6-error-handling)
7. [Testing Issues](#7-testing-issues)
8. [Architecture Improvements](#8-architecture-improvements)
9. [Minor Issues](#9-minor-issues)

---

## 1. Critical Issues

No critical issues remaining. All have been fixed.

---

## 2. Code Duplication

### ~~2.1 Massive Duplication in main.go Run Functions~~ - FIXED

**Status:** Resolved. The code has been refactored to use:
- `AttackConfig` struct for all configuration
- `parseAttackConfig()` to parse flags
- `runAttack()` single generic function for all protocols
- `runMultiTarget()` for multi-target mode
- `setupProtocolCommand()` for flag setup
- Protocol-specific handlers are now thin wrappers (~10 lines each)

### ~~2.2 Duplication in Multi-Target Functions~~ - FIXED

**Status:** Resolved. Multi-target logic consolidated into single `runMultiTarget()` function.

### 2.3 Duplicate Mock Implementations

**Location:**
- `internal/modules/mock/mock.go`
- `internal/core/mock_module.go`
- `internal/testutil/mocks.go`

**Issue:** Three separate mock module implementations still exist. The `testutil` package was created but other mocks weren't consolidated into it.

**Suggestion:** Remove `internal/modules/mock/mock.go` and `internal/core/mock_module.go`, consolidate all mocks into `internal/testutil/mocks.go`.

### 2.4 Duplicate appendLengthPrefixed Function

**Location:**
- `internal/modules/mikrotik/v6/client.go:187-195`
- `internal/modules/mikrotik/v7/client.go:442-450`

**Issue:** Same function duplicated across v6 and v7 modules.

**Suggestion:** Move to a shared package like `internal/modules/mikrotik/common/encoding.go`:

```go
package common

func AppendLengthPrefixed(buf []byte, word string) []byte {
    wordBytes := []byte(word)
    if len(wordBytes) > 255 {
        wordBytes = wordBytes[:255]
    }
    buf = append(buf, byte(len(wordBytes)))
    buf = append(buf, wordBytes...)
    return buf
}
```

### ~~2.5 Duplicate CLI Flag Definitions~~ - FIXED

**Status:** Resolved. Helper functions implemented:
- `addCommonFlags()` - adds all common flags to a command
- `setupProtocolCommand()` - sets up flags and marks required flags

---

## 3. Clean Code Violations

### 3.1 Function Length - God Functions

**Location:** `cmd/router-brute/main.go:130-261`

**Issue:** `runMikrotikV6` is 131 lines, violating the single responsibility principle.

**Suggestion:** Break into smaller functions:
- `parseAttackFlags(cmd) -> AttackConfig`
- `setupModule(cfg) -> RouterModule`
- `runSingleTargetAttack(engine) -> AttackResult`
- `displayResults(results)`

### 3.2 Magic Numbers

**Location:** Multiple files

**Examples:**
```go
// internal/core/engine.go:61
results: make(chan Result, workers*2)  // Why *2?

// internal/modules/mikrotik/v6/client.go:221
buf := make([]byte, 4096)  // Magic buffer size

// internal/modules/mikrotik/v7/webfig/webfig.go:130
drop768 := strings.Repeat("\x00", 768)  // Why 768?
```

**Suggestion:** Define named constants:

```go
const (
    resultChannelMultiplier = 2
    readBufferSize          = 4096
    rc4DropBytes            = 768  // RouterOS RC4 stream drops first 768 bytes
)
```

### 3.3 Inconsistent Error Handling Patterns

**Location:** Throughout codebase

**Issue:** Error handling is inconsistent - sometimes errors are returned, sometimes logged and ignored.

**Examples:**
```go
// Sometimes errors are silently ignored
if err := m.conn.Close(); err != nil {
    zlog.Trace().Err(err).Msg("Error closing connection")
}

// Other times they're returned
if err := m.conn.Close(); err != nil {
    return err
}
```

**Suggestion:** Establish consistent error handling policies documented in the codebase.

### 3.4 Poor Variable Names

**Location:** Multiple files

**Examples:**
```go
// internal/modules/mikrotik/v7/webfig/webfig.go:35
for i := range len(copied) / 2 {
    j := len(copied) - 1 - i
    copied[i], copied[j] = copied[j], copied[i]
}

// internal/core/target_parser.go:51-62
parts := strings.Split(line, ":")
if len(parts) > 0 && parts[0] == "" {
    parts = parts[1:]
}
```

**Suggestion:** Use descriptive names:
```go
for leftIdx := range len(slice) / 2 {
    rightIdx := len(slice) - 1 - leftIdx
    slice[leftIdx], slice[rightIdx] = slice[rightIdx], slice[leftIdx]
}
```

### 3.5 Comments That Explain "What" Instead of "Why"

**Location:** Multiple files

**Example:**
```go
// Read length
length := int(data[i])
i++

// Read word
if i+length > len(data) {
    return nil, errors.New("invalid word length")
}
```

**Suggestion:** Comments should explain *why*, not *what*:
```go
// Length prefix indicates the byte count of the following word
length := int(data[i])
i++

// Validate we have enough data remaining to avoid buffer overflow
if i+length > len(data) {
    return nil, errors.New("invalid word length")
}
```

### 3.6 Unused Parameters

**Location:** `cmd/router-brute/main.go`

**Issue:** `user` and `timeout` parameters are passed to multi-target functions but never used.

```go
func runMultiTargetV6(targetFile, wordlist, user string, port int, timeout time.Duration,
    workers int, rateLimit time.Duration, concurrentTargets int) {
    // 'user' and 'timeout' are never used!
}
```

**Suggestion:** Either remove unused parameters or implement the functionality to use them (e.g., override username from command line).

---

## 4. Potential Bugs

### 4.1 Unchecked Type Assertions

**Location:** `internal/modules/mikrotik/*/client.go`

**Issue:** Type assertions from options map are wrapped in error handling but use fmt.Sprintf which could mask issues.

```go
if port, ok := options["port"]; ok {
    if p, err := strconv.Atoi(fmt.Sprintf("%v", port)); err == nil {
        m.port = p
    }
}
```

**Suggestion:** Use type switches for clarity:

```go
if port, ok := options["port"]; ok {
    switch v := port.(type) {
    case int:
        m.port = v
    case string:
        if p, err := strconv.Atoi(v); err == nil {
            m.port = p
        }
    }
}
```

### 4.2 Response Buffer Truncation

**Location:** `internal/modules/mikrotik/v6/client.go:220-228`

**Issue:** Fixed 4096 byte buffer may truncate responses for large configurations or error messages.

```go
buf := make([]byte, 4096)
n, err := m.conn.Read(buf)
```

**Suggestion:** Implement proper length-aware reading or use a growing buffer:

```go
func (m *MikrotikV6Module) readResponse() ([]byte, error) {
    var response bytes.Buffer
    buf := make([]byte, 4096)
    for {
        n, err := m.conn.Read(buf)
        if n > 0 {
            response.Write(buf[:n])
        }
        if err == io.EOF || n < len(buf) {
            break
        }
        if err != nil {
            return nil, err
        }
    }
    return response.Bytes(), nil
}
```

### ~~4.3 Target Parsing Edge Cases~~ - FIXED

**Status:** Resolved. The parser now uses a clean switch-based approach with explicit field handling. All 13 parser tests pass including edge case handling for empty fields.

### 4.4 Goroutine Leak in Test

**Location:** `internal/core/engine_test.go:43-56`

**Issue:** Using `goto` to exit a loop can leave the result-collecting logic in an inconsistent state.

```go
for {
    select {
    case result := <-engine.Results():
        // ...
        goto done
    case <-time.After(200 * time.Millisecond):
        goto done
    }
}
done:
```

**Suggestion:** Use a more conventional pattern:

```go
timeout := time.After(200 * time.Millisecond)
collecting:
for {
    select {
    case result, ok := <-engine.Results():
        if !ok {
            break collecting
        }
        resultCount++
        if result.Success {
            successCount++
        }
        if resultCount >= len(passwords) {
            break collecting
        }
    case <-timeout:
        break collecting
    }
}
```

---

## 5. Extensibility Issues

### 5.1 Hardcoded Protocol Selection

**Location:** `cmd/router-brute/main.go`

**Issue:** Adding a new protocol requires modifying `init()` and adding a new command with duplicated code.

**Suggestion:** Implement a protocol registry:

```go
type ProtocolRegistry struct {
    protocols map[string]ProtocolInfo
}

type ProtocolInfo struct {
    Factory     func() interfaces.RouterModule
    DefaultPort int
    Description string
}

func (r *ProtocolRegistry) Register(name string, info ProtocolInfo) {
    r.protocols[name] = info
}

// Usage
registry.Register("mikrotik-v6", ProtocolInfo{
    Factory:     v6.NewMikrotikV6Module,
    DefaultPort: 8728,
    Description: "MikroTik RouterOS v6 binary API",
})
```

### 5.2 Missing Configuration Abstraction

**Location:** Multiple modules

**Issue:** Each module parses options differently with no common configuration interface.

**Suggestion:** Create a configuration interface:

```go
type ModuleConfig interface {
    Port() int
    Timeout() time.Duration
    Validate() error
}

type BaseConfig struct {
    port    int
    timeout time.Duration
}

func (c *BaseConfig) FromOptions(opts map[string]interface{}) error {
    // Common parsing logic
}
```

### 5.3 Missing Plugin Architecture

**Issue:** Adding new router types requires modifying core code.

**Suggestion:** Consider implementing a plugin system using Go's plugin package or build-time registration:

```go
// Each module self-registers
func init() {
    registry.Register("mikrotik-v6", &v6.MikrotikV6Factory{})
}
```

---

## 6. Error Handling

### 6.1 Swallowed Errors

**Location:** Multiple files

**Examples:**
```go
// internal/modules/mikrotik/v7/webfig/webfig.go:46-47
decodedData, _ := decoder.Bytes(data)  // Error ignored!

// internal/modules/mikrotik/v7/rest/client.go:184
body, _ := io.ReadAll(resp.Body)  // Error ignored!
```

**Suggestion:** Handle all errors explicitly:

```go
decodedData, err := decoder.Bytes(data)
if err != nil {
    return "", fmt.Errorf("failed to decode data: %w", err)
}
```

### 6.2 Missing Error Context

**Location:** Multiple files

**Issue:** Many errors don't include enough context for debugging.

```go
return errors.New("not connected")
```

**Suggestion:** Add context:

```go
return fmt.Errorf("mikrotik-v6: cannot send login - not connected to %s", m.GetTarget())
```

### 6.3 Inconsistent Error Types

**Location:** Throughout codebase

**Issue:** Mix of custom error types (`ConnectionError`, `AuthenticationError`) and string errors.

**Suggestion:** Consistently use error types for categorizable errors:

```go
type AuthError struct {
    Target   string
    Username string
    Cause    error
}

func (e *AuthError) Error() string {
    return fmt.Sprintf("authentication failed for %s@%s: %v", e.Username, e.Target, e.Cause)
}

func (e *AuthError) Is(target error) bool {
    _, ok := target.(*AuthError)
    return ok
}
```

---

## 7. Testing Issues

### 7.1 Test Coverage Gaps

**Missing Tests:**
- `internal/modules/mikrotik/v7/webfig/webfig.go` - No unit tests for encryption functions
- `internal/modules/mikrotik/v7/webfig/m2message.go` - No unit tests for M2 message serialization
- `cmd/router-brute/main.go` - No tests for CLI parsing or run functions
- `pkg/utils/errors.go` - No tests for error types

### 7.2 Integration Tests Depend on Real Infrastructure

**Location:** `internal/integration/engine_test.go`, `debugging/`

**Issue:** Some tests appear to require real routers to run.

**Suggestion:** Use test containers or more comprehensive mocks:

```go
func TestWithMockServer(t *testing.T) {
    server := startMockMikrotikServer(t)
    defer server.Close()

    module := v6.NewMikrotikV6Module()
    module.Initialize(server.Addr, "admin", nil)
    // ...
}
```

### 7.3 Flaky Tests Due to Timing

**Location:** `internal/core/multi_engine_test.go:271-291`

**Issue:** Test relies on timing for cancellation testing.

```go
ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
// ...
assert.True(t, results[0].Attempts >= 0 && results[0].Attempts <= 2)  // Flaky!
```

**Suggestion:** Use synchronization primitives instead of time-based assumptions.

### 7.4 Test Helpers Partially Centralized

**Issue:** A `testutil` package was created (`internal/testutil/mocks.go`) but mock implementations are still scattered across multiple files (`internal/modules/mock/mock.go`, `internal/core/mock_module.go`).

**Suggestion:** Complete the consolidation by removing duplicate mocks and using only the `testutil` package.

---

## 8. Architecture Improvements

### 8.1 Missing Dependency Injection

**Issue:** Modules create their own HTTP clients and connections, making testing harder.

**Suggestion:** Inject dependencies:

```go
type MikrotikV7RestModule struct {
    httpClient HTTPClient  // interface instead of concrete type
    // ...
}

type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

func NewMikrotikV7RestModule(opts ...Option) *MikrotikV7RestModule {
    m := &MikrotikV7RestModule{
        httpClient: http.DefaultClient,
    }
    for _, opt := range opts {
        opt(m)
    }
    return m
}

func WithHTTPClient(c HTTPClient) Option {
    return func(m *MikrotikV7RestModule) {
        m.httpClient = c
    }
}
```

### 8.2 Missing Logging Abstraction

**Issue:** Direct use of zerolog throughout makes it hard to change logging behavior.

**Suggestion:** Use an interface:

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Error(err error, msg string, fields ...Field)
}
```

### 8.3 Missing Metrics/Observability

**Issue:** No metrics collection for monitoring attack progress, success rates, etc.

**Suggestion:** Add metrics interface:

```go
type Metrics interface {
    IncAttempts(target, protocol string)
    IncSuccess(target, protocol string)
    IncFailure(target, protocol string)
    ObserveLatency(target, protocol string, duration time.Duration)
}
```

### 8.4 Consider Using Functional Options Pattern

**Location:** Module constructors

**Current:**
```go
module.Initialize(target, user, map[string]interface{}{
    "port":    port,
    "timeout": timeoutDuration,
})
```

**Suggestion:**
```go
module := v6.NewMikrotikV6Module(
    v6.WithTarget(target),
    v6.WithUsername(user),
    v6.WithPort(port),
    v6.WithTimeout(timeoutDuration),
)
```

---

## 9. Minor Issues

### 9.1 Unused Imports Warning Potential

**Location:** `internal/modules/interface.go:6`

**Issue:** Imports `core` package which could cause import cycles in the future.

### 9.2 Inconsistent File Naming

**Issue:** Some files use underscores (`password_queue.go`), others don't (`webfig.go`).

**Suggestion:** Follow Go conventions: `passwordqueue.go` or keep underscores consistently.

### 9.3 Missing Package Documentation

**Issue:** Most packages lack package-level documentation.

**Suggestion:** Add doc.go files:

```go
// Package core provides the brute-forcing engine and supporting
// infrastructure for concurrent password testing.
package core
```

### 9.4 Unused Code

**Location:**
- `internal/modules/interface.go:89-100` - `CreateResult` method is defined but never used
- `internal/modules/mikrotik/v6/client.go:197-217` - `encodeSentence` is unused
- `internal/modules/mikrotik/v6/client.go:307-318` - `DebugMikrotikV6` struct appears to be debug-only

### 9.5 TODO Comments Without Tracking

**Issue:** No TODO comments found but also no documentation of known limitations.

**Suggestion:** Add a KNOWN_ISSUES.md or use structured TODO format:

```go
// TODO(username, issue-123): Description of what needs to be done
```

### 9.6 Missing Input Validation

**Location:** `cmd/router-brute/main.go`

**Issue:** Limited validation of user inputs (port ranges, file existence before use, etc.)

**Suggestion:**
```go
func validatePort(port int) error {
    if port < 1 || port > 65535 {
        return fmt.Errorf("invalid port: %d (must be 1-65535)", port)
    }
    return nil
}

func validateFile(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return fmt.Errorf("file not accessible: %w", err)
    }
    if info.IsDir() {
        return fmt.Errorf("path is a directory, not a file: %s", path)
    }
    return nil
}
```

---

## Summary

### Recently Fixed

1. ~~Race condition in MultiTargetEngine result collection~~ - **FIXED** (channels now drained via goroutines before closing)
2. ~~Potential deadlock from premature context cancellation~~ - **FIXED** (cancel function stored instead of deferred)
3. ~~PasswordQueue not copied for multi-target~~ - **FIXED** (explicit slice copy added)
4. ~~Test helpers scattered~~ - **PARTIALLY FIXED** (`testutil` package created)
5. ~~Nil context handling causing panics~~ - **FIXED** (all modules now return error on nil context)
6. ~~Target parser edge cases~~ - **FIXED** (proper test expectations, all parser tests pass)
7. ~~Code duplication in main.go~~ - **FIXED** (refactored to use `AttackConfig`, `runAttack()`, `setupProtocolCommand()`)

### High Priority Fixes

No high priority fixes remaining. All have been addressed.

### Medium Priority Improvements

1. Complete mock consolidation (remove `internal/modules/mock/` and `internal/core/mock_module.go`)
2. Extract shared encoding functions to common package
3. Improve error handling consistency
4. Add missing unit tests for webfig and m2message packages

### Low Priority Enhancements

1. Implement protocol registry for extensibility
2. Add dependency injection for better testability
3. Add metrics/observability hooks
4. Improve input validation

---

## Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Lines of duplicate code | ~100 | <100 |
| Test coverage (estimate) | ~50% | >80% |
| Functions >50 lines | 3 | 0 |
| Magic numbers | 15+ | 0 |
| Unchecked errors | 10+ | 0 |
| Failing tests | 0 | 0 |
