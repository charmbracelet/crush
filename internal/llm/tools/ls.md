Directory listing tool that shows files and subdirectories in a tree structure, helping you explore and understand the project organization.

WHEN TO USE THIS TOOL:

- Use when you need to explore the structure of a directory
- Helpful for understanding the organization of a project
- Good first step when getting familiar with a new codebase

HOW TO USE:

- Provide a path to list (defaults to current working directory)
- Optionally specify glob patterns to ignore
- Results are displayed in a tree structure

FEATURES:

- Displays a hierarchical view of files and directories
- Automatically skips hidden files/directories (starting with '.')
- Skips common system directories like **pycache**
- Can filter out files matching specific patterns

LIMITATIONS:

- Results are limited to 1000 files
- Very large directories will be truncated
- Does not show file sizes or permissions
- Cannot recursively list all directories in a large project

WINDOWS NOTES:

- Hidden file detection uses Unix convention (files starting with '.')
- Windows-specific hidden files (with hidden attribute) are not automatically skipped
- Common Windows directories like System32, Program Files are not in default ignore list
- Path separators are handled automatically (both / and \ work)

TIPS:

- Use Glob tool for finding files by name patterns instead of browsing
- Use Grep tool for searching file contents
- Combine with other tools for more effective exploration
