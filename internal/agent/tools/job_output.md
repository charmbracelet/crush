Get stdout/stderr from a background shell by ID. Set wait=true to block until completion.

<parameters>
- shell_id: Background shell ID (required)
- wait: Block until shell completes (default false)
</parameters>

Returns current output, done status, and exit code. Call multiple times for incremental output. Use wait=true when you need final output.
