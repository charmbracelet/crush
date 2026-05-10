Edit a file by exact find-and-replace; can also create or delete content. For renames/moves use bash. For large edits use write.

<parameters>
- file_path: Absolute path to file (required)
- old_string: Text to replace (required for edits; empty to create)
- new_string: Replacement text (required for edits; empty to delete)
- replace_all: Replace all occurrences (default false)
</parameters>

<critical_requirements>
**EXACT MATCHING**: The tool is extremely literal. Text must match exactly — every space, tab, blank line, and indentation level. "Close enough" will fail.

Common failures:
- Wrong indentation: 2 spaces vs 4 spaces vs tabs
- Missing or extra blank lines between functions/blocks
- Brace positioning: `func foo() {` vs `func foo(){`
- Comment spacing: `// comment` vs `//comment`

**UNIQUENESS** (when replace_all=false): old_string MUST uniquely identify a single target. Include 3-5 lines of surrounding context. If the text appears multiple times, add more context.

**BEFORE EVERY EDIT**: View the file, copy the exact text character-for-character, count indentation, verify uniqueness, include surrounding context. If edit fails, View again — never guess the text.
</critical_requirements>

<examples>
✅ Unique match with context:
old_string: "func ProcessData(input string) error {\n    if input == \"\" {\n        return errors.New(\"empty input\")\n    }\n    return nil\n}"

❌ Not unique:
old_string: "return nil"  // Appears many times
</examples>
