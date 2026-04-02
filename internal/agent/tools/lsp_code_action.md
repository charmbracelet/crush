Request and optionally apply code actions for a symbol position using the Language Server Protocol (LSP).

<usage>
- Provide file path, line, and character to request actions at that position.
- Optionally set action_kind to filter returned actions (for example quickfix or refactor).
- Set apply=true to apply a code action workspace edit.
- When apply=true, use action_index (1-based) to choose which action to apply (defaults to 1).
</usage>

<features>
- Lists available code actions with title, kind, and whether they include edit/command payloads.
- Supports action kind filtering through LSP code action kinds.
- Applies workspace edits from code actions directly to local files.
- Reuses existing LSP workspace edit and diagnostics notification flow.
</features>

<limitations>
- Requires an LSP server that supports textDocument/codeAction.
- Only code actions containing workspace edits can be applied.
- Command-only code actions are listed but not executed.
</limitations>

<tips>
- Start with apply=false to inspect available actions first.
- Use action_kind to reduce noise when a file returns many actions.
- After applying a code action, run diagnostics/tests to verify behavior.
</tips>
