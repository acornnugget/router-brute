# Resume Functionality

## Overview

Router-brute now supports **save/resume** functionality, allowing you to:
- Save attack progress automatically during execution
- Resume interrupted attacks from where they left off
- Track progress per-target in multi-target mode
- Maintain complete state across sessions

## Features

### Automatic Progress Saving

Progress is automatically saved at regular intervals during attacks:
- Default interval: **30 seconds**
- Customizable via `--save-progress` flag
- Saves to timestamped files for multiple resume points
- Tracks passwords tried per target

### Resume from Previous Session

Resume an interrupted or stopped attack:
- Load complete state from resume file
- Skip already-completed targets
- Skip already-tried passwords for each target
- Continue with exact same configuration

### Timestamped Resume Files

Resume files are timestamped to allow multiple save points:
```
./resume/resume_20260105_150405.json
./resume/resume_20260105_151230.json
./resume/resume_20260105_152045.json
```

## Usage

### Basic Multi-Target Attack with Auto-Save

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --user admin \
  --workers 5 \
  --save-progress 30s \
  --save-dir ./resume
```

This will:
- Attack all targets in `targets.txt`
- Save progress every 30 seconds to `./resume/`
- Create timestamped resume files

### Resume from Previous Session

```bash
./router-brute mikrotik-v6 \
  --resume ./resume/resume_20260105_150405.json
```

This will:
- Load targets, passwords, and progress from the resume file
- Skip completed targets
- Resume incomplete targets from their last password index
- Continue with same workers/rate-limit settings

### Disable Auto-Save

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --user admin \
  --save-progress 0
```

Set `--save-progress` to `0` to disable auto-save.

## Resume File Format

Resume files are JSON with complete attack state:

```json
{
  "timestamp": "2026-01-05T15:04:05Z",
  "protocol": "mikrotik-v6",
  "username": "admin",
  "password_file": "/path/to/passwords.txt",
  "target_file": "/path/to/targets.txt",
  "workers": 5,
  "rate_limit": "100ms",
  "targets": [
    {
      "ip": "192.168.1.1",
      "port": 8728,
      "username": "admin",
      "passwords_tried": 150,
      "completed": false,
      "success": false
    },
    {
      "ip": "192.168.1.2",
      "port": 8728,
      "username": "admin",
      "passwords_tried": 0,
      "completed": true,
      "success": true,
      "found_password": "admin123"
    }
  ]
}
```

### Fields Explained

- **timestamp**: When this resume state was saved
- **protocol**: Protocol being attacked (mikrotik-v6, mikrotik-v7, etc.)
- **username**: Username for authentication
- **password_file**: Path to password wordlist
- **target_file**: Path to targets file (multi-target mode)
- **workers**: Number of concurrent workers
- **rate_limit**: Rate limit between attempts
- **targets**: Array of target progress:
  - **ip**: Target IP address
  - **port**: Target port
  - **username**: Username for this target
  - **passwords_tried**: Number of passwords attempted
  - **completed**: Whether this target is finished
  - **success**: Whether valid credentials were found
  - **found_password**: The successful password (if any)

## Command Line Flags

### `--resume <file>`
- **Purpose**: Resume from a previous session
- **Overrides**: When set, `--target`, `--target-file`, and `--wordlist` are optional
- **Loads**: All configuration from the resume file
- **Example**: `--resume ./resume/resume_20260105_150405.json`

### `--save-progress <interval>`
- **Purpose**: Auto-save progress at regular intervals
- **Default**: `30s`
- **Disable**: Set to `0` or `0s`
- **Format**: Duration string (e.g., `30s`, `1m`, `5m`)
- **Example**: `--save-progress 1m`

### `--save-dir <directory>`
- **Purpose**: Directory to save resume files
- **Default**: `./resume`
- **Creates**: Directory if it doesn't exist
- **Example**: `--save-dir /tmp/router-brute-resume`

## Resume Summary

When resuming, a summary is displayed:

```
=== Resume State Summary ===
Timestamp:     2026-01-05 15:04:05
Protocol:      mikrotik-v6
Username:      admin
Password File: /home/user/passwords.txt
Target File:   /home/user/targets.txt
Workers:       5
Rate Limit:    100ms

Progress:      2/10 targets completed (1 successful)

Successful credentials:
  192.168.1.2:8728 - admin:admin123

Remaining targets: 8
  192.168.1.1:8728 (150 passwords tried)
  192.168.1.3:8728 (0 passwords tried)
  192.168.1.4:8728 (0 passwords tried)
  192.168.1.5:8728 (0 passwords tried)
  192.168.1.6:8728 (0 passwords tried)
  ... and 3 more
=============================
```

## How It Works

### Progress Tracking

During attack execution:
1. Progress tracker runs in background
2. Every N seconds (configurable), current state is saved
3. State includes:
   - How many passwords tried per target
   - Which targets are completed
   - Which targets were successful
   - Found credentials

### Resume Process

When resuming:
1. Load resume file
2. Parse targets and configuration
3. For each target:
   - If completed, skip entirely
   - If incomplete, start from `passwords_tried` index
4. Continue attack from saved state

### Incremental Updates

Progress is updated:
- **Periodically**: Every 10 authentication attempts
- **On completion**: When a target finishes (success or all passwords tried)
- **On save**: When auto-save interval triggers

## Performance Impact

Resume functionality has minimal overhead:
- **Memory**: ~1KB per target
- **Disk I/O**: Only during auto-save (configurable interval)
- **CPU**: Negligible (progress tracking is lock-protected)

## Use Cases

### Long-Running Attacks

For attacks that take hours or days:
```bash
./router-brute mikrotik-v6 \
  --target-file 1000_targets.txt \
  --wordlist rockyou.txt \
  --save-progress 5m \
  --save-dir ./resume
```

If interrupted (Ctrl+C, crash, network issue), resume:
```bash
./router-brute mikrotik-v6 \
  --resume ./resume/resume_<timestamp>.json
```

### Testing and Development

Create checkpoints during testing:
```bash
# Start attack with frequent saves
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 10s

# Test resume from any checkpoint
./router-brute mikrotik-v6 \
  --resume ./resume/resume_20260105_150405.json
```

### Parallel Attack Strategies

Split large wordlist, save progress, analyze, resume with different strategy:
```bash
# First pass: common passwords
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist common.txt \
  --save-progress 30s

# Resume with full wordlist on remaining targets
./router-brute mikrotik-v6 \
  --resume ./resume/resume_<timestamp>.json \
  # Note: Will use password_file from resume state
```

## Limitations

### Current Limitations

1. **Multi-target mode only**: Resume currently only works with `--target-file`
2. **Manual file selection**: Must specify exact resume file path
3. **No automatic latest**: Doesn't auto-select most recent resume file
4. **Password file dependency**: Resume file stores path, not password content

### Single-Target Mode

Single-target attacks don't currently support resume (only multi-target):
```bash
# NOT supported:
./router-brute mikrotik-v6 \
  --target 192.168.1.1 \
  --wordlist passwords.txt \
  --save-progress 30s  # Will be ignored in single-target mode
```

### Workaround

Convert single target to multi-target file:
```bash
echo "192.168.1.1:8728 admin" > target.txt

./router-brute mikrotik-v6 \
  --target-file target.txt \
  --wordlist passwords.txt \
  --save-progress 30s
```

## Future Enhancements

Potential improvements:
- [ ] Auto-resume from latest save file (`--resume auto`)
- [ ] Single-target mode support
- [ ] Compress resume files for large target lists
- [ ] Resume file browser/selector UI
- [ ] Differential saves (only changed targets)
- [ ] Resume file merging (combine multiple sessions)

## Examples

### Example 1: Basic Resume Workflow

```bash
# Start attack
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 30s

# Attack runs for a while...
# Press Ctrl+C to interrupt

# Resume from latest save
ls -lt ./resume/
# resume_20260105_152345.json  <- most recent

./router-brute mikrotik-v6 \
  --resume ./resume/resume_20260105_152345.json

# Attack continues from where it left off
```

### Example 2: Custom Save Interval

```bash
# Save every 10 seconds (frequent, for testing)
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 10s

# Save every 10 minutes (less frequent, production)
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 10m
```

### Example 3: Custom Save Directory

```bash
# Save to /tmp
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-dir /tmp/attack-progress
```

## Implementation Details

### Files Created

- `internal/core/resume.go` - Resume state management
- `internal/core/progress_tracker.go` - Progress tracking and auto-save
- Updated `internal/core/multi_engine.go` - Resume support in multi-target engine
- Updated `cmd/router-brute/main.go` - CLI flags and resume integration

### Key Components

**ResumeState**: Complete attack state
- Targets and their progress
- Configuration (workers, rate limit, etc.)
- File paths (targets, passwords)

**ProgressTracker**: Auto-save manager
- Background goroutine for periodic saves
- Thread-safe progress updates
- Configurable save interval

**MultiTargetEngine Integration**:
- Checks for existing progress on startup
- Skips completed targets
- Resumes from last password index
- Updates progress after each result

## Troubleshooting

### Resume File Not Found

```
Error: failed to load resume file: no such file or directory
```

**Solution**: Check path, ensure file exists:
```bash
ls -la ./resume/
./router-brute mikrotik-v6 --resume ./resume/<correct-filename>.json
```

### Password File Changed

```
Error: failed to load wordlist from resume state
```

**Solution**: Resume file stores absolute path. If you moved the password file:
1. Edit resume file JSON and update `password_file` path, OR
2. Move password file back to original location

### Invalid Resume File

```
Error: failed to parse resume file: invalid character...
```

**Solution**: Resume file corrupted. Try:
1. Use an earlier resume file
2. Start fresh attack without `--resume`

## Best Practices

1. **Regular Saves**: Use reasonable interval (30s-5m) based on attack speed
2. **Backup Resume Files**: Copy important resume files before resuming
3. **Clean Old Files**: Periodically delete old resume files to save space
4. **Absolute Paths**: Use absolute paths for password/target files for portability
5. **Test Resume**: Test resume functionality on small wordlists first

## Related Documentation

- [Connection Reuse Optimization](OPTIMIZATION_CONNECTION_REUSE.md)
- [Multi-Target Mode](BUG_FIX_MULTI_TARGET.md)
- [Target File Format](internal/core/target_parser.go)
