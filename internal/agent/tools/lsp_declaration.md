Find the declaration location for the symbol at a specific source position using the Language Server Protocol (LSP).

<usage>
- Provide the file path, line, and character position of the symbol.
- Line and character are 1-based.
- Returns the declaration locations discovered by the active LSP client.
</usage>

<features>
- Semantic-aware navigation to symbol declarations.
- Uses the active LSP server for the file language.
- Returns file paths with line and column numbers.
</features>

<limitations>
- Requires an LSP client that supports declaration requests.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Use this when you need where a symbol is originally declared.
- Combine with lsp_definition and lsp_references for complete navigation.
</tips>
