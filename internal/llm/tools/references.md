Find all references to/usage of a symbol by name using the Language Server Protocol (LSP).

WHEN TO USE THIS TOOL:

- **ALWAYS USE THIS FIRST** when searching for where a function, method, variable, type, or constant is used
- **DO NOT use grep/glob for symbol searches** - this tool is semantic-aware and much more accurate
- Use when you need to find all usages of a specific symbol (function, variable, type, class, method, etc.)
- More accurate than grep because it understands code semantics and scope
- Finds only actual references, not string matches in comments or unrelated code
- Helpful for understanding where a symbol is used throughout the codebase
- Useful for refactoring or analyzing code dependencies
- Good for finding all call sites of a function, method, type, package, constant, variable, etc.

HOW TO USE:

- Provide the symbol name (e.g., "MyFunction", "myVariable", "MyType")
- Optionally specify a path to narrow the search to a specific directory
- The tool will automatically find the symbol and locate all references

FEATURES:

- Returns all references grouped by file
- Shows line and column numbers for each reference
- Supports multiple programming languages through LSP
- Automatically finds the symbol without needing exact position

LIMITATIONS:

- May not find references in files that haven't been opened or indexed
- Results depend on the LSP server's capabilities

TIPS:

- **Use this tool instead of grep when looking for symbol references** - it's more accurate and semantic-aware
- Simply provide the symbol name and let the tool find it for you
- This tool understands code structure, so it won't match unrelated strings or comments
