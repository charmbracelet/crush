Edits files by replacing text, creating new files, or deleting content. For moving/renaming use Bash 'mv'. For large edits use Write tool.

<prerequisites>
1. Use View tool to understand file contents and context
2. For new files: Use LS tool to verify parent directory exists
3. **CRITICAL**: Note exact whitespace, indentation, and formatting from View output
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
EXACT MATCHING: The tool is extremely literal. Text must match **EXACTLY**

- Every space and tab character
- Every blank line
- Every newline character
- Indentation level (count the spaces/tabs)
- Comment spacing (`// comment` vs `//comment`)
- Brace positioning (`func() {` vs `func(){`)

Common failures:

```
Expected: "    func foo() {"     (4 spaces)
Provided: "  func foo() {"       (2 spaces) ❌ FAILS

Expected: "}\n\nfunc bar() {"    (2 newlines)
Provided: "}\nfunc bar() {"      (1 newline) ❌ FAILS

Expected: "// Comment"           (space after //)
Provided: "//Comment"            (no space) ❌ FAILS
```

UNIQUENESS (when replace_all=false): old_string MUST uniquely identify target instance

- Include 3-5 lines context BEFORE and AFTER change point
- Include exact whitespace, indentation, surrounding code
- If text appears multiple times, add more context to make it unique

SINGLE INSTANCE: Tool changes ONE instance when replace_all=false

- For multiple instances: set replace_all=true OR make separate calls with unique context
- Plan calls carefully to avoid conflicts

VERIFICATION BEFORE USING: Before every edit

1. View the file and locate exact target location
2. Check how many instances of target text exist
3. Copy the EXACT text including all whitespace
4. Verify you have enough context for unique identification
5. Double-check indentation matches (count spaces/tabs)
6. Plan separate calls or use replace_all for multiple changes
   </critical_requirements>

<warnings>
Tool fails if:
- old_string matches multiple locations and replace_all=false
- old_string doesn't match exactly (including whitespace)
- Insufficient context causes wrong instance change
- Indentation is off by even one space
- Missing or extra blank lines
- Wrong tabs vs spaces
</warnings>

<recovery_steps>
If you get "old_string not found in file":

1. **View the file again** at the specific location
2. **Copy more context** - include entire function if needed
3. **Check whitespace**:
   - Count indentation spaces/tabs
   - Look for blank lines
   - Check for trailing spaces
4. **Verify character-by-character** that your old_string matches
5. **Never guess** - always View the file to get exact text
   </recovery_steps>

<best_practices>

- Ensure edits result in correct, idiomatic code
- Don't leave code in broken state
- Use absolute file paths (starting with /)
- Use forward slashes (/) for cross-platform compatibility
- Multiple edits to same file: send all in single message with multiple tool calls
- **When in doubt, include MORE context rather than less**
- Match the existing code style exactly (spaces, tabs, blank lines)
  </best_practices>

<whitespace_checklist>
Before submitting an edit, verify:

- [ ] Viewed the file first
- [ ] Counted indentation spaces/tabs
- [ ] Included blank lines if they exist
- [ ] Matched brace/bracket positioning
- [ ] Included 3-5 lines of surrounding context
- [ ] Verified text appears exactly once (or using replace_all)
- [ ] Copied text character-for-character, not approximated
      </whitespace_checklist>

<examples>
✅ Correct: Exact match with context

```
old_string: "func ProcessData(input string) error {\n    if input == \"\" {\n        return errors.New(\"empty input\")\n    }\n    return nil\n}"

new_string: "func ProcessData(input string) error {\n    if input == \"\" {\n        return errors.New(\"empty input\")\n    }\n    // New validation\n    if len(input) > 1000 {\n        return errors.New(\"input too long\")\n    }\n    return nil\n}"
```

❌ Incorrect: Not enough context

```
old_string: "return nil"  // Appears many times!
```

❌ Incorrect: Wrong indentation

```
old_string: "  if input == \"\" {"  // 2 spaces
// But file actually has:        "    if input == \"\" {"  // 4 spaces
```

✅ Correct: Including context to make unique

```
old_string: "func ProcessData(input string) error {\n    if input == \"\" {\n        return errors.New(\"empty input\")\n    }\n    return nil"
```

</examples>

<windows_notes>

- Forward slashes work throughout (C:/path/file)
- File permissions handled automatically
- Line endings converted automatically (\n ↔ \r\n)
  </windows_notes>
