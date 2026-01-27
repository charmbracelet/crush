Search within a cached tool output using regex.

<usage>
- Provide the tool_call_id from a previous tool result
- Provide a regex pattern to search for
</usage>

<features>
- Find specific lines matching a pattern in large outputs
- Returns line numbers and matching content
- Limited to 100 matches to prevent huge responses
</features>

<tips>
- Use this to locate specific errors, warnings, or content
- Pattern is a regex - escape special characters if needed
- After finding matches, use output_head/output_tail with offset to see context
</tips>
