Edits files by replacing text, creating new files, or deleting content. For moving/renaming use Bash 'mv'. For large edits use Write tool.

<prerequisites>
1. Use View tool to understand file contents and context
2. For new files: Use LS tool to verify parent directory exists
</prerequisites>

<parameters>
1. file_path: Absolute path to file (required)
2. old_string: Text to replace (must match exactly including whitespace/indentation)
3. new_string: Replacement text
4. replace_all: Replace all occurrences (default false)
</parameters>

<special_cases>

- Create file: provide file_path + new_string, leave old_string empty
- Delete content: provide file_path + old_string, leave new_string empty
  </special_cases>

<critical_requirements>
UNIQUENESS (when replace_all=false): old_string MUST uniquely identify target instance

- Include 3-5 lines context BEFORE and AFTER change point
- Include exact whitespace, indentation, surrounding code

SINGLE INSTANCE: Tool changes ONE instance when replace_all=false

- For multiple instances: set replace_all=true OR make separate calls with unique context
- Plan calls carefully to avoid conflicts

VERIFICATION: Before using

- Check how many instances of target text exist
- Gather sufficient context for unique identification
- Plan separate calls or use replace_all
  </critical_requirements>

<warnings>
Tool fails if:
- old_string matches multiple locations and replace_all=false
- old_string doesn't match exactly (including whitespace)
- Insufficient context causes wrong instance change
</warnings>

<best_practices>

- Ensure edits result in correct, idiomatic code
- Don't leave code in broken state
- Use absolute file paths (starting with /)
- Use forward slashes (/) for cross-platform compatibility
- Multiple edits to same file: send all in single message with multiple tool calls
  </best_practices>

<windows_notes>

- Forward slashes work throughout (C:/path/file)
- File permissions handled automatically
- Line endings converted automatically (\n ↔ \r\n)
  </windows_notes>
