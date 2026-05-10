Apply multiple find-and-replace edits to a single file; edits run sequentially. Prefer over edit for multiple changes to the same file. Same exact-match rules as edit apply.

<parameters>
- file_path: Absolute path to file (required)
- edits: Array of {old_string, new_string, replace_all?}
</parameters>
