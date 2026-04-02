Find the definition location for the symbol at a specific source position using the Language Server Protocol (LSP).

<usage>
- Provide the file path, line, and character position of the symbol.
- Line and character are 1-based.
- Returns the definition locations discovered by the active LSP client.
</usage>

<features>
- Semantic-aware navigation to symbol definitions.
- Uses the active LSP server for the file language.
- Returns file paths with line and column numbers.
</features>

<limitations>
- Requires an LSP client that supports definition requests.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Use this after view/grep when you know the exact symbol position.
- Prefer this over text search for real code navigation.
