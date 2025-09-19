Fast file pattern matching tool that finds files by name and pattern, returning matching paths sorted by modification time (newest first).

WHEN TO USE THIS TOOL:

- Use when you need to find files by name patterns or extensions
- Great for finding specific file types across a directory structure
- Useful for discovering files that match certain naming conventions

HOW TO USE:

- Provide a glob pattern to match against file paths
- Optionally specify a starting directory (defaults to current working directory)
- Results are sorted with most recently modified files first

GLOB PATTERN SYNTAX:

- '\*' matches any sequence of non-separator characters
- '\*\*' matches any sequence of characters, including separators
- '?' matches any single non-separator character
- '[...]' matches any character in the brackets
- '[!...]' matches any character not in the brackets

COMMON PATTERN EXAMPLES:

- '\*.js' - Find all JavaScript files in the current directory
- '\*_/_.js' - Find all JavaScript files in any subdirectory
- 'src/\*_/_.{ts,tsx}' - Find all TypeScript files in the src directory
- '\*.{html,css,js}' - Find all HTML, CSS, and JS files

LIMITATIONS:

- Results are limited to 100 files (newest first)
- Does not search file contents (use Grep tool for that)
- Hidden files (starting with '.') are skipped

WINDOWS NOTES:

- Path separators are handled automatically (both / and \ work)
- Uses ripgrep (rg) command if available, otherwise falls back to built-in Go implementation

TIPS:

- Patterns should use forward slashes (/) for cross-platform compatibility
- For the most useful results, combine with the Grep tool: first find files with Glob, then search their contents with Grep
- When doing iterative exploration that may require multiple rounds of searching, consider using the Agent tool instead
- Always check if results are truncated and refine your search pattern if needed
