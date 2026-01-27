View the end of a cached tool output.

<usage>
- Provide the tool_call_id from a previous tool result
- Optional: specify number of lines (default: 100, max: 500)
- Optional: use offset for pagination (0 = last lines, higher = earlier)
</usage>

<features>
- View last N lines of large outputs (most recent output)
- Paginate backwards through output with offset parameter
- Shows total line count and pagination info
</features>

<tips>
- Use this when you need to see recent/final output
- Default view already shows last 100 lines, use this to see more
- Offset works backwards: offset=100 skips the last 100 lines
</tips>
