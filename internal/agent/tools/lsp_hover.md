Get hover information for the symbol at a specific source position using the Language Server Protocol (LSP).

<usage>
- Provide the file path, line, and character position of the symbol.
- Line and character are 1-based.
- Returns the hover text provided by the LSP server.
</usage>

<features>
- Semantic-aware type and documentation lookup.
- Uses the active LSP server for the file language.
- Safe read-only inspection suitable for planning.
</features>

<limitations>
- Requires an LSP client that supports hover requests.
- Hover content quality depends on the active LSP provider.
</limitations>

<tips>
- Use this to inspect types, signatures, and docs without leaving the CLI.
- Combine with lsp_definition for deeper code navigation.
