Create new files or completely rewrite existing files.

<when_to_use>
Use Write when:
- Creating new files
- Complete file rewrite (>50% changes)
- Generating new code from scratch
- Replacing entire file contents

Do NOT use Write when:
- Making targeted edits → use `edit`
- Multiple surgical changes → use `multiedit`
- File exists and only needs small changes → use `edit`
</when_to_use>

<parameters>
- file_path: Path to write (required)
- content: Complete file content (required)
</parameters>

<behavior>
- Creates parent directories automatically
- Overwrites existing files
- Checks if file modified since last read (safety check)
- Skips write if content unchanged
</behavior>

<guidelines>
- Use `view` first to check if file exists
- Use `ls` to verify target directory
- Use absolute paths when possible
- Match existing code style in the project
</guidelines>

<examples>
Good: Creating a new test file
```
file_path: "/project/src/utils_test.go"
content: "package utils\n\nimport \"testing\"\n\nfunc TestHelper(t *testing.T) {\n    // test code\n}"
```

Bad: Using write for a small change to existing file → Use `edit` instead
</examples>
