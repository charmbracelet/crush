Read Crush's internal application logs (default 50 entries, max 100); useful for diagnosing provider errors, tool failures, LSP/MCP issues.

<usage>
- Returns recent log entries from Crush's internal log file
- Use to diagnose issues with Crush itself (provider errors, tool failures,
  LSP problems, MCP connection issues)
- Entries shown in compact format: TIME LEVEL SOURCE MESSAGE key=value...
</usage>

<tips>
- Default returns last 50 entries; use lines parameter for more (max 100)
- Look for ERROR and WARN entries first when diagnosing problems
</tips>
