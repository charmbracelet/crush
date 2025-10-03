Creates or updates files in filesystem for saving/modifying text content.

<usage>
- Provide file path to write
- Include content to write to file
- Tool creates necessary parent directories automatically
</usage>

<features>
- Creates new files or overwrites existing ones
- Auto-creates parent directories if missing
- Checks if file modified since last read for safety
- Avoids unnecessary writes when content unchanged
</features>

<limitations>
- Read file before writing to avoid conflicts
- Cannot append (rewrites entire file)
</limitations>

<cross_platform>
- Use forward slashes (/) for compatibility
</cross_platform>

<tips>
- Use View tool first to examine existing files before modifying
- Use LS tool to verify location when creating new files
- Combine with Glob/Grep to find and modify multiple files
- Include descriptive comments when changing existing code
</tips>
