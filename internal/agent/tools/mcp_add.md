Add, replace, start, and verify one exact user-requested MCP server.

Inspect current state with `mcp_manage` using `action=status`, then set exactly
one transport object. Use the native `recode_info` tool only when canonical
configuration details are required; never invoke it through Bash or `skill`.
`source_url` is optional supporting context shown during approval; a
failed or unavailable documentation fetch must not prevent an otherwise exact
configuration from being proposed to the user.

- `stdio`: `command`, optional `args`, and optional `env`.
- `http`: `url` and optional `headers`.
- `sse`: `url` and optional `headers`.

Transport fields cannot be mixed. Correcting a different saved configuration
requires `replace=true`. The exact configuration and scope require user
approval before Crush writes or starts anything. Dependency and credential
failures require a changed strategy or exact user input. A failed candidate is
rolled back and returned as a recoverable error; do not repeat the same call.
