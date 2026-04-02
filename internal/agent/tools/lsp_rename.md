Rename a symbol and apply resulting workspace edits using the Language Server Protocol (LSP).

<usage>
- Provide file path, line, and character for the symbol to rename.
- Provide new_name with the replacement symbol name.
- Tool requests textDocument/rename and applies returned workspace edits.
</usage>

<features>
- Semantic symbol rename through the active LSP server.
- Applies multi-file workspace edits produced by rename operations.
- Reuses existing LSP edit application and diagnostics update path.
</features>

<limitations>
- Requires an LSP server that supports textDocument/rename.
- Rename can fail if new_name is invalid for the language/server rules.
- If server returns no edits, tool reports a no-op rename result.
</limitations>

<tips>
- Use precise symbol coordinates to avoid renaming the wrong identifier.
- Prefer running lsp_references before rename for high-impact symbols.
- Run tests after rename to catch semantic regressions.
</tips>
