# Cliffy Tools Reference

This document lists all available tools that cliffy's AI agent can use when executing tasks.

## Core Built-in Tools

These tools are always available (at `internal/llm/agent/agent.go:182-194`):

### File Operations

1. **view** - Read file contents
   - Location: `internal/llm/tools/view.go`
   - Reads files and displays contents
   - Integrates with LSP for code intelligence

2. **write** - Create or overwrite files
   - Location: `internal/llm/tools/write.go`
   - Creates new files or completely replaces existing ones
   - Tracks additions/deletions for metrics

3. **edit** - Make precise edits to existing files
   - Location: `internal/llm/tools/edit.go`
   - String replacement within files
   - Integrates with LSP for validation

4. **multiedit** - Apply multiple edits to a file in one operation
   - Location: `internal/llm/tools/multiedit.go`
   - Batch editing for efficiency
   - Validates all edits before applying

### Search & Discovery

5. **glob** - Find files by pattern
   - Location: `internal/llm/tools/glob.go`
   - Supports glob patterns like `**/*.js`, `src/**/*.ts`
   - Returns matching file paths

6. **grep** - Search file contents with regex
   - Location: `internal/llm/tools/grep.go`
   - Full regex support
   - Filter by file type or glob pattern
   - Multiple output modes (content, files, counts)

7. **ls** - List directory contents
   - Location: `internal/llm/tools/ls.go`
   - Shows files and directories
   - File size and metadata

### Shell & Network

8. **bash** - Execute shell commands
   - Location: `internal/llm/tools/bash.go`
   - Persistent shell with working directory tracking
   - Timeout support
   - Exit code tracking

9. **fetch** - Fetch web content
   - Location: `internal/llm/tools/fetch.go`
   - HTTP GET requests
   - Converts HTML to markdown

10. **download** - Download files from URLs
    - Location: `internal/llm/tools/download.go`
    - Saves remote files locally
    - Progress tracking

### Code Intelligence

11. **sourcegraph** - Search code across repositories
    - Location: `internal/llm/tools/sourcegraph.go`
    - Integration with Sourcegraph API
    - Cross-repo code search

## Coder-Only Tools

These tools are only available to the "coder" agent (at `agent.go:200-207`):

12. **diagnostics** - Get LSP diagnostics (errors, warnings)
    - Location: `internal/llm/tools/diagnostics.go`
    - Requires LSP server running
    - Shows compiler/linter errors

13. **MCP Tools** - Dynamically loaded Model Context Protocol tools
    - Location: `internal/llm/agent/mcp-tools.go`
    - Configured via `~/.config/cliffy/cliffy.json`
    - Examples: filesystem, database access, API integrations
    - See MCP docs: https://modelcontextprotocol.io/

## Tool Filtering

Tools can be filtered per agent via the `allowed_tools` config:

```json
{
  "agents": {
    "task": {
      "allowed_tools": ["bash", "fetch", "view"]
    }
  }
}
```

If `allowed_tools` is null/empty, all tools are available.

## Tool Usage Statistics

When running with `--verbose`, cliffy tracks and displays:
- Tool execution count
- Tool duration
- File operations (bytes, lines)
- Shell command exit codes
- Search match counts

Example collapsed output:
```
1 ╮ ● task [write 2×bash view] 19.6k tokens $0.0000  82.6s
```

## Adding New Tools

To add a new tool:

1. Create `internal/llm/tools/newtool.go` implementing `BaseTool` interface
2. Add to `allTools` list in `agent.go:182-194`
3. Update this documentation

## Tool Safety

Some tools have safety features:
- **bash**: Prevents destructive commands in safe mode
- **write**: Prevents overwriting without confirmation
- **edit**: Validates before applying changes
- All tools respect working directory boundaries

## Related Files

- Tool interface: `internal/llm/tools/tools.go`
- Agent setup: `internal/llm/agent/agent.go`
- Tool execution tracking: `internal/llm/tools/tools.go:ExecutionMetadata`
- Progress display: `internal/volley/progress.go:formatToolSummary()`
