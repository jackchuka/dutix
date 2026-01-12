# Logging

## Overview

Dutix maintains logs at `~/.dutix/dutix.log` to help with troubleshooting and debugging.

## Log Rotation

To prevent logs from growing indefinitely, Dutix implements automatic log rotation:

### Size-Based Rotation
- When `dutix.log` exceeds **10MB**, it's automatically rotated
- Rotated logs are renamed: `dutix.log.1`, `dutix.log.2`, `dutix.log.3`
- Maximum of **3 old log files** are kept
- Oldest logs are deleted when the limit is reached

### Time-Based Cleanup
- On each startup, logs older than **30 days** are automatically deleted
- This prevents accumulation of old rotated logs

## Log Locations

```
~/.dutix/
├── dutix.log           # Current log file
├── dutix.log.1         # Most recent rotated log
├── dutix.log.2         # Second most recent
└── dutix.log.3         # Oldest rotated log
```

## Debug Mode

Enable debug logging for more detailed output:

```bash
# Logs to both file and stderr
./dutix --debug
```

Debug mode includes:
- All bridge API calls (GetDefaultApp, SetDefaultApp, etc.)
- Detailed operation timing
- State transitions
- Internal operations

## Viewing Logs

### View current log
```bash
tail -f ~/.dutix/dutix.log
```

### View recent entries
```bash
tail -50 ~/.dutix/dutix.log
```

### View all logs (including rotated)
```bash
cat ~/.dutix/dutix.log*
```

### Search logs
```bash
grep "ERROR" ~/.dutix/dutix.log*
```

## Log Format

Logs follow this format:
```
2026/01/11 15:30:45 [LEVEL] Message key=value key2=value2
```

Example:
```
2026/01/11 15:30:45 [INFO] Applying item kind=uti identifier=public.plain-text app=/Applications/Visual Studio Code.app
2026/01/11 15:30:45 [INFO] Item applied successfully identifier=public.plain-text app=/Applications/Visual Studio Code.app
```

## Log Levels

- **INFO**: General information about operations
- **DEBUG**: Detailed debugging information (only with --debug)
- **WARN**: Warnings that don't prevent operation
- **ERROR**: Errors that caused operation failure

## Manual Cleanup

To manually clean up all logs:

```bash
rm ~/.dutix/dutix.log*
```

Logs will be recreated on next run.

## Troubleshooting

### Disk Space Issues

If logs are taking up too much space:

1. Check current log sizes:
```bash
du -sh ~/.dutix/dutix.log*
```

2. Reduce retention period in code (edit `internal/logger/logger.go`):
```go
const (
    maxLogSize = 5 * 1024 * 1024  // Reduce to 5MB
    maxOldLogs = 2                // Keep only 2 old logs
)
```

3. Or manually delete old logs:
```bash
rm ~/.dutix/dutix.log.[2-9]*
```

### Log Permissions

If you see "Permission denied" errors:

```bash
chmod 644 ~/.dutix/dutix.log*
```

### Rotation Issues

If rotation fails, check permissions on the `.dutix` directory:

```bash
chmod 755 ~/.dutix
```

## Configuration

Log settings are defined in `internal/logger/logger.go`:

```go
const (
    maxLogSize = 10 * 1024 * 1024 // 10MB - Size before rotation
    maxOldLogs = 3                // Keep 3 old log files
)

// In Init():
CleanOldLogs(30) // Delete logs older than 30 days
```

Modify these constants to adjust rotation behavior.
