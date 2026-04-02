Manages persistent long-term memory entries across sessions.

<usage>
- Set `action` to one of: `store`, `get`, `delete`, `search`, `list`.
- `store` requires `key` and `value`.
- `get` and `delete` require `key`.
- `search` requires `query`; `list` does not.
- `category`, `type`, and `tags` are optional metadata for `store`, `search`, and `list`.
- `limit` is optional for `search` and `list`.
</usage>

<parameters>
- action: Operation to perform (`store`, `get`, `delete`, `search`, `list`) (required)
- key: Memory key for `store`, `get`, `delete`
- value: Memory value for `store`
- scope: Optional memory scope such as `session` or `project`
- category: Optional higher-level grouping for a memory entry
- type: Optional memory type or subtype
- tags: Optional list of tags for the entry
- query: Text query for `search`
- limit: Maximum entries to return for `search`/`list`
</parameters>

<notes>
- Entries are stored in local data directory and persist across sessions.
- Results are sorted by most recently updated first.
- Search matches key, value, scope, category, type, and tags.
- `category`, `type`, and `tags` can also be used as exact-match filters for `search` and `list`.
- Write operations (`store`, `delete`) require permission checks.
