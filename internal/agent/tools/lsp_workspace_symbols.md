Search workspace symbols using the Language Server Protocol (LSP).

<usage>
- Provide an optional query string to filter symbols.
- Leave query empty to request all available symbols from active LSP clients.
</usage>

<features>
- Semantic-aware symbol search across the workspace.
- Returns symbol names, kinds, and source locations.
- Safe read-only inspection suitable for planning.
</features>

<limitations>
- Requires an LSP client that supports workspace symbol requests.
- Large workspaces may return partial results depending on the LSP server.
</limitations>

<tips>
- Use this when you know a symbol name but not the file.
- Combine with lsp_definition or view once you identify the target file.
