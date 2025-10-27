Fast file pattern matching tool that finds files by name/pattern, returning paths sorted by modification time (newest first).

<usage>
- Provide glob pattern to match against file paths
- Optional starting directory (defaults to current working directory)
- Results sorted with most recently modified files first
</usage>

<pattern_syntax>
- '\*' matches any sequence of non-separator characters
- '\*\*' matches any sequence including separators
- '?' matches any single non-separator character
- '[...]' matches any character in brackets
- '[!...]' matches any character not in brackets
</pattern_syntax>

<examples>
- '*.js' - JavaScript files in current directory
- '**/*.js' - JavaScript files in any subdirectory
- 'src/**/*.{ts,tsx}' - TypeScript files in src directory
- '*.{html,css,js}' - HTML, CSS, and JS files
</examples>

<limitations>
- Results limited to 100 files (newest first)
- Does not search file contents (use Grep for that)
- Hidden files (starting with '.') skipped
</limitations>

<cross_platform>
- Path separators handled automatically (/ and \ work)
- Uses ripgrep (rg) if available, otherwise Go implementation
- Patterns should use forward slashes (/) for compatibility
</cross_platform>

<tips>
- Combine with Grep: find files with Glob, search contents with Grep
- For iterative exploration requiring multiple searches, consider Agent tool
- Check if results truncated and refine pattern if needed
</tips>
