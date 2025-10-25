Find all references to/usage of a symbol by name using the Language Server Protocol (LSP).

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
</features>

<limitations>
- May not find references in files not opened or indexed by the LSP server.
- Results depend on the capabilities of the active LSP providers.
</limitations>

<tips>
- Use this first when searching for where a symbol is used.
- Do not use grep/glob for symbol searches.
- Narrow scope with the path parameter for faster, more relevant results.
- Use qualified names (e.g., pkg.Func, Class.method) for higher precision.
</tips>
