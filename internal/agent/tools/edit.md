Edit files by replacing text. For moving/renaming use Bash 'mv'. For large edits use Write tool.

<parameters>
- file_path: Absolute path (required)
- old_string: Text to replace (must match EXACTLY including whitespace)
- new_string: Replacement text
- replace_all: Replace all occurrences (default false)

Create file: provide file_path + new_string, leave old_string empty.
Delete content: provide file_path + old_string, leave new_string empty.
</parameters>

<critical>
EXACT MATCHING: Text must match character-for-character:
- Every space, tab, blank line, newline
- Indentation level (count spaces/tabs)
- Comment spacing (`// comment` vs `//comment`)
- Brace positioning (`func() {` vs `func(){`)

UNIQUENESS: old_string must uniquely identify target. Include 3-5 lines context before/after.

Before editing:
1. View file and locate exact target
2. Copy EXACT text including all whitespace
3. Verify text appears exactly once
4. Double-check indentation (count spaces)
</critical>

<recovery>
If "old_string not found":
1. View file again at target location
2. Copy more context (entire function if needed)
3. Check tabs vs spaces, blank lines
4. Never guess—get exact text
</recovery>
