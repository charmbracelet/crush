Deletes files or directories from the filesystem.

<critical_rules>
- NEVER use `rm` command in bash - always use this tool for all file/directory deletions
</critical_rules>

<usage>
- Provide file path to delete
- For directories, always set recursive=true to delete the directory
- Tool handles LSP cleanup automatically (closes open files, clears diagnostics)
</usage>

<limitations>
- Cannot delete files outside the working directory
- Deleting directories requires recursive=true (even if empty)
- Deleted files cannot be recovered (no trash/recycle bin)
</limitations>
