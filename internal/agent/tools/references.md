Find all references to a symbol using LSP. More accurate than grep for code symbols.

<when_to_use>
Use References when:
- Finding where a function/method is called
- Finding usages of a type, variable, or constant
- Understanding impact before renaming/refactoring
- Need semantic accuracy (grep finds strings, this finds actual references)

Do NOT use References when:
- Searching for arbitrary text → use `grep`
- Finding files by name → use `glob`
- Symbol isn't in a language with LSP support
</when_to_use>

<parameters>
- symbol: Name to search for (e.g., "MyFunction", "UserService", "configPath")
- path: Directory to narrow search (optional, default: current directory)
</parameters>

<output>
- References grouped by file
- Line and column numbers for each usage
- Only real code references (not comments or strings)
</output>

<tips>
- Use qualified names for precision: "pkg.Function", "Class.method"
- Narrow scope with path parameter for faster results
- Works best with statically typed languages
- Depends on LSP server capabilities and indexing
</tips>

<example>
Before refactoring `handleRequest`:
```
symbol: "handleRequest"
path: "src/handlers"
```
→ Shows all callers so you know what might break.
</example>
