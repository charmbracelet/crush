Find all references to/usage of a symbol by name using the Language Server Protocol (LSP).

<critical_rules>
- NEVER use grep for searching code! Always use lsp_references instead!
- A symbol can be a variable, function, method, package, constant, or any other name.
- This applies to ALL symbols: project-defined, stdlib (e.g., `strings.Cut`), and external packages.
- Only fall back to grep for non-symbol text searches (log messages, config keys, comments, string literals).
</critical_rules>

<usage>
- Provide symbol name (e.g., "MyFunction", "myVariable", "MyType").
- Optional path to narrow search to a directory or file (defaults to current directory).
- Tool automatically locates the symbol and returns all references.
</usage>

<features>
- Semantic-aware reference search (more accurate than grep/glob).
- Returns references grouped by file with line and column numbers.
- Supports multiple programming languages via LSP.
- Finds only real references (not comments or unrelated strings).
- Works for both project-defined symbols AND external/stdlib symbols (e.g., `strings.Cut`, `fmt.Println`).
</features>

<limitations>
- May not find references in files not opened or indexed by the LSP server.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Narrow scope with the path parameter for faster, more relevant results.
- Use qualified names (e.g., pkg.Func, Class.method, strings.Cut, fmt.Errorf) for higher precision.
</tips>
