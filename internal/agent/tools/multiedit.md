Make multiple edits to a single file in one operation. Prefer over `edit` for multiple changes.

<when_to_use>
Use MultiEdit when:
- 2+ changes to the same file
- Changes are in different parts of the file
- Want atomic success/failure per edit

Do NOT use MultiEdit when:
- Single change → use `edit`
- Different files → use separate `edit` calls
- Complete rewrite → use `write`
</when_to_use>

<parameters>
- file_path: Absolute path (required)
- edits: Array of {old_string, new_string, replace_all} objects
</parameters>

<critical_rules>
1. **View first**: Read the file before editing
2. **Exact match**: Same rules as `edit` - whitespace matters
3. **Sequential**: Edits apply in order; each operates on result of previous
4. **Partial success**: If edit #2 fails, edit #1 is still applied
5. **Plan ahead**: Earlier edits change content that later edits must match
</critical_rules>

<common_mistake>
Edit #1 adds a blank line. Edit #2 tries to match old content that no longer exists:

```
❌ Wrong:
edits: [
  { old_string: "func A() {", new_string: "func A() {\n" },  // Adds newline
  { old_string: "func A() {", new_string: "..." }            // Fails - content changed!
]
```
</common_mistake>

<recovery>
If some edits fail:
1. Check response for failed edits list
2. `view` file to see current state
3. Retry failed edits with corrected old_string
</recovery>

<example>
Rename function and update its call site:
```
edits: [
  {
    old_string: "func oldName() {\n    return nil\n}",
    new_string: "func newName() {\n    return nil\n}"
  },
  {
    old_string: "result := oldName()",
    new_string: "result := newName()"
  }
]
```
</example>
