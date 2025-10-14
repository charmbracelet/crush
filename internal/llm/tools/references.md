Find all references to a symbol at a specific position in a file using the Language Server Protocol (LSP).

WHEN TO USE THIS TOOL:

- Use when you need to find all usages of a function, variable, type, or other symbol
- Helpful for understanding where a symbol is used throughout the codebase
- Useful for refactoring or analyzing code dependencies
- Good for finding all call sites of a function

HOW TO USE:

- Provide the file path containing the symbol
- Specify the line number (0-based) where the symbol is located
- Specify the character position (0-based) where the symbol starts
- Optionally specify whether to include the declaration (default: true)

FEATURES:

- Returns all references grouped by file
- Shows line and column numbers for each reference
- Supports multiple programming languages through LSP
- Can include or exclude the symbol's declaration

LIMITATIONS:

- Requires an LSP server to be running for the file type
- Line and character positions are 0-based (first line is 0, first character is 0)
- May not find references in files that haven't been opened or indexed
- Results depend on the LSP server's capabilities

TIPS:

- Use the View tool first to find the exact line and character position of the symbol
- Remember that line and character positions are 0-based
- Include the declaration to see where the symbol is defined
- Combine with other LSP tools for comprehensive code analysis
