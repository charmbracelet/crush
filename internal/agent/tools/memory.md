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

<memory_types>
Use the `type` field to classify what kind of memory you are storing:

- `user` — User identity, preferences, and work style (e.g. preferred language, formatting conventions, timezone). Always private per-user.
- `feedback` — Corrections and confirmations about how to work (e.g. "always run tests before committing", "user prefers concise responses"). Save when the user corrects a mistake or confirms a good approach.
- `project` — Ongoing work state: decisions made, patterns in use, milestones reached, key file paths. Use `scope: project` to isolate from other projects.
- `reference` — Pointers to external systems or resources (e.g. API endpoints, Jira board URLs, database schema locations).
</memory_types>

<when_to_save>
Save to memory when you observe something that will be useful in future sessions:
- The user states a persistent preference or constraint ("always use tabs", "don't use lodash").
- You discover a non-obvious project pattern or architecture decision.
- A command or workflow is confirmed to work and is likely to be needed again.
- The user corrects your behavior in a way that should be remembered.
- Important project context that would take significant effort to re-discover.
</when_to_save>

<what_not_to_save>
Do NOT save:
- Information already in the codebase (code patterns, comments, documentation).
- Transient state: current task progress, which files were opened this session.
- Generic knowledge about programming languages, frameworks, or tools.
- Large content that belongs in files (save a pointer, not the content itself).
- Sensitive credentials or secrets.
</what_not_to_save>

<notes>
- Entries are stored in local data directory and persist across sessions.
- Results are sorted by most recently updated first.
- Search matches key, value, scope, category, type, and tags.
- `category`, `type`, and `tags` can also be used as exact-match filters for `search` and `list`.
- Write operations (`store`, `delete`) require permission checks.
- **Drift caveat**: Memories may become stale. Before acting on a recalled memory about file locations or function names, verify the information still holds in the current codebase.
</notes>
