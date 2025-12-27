Get linter errors and warnings from LSP. Check files after editing.

<when_to_use>
Use after substantive edits to check for errors you may have introduced. Fix errors if the fix is clear.

Skip for files you didn't change - those errors aren't your responsibility.
</when_to_use>

<parameters>
- file_path: Specific file to check (optional)
- Leave empty to get project-wide diagnostics
</parameters>

<output>
- Errors, warnings, and hints grouped by severity
- File paths and line numbers for each issue
- Diagnostic messages from the language server
</output>

<guidelines>
- Check files you edited before finishing
- Fix errors you introduced
- Ignore pre-existing errors in untouched files
- Use with `edit` to fix issues at specific locations
</guidelines>
