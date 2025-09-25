Edits files by replacing text, creating new files, or deleting content. For moving or renaming files, use the Bash tool with the 'mv' command instead. For larger file edits, use the FileWrite tool to overwrite files.

Before using this tool:

1. Use the FileRead tool to understand the file's contents and context

2. Verify the directory path is correct (only applicable when creating new files):
   - Use the LS tool to verify the parent directory exists and is the correct location

To make a file edit, provide the following:

1. file_path: The absolute path to the file to modify (must be absolute, not relative)
2. old_string: The text to replace (must be unique within the file, and must match the file contents exactly, including all whitespace and indentation)
3. new_string: The edited text to replace the old_string
4. replace_all: Replace all occurrences of old_string (default false)

Special cases:

- To create a new file: provide file_path and new_string, leave old_string empty
- To delete content: provide file_path and old_string, leave new_string empty

The tool will replace ONE occurrence of old_string with new_string in the specified file by default. Set replace_all to true to replace all occurrences.

CRITICAL REQUIREMENTS FOR USING THIS TOOL:

1. UNIQUENESS: When replace_all is false (default), the old_string MUST uniquely identify the specific instance you want to change. This means:
   - Include AT LEAST 3-5 lines of context BEFORE the change point
   - Include AT LEAST 3-5 lines of context AFTER the change point
   - Include all whitespace, indentation, and surrounding code exactly as it appears in the file

2. SINGLE INSTANCE: When replace_all is false, this tool can only change ONE instance at a time. If you need to change multiple instances:
   - Set replace_all to true to replace all occurrences at once
   - Or make separate calls to this tool for each instance
   - Each call must uniquely identify its specific instance using extensive context

3. VERIFICATION: Before using this tool:
   - Check how many instances of the target text exist in the file
   - If multiple instances exist and replace_all is false, gather enough context to uniquely identify each one
   - Plan separate tool calls for each instance or use replace_all

WARNING: If you do not follow these requirements:

- The tool will fail if old_string matches multiple locations and replace_all is false
- The tool will fail if old_string doesn't match exactly (including whitespace)
- You may change the wrong instance if you don't include enough context

When making edits:

- Ensure the edit results in idiomatic, correct code
- Do not leave the code in a broken state
- Always use absolute file paths (starting with /)

WINDOWS NOTES:

- File paths should use forward slashes (/) for cross-platform compatibility
- On Windows, absolute paths start with drive letters (C:/) but forward slashes work throughout
- File permissions are handled automatically by the Go runtime
- Always assumes \n for line endings. The tool will handle \r\n conversion automatically if needed.

Remember: when making multiple file edits in a row to the same file, you should prefer to send all edits in a single message with multiple calls to this tool, rather than multiple messages with a single call each.
