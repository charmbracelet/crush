Find type definition locations for the symbol at a specific source position using the Language Server Protocol (LSP).

<usage>
- Provide the file path, line, and character position of the symbol.
- Line and character are 1-based.
- Returns type definition locations discovered by the active LSP client.
</usage>

<features>
- Semantic-aware navigation to symbol type definitions.
- Uses the active LSP server for the file language.
- Returns file paths with line and column numbers.
</features>

<limitations>
- Requires an LSP client that supports type definition requests.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Use this to jump from a value to its type declaration.
- Combine with lsp_hover to inspect type details quickly.
</tips>
