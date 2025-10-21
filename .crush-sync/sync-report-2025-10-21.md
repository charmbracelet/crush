# Crush Repository Sync Report
**Generated:** 2025-10-21
**Last Sync Commit:** fccc49f4cb9ba90027b6eda64ae7257b633def66
**Current Crush HEAD:** 74270e2e (v0.12.0)
**Fork Point:** c2caa3c73306e5ef7b1bf76f41514db92f1c7bb4

## Summary

Since the last sync, Crush has released versions v0.11.0, v0.11.1, v0.11.2, and v0.12.0. This report analyzes commits from the tracked directories and categorizes them by priority for incorporation into Cliffy.

## Tracked Directories
- `internal/llm/agent` - Core agent implementation
- `internal/llm/tools` - Tool implementations (grep, ls, etc.)
- `internal/lsp` - LSP integration
- `internal/fsext` - Filesystem extensions
- `internal/llm/provider` - Provider implementations
- `internal/config` - Configuration handling

---

## Critical Bug Fixes (High Priority)

These fixes address bugs that could affect Cliffy's stability and correctness:

### 1. MCP Error Handling & Stability
- **4519e198** - fix(mcp): improve cache hits when using MCPs (#1271) - *Oct 20*
- **015632a1** - fix(mcp): make sure to cancel context on error (#1246) - *Oct 16*
- **3a995429** - fix(mcp): improve STDIO error handling (#1244) - *Oct 16*
- **b896a258** - fix(mcp): avoid nil errors for tool parameters (#1245) - *Oct 16*
- **ce72a483** - fix(mcp): append to os.Environ() (#1242) - *Oct 16*
- **23e0fd44** - fix(mcp): add type assertion guards (#1239) - *Oct 15*

**Impact:** These commits fix multiple MCP-related crashes and improve error handling. Critical for Cliffy's MCP integration.

**Files affected:**
- `internal/llm/agent/agent.go`
- `internal/llm/agent/mcp-tools.go`

### 2. Grep Tool Fix
- **1a40fbab** - fix(grep): check mime type (#1228) - *Oct 15*

**Impact:** Prevents grep from trying to search binary files, improving performance and preventing errors.

**Files affected:**
- `internal/llm/tools/grep.go`
- `internal/llm/tools/grep_test.go` (192 new test lines)

### 3. Tool Limits
- **9ffa5872** - fix(ls): properly handle limits (#1230) - *Oct 14*

**Impact:** Fixes limit handling in ls tool to prevent excessive output.

**Files affected:**
- `internal/llm/tools/ls.go` (likely)
- `internal/config/config.go`

### 4. Provider Fixes
- **7ac96ef0** - fix(vertex): small fix for anthropic models via google vertex (#1214) - *Oct 10*
- **69be8c20** - fix(bedrock): detect credentials set by `aws configure` (#1232) - *Oct 15*

**Impact:** Improves provider reliability for Vertex AI and Bedrock.

**Files affected:**
- `internal/llm/provider/vertex.go`
- `internal/config/bedrock.go`

### 5. Logging Fix
- **a4300436** - fix: move some logs to debug - *Oct 10*

**Impact:** Reduces noise in standard output by moving verbose logs to debug level.

**Files affected:**
- `internal/llm/agent/agent.go`
- `internal/fsext/`

---

## Important Improvements (Medium Priority)

### 1. MCP Library Migration
- **ca66a11a** - refactor(mcp): use the new mcp library (#1208) - *Oct 10*

**Impact:** Migrates from `github.com/mark3labs/mcp-go` to official `github.com/modelcontextprotocol/go-sdk v1.0.0`. This is a significant refactor that improves MCP compatibility.

**Files affected:**
- `go.mod` - Changes MCP dependency
- `internal/llm/agent/mcp-tools.go` - Refactored implementation
- `internal/llm/agent/agent.go`
- `internal/config/config.go`

**Note:** This is a breaking change that requires updating dependencies and potentially MCP-related code.

### 2. Dependency Updates
- **5e315538** - chore(deps): pin `anthropic-sdk-go` to our branch with fixes - *Oct 20*
- Updates Anthropic SDK from v1.12.0 to v1.14.0

**Impact:** Gets latest bug fixes from Anthropic SDK.

---

## New Features (Low Priority)

These are new features that could enhance Cliffy but aren't critical:

### 1. LSP Find References Tool
- **a64a4def** - feat(lsp): find references tool (#1233) - *Oct 17*

**Impact:** Adds ability to find all references to a symbol in code. Useful for code analysis tasks.

**Files affected:**
- `internal/lsp/` - New tool implementation
- `internal/llm/agent/agent.go` - Tool registration
- `internal/llm/tools/` - Tool interface

### 2. Bedrock Bearer Token Support
- **2708121a** - feat(bedrock): add support for `AWS_BEARER_TOKEN_BEDROCK` for bedrock - *Oct 20*

**Impact:** Adds support for bearer token authentication with Bedrock.

**Files affected:**
- `internal/config/bedrock.go`

---

## Not Relevant (TUI-Specific)

These commits are specific to Crush's TUI and don't apply to Cliffy's headless architecture:

- **beb3bc0d** - fix(tui): remove ctrl+d deny keybind (#1269)
- **4b1001cf** - fix(tui): paste on arguments input (#1240)
- **a824240d** - fix(tui): fix progress not cleaning up some times (#1219)
- **4969c34d** - fix(tui): panic (#1220)
- **8c9ce8e7** - feat: paste/close bindings in user cmd dialog (#1221)
- **04210801** - fix(lsp): small UI improvements (#1211)
- **886bb7c7** - fix(mcp): fix ui description, double spaces (#1210)
- **d0724b16** - feat(tui): progress bar (#1162)

---

## Dependency Changes Summary

### go.mod Changes in Crush (since last sync)

1. **MCP Library:** `github.com/mark3labs/mcp-go v0.41.0` → `github.com/modelcontextprotocol/go-sdk v1.0.0`
2. **Anthropic SDK:** `v1.12.0` → `v1.14.0` (Cliffy currently on v1.12.0)
3. **Catwalk:** `v0.6.1` → `v0.7.0` (Cliffy currently on v0.6.1)
4. **New dependency:** `github.com/charmbracelet/x/exp/ordered v0.1.0` (for clamp function)

---

## Recommendations

### Immediate Actions (Priority 1)
1. **Apply MCP bug fixes** - The 6 MCP-related commits fix critical stability issues
2. **Apply grep mime type fix** - Prevents binary file search issues
3. **Apply ls limits fix** - Prevents excessive output
4. **Update Anthropic SDK** - Get latest bug fixes

### Near-term Actions (Priority 2)
1. **Evaluate MCP library migration** - Consider migrating to official MCP SDK
2. **Apply provider fixes** - Vertex and Bedrock improvements
3. **Apply logging improvements** - Better log level handling

### Future Considerations (Priority 3)
1. **LSP find references tool** - Nice enhancement for code analysis
2. **Bedrock bearer token support** - If using Bedrock

### Not Needed
- All TUI-specific changes can be ignored

---

## Cherry-Pick Commands

To apply critical bug fixes individually:

```bash
# MCP fixes (apply in order)
git cherry-pick 23e0fd44  # type assertion guards
git cherry-pick ce72a483  # append to os.Environ
git cherry-pick b896a258  # avoid nil errors
git cherry-pick 3a995429  # improve STDIO error handling
git cherry-pick 015632a1  # cancel context on error
git cherry-pick 4519e198  # improve cache hits

# Tool fixes
git cherry-pick 1a40fbab  # grep mime type fix
git cherry-pick 9ffa5872  # ls limits fix

# Provider fixes
git cherry-pick 7ac96ef0  # vertex fix
git cherry-pick 69be8c20  # bedrock credentials

# Logging
git cherry-pick a4300436  # move logs to debug
```

**Note:** Cherry-picking may require conflict resolution if the codebase has diverged significantly.

---

## Alternative: Selective Directory Merge

For heavily changed areas like MCP tools, consider merging entire files:

```bash
# Review changes first
git diff HEAD..upstream/main -- internal/llm/agent/mcp-tools.go

# If changes look good, merge specific files
git checkout upstream/main -- internal/llm/agent/mcp-tools.go
git checkout upstream/main -- internal/llm/tools/grep.go

# Review and test before committing
```

---

## Update Tracking

After incorporating changes, update the sync marker:

```bash
# Update to latest Crush commit
echo "74270e2e" > .crush-sync/last-sync.txt

# Or update to specific commit if partial sync
echo "<commit-hash>" > .crush-sync/last-sync.txt
```

---

## Testing Recommendations

After applying any changes:

1. **Run test suite:** `go test ./...`
2. **Test MCP integration:** Verify MCP servers still work correctly
3. **Test grep tool:** Verify binary files are handled correctly
4. **Test providers:** Verify Anthropic, Bedrock, Vertex AI still work
5. **Integration test:** Run several representative Cliffy commands

---

## Notes

- Crush has evolved significantly with v0.11.x and v0.12.0 releases
- Most changes are MCP and tool-related improvements
- The MCP library migration (ca66a11a) is the most significant architectural change
- Many TUI-specific changes don't apply to Cliffy's headless design
- Consider selective cherry-picking rather than bulk merging to avoid TUI dependencies
