Fetches the complete assistant response from a child session created by a previous Agent tool call.

When an Agent tool returns a truncated response with a child session ID, use this tool to retrieve the full output.

Parameters:
- session_id (required): The child session ID from the Agent tool result (format: "messageID$$toolCallID")
- offset (optional): Character offset to start from for pagination (0-based)
- limit (optional): Maximum characters to return (default 16000, max 64000)

Usage notes:
- Use this when you need more detail from a subagent's work than the summary provided in the Agent tool response
- The response may still be truncated if very long; use offset/limit to paginate
- This only retrieves the final assistant response, not intermediate tool calls