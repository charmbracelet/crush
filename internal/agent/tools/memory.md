Manages persistent long-term memory entries across sessions.

<usage>
- Set `action` to one of: `store`, `get`, `delete`, `search`, `list`.
- `store` requires `key` and `value`.
- `get` and `delete` require `key`.
- `search` requires `query`; `list` does not.
- `limit` is optional for `search` and `list`.
</usage>

<parameters>
- action: Operation to perform (`store`, `get`, `delete`, `search`, `list`) (required)
- key: Memory key for `store`, `get`, `delete`
- value: Memory value for `store`
- query: Text query for `search`
- limit: Maximum entries to return for `search`/`list`
</parameters>

<notes>
- Entries are stored in local data directory and persist across sessions.
- Results are sorted by most recently updated first.
- Write operations (`store`, `delete`) require permission checks.
