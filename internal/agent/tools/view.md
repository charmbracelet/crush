Reads and displays file contents with line numbers for examining code, logs, or text data.

<usage>
- Provide file path to read
- Optional offset: start reading from specific line (0-based)
- Optional limit: control lines read (default 2000)
- Don't use for directories (use LS tool instead)
</usage>

<features>
- Displays contents with line numbers
- Can read from any file position using offset
- Handles large files by limiting lines read
- Auto-truncates very long lines for display
- Suggests similar filenames when file not found
</features>

<limitations>
- Max file size: 250KB
- Default limit: 2000 lines
- Lines >2000 chars truncated
- Cannot display binary files/images (identifies them)
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
</tips>
