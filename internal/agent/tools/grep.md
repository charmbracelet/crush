Fast content search tool that finds files containing specific text/patterns, returning matching paths sorted by modification time (newest first).

<usage>
- Provide regex pattern to search within file contents
- Set literal_text=true for exact text with special characters (recommended for non-regex users)
- Optional starting directory (defaults to current working directory)
- Optional include pattern to filter which files to search
- Results sorted with most recently modified files first
</usage>

<regex_syntax>
When literal_text=false (supports standard regex):

- 'function' searches for literal text "function"
- 'log\..\*Error' finds text starting with "log." and ending with "Error"
- 'import\s+.\*\s+from' finds import statements in JavaScript/TypeScript
</regex_syntax>

<include_patterns>
- '\*.js' - Only search JavaScript files
- '\*.{ts,tsx}' - Only search TypeScript files
- '\*.go' - Only search Go files
</include_patterns>

<limitations>
- Results limited to 100 files (newest first)
- Performance depends on number of files searched
- Very large binary files may be skipped
- Hidden files (starting with '.') skipped
</limitations>

<ignore_support>
- Respects .gitignore patterns to skip ignored files/directories
- Respects .crushignore patterns for additional ignore rules
- Both ignore files auto-detected in search root directory
</ignore_support>

<cross_platform>
- Uses ripgrep (rg) if available for better performance
- Falls back to Go implementation if ripgrep unavailable
- File paths normalized automatically for compatibility
</cross_platform>

<tips>
- For faster searches: use Glob to find relevant files first, then Grep
- For iterative exploration requiring multiple searches, consider Agent tool
- Check if results truncated and refine search pattern if needed
- Use literal_text=true for exact text with special characters (dots, parentheses, etc.)
</tips>
