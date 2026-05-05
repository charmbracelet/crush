Create an empty file; auto-creates parent dirs. Fails if the file already exists.

<usage>
- Provide file path to create
- Tool creates necessary parent directories automatically
</usage>

<features>
- Creates new empty files
- Auto-creates parent directories if missing
- Refuses to overwrite existing files
</features>

<limitations>
- Cannot write content
- Cannot update modification times for existing files
- Cannot create directories
</limitations>

<cross_platform>
- Use forward slashes (/) for compatibility
</cross_platform>

<tips>
- Use Write tool when the file should contain content
- Use LS tool to verify location when creating new files
</tips>
