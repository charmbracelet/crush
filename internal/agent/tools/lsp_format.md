Format a document via Language Server Protocol (LSP) and apply returned text edits.

<usage>
- Provide file_path for the document to format.
- Optional formatting fields can tune behavior (tab_size, insert_spaces, trim_* flags).
- Tool requests textDocument/formatting and applies returned edits.
</usage>

<features>
- Uses language-aware formatting from the active LSP server.
- Supports configurable formatting options passed to the LSP request.
- Applies formatting edits through existing workspace edit logic.
- Triggers LSP notifications/diagnostics refresh after applying edits.
</features>

<limitations>
- Requires an LSP server that supports textDocument/formatting.
- Formatting result depends on server formatter and project configuration.
- Returns no-op when server reports zero edits.
</limitations>

<tips>
- Use defaults first; only pass explicit options when needed.
- Run package tests after formatting if generated code or strict linters are present.
- Prefer this tool over ad-hoc format commands for LSP-consistent edits.
</tips>
