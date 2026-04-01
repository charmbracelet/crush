Searches messages from session history using a text query.

<usage>
- Provide `query` to search message text content.
- Optionally provide `session_id` to scope results to one session; omit for global search.
- Optionally provide `limit` (default 10, max 100).
</usage>

<parameters>
- query: Text to search for in message content (required)
- session_id: Session ID filter (optional)
- limit: Maximum results to return (optional)
</parameters>

<notes>
- Matches are case-insensitive substring matches.
- Results are returned newest first.
- Output is a concise summary to help locate relevant messages quickly.
