Edit files using find-and-replace. Must read file first with `view`.

<when_to_use>
Use Edit when:
- Making targeted changes to existing code
- Changing specific functions, lines, or blocks
- Single file, 1-3 changes

Do NOT use Edit when:
- Creating new files → use `write`
- Complete file rewrite → use `write`
- Multiple changes to same file → use `multiedit`
- Moving/renaming files → use `bash` with `mv`
</when_to_use>

<critical_rule>
**ALWAYS `view` the file first.** The old_string must match EXACTLY—every space, tab, newline, and blank line.
</critical_rule>

<parameters>
- file_path: Absolute path (required)
- old_string: Exact text to find (required for edits, empty for new file)
- new_string: Replacement text (required)
- replace_all: Replace all occurrences (default: false)
</parameters>

<special_cases>
- Create file: provide file_path + new_string, leave old_string empty
- Delete content: provide file_path + old_string, leave new_string empty
</special_cases>

<matching_rules>
Include 3-5 lines of context to ensure unique match:

```
Good: Match entire function signature + first lines
old_string: "func ProcessUser(id string) error {\n    if id == \"\" {\n        return errors.New(\"empty\")\n    }"

Bad: Match just one line that appears many times
old_string: "return nil"
```

**Tip:** In large files, include the function or class signature as context to disambiguate similar code blocks.
</matching_rules>

<common_failures>
```
Expected: "    func foo() {"     (4 spaces)
Provided: "  func foo() {"       (2 spaces) ❌

Expected: "}\n\nfunc bar()"      (blank line between)
Provided: "}\nfunc bar()"        (no blank line) ❌

Expected: "// comment"           (space after //)
Provided: "//comment"            ❌
```
</common_failures>

<recovery>
If "old_string not found":
1. `view` the file at target location
2. Copy exact text character-by-character
3. Include more surrounding context
4. Check tabs vs spaces, blank lines
</recovery>
