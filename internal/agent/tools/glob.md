Find files by name/pattern (glob syntax), sorted by modification time; max 100 results; skips hidden files. Use grep to search file contents.

<parameters>
- pattern: Glob pattern (required)
- path: Directory to search (defaults to cwd)
</parameters>

<pattern_syntax>
- `*` — any sequence of non-separator characters
- `**` — any sequence including directory separators
- `?` — single non-separator character
- `[...]` / `[!...]` — character class / negated class

Examples: `*.js`, `**/*.go`, `src/**/*.{ts,tsx}`
</pattern_syntax>
