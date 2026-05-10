Search file contents by regex or literal text; returns matching file paths sorted by modification time (max 100); respects .gitignore. Use glob to filter by filename.

<parameters>
- pattern: Regex pattern (required)
- literal_text: Treat pattern as exact text, escaping special chars (default false)
- path: Directory to search (defaults to cwd)
- include: File pattern filter, e.g. "*.go", "*.{ts,tsx}"

Examples with literal_text=false: `function`, `log\..*Error`, `import\s+.*\s+from`
</parameters>
