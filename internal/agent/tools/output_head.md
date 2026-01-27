View the beginning of a cached tool output.

<usage>
- Provide the tool_call_id from a previous tool result
- Optional: specify number of lines (default: 100, max: 500)
- Optional: use offset for pagination
</usage>

<features>
- View first N lines of large outputs
- Paginate through output with offset parameter
- Shows total line count and pagination info
</features>

<tips>
- Use this when you need to see the start of a large output
- Combine with output_grep to find specific content first
- The tool_call_id comes from the tool result you want to explore
</tips>
