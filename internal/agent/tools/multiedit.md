Makes multiple edits to a single file in one operation. Built on Edit tool for efficient multiple find-and-replace operations. Prefer over Edit tool for multiple edits to same file.

<prerequisites>
1. Use View tool to understand file contents and context
2. Verify directory path is correct
</prerequisites>

<parameters>
1. file_path: Absolute path to file (required)
2. edits: Array of edit operations, each containing:
   - old_string: Text to replace (must match exactly including whitespace/indentation)
   - new_string: Replacement text
   - replace_all: Replace all occurrences (optional, defaults to false)
</parameters>

<operation>
- Edits applied sequentially in provided order
- Each edit operates on result of previous edit
- All edits must be valid for operation to succeed - if any fails, none applied
- Ideal for several changes to different parts of same file
</operation>

<critical_requirements>

1. All edits follow same requirements as single Edit tool
2. Edits are atomic - either all succeed or none applied
3. Plan edits carefully to avoid conflicts between sequential operations
   </critical_requirements>

<warnings>
- Tool fails if old_string doesn't match file contents exactly (including whitespace)
- Tool fails if old_string and new_string are identical
- Earlier edits may affect text that later edits try to find - plan sequence carefully
</warnings>

<best_practices>

- Ensure all edits result in correct, idiomatic code
- Don't leave code in broken state
- Use absolute file paths (starting with /)
- Use replace_all for renaming variables across file
- Avoid adding emojis unless user explicitly requests
  </best_practices>

<new_file_creation>

- Provide new file path (including directory if needed)
- First edit: empty old_string, new file contents as new_string
- Subsequent edits: normal edit operations on created content
  </new_file_creation>
