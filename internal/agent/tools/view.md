Read file contents. Use this instead of `cat`, `head`, or `tail` commands. **Only works on files, not directories.**

<when_to_use>
Use View when:
- Reading any file before editing
- Examining code, configs, logs, or data files
- Checking file contents after changes
- Viewing images (PNG, JPEG, GIF, WebP supported)

Do NOT use View when:
- **Listing directory contents → use `ls`** (View fails on directories)
- Finding files by name → use `glob`
- Searching file contents → use `grep`
</when_to_use>

<parameters>
- file_path: Path to file (required)
- offset: Start line, 0-based (optional, for large files)
- limit: Number of lines (default 2000)
</parameters>

<output>
- Lines prefixed with "L123:" line numbers
- Treat "Lxxx:" as metadata, not actual code
- Long lines (>2000 chars) truncated
- Binary files show error (except images)
</output>

<limits>
- Max file size: 5MB
- Default: 2000 lines
- Hidden files readable
</limits>

<tips>
- Always view before editing to get exact whitespace
- For large files, use offset to read specific sections
- Use with grep: find files first, then view relevant ones
- Suggests similar filenames if file not found
</tips>
