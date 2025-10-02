# Fork Sustainability Strategy

ᕕ( ᐛ )ᕗ  Staying nimble while tracking innovation

## The Challenge

We want Cliffy to:
1. Pull in improvements from Crush as they develop
2. Learn from OpenCode, Codex, Gemini innovations
3. Maintain our specialized focus (fast, one-off, CLI-first)
4. Not get bogged down in merge conflicts
5. Keep our codebase simple and fast

## Current Architecture

```
cliffy/
├── cmd/cliffy/          # Cliffy-specific entry point
├── internal/
│   ├── llm/            # Shared with Crush (providers, tools, agent)
│   ├── config/         # Shared with Crush
│   ├── lsp/            # Shared with Crush
│   └── fsext/          # Shared with Crush
└── go.mod              # Direct dependencies
```

**Key insight:** Most of our value comes from the `internal/llm/` system, which Crush actively maintains.

## Dependency Options

### Option 1: Full Crush Dependency (Go Module)

**Structure:**
```go
// go.mod
module github.com/ettio/cliffy

require (
    github.com/charmbracelet/crush v0.10.4
)
```

```
cliffy/
├── cmd/cliffy/main.go
├── internal/
│   └── runner/         # Cliffy-specific runner
└── go.mod
```

**Pros:**
- Get Crush updates automatically via `go get -u`
- Minimal code duplication
- Always compatible with Crush's latest

**Cons:**
- Pulls in ALL Crush dependencies (TUI, database, etc.)
- Binary bloat from unused code
- Can't easily customize shared components
- Tied to Crush's release schedule

**Binary size impact:** Likely 2-3x larger due to unused deps

### Option 2: Catwalk-Only Dependency

**Structure:**
```go
// go.mod
module github.com/ettio/cliffy

require (
    github.com/charmbracelet/catwalk v0.x.x  // LLM provider abstraction
)
```

```
cliffy/
├── cmd/cliffy/main.go
├── internal/
│   ├── agent/          # Forked from Crush
│   ├── tools/          # Forked from Crush
│   ├── config/         # Cliffy-specific
│   └── runner/         # Cliffy-specific
└── go.mod
```

**Pros:**
- Minimal dependencies (just provider layer)
- Full control over agent/tools logic
- Smaller binary
- Can optimize without Crush constraints

**Cons:**
- Need to manually port Crush improvements
- Tool updates require active tracking
- More maintenance burden

### Option 3: Structured Sync (Recommended)

**Structure:** Keep forked code but maintain sync process

```
cliffy/
├── cmd/cliffy/          # Cliffy-specific
├── internal/
│   ├── llm/            # SYNCED from Crush
│   │   ├── agent/
│   │   ├── tools/
│   │   └── provider/   # Via catwalk
│   ├── config/         # DIVERGED (Cliffy-specific)
│   ├── runner/         # CLIFFY-ONLY
│   └── fsext/          # SYNCED from Crush
├── .crush-sync/
│   ├── last-sync.txt   # Track last merged commit
│   └── sync.sh         # Semi-automated sync script
└── go.mod
```

**Sync process:**
```bash
# Monthly (or when Crush has exciting changes)
./scripts/sync-from-crush.sh

# Script does:
# 1. Fetch latest Crush commits
# 2. Show changes to synced directories
# 3. Let us cherry-pick relevant changes
# 4. Skip TUI/database/session changes
# 5. Update last-sync marker
```

**Pros:**
- Keep Crush innovations in tools/agent
- Full control over what we adopt
- No unused dependencies
- Clear process for staying current

**Cons:**
- Requires discipline to run sync
- Manual merge conflicts (but rare)
- Need to track which components sync vs diverge

## Tracking Other Innovations

### OpenCode, Codex, Gemini

These projects innovate in different areas:
- **OpenCode:** Tool definitions, prompt engineering
- **Codex:** Code understanding, context management
- **Gemini:** Multi-modal inputs, long context handling

**Strategy:** Watch, don't couple

1. **Monitor:** Follow releases and changelogs
2. **Evaluate:** Does this fit Cliffy's fast-execution model?
3. **Extract:** Port the idea, not the code
4. **Credit:** Note inspiration in comments

**Example:**
```go
// internal/llm/tools/analyze.go
// Inspired by OpenCode's structural analysis approach
// Adapted for Cliffy's streaming execution model
```

### Specific Areas to Watch

| Project | Watch For | Cliffy Application |
|---------|-----------|-------------------|
| Crush | New tools, agent improvements, provider support | Direct sync via structured process |
| OpenCode | Tool schemas, prompt strategies | Port ideas when relevant |
| Codex | Context optimization, caching | Performance wins for one-off execution |
| Gemini | Long-context handling, vision | Add when API stabilizes |
| Continue | IDE integration patterns | Not applicable (CLI-focused) |
| Aider | Git workflow tools | Potentially sync commit/PR tools |

## Recommended Approach: Structured Sync

### Phase 1: Set Up Sync Infrastructure (Week 1)

**Create sync script:**
```bash
#!/usr/bin/env bash
# scripts/sync-from-crush.sh

CRUSH_REMOTE="https://github.com/charmbracelet/crush"
LAST_SYNC=$(cat .crush-sync/last-sync.txt)

# Add Crush as remote if needed
git remote add crush $CRUSH_REMOTE 2>/dev/null || true
git fetch crush

echo "ᕕ( ᐛ )ᕗ  Checking for Crush updates since $LAST_SYNC"

# Show what changed in synced directories
SYNCED_DIRS=(
    "internal/llm/agent"
    "internal/llm/tools"
    "internal/lsp"
    "internal/fsext"
)

for dir in "${SYNCED_DIRS[@]}"; do
    echo "\n📁 Changes in $dir:"
    git log $LAST_SYNC..crush/main --oneline -- $dir
done

echo "\n🤔 Review changes above, then:"
echo "  1. Cherry-pick commits you want: git cherry-pick <commit>"
echo "  2. Or merge directory: git checkout crush/main -- internal/llm/tools"
echo "  3. Update sync marker: echo <commit> > .crush-sync/last-sync.txt"
```

**Create sync documentation:**
- Which directories sync from Crush
- Which directories are Cliffy-specific
- When to sync (monthly? before releases?)
- How to handle conflicts

### Phase 2: Initial Sync (Week 1)

1. Document current Crush commit as baseline
2. Mark which files/directories will sync vs diverge
3. Test that sync process works
4. Set up reminder to check monthly

### Phase 3: Regular Cadence (Ongoing)

**Monthly sync ritual:**
1. Check Crush changelog for exciting features
2. Run sync script to see changes
3. Evaluate: Does this fit Cliffy's model?
4. If yes: Merge and test
5. If no: Document why we skipped
6. Update sync marker

**Ad-hoc sync:**
- When Crush fixes a critical bug in tools
- When Crush adds a new provider
- When Crush improves prompt engineering

### Phase 4: Innovation Tracking (Ongoing)

**Set up watch list:**
```markdown
# .innovation-watch/README.md

## Active Monitoring

### Crush
- Watch: New releases, tool additions, agent improvements
- Sync: Monthly via sync script
- Last checked: 2025-10-01

### OpenCode
- Watch: Tool schemas, prompt strategies
- Check: Quarterly
- Last checked: -

### Codex
- Watch: Context optimization, caching
- Check: When relevant to performance work
- Last checked: -
```

## Component Classification

### SYNC Components (Keep in sync with Crush)

These benefit from Crush's active development:

- `internal/llm/agent/` - Core agent loop, streaming, events
- `internal/llm/tools/` - All tool implementations
- `internal/lsp/` - LSP client and handlers
- `internal/fsext/` - File system utilities
- `internal/message/` - Message types

**Sync frequency:** Monthly or on critical updates

### DIVERGED Components (Cliffy-specific)

These are intentionally different:

- `cmd/cliffy/` - Entry point and CLI args
- `internal/config/` - Headless-optimized config
- `internal/runner/` - Direct execution, no sessions
- `internal/output/` - Streaming output formatter

**Never sync** - but watch for ideas

### REMOVED Components (Don't sync)

We intentionally don't have:

- `internal/tui/` - Interactive UI
- `internal/db/` - Session persistence
- `internal/session/` - Session management
- `internal/permission/` - Interactive permissions

**Skip entirely** in sync process

## Measuring Success

### Good Signs

- ✓ Pull in Crush tool improvements within 1 month of release
- ✓ Zero conflicts when syncing unmodified directories
- ✓ Clear documentation of why we skip certain Crush features
- ✓ Cliffy-specific optimizations don't conflict with Crush patterns

### Warning Signs

- ⚠ Sync script hasn't run in 3+ months
- ⚠ Manual changes to synced directories (should stay pure Crush)
- ⚠ Falling behind on critical bug fixes
- ⚠ Sync process takes more than 1 hour

## Decision Framework

When evaluating Crush or other project's features:

```
┌─────────────────────────────────────┐
│ Does it fit Cliffy's model?         │
│ - One-off execution                 │
│ - No persistence needed             │
│ - Fast startup critical             │
│ - CLI/automation focused            │
└──────────┬──────────────────────────┘
           │
    ┌──────┴──────┐
    │             │
  YES            NO
    │             │
    ├─ Sync it   └─ Skip but document why
    │
    ├─ Does it add latency?
    │   ├─ NO → Merge as-is
    │   └─ YES → Optimize for streaming
    │
    └─ Test impact on cold start time
```

## Next Steps

1. **Week 1:** Set up sync infrastructure
   - Create sync script
   - Document component classification
   - Establish baseline Crush commit

2. **Week 2:** Test sync process
   - Run initial sync dry-run
   - Resolve any conflicts
   - Document learnings

3. **Monthly:** Run sync cadence
   - Check Crush releases
   - Sync relevant changes
   - Update innovation watch list

4. **Quarterly:** Review strategy
   - Is sync process working?
   - Do we need tighter/looser coupling?
   - What innovations should we track?

## The Philosophy

**Cliffy is a specialized tool built on Crush's excellent foundation.**

We're not trying to be a separate fork that drifts apart. We're trying to be a focused variant that:
- Keeps the best parts of Crush (tools, providers, agent loop)
- Optimizes ruthlessly for our use case (fast, one-off, headless)
- Stays current with Crush innovations via structured sync
- Contributes back when we find universal improvements

Think of it like Formula 1 vs Rally racing. Same core technology (combustion engine, aerodynamics) but optimized for totally different tracks. F1 teams still learn from rally innovations and vice versa.

ᕕ( ᐛ )ᕗ  Standing on Crush's shoulders, sprinting our own race
