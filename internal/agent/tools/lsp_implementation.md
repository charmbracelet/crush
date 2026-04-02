Find implementation locations for the symbol at a specific source position using the Language Server Protocol (LSP).

<usage>
- Provide the file path, line, and character position of the symbol.
- Line and character are 1-based.
- Returns implementation locations discovered by the active LSP client.
</usage>

<features>
- Semantic-aware navigation to concrete implementations.
- Uses the active LSP server for the file language.
- Returns file paths with line and column numbers.
</features>

<limitations>
- Requires an LSP client that supports implementation requests.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Use this on interfaces or abstract symbols to find concrete code paths.
- Combine with lsp_definition for richer exploration.
</tips>
