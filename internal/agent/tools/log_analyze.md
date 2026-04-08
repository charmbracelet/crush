# Log Analyze Tool

Analyzes system and application log files to identify patterns, anomalies, and issues.

## Capabilities
- Search through log files with regex patterns
- Filter by severity level (ERROR, WARN, INFO, DEBUG)
- Time-range based analysis
- Aggregate results by message, hour, host, or service
- Supports syslog, journalctl, and standard log file formats

## Usage
- Use `source` to specify the log file path (e.g., `/var/log/syslog`) or systemd unit (e.g., `systemd:nginx`)
- Use `pattern` to filter entries by regex
- Use `time_range` to limit the time window (e.g., `15m`, `1h`, `24h`, `7d`)
- Use `severity` to filter by log level
- Use `aggregate_by` to group and count results

## Security
- This tool operates in read-only mode
- All operations are sandboxed with resource limits
- Every invocation is recorded in the audit log
- Requires appropriate permissions before execution
