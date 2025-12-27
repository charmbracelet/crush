Find files by name or path pattern. Use this instead of `find` command.

<when_to_use>
Use Glob when:
- Finding files by name: "*.go", "config.*"
- Finding files in specific directories: "src/**/*.ts"
- Locating test files, configs, or specific extensions

Do NOT use Glob when:
- Searching file contents → use `grep`
- Need file contents → use `view` after finding
- Looking for symbol definitions → use `lsp_references`
</when_to_use>

<parameters>
- pattern: Glob pattern to match (required)
- path: Starting directory (default: current directory)
</parameters>

<pattern_syntax>
- `*` matches any characters except path separator
- `**` matches any characters including path separators
- `?` matches single character
- `{a,b}` matches alternatives
- `[abc]` matches character class
</pattern_syntax>

<examples>
`"*.go"` → Go files in current directory
`"**/*.go"` → Go files anywhere in tree
`"src/**/*.{ts,tsx}"` → TypeScript files in src
`"**/test_*.py"` → Python test files anywhere
`"config.*"` → Any file named config with any extension
</examples>

<output>
- Returns file paths sorted by modification time (newest first)
- Limited to 100 files
- Hidden files (starting with '.') skipped
</output>

<tip>
Combine with grep for efficient search: glob to find candidate files, grep to search their contents.
</tip>
