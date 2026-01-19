List directory contents in tree structure. Use this instead of shell `ls`.

<when_to_use>
Use LS when:
- Exploring project structure
- Finding what's in a directory
- Understanding folder organization
- Checking if files/directories exist

Do NOT use LS when:
- Finding files by pattern → use `glob`
- Searching file contents → use `grep`
- Reading file contents → use `view`
</when_to_use>

<parameters>
- path: Directory to list (default: current directory)
- ignore: Glob patterns to exclude (optional)
- depth: Max traversal depth (optional)
</parameters>

<output>
- Hierarchical tree structure
- Skips hidden files (starting with '.')
- Skips common system dirs (__pycache__, node_modules, etc.)
- Limited to 1000 files
</output>

<examples>
List project root:
```
path: "."
```

List src excluding tests:
```
path: "src"
ignore: ["*_test.go", "*.test.ts"]
```
</examples>

<tip>
For large projects, use `glob` to find specific files instead of browsing the entire tree.
</tip>
