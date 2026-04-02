List document symbols for a specific file using the Language Server Protocol (LSP).

<usage>
- Provide the file path to inspect.
- Returns top-level and nested document symbols for that file.
</usage>

<features>
- Semantic-aware outline of functions, types, methods, and other symbols.
- Preserves nesting when the LSP provides hierarchical document symbols.
- Safe read-only inspection suitable for planning.
</features>

<limitations>
- Requires an LSP client that supports document symbol requests.
- Results depend on the capabilities of the active LSP provider.
</limitations>

<tips>
- Use this to understand file structure before making edits.
- Combine with view and lsp_definition for targeted navigation.
