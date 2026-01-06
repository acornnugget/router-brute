# CLI Refactoring - Common Parameters

## Overview

Refactored the command-line interface to reduce duplication by moving common parameters to the root command level, simplifying usage and maintenance.

## Changes

### Before (Duplicated Flags)

Each subcommand had its own copy of common flags:

```bash
# mikrotik-v6 had its own flags
./router-brute mikrotik-v6 --target X --user Y --wordlist Z --workers N ...

# mikrotik-v7 had its own flags
./router-brute mikrotik-v7 --target X --user Y --wordlist Z --workers N ...

# mikrotik-v7-rest had its own flags
./router-brute mikrotik-v7-rest --target X --user Y --wordlist Z --workers N ...
```

**Issues:**
- Code duplication (addCommonFlags called 3 times)
- Harder to maintain (update flags in multiple places)
- Inconsistent help output

### After (Persistent Flags)

Common parameters moved to root level:

```bash
# Common flags available to all subcommands
./router-brute --target X --user Y --wordlist Z mikrotik-v6

# Or after the subcommand (both work)
./router-brute mikrotik-v6 --target X --user Y --wordlist Z
```

**Benefits:**
- Single definition of common flags
- Consistent across all protocols
- Easier to add new protocols
- Cleaner help output

## Flag Categories

### Global Flags (Root Level - Persistent)

Available to all subcommands:

**Logging:**
- `--debug` - Enable debug logging
- `--trace` - Enable trace logging

**Target Selection:**
- `--target` - Router IP address or hostname
- `--target-file` - File containing target specifications

**Authentication:**
- `--user` - Username to test (default: admin)
- `--wordlist` - Path to password wordlist file

**Performance:**
- `--workers` - Number of concurrent workers (default: 5)
- `--rate` - Rate limit between attempts (default: 100ms)
- `--timeout` - Connection timeout (default: 10s)
- `--concurrent-targets` - Number of targets to attack simultaneously (default: 1)

**Resume:**
- `--resume` - Resume from a previous session
- `--save-progress` - Auto-save progress interval (default: 30s)
- `--save-dir` - Directory to save resume files (default: ./resume)

### Protocol-Specific Flags (Subcommand Level)

**Port (Different defaults per protocol):**
- `mikrotik-v6 --port` - Default: 8728
- `mikrotik-v7 --port` - Default: 8729
- `mikrotik-v7-rest --port` - Default: 80

**Module-Specific:**
- `mikrotik-v7-rest --https` - Use HTTPS instead of HTTP

## Usage Examples

### Before

```bash
# Long command with all flags after subcommand
./router-brute mikrotik-v6 \
  --target 192.168.1.1 \
  --user admin \
  --wordlist passwords.txt \
  --workers 5 \
  --rate 100ms
```

### After (Both styles work)

Style 1: Flags before subcommand
```bash
./router-brute \
  --target 192.168.1.1 \
  --user admin \
  --wordlist passwords.txt \
  --workers 5 \
  mikrotik-v6
```

Style 2: Flags after subcommand (traditional)
```bash
./router-brute mikrotik-v6 \
  --target 192.168.1.1 \
  --user admin \
  --wordlist passwords.txt \
  --workers 5
```

Style 3: Mixed (flexible)
```bash
./router-brute --debug \
  mikrotik-v6 \
  --target 192.168.1.1 \
  --wordlist passwords.txt
```

## Help Output

### Root Command

```bash
$ ./router-brute --help

Usage:
  router-brute [command]

Available Commands:
  mikrotik-v6      Brute force MikroTik RouterOS v6
  mikrotik-v7      Brute force MikroTik RouterOS v7 (binary API)
  mikrotik-v7-rest Brute force MikroTik RouterOS v7 (REST API)

Flags:
      --concurrent-targets int   Number of targets to attack simultaneously (default 1)
      --debug                    Enable debug logging
      --rate string              Rate limit between attempts (default "100ms")
      --resume string            Resume from a previous session
      --save-dir string          Directory to save resume files (default "./resume")
      --save-progress string     Auto-save progress interval (default "30s")
      --target string            Router IP address or hostname
      --target-file string       File containing target specifications
      --timeout string           Connection timeout (default "10s")
      --trace                    Enable trace logging
      --user string              Username to test (default "admin")
      --wordlist string          Path to password wordlist file
      --workers int              Number of concurrent workers (default 5)
```

### Subcommand (mikrotik-v6)

```bash
$ ./router-brute mikrotik-v6 --help

Brute force MikroTik RouterOS v6

Usage:
  router-brute mikrotik-v6 [flags]

Flags:
  -h, --help       help for mikrotik-v6
      --port int   RouterOS v6 API port (default 8728)

Global Flags:
      --concurrent-targets int   Number of targets to attack simultaneously (default 1)
      --debug                    Enable debug logging
      --rate string              Rate limit between attempts (default "100ms")
      --resume string            Resume from a previous session
      ... (all global flags shown)
```

### Subcommand (mikrotik-v7-rest)

```bash
$ ./router-brute mikrotik-v7-rest --help

Brute force MikroTik RouterOS v7 (REST API)

Usage:
  router-brute mikrotik-v7-rest [flags]

Flags:
  -h, --help       help for mikrotik-v7-rest
      --https      Use HTTPS instead of HTTP
      --port int   RouterOS v7 REST API port (default 80)

Global Flags:
      ... (all global flags shown)
```

## Implementation Details

### Code Changes

**Removed:**
- `setupProtocolCommand()` function
- `addCommonFlags()` function
- Per-command flag duplication

**Added:**
- `rootCmd.PersistentFlags()` - Common flags defined once
- `rootCmd.PersistentPreRunE` - Centralized validation
- Subcommand-specific `cmd.Flags()` - Only for unique flags

**File Modified:**
- `cmd/router-brute/main.go` - ~50 lines removed, cleaner structure

### Validation

Validation moved to root level (PersistentPreRunE):
- Checks for --target or --target-file requirement
- Validates mutual exclusivity
- Handles --resume mode (overrides requirements)
- Runs before each subcommand execution

## Benefits

### For Users

1. **Clearer help output** - Global flags shown separately
2. **Flexible flag placement** - Before or after subcommand
3. **Consistent interface** - Same flags work everywhere
4. **Better discoverability** - `--help` shows all available flags

### For Developers

1. **Less code duplication** - Define flags once
2. **Easier maintenance** - Update in one place
3. **Simpler to add protocols** - No flag setup boilerplate
4. **Centralized validation** - Single validation function

### For the Codebase

1. **Reduced lines of code** - ~50 lines removed
2. **Better organization** - Clear separation of concerns
3. **Type safety** - Consistent flag parsing
4. **Maintainability** - Single source of truth

## Migration Guide

### Adding New Protocol

Before (required flag setup):
```go
var newProtocolCmd = &cobra.Command{
    Use: "new-protocol",
    Run: runNewProtocol,
}

func init() {
    setupProtocolCommand(newProtocolCmd, 9999)
    rootCmd.AddCommand(newProtocolCmd)
}
```

After (minimal setup):
```go
var newProtocolCmd = &cobra.Command{
    Use: "new-protocol",
    Run: runNewProtocol,
}

func init() {
    // Only add protocol-specific flags
    newProtocolCmd.Flags().Int("port", 9999, "Protocol port")
    rootCmd.AddCommand(newProtocolCmd)
}
```

### Adding New Common Flag

Before (update 3+ places):
```go
func addCommonFlags(cmd *cobra.Command, defaultPort int) {
    // ... existing flags ...
    cmd.Flags().String("new-flag", "default", "Description")
}
// Repeat for each command setup
```

After (update 1 place):
```go
func init() {
    // ... existing flags ...
    rootCmd.PersistentFlags().String("new-flag", "default", "Description")
}
```

## Testing

All tests passing:
- ✅ Build successful
- ✅ Linter clean (0 issues)
- ✅ All unit tests passing
- ✅ Help output verified
- ✅ Flag inheritance working

## Backward Compatibility

✅ **Fully backward compatible** - All existing command-line invocations still work:

```bash
# Old style (still works)
./router-brute mikrotik-v6 --target X --wordlist Y

# New style (also works)
./router-brute --target X --wordlist Y mikrotik-v6
```

## Future Improvements

Potential enhancements:
- [ ] Add config file support (e.g., `.router-brute.yaml`)
- [ ] Environment variable support for flags
- [ ] Flag aliases (e.g., `-t` for `--target`)
- [ ] Auto-completion improvements
- [ ] Flag grouping in help output
