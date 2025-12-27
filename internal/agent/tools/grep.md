Search file contents for text or patterns. Use this instead of shell `grep`.

<when_to_use>
Use Grep when:
- Searching for text/patterns across files
- Finding where a function or variable is used
- Locating error messages, log strings, or comments

Do NOT use Grep when:
- Finding files by name → use `glob`
- Semantic symbol lookup → use `lsp_references` (more accurate)
- Need to understand code flow → use `agent`
- Reading a known file → use `view`
</when_to_use>

<parameters>
- pattern: Regex pattern (or literal text with literal_text=true)
- path: Directory to search (default: current directory)
- include: File pattern filter, e.g., "*.go", "*.{ts,tsx}"
- literal_text: Set true for exact text with special chars (dots, parens)
</parameters>

<pattern_tips>
- Simple text: `"handleLogin"` finds literal matches
- Regex: `"log\..*Error"` finds log.SomethingError
- Use `literal_text=true` for text with special chars: `"user.name"` with literal_text finds "user.name" exactly
</pattern_tips>

<output>
- Returns matching file paths sorted by modification time (newest first)
- Limited to 100 files - if results show "at least N matches", refine your query
- Respects .gitignore and .crushignore
</output>

<examples>
Good: `pattern: "func.*Config", include: "*.go"` → Find Go functions with Config in name

Good: `pattern: "TODO", path: "src/"` → Find TODOs in src directory

Bad: `pattern: "*.go"` → This searches content, not filenames. Use `glob` for filenames.
</examples>
