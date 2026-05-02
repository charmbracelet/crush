Read a file by path with line numbers; supports offset and line limit (default 2000, max 200KB returned file content section); renders images (PNG, JPEG, GIF, BMP, SVG, WebP); use ls for directories.

<usage>
- Provide file path to read
- Optional offset: start reading from specific line (0-based)
- Optional limit: control lines read (default 2000)
- Don't use for directories (use LS tool instead)
- Supports image files (PNG, JPEG, GIF, BMP, SVG, WebP)
</usage>

<features>
- Displays contents with line numbers
- Can read from any file position using offset
- Handles large files by limiting lines read
- Auto-truncates very long lines for display
- Suggests similar filenames when file not found
- Renders image files directly in terminal
</features>

<limitations>
- Max returned file content section: 200KB after offset/limit are applied
- Default limit: 2000 lines
- Lines >2000 chars truncated
- Binary files (except images) cannot be displayed
</limitations>

<cross_platform>
- Handles Windows (CRLF) and Unix (LF) line endings
- Works with forward slashes (/) and backslashes (\)
- Auto-detects text encoding for common formats
</cross_platform>

<tips>
- Use with Glob to find files first
- For code exploration: Grep to find relevant files, then View to examine
- For large files: use offset parameter for specific sections
- View tool automatically detects and renders image files
</tips>
