File writing tool that creates or updates files in the filesystem, allowing you to save or modify text content.

WHEN TO USE THIS TOOL:

- Use when you need to create a new file
- Helpful for updating existing files with modified content
- Perfect for saving generated code, configurations, or text data

HOW TO USE:

- Provide the path to the file you want to write
- Include the content to be written to the file
- The tool will create any necessary parent directories

FEATURES:

- Can create new files or overwrite existing ones
- Creates parent directories automatically if they don't exist
- Checks if the file has been modified since last read for safety
- Avoids unnecessary writes when content hasn't changed

LIMITATIONS:

- You should read a file before writing to it to avoid conflicts
- Cannot append to files (rewrites the entire file)

WINDOWS NOTES:

- File permissions (0o755, 0o644) are Unix-style but work on Windows with appropriate translations
- Use forward slashes (/) in paths for cross-platform compatibility
- Windows file attributes and permissions are handled automatically by the Go runtime

TIPS:

- Use the View tool first to examine existing files before modifying them
- Use the LS tool to verify the correct location when creating new files
- Combine with Glob and Grep tools to find and modify multiple files
- Always include descriptive comments when making changes to existing code
