Replace, insert, or delete an entire symbol (function, method, class, struct) by name using LSP document symbols to find exact boundaries. Prefer this over `edit` for whole-symbol changes: it eliminates whitespace-matching failures by resolving symbol ranges through the language server instead of exact text matching.

Actions:
- `replace` (default): replace the entire symbol including signature and body
- `add_before`: insert text before the symbol
- `add_after`: insert text after the symbol
- `delete`: remove the symbol entirely

Returns diagnostics after the edit.
