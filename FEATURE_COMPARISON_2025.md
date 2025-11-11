# Crush Feature Comparison & Enhancement Plan (2025)

**Date:** November 11, 2025
**Purpose:** Comprehensive analysis of Crush vs competing AI CLI tools and implementation roadmap

---

## Executive Summary

Crush is a well-architected AI coding assistant CLI with excellent foundations in:
- Terminal UI/UX with theming
- Multi-provider LLM support
- Model Context Protocol (MCP) extensibility
- LSP integration
- Permission system
- Session management

However, it has **significant gaps** compared to market leaders like Aider, GitHub Copilot CLI, and Continue. This document outlines the competitive landscape and proposes actionable enhancements.

---

## Competitive Landscape Overview

### Top AI Coding CLI Tools (2025)

| Tool | GitHub Stars | Primary Strength | Pricing Model |
|------|--------------|------------------|---------------|
| **Aider** | 30k+ | Git integration, test-driven coding | Open source / Free |
| **GitHub Copilot CLI** | Official | Natural language commands, GitHub integration | $10-19/mo |
| **Claude Code** | 18k+ | Conversational coding, large context | Pay-per-use |
| **Continue** | 15k+ | IDE integration, background agents | Open source / Free |
| **Crush** | - | MCP, LSP, multi-provider | Open source / Free |
| **Codeium/Termium** | - | Terminal autocomplete, natural language CLI | Freemium |

---

## Feature Comparison Matrix

### Core Capabilities

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **Interactive TUI** | ✅ Excellent | ⚠️ Basic | ⚠️ Basic | ✅ Good | - |
| **Non-interactive mode** | ⚠️ Limited | ✅ Full | ✅ Full | ✅ Full | 🔴 HIGH |
| **Programmatic mode (-p flag)** | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes | 🔴 HIGH |
| **Multi-model support** | ✅ Excellent | ✅ Good | ⚠️ Limited | ✅ Good | - |
| **Model switching** | ✅ Runtime | ⚠️ Config | ✅ /model | ✅ Good | - |
| **Extended thinking** | ✅ Yes | ⚠️ Limited | ✅ Yes | ⚠️ Limited | - |
| **Session management** | ✅ Excellent | ⚠️ Basic | ❌ No | ⚠️ Basic | - |

### Git Integration

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **Git status** | ⚠️ Via bash | ✅ Native | ✅ Native | ✅ Native | 🔴 HIGH |
| **Git diff** | ⚠️ Via bash | ✅ /diff cmd | ✅ Native | ✅ Native | 🔴 HIGH |
| **Git commit** | ⚠️ Via bash | ✅ /commit | ✅ Native | ✅ Native | 🔴 HIGH |
| **Git add/drop files** | ❌ No | ✅ /add /drop | ⚠️ Limited | ⚠️ Limited | 🔴 HIGH |
| **Git undo** | ❌ No | ✅ /undo | ❌ No | ❌ No | 🟡 MED |
| **Auto-commit messages** | ⚠️ Manual | ✅ AI-generated | ✅ AI-generated | ✅ AI-generated | 🟡 MED |
| **Co-authored-by** | ✅ Yes | ✅ Yes (default) | ⚠️ Limited | ⚠️ Limited | - |

### Advanced Features

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **Plan/architect mode** | ❌ No | ✅ /architect | ⚠️ Limited | ⚠️ Limited | 🔴 HIGH |
| **Test integration** | ❌ No | ✅ /test | ⚠️ Limited | ✅ Yes | 🟡 MED |
| **Daemon/background mode** | ❌ No | ❌ No | ❌ No | ✅ Headless | 🟡 MED |
| **Watch mode** | ❌ No | ❌ No | ❌ No | ⚠️ Limited | 🟢 LOW |
| **Pre-commit hooks** | ❌ No | ⚠️ Limited | ❌ No | ✅ Yes | 🟢 LOW |
| **Image support** | ⚠️ Limited | ❌ No | ✅ Yes | ⚠️ Limited | 🟢 LOW |
| **Natural language CLI** | ❌ No | ❌ No | ✅ Yes | ❌ No | 🟡 MED |

### Slash Commands

| Command | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **/model** | ❌ No | ⚠️ Limited | ✅ Yes | ✅ Yes | 🔴 HIGH |
| **/think** | ❌ No | ✅ /think-tokens | ⚠️ Limited | ❌ No | 🔴 HIGH |
| **/reasoning** | ❌ No | ✅ /reasoning-effort | ✅ Yes | ❌ No | 🔴 HIGH |
| **/diff** | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes | 🔴 HIGH |
| **/commit** | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes | 🔴 HIGH |
| **/add** | ❌ No | ✅ Yes | ⚠️ Limited | ⚠️ Limited | 🔴 HIGH |
| **/drop** | ❌ No | ✅ Yes | ⚠️ Limited | ⚠️ Limited | 🔴 HIGH |
| **/undo** | ❌ No | ✅ Yes | ❌ No | ❌ No | 🟡 MED |
| **/test** | ❌ No | ✅ Yes | ⚠️ Limited | ✅ Yes | 🟡 MED |
| **/clear** | ❌ No | ✅ /clear | ⚠️ Limited | ✅ Yes | 🟢 LOW |
| **/help** | ⚠️ Basic | ✅ Detailed | ✅ Detailed | ✅ Detailed | 🟢 LOW |

### Extensibility

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **MCP support** | ✅ stdio/http/sse | ❌ No | ✅ Limited | ⚠️ Planned | - |
| **LSP integration** | ✅ Excellent | ⚠️ Limited | ❌ No | ✅ Good | - |
| **Custom providers** | ✅ OpenAI/Anthropic | ✅ OpenAI-only | ⚠️ Limited | ✅ Full | - |
| **Plugin system** | ⚠️ MCP-only | ❌ No | ⚠️ Limited | ✅ Extensions | 🟡 MED |
| **Custom tools** | ✅ Via MCP | ⚠️ Limited | ⚠️ Limited | ✅ Yes | - |

### Permission & Safety

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **Permission system** | ✅ Excellent | ⚠️ Basic | ✅ Preview mode | ⚠️ Basic | - |
| **YOLO mode** | ✅ --yolo | ✅ --yes | ⚠️ Limited | ✅ Auto mode | - |
| **Allowlist** | ✅ Per-tool | ⚠️ Limited | ⚠️ Limited | ✅ Allow/deny lists | - |
| **Audit log** | ❌ No | ❌ No | ⚠️ Limited | ❌ No | 🟡 MED |
| **Dry-run mode** | ❌ No | ❌ No | ✅ Preview | ❌ No | 🟡 MED |
| **Sandboxing** | ⚠️ Basic | ⚠️ Basic | ⚠️ Basic | ⚠️ Basic | 🟢 LOW |

### Session Management

| Feature | Crush | Aider | Copilot CLI | Continue | Priority |
|---------|-------|-------|-------------|----------|----------|
| **Multiple sessions** | ✅ Excellent | ⚠️ Limited | ❌ No | ⚠️ Basic | - |
| **Session switching** | ✅ Runtime | ❌ No | ❌ No | ⚠️ Limited | - |
| **Session export** | ❌ No | ⚠️ Limited | ❌ No | ⚠️ Limited | 🟡 MED |
| **Session branching** | ❌ No | ❌ No | ❌ No | ❌ No | 🟡 MED |
| **Auto-summarization** | ✅ Yes | ⚠️ Limited | ❌ No | ⚠️ Limited | - |
| **Cost tracking** | ✅ Per-session | ⚠️ Global | ⚠️ Limited | ⚠️ Limited | - |

---

## Key Competitor Features Analysis

### 1. Aider's Standout Features

**Git Integration Excellence:**
- `/add <files>` - Add files to context
- `/drop <files>` - Remove files from context
- `/commit` - Generate AI commit message
- `/diff` - Show pending changes
- `/undo` - Revert last changes
- Auto-generated commit messages following conventional commits

**Testing Integration:**
- `/test` - Run tests and auto-fix failures
- Test-driven development workflow
- Automatic test result analysis

**Advanced Reasoning:**
- `/think-tokens <N>` - Set extended thinking budget (8k, 10.5k, 0.5M format)
- `/reasoning-effort <level>` - Control reasoning intensity (low/medium/high)
- `/architect` - Planning mode with explicit plan review before execution

**File Management:**
- `/add-gitignore-files` - Include gitignored files
- Smart file selection based on LSP symbols
- Repository map for large codebases

**Commit Features:**
- `--commit-language` - Multi-language commit messages
- Co-authored-by attribution (now default)
- Conventional commits support

**OAuth & Authentication:**
- OAuth flow for OpenRouter (no API key needed)
- Secure credential management

### 2. GitHub Copilot CLI's Standout Features

**Natural Language Interface:**
- Describe what you want in plain English
- AI figures out the correct commands
- Example: "deploy the staging environment" → actual deployment commands

**Dual Modes:**
- **Interactive mode**: `copilot` - Full conversation
- **Programmatic mode**: `copilot -p "prompt"` - One-off execution

**Model Selection:**
- `/model` slash command in session
- Choose from Claude Sonnet 4.5, GPT-5, etc.
- Per-task model switching

**GitHub Integration:**
- Native GitHub MCP server pre-configured
- Merge PRs from CLI
- Issue management
- Repository operations

**Image Support:**
- Attach screenshots/diagrams
- Visual debugging
- Architecture diagram analysis

**Safety:**
- Preview mode (default) - shows what will happen
- Explicit approval required
- Rollback capabilities

### 3. Continue's Standout Features

**Headless/Daemon Mode:**
- Run as background service
- Process tasks without UI
- Integration with CI/CD pipelines

**Background Agents:**
- Event-triggered automation
- Scheduled tasks (cron-like)
- Webhook integrations

**IDE Integration:**
- VS Code extension
- JetBrains plugins
- Editor protocol support

**REPL Mode:**
- Test agents interactively
- Debug agent behavior
- Rapid iteration

**Deployment Flexibility:**
- Local bash scripts
- GitHub Actions
- Jenkins/GitLab CI
- Custom infrastructure

### 4. Codeium/Termium's Standout Features

**Natural Language to CLI:**
- Type what you want to do
- Generates shell commands
- Terminal autocomplete

**Auto-execution Modes:**
- **Auto mode**: AI decides if approval needed
- **Turbo mode**: Execute unless denied
- Allow/deny lists for commands

**Terminal Context Awareness:**
- Reads stack traces
- Analyzes error messages
- Suggests fixes based on terminal output

**Smart Completions:**
- Context from terminal history
- Project-aware suggestions
- Test failure analysis

---

## Crush's Unique Strengths

### What Crush Does Better

1. **MCP Extensibility** ⭐⭐⭐
   - Full MCP support (stdio, http, sse)
   - Most comprehensive MCP integration in market
   - Environment variable expansion in configs

2. **Permission System** ⭐⭐⭐
   - Most sophisticated permission model
   - Per-tool, per-session, per-path granularity
   - Excellent UX with diff previews

3. **Multi-Provider Support** ⭐⭐⭐
   - 15+ LLM providers out of the box
   - Auto-updating provider database (Catwalk)
   - Custom OpenAI/Anthropic-compatible APIs

4. **Session Management** ⭐⭐⭐
   - Best-in-class session system
   - SQLite-backed persistence
   - Runtime session switching
   - Cost tracking per session

5. **Terminal UI** ⭐⭐
   - Beautiful Bubble Tea TUI
   - Theme support
   - Mouse support
   - Command palette

6. **LSP Integration** ⭐⭐⭐
   - Deep LSP integration for code intelligence
   - Multiple LSPs simultaneously
   - Diagnostics and references

7. **Extended Thinking** ⭐⭐
   - Anthropic extended thinking support
   - OpenAI reasoning effort
   - Thinking visualization

---

## Critical Gaps to Address

### 🔴 HIGH Priority (Must-Have for Competitiveness)

#### 1. Git Integration Commands
**Why:** Every competitor has native git commands. Currently, Crush relies on bash tool.

**Implement:**
```bash
crush add <files>      # Add files to git + context
crush drop <files>     # Remove from git + context
crush commit           # AI-generated commit message
crush diff             # Show git diff
crush undo             # Revert last changes
```

**Config:**
```json
{
  "git": {
    "auto_commit": false,
    "commit_style": "conventional",  // conventional, semantic, custom
    "commit_language": "en",         // en, zh, ja, etc.
    "show_diff_before_commit": true
  }
}
```

#### 2. Programmatic Mode
**Why:** Scripting and automation require non-interactive prompts.

**Implement:**
```bash
crush -p "fix all linting errors"       # One-off prompt
crush --prompt "generate tests for main.go"
echo "add error handling" | crush -p -  # Stdin support
```

**Features:**
- Auto-approve mode (no permission prompts)
- JSON output option: `--output json`
- Exit codes for CI/CD
- Progress indicator

#### 3. Slash Commands System
**Why:** Industry standard for in-session commands. Better UX than typing full commands.

**Implement:**
```
/model [name]          # Switch model
/think [budget]        # Set thinking token budget
/reasoning [effort]    # Set reasoning effort (low/med/high)
/diff                  # Show pending changes
/commit [message]      # Commit with optional message
/add <files>           # Add files to context
/drop <files>          # Remove files from context
/undo                  # Revert last change
/test [command]        # Run tests
/clear                 # Clear session
/help [command]        # Get help
```

**Implementation:**
- Parse `/` prefix in user input
- Execute before sending to LLM
- Add to autocomplete
- Document in help system

#### 4. Plan/Architect Mode
**Why:** Aider's killer feature. Reduces errors, increases trust.

**How it works:**
1. User requests complex change
2. AI generates step-by-step plan
3. User reviews/edits plan
4. AI executes approved plan
5. User can pause/abort between steps

**Implement:**
```json
{
  "options": {
    "architect_mode": "auto",  // auto, always, never
    "plan_threshold": 3        // Num of file edits to trigger
  }
}
```

**UI:**
- Show plan in structured format
- Allow inline edits to plan
- Step-by-step execution with confirmations
- Progress indicator

#### 5. Enhanced Non-Interactive Mode
**Why:** Scripting, CI/CD, automation all need better non-interactive support.

**Implement:**
- `--output json` - Structured output
- `--output text` - Plain text (default)
- `--exit-code` - Return non-zero on errors
- `--timeout <duration>` - Max execution time
- `--max-tokens <N>` - Token budget
- `--files <glob>` - Pre-load files into context

**Example:**
```bash
#!/bin/bash
# CI/CD script
crush -p "run all tests and fix failures" \
  --yolo \
  --output json \
  --timeout 5m \
  --files "**/*.test.js" > results.json

if [ $? -ne 0 ]; then
  echo "Tests failed"
  exit 1
fi
```

---

### 🟡 MEDIUM Priority (Competitive Advantages)

#### 6. Test Integration
**Implement:**
```bash
crush test                    # Run default test command
crush test --fix              # Auto-fix failures
crush test "npm run test:e2e" # Custom command
```

**Config:**
```json
{
  "test": {
    "command": "npm test",
    "auto_fix": true,
    "max_iterations": 3
  }
}
```

#### 7. Session Export/Import
**Why:** Share sessions, backup conversations, compliance.

**Implement:**
```bash
crush export session <id> --output session.json
crush import session.json
crush export session <id> --format markdown
```

**Formats:**
- JSON (full session data)
- Markdown (readable transcript)
- HTML (formatted report)

#### 8. Audit Log
**Why:** Compliance, debugging, trust.

**Track:**
- All permission grants/denials
- Tool executions
- File modifications
- Cost per operation

**Query:**
```bash
crush audit --session <id>
crush audit --tool bash --date 2025-11-10
crush audit --export audit.csv
```

#### 9. Daemon Mode
**Why:** Background processing, scheduled tasks, CI/CD integration.

**Implement:**
```bash
crush daemon start            # Start background service
crush daemon stop             # Stop service
crush daemon status           # Check status
crush daemon run <prompt>     # Execute in background
crush daemon queue <prompt>   # Add to queue
crush daemon logs             # View daemon logs
```

**Use Cases:**
- Watch files and auto-respond to changes
- Scheduled code reviews
- Background test generation
- CI/CD integration

#### 10. Natural Language CLI Generation
**Why:** Codeium has this, it's a great UX feature.

**Implement:**
- Detect terminal error messages
- Offer to explain errors
- Generate correct commands
- Learn from user corrections

**Example:**
```
$ crush cli "list all processes using port 8080"
→ lsof -i :8080

$ crush cli "compress all .log files older than 30 days"
→ find . -name "*.log" -mtime +30 -exec gzip {} \;
```

---

### 🟢 LOW Priority (Nice-to-Have)

#### 11. Watch Mode
```bash
crush watch "**/*.ts" --command "regenerate tests when files change"
```

#### 12. Pre-commit Hook Integration
```bash
crush install-hooks           # Install git hooks
crush pre-commit              # Run pre-commit checks
```

#### 13. Image Analysis Improvements
- Drag-drop images in TUI
- Screenshot annotation
- Diagram generation

#### 14. Model Comparison Mode
```bash
crush compare "write a binary search" --models gpt-4,claude-3,gemini-pro
```

#### 15. Cost Optimization
```json
{
  "cost": {
    "budget_daily": 10.00,
    "auto_switch_cheap": true,
    "warn_expensive": true
  }
}
```

---

## Implementation Roadmap

### Phase 1: Critical Features (Week 1-2)
- ✅ Git integration commands
- ✅ Programmatic mode (-p flag)
- ✅ Slash commands system
- ✅ Enhanced help system

### Phase 2: Advanced Features (Week 3-4)
- ✅ Plan/architect mode
- ✅ Test integration
- ✅ Session export/import
- ✅ Audit log

### Phase 3: Automation (Week 5-6)
- ✅ Daemon mode
- ✅ Watch mode
- ✅ CI/CD examples
- ✅ Pre-commit hooks

### Phase 4: UX Improvements (Week 7-8)
- ✅ Natural language CLI
- ✅ Image improvements
- ✅ Model comparison
- ✅ Cost optimization

---

## Recommended Immediate Actions

### 1. Quick Wins (Can implement today)

**Add -p/--prompt flag:**
```go
// cmd/root.go
var promptFlag string

func init() {
    rootCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "Run single prompt and exit")
}
```

**Add /model command:**
```go
// Parse slash commands in user input
if strings.HasPrefix(input, "/") {
    return handleSlashCommand(input)
}
```

**Add git helper commands:**
```go
// cmd/git.go
var gitCmd = &cobra.Command{
    Use:   "git",
    Short: "Git integration commands",
}

var commitCmd = &cobra.Command{
    Use:   "commit",
    Short: "AI-generated commit message",
    Run:   runCommit,
}
```

### 2. Medium Effort (This week)

- Implement plan/architect mode dialog
- Add session export functionality
- Create audit log table in SQLite
- Build slash command parser

### 3. Larger Projects (This month)

- Daemon mode architecture
- Test integration framework
- Natural language CLI parser
- Enhanced image handling

---

## Conclusion

Crush has a **strong foundation** but needs **feature parity** with competitors in key areas:

**Must-Have:**
1. Git integration (like Aider)
2. Programmatic mode (like Copilot CLI)
3. Slash commands (industry standard)
4. Plan/architect mode (Aider's killer feature)

**Should-Have:**
5. Test integration
6. Session export
7. Audit logging
8. Daemon mode

**Nice-to-Have:**
9. Natural language CLI
10. Watch mode
11. Model comparison

Implementing **Phase 1 + 2** (git, programmatic mode, slash commands, plan mode) would make Crush **highly competitive** with Aider and Copilot CLI, while maintaining its unique strengths in MCP, LSP, and session management.

The combination of Crush's **superior architecture** (MCP, LSP, sessions, permissions) with competitors' **workflow features** (git, planning, testing) would create the **most powerful AI CLI tool** in the market.

---

## Appendix: Feature Implementation Estimates

| Feature | Complexity | Effort | Dependencies |
|---------|-----------|--------|--------------|
| Git commands | Low | 1-2 days | None |
| -p flag | Low | 4 hours | None |
| Slash commands | Medium | 2-3 days | Parser, autocomplete |
| Plan mode | High | 1 week | UI dialogs, state machine |
| Test integration | Medium | 3-4 days | Config system |
| Session export | Low | 1 day | Serialization |
| Audit log | Medium | 2-3 days | Database schema |
| Daemon mode | High | 2 weeks | Process management, IPC |
| Natural language CLI | High | 1 week | Prompt engineering, validation |
| Watch mode | Medium | 3-4 days | File watching, debouncing |

**Total Phase 1+2 Effort:** 3-4 weeks for 1 developer
