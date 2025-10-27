Makes multiple edits to a single file in one operation. Built on Edit tool for efficient multiple find-and-replace operations. Prefer over Edit tool for multiple edits to same file.

<prerequisites>
1. Use View tool to understand file contents and context
2. Verify directory path is correct
3. CRITICAL: Note exact whitespace, indentation, and formatting from View output
</prerequisites>

<parameters>
1. file_path: Absolute path to file (required)
2. edits: Array of edit operations, each containing:
   - old_string: Text to replace (must match exactly including whitespace/indentation)
   - new_string: Replacement text
   - replace_all: Replace all occurrences (optional, defaults to false)
</parameters>

<operation>
- Edits applied sequentially in provided order.
- Each edit operates on result of previous edit.
- ATOMIC: If any single edit fails, the entire operation fails and no changes are applied.
- Ideal for several changes to different parts of same file.
</operation>

<inherited_rules>
All instructions from the Edit tool documentation apply verbatim to every edit item:
- Critical requirements for exact matching and uniqueness
- Warnings and common failures (tabs vs spaces, blank lines, brace placement, etc.)
- Verification steps before using, recovery steps, best practices, and whitespace checklist
Use the same level of precision as Edit. Multiedit often fails due to formatting mismatches—double-check whitespace for every edit.
</inherited_rules>

<critical_requirements>
1. Apply Edit tool rules to EACH edit (see edit.md).
2. Edits are atomic—either all succeed or none are applied.
3. Plan sequence carefully: earlier edits change the file content that later edits must match.
4. Ensure each old_string is unique at its application time (after prior edits).
</critical_requirements>

<verification_before_using>
1. View the file and copy exact text (including whitespace) for each target.
2. Check how many instances each old_string has BEFORE the sequence starts.
3. Dry-run mentally: after applying edit #N, will edit #N+1 still match? Adjust old_string/new_string accordingly.
4. Prefer fewer, larger context blocks over many tiny fragments that are easy to misalign.
5. If edits are independent, consider separate multiedit batches per logical region.
</verification_before_using>

<warnings>
- Operation fails if any old_string doesn’t match exactly (including whitespace) or equals new_string.
- Earlier edits can invalidate later matches (added/removed spaces, lines, or reordered text).
- Mixed tabs/spaces, trailing spaces, or missing blank lines commonly cause failures.
- replace_all may affect unintended regions—use carefully or provide more context.
</warnings>

<recovery_steps>
If the operation fails:
1. Identify the first failing edit (start from top; test subsets to isolate).
2. View the file again and copy more surrounding context for that edit.
3. Recalculate later old_string values based on the file state AFTER preceding edits.
4. Reduce the batch (apply earlier stable edits first), then follow up with the rest.
</recovery_steps>

<best_practices>
- Ensure all edits result in correct, idiomatic code; don’t leave code broken.
- Use absolute file paths (starting with /).
- Use replace_all only when you’re certain; otherwise provide unique context.
- Match existing style exactly (spaces, tabs, blank lines).
- Test after the operation; if it fails, fix and retry in smaller chunks.
</best_practices>

<whitespace_checklist>
For EACH edit, verify:
- [ ] Viewed the file first
- [ ] Counted indentation spaces/tabs
- [ ] Included blank lines if present
- [ ] Matched brace/bracket positioning
- [ ] Included 3–5 lines of surrounding context
- [ ] Verified text appears exactly once (or using replace_all deliberately)
- [ ] Copied text character-for-character, not approximated
</whitespace_checklist>

<examples>
✅ Correct: Sequential edits where the second match accounts for the first change

```
edits: [
  {
    old_string: "func A() {\n    doOld()\n}",
    new_string: "func A() {\n    doNew()\n}",
  },
  {
    // Uses context that still exists AFTER the first replacement
    old_string: "func B() {\n    callA()\n}",
    new_string: "func B() {\n    callA()\n    logChange()\n}",
  },
]
```

❌ Incorrect: Second old_string no longer matches due to whitespace change introduced by the first edit

```
edits: [
  {
    old_string: "func A() {\n    doOld()\n}",
    new_string: "func A() {\n\n    doNew()\n}", // Added extra blank line
  },
  {
    old_string: "func A() {\n    doNew()\n}", // Missing the new blank line, will FAIL
    new_string: "func A() {\n    doNew()\n    logChange()\n}",
  },
]
```
</examples>
