Fast content search tool that finds files containing specific text or patterns, returning matching file paths sorted by modification time (newest first).

WHEN TO USE THIS TOOL:

- Use when you need to find files containing specific text or patterns
- Great for searching code bases for function names, variable declarations, or error messages
- Useful for finding all files that use a particular API or pattern

HOW TO USE:

- Provide a regex pattern to search for within file contents
- Set literal_text=true if you want to search for the exact text with special characters (recommended for non-regex users)
- Optionally specify a starting directory (defaults to current working directory)
- Optionally provide an include pattern to filter which files to search
- Results are sorted with most recently modified files first

REGEX PATTERN SYNTAX (when literal_text=false):

- Supports standard regular expression syntax
- 'function' searches for the literal text "function"
- 'log\..\*Error' finds text starting with "log." and ending with "Error"
- 'import\s+.\*\s+from' finds import statements in JavaScript/TypeScript

COMMON INCLUDE PATTERN EXAMPLES:

- '\*.js' - Only search JavaScript files
- '\*.{ts,tsx}' - Only search TypeScript files
- '\*.go' - Only search Go files

LIMITATIONS:

- Results are limited to 100 files (newest first)
- Performance depends on the number of files being searched
- Very large binary files may be skipped
- Hidden files (starting with '.') are skipped

IGNORE FILE SUPPORT:

- Respects .gitignore patterns to skip ignored files and directories
- Respects .crushignore patterns for additional ignore rules
- Both ignore files are automatically detected in the search root directory

CROSS-PLATFORM NOTES:

- Uses ripgrep (rg) command if available for better performance
- Falls back to built-in Go implementation if ripgrep is not available
- File paths are normalized automatically for cross-platform compatibility

TIPS:

- For faster, more targeted searches, first use Glob to find relevant files, then use Grep
- When doing iterative exploration that may require multiple rounds of searching, consider using the Agent tool instead
- Always check if results are truncated and refine your search pattern if needed
- Use literal_text=true when searching for exact text containing special characters like dots, parentheses, etc.
