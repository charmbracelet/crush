# Fork Strategy: Sync vs. Hard Fork

## TL;DR Recommendation

**Hybrid approach:** Structured sync with selective imports

**Why:**
- 82% of Crush development is on shared components (tools, providers, fixes)
- Headless benefits massively from these improvements
- But: Architecture divergence makes full merges painful
- Solution: Import specific packages as Go modules, not git merges

## Analysis

### Commit Activity (Since Jan 2024)

Based on git history:
- **Tool/provider/fix commits:** 821
- **TUI/UI/session commits:** 173
- **Ratio:** 4.7:1 in favor of shared components

**What this means:**
- Most Crush development is on components headless **needs**
- UI work (which headless ignores) is minority
- Hard fork = missing 82% of valuable improvements

### Component Breakdown

#### Shared Components (82% of changes)

**1. Provider Layer (`internal/llm/provider/`)**

Recent improvements you'd miss:
- Gemini MIME type fix (Dec 2024)
- Stream timeout handling (Dec 2024)
- OpenAI 429 retry logic (Dec 2024)
- Bedrock support improvements
- New provider additions

**2. Tool Implementations (`internal/llm/tools/`)**

Recent improvements:
- MCP tool fixes
- Tool error handling
- New tool types
- Performance optimizations

**3. LSP Integration (`internal/lsp/`)**

Recent improvements:
- Root marker improvements
- Initialization fixes
- Diagnostic enhancements

**4. Config System (`internal/config/`)**

Recent improvements:
- Provider resolution
- Model management
- Environment expansion

**5. Core Libraries**

Dependencies that update frequently:
- `anthropic-sdk-go` - API changes, new features
- `openai-go` - Model updates
- `mark3labs/mcp-go` - MCP protocol updates
- `catwalk` - Model definitions

#### Divergent Components (18% of changes)

These you DON'T want:
- `internal/tui/` - Full TUI system
- `internal/db/` - SQLite persistence
- `internal/pubsub/` - Event system
- `internal/session/` - Session management
- `internal/message/` - Message CRUD (keep types only)

## Fork Strategy Options

### Option 1: Hard Fork (Independent)

**What it means:**
```
crush/ (upstream)
  └─ continues independently

crush-headless/ (fork)
  └─ never syncs, fully independent
```

**Pros:**
- ✅ Complete independence
- ✅ No merge conflicts
- ✅ Simplest initial setup
- ✅ Custom architecture

**Cons:**
- ❌ Miss 821 improvements/year
- ❌ Manually port provider updates
- ❌ Manually port tool fixes
- ❌ Manually port security fixes
- ❌ Duplicate bug fixing
- ❌ Drift over time

**Maintenance burden:** HIGH
- Every provider fix needs manual porting
- Every tool improvement needs reimplementation
- New models need manual addition
- Security issues need separate patches

**Best for:** If Crush changes architecture radically or headless diverges completely.

### Option 2: Git Sync Fork (Continuous Merge)

**What it means:**
```
crush/ (upstream)
  ├─ git merge regularly
  └─ crush-headless/ (fork)
```

**Pros:**
- ✅ Automatic updates
- ✅ Full improvement stream

**Cons:**
- ❌ Constant merge conflicts (DB, pubsub, TUI changes)
- ❌ Architecture drift = painful merges
- ❌ Headless removed 65% of code, merges break
- ❌ Time-consuming maintenance

**Maintenance burden:** VERY HIGH
- Every merge requires conflict resolution
- Changes to removed components cause conflicts
- Refactors across boundaries break everything

**Best for:** If headless was a mode, not a fork.

### Option 3: Structured Sync (Go Module Import) ⭐ RECOMMENDED

**What it means:**
```
crush/ (upstream)
  ├─ published as versioned packages
  └─ crush-headless/ (independent repo)
      └─ imports specific packages as dependencies

// go.mod in crush-headless:
require (
  github.com/charmbracelet/crush/internal/llm/provider v0.10.4
  github.com/charmbracelet/crush/internal/llm/tools v0.10.4
  github.com/charmbracelet/crush/internal/lsp v0.10.4
  github.com/charmbracelet/crush/internal/config v0.10.4
  // ... only what you need
)
```

**Pros:**
- ✅ Automatic updates via `go get -u`
- ✅ Import only shared components
- ✅ No merge conflicts
- ✅ Semantic versioning
- ✅ Test before upgrading
- ✅ Pin to stable versions

**Cons:**
- ⚠️ Requires Crush to export packages (minor refactor)
- ⚠️ Breaking changes need version management
- ⚠️ Can't cherry-pick individual commits

**Maintenance burden:** LOW
- `go get -u github.com/charmbracelet/crush/internal/llm/provider`
- Test
- Update if needed

**Best for:** When you want improvements but not merges.

### Option 4: Monorepo with Build Tags

**What it means:**
```
crush/ (single repo)
├─ internal/
│   ├─ llm/         // shared
│   ├─ tui/         // +build !headless
│   ├─ db/          // +build !headless
│   └─ headless/    // +build headless
├─ cmd/
│   ├─ crush/       // +build !headless
│   └─ headless/    // +build headless
```

**Build:**
```bash
go build -tags headless ./cmd/headless
```

**Pros:**
- ✅ Single codebase
- ✅ Automatic sync
- ✅ Shared CI/CD
- ✅ No import issues

**Cons:**
- ❌ Headless code in main repo
- ❌ Confusing for contributors
- ❌ Build tags everywhere
- ❌ Harder to optimize separately

**Best for:** If Crush team wants to own headless.

## Recommended: Structured Sync (Option 3)

### Implementation Plan

#### Phase 1: Crush Refactor (Upstream Work)

**Make packages importable:**

```go
// Currently internal:
github.com/charmbracelet/crush/internal/llm/provider

// Refactor to:
github.com/charmbracelet/crush/pkg/provider
github.com/charmbracelet/crush/pkg/tools
github.com/charmbracelet/crush/pkg/lsp
github.com/charmbracelet/crush/pkg/config
```

**Or use internal with replace directive:**
```go
// go.mod in crush-headless
replace github.com/charmbracelet/crush/internal/llm/provider => ../crush/internal/llm/provider
```

#### Phase 2: Headless Setup

**Directory structure:**
```
crush-headless/
├─ go.mod
│   require github.com/charmbracelet/crush v0.10.4
├─ internal/
│   ├─ runner/      // headless-specific
│   ├─ stream/      // headless-specific
│   └─ executor/    // headless-specific
└─ cmd/headless/main.go
```

**Import shared:**
```go
import (
    "github.com/charmbracelet/crush/internal/llm/provider"
    "github.com/charmbracelet/crush/internal/llm/tools"
    "github.com/charmbracelet/crush/internal/config"
    "github.com/charmbracelet/crush/internal/lsp"
)
```

#### Phase 3: Update Workflow

**When Crush releases v0.11.0:**

```bash
cd crush-headless
go get github.com/charmbracelet/crush@v0.11.0
go test ./...
# If tests pass, commit
git commit -m "deps: upgrade crush to v0.11.0"
```

**When there's a breaking change:**
```bash
# Read CHANGELOG
# Update headless code to match new API
# Then upgrade
```

### Version Management

**Strategy:** Lag behind Crush by 1-2 minor versions

```
Crush releases:     v0.10.0 → v0.10.5 → v0.11.0 → v0.11.3
Headless adopts:    ------- → v0.10.5 --------- → v0.11.3
                    (test period)     (test)
```

**Why lag:**
- Let bugs shake out in main Crush
- Multiple patch releases = stable
- Breaking changes get documented

### Handling Breaking Changes

**Example: Provider API changes**

**Crush v0.10.x:**
```go
provider.NewProvider(cfg, opts...)
```

**Crush v0.11.0:**
```go
provider.NewProvider(ctx, cfg, opts...)  // Breaking: added ctx
```

**Headless response:**
1. Pin to v0.10.x until ready
2. Update headless code to match
3. Upgrade when compatible

**In go.mod:**
```go
require github.com/charmbracelet/crush v0.10.5  // Stay here until v0.11 compat
```

### What You'd Get from Syncing

**Recent improvements (last 3 months):**

1. **MCP SSE fix** (v0.10.4)
   - Headless benefits: MCP tools work better

2. **Gemini MIME type fix**
   - Headless benefits: Correct image handling

3. **Stream timeout handling**
   - Headless benefits: No hanging streams

4. **OpenAI 429 retry logic**
   - Headless benefits: Better rate limit handling

5. **LSP root marker improvements**
   - Headless benefits: Better LSP initialization

**Annual value: ~800 improvements you'd otherwise miss**

### What You'd Ignore from Syncing

**UI changes you don't need:**
- TUI component updates
- Session list improvements
- Interactive key bindings
- Spinner animations
- Theme updates

**These don't affect headless at all.**

## Dependency Considerations

### Critical Dependencies

**Direct from Crush:**
- Provider implementations
- Tool implementations
- LSP integration
- Config system

**Indirect (through Crush):**
- `anthropic-sdk-go`
- `openai-go`
- `google.generativeai`
- `mark3labs/mcp-go`

**If you hard fork:** You maintain these relationships yourself.

**If you sync:** Crush maintains them, you inherit.

### Breaking Change Risk

**Low risk areas (stable APIs):**
- Tool interface (hasn't changed in months)
- Provider interface (stable)
- Config structure (additive changes)

**Medium risk areas:**
- Provider options (new fields added)
- Tool metadata (schema evolution)

**High risk areas:**
- Internal message format (but headless uses minimal)
- Database schema (irrelevant to headless)

**Mitigation:**
- Comprehensive test suite
- Integration tests against Crush packages
- Version pinning

## Cost-Benefit Analysis

### Hard Fork

**Time cost:**
- Initial: Low (just copy code)
- Ongoing: 5-10 hours/month porting fixes
- Annual: 60-120 hours

**Risk:**
- Miss critical security fixes
- Lag behind on provider updates
- Duplicate bug fixing effort

### Structured Sync

**Time cost:**
- Initial: Medium (Crush needs to export packages)
- Ongoing: 1-2 hours/quarter for upgrades
- Annual: 4-8 hours

**Risk:**
- Breaking changes need adaptation
- Can pin to last-known-good version

**Winner:** Structured sync (93% less maintenance)

## Real-World Examples

### Success: Docker CLI & Docker Engine

**Structure:**
- `docker/cli` (separate repo)
- Imports `docker/docker` packages
- Independent release cycles
- Syncs when needed

**Result:** CLI stays lean, benefits from engine improvements

### Success: kubectl & Kubernetes

**Structure:**
- `kubernetes/kubernetes` (main)
- `kubernetes/kubectl` (could be separate)
- Shared client libraries
- Independent versioning

**Result:** Client tools benefit from API improvements

### Failure: MySQL Forks (MariaDB, Percona)

**Structure:**
- Hard forks that diverged
- Manually port features
- Duplicate effort

**Result:** Eventually become incompatible, separate ecosystems

## Decision Matrix

| Criteria | Hard Fork | Git Sync | Structured Sync | Monorepo |
|----------|-----------|----------|-----------------|----------|
| Get improvements | ❌ Manual | ✅ Auto | ✅ Auto | ✅ Auto |
| Merge conflicts | ✅ None | ❌ Many | ✅ None | ⚠️ Some |
| Maintenance time | ❌ High | ❌ Very high | ✅ Low | ⚠️ Medium |
| Independence | ✅ Full | ❌ Limited | ✅ High | ❌ Shared |
| Breaking changes | ✅ Isolated | ❌ Immediate | ⚠️ Controlled | ⚠️ Controlled |
| Test before adopt | ❌ N/A | ❌ Hard | ✅ Easy | ⚠️ Possible |
| Code clarity | ✅ Clean | ❌ Mixed | ✅ Clean | ⚠️ Tags |

**Score:**
- Structured Sync: 6/7 ✅
- Hard Fork: 3/7
- Git Sync: 1/7
- Monorepo: 4/7

## Recommendation

**Go with Structured Sync:**

1. **Short term (Phase 1):**
   - Hard fork initially to move fast
   - Get headless working
   - Prove the concept

2. **Medium term (Phase 2-3):**
   - Coordinate with Crush team to export packages
   - Migrate to module imports
   - Set up automated dependency updates

3. **Long term (Phase 4+):**
   - Quarterly sync reviews
   - Test against new versions
   - Upgrade when stable

**Fallback plan:**
- If Crush won't export packages: Hard fork
- If headless diverges too much: Hard fork
- If breaking changes too frequent: Pin versions longer

## Implementation Checklist

**Week 1-4: Initial Hard Fork**
- [x] Copy shared code
- [x] Build headless architecture
- [x] Prove performance gains

**Week 5-8: Prepare for Sync**
- [ ] Document shared dependencies
- [ ] Create integration test suite
- [ ] Identify API boundaries

**Week 9-12: Enable Sync (if Crush team agrees)**
- [ ] Crush exports packages
- [ ] Migrate to module imports
- [ ] Set up CI to test against multiple versions

**Ongoing:**
- [ ] Quarterly upgrade reviews
- [ ] Monitor Crush releases
- [ ] Contribute fixes upstream when applicable

## Questions for Crush Team

Before committing to structured sync:

1. **Would you export packages?**
   - Move `internal/llm`, `internal/lsp`, etc. to `pkg/`?
   - Or comfortable with `internal` import via replace?

2. **Semantic versioning?**
   - Tag releases with semver?
   - Document breaking changes?

3. **API stability?**
   - Intention to keep provider/tool interfaces stable?
   - Deprecation policy?

4. **Collaboration?**
   - Accept PRs from headless discoveries?
   - Shared issue tracking for providers/tools?

If answers are positive → Structured sync
If answers are negative → Hard fork

## Conclusion

**Recommended strategy: Structured Sync**

**Reasoning:**
- 82% of Crush development is shared components
- Missing improvements = high opportunity cost
- Structured sync = 93% less maintenance than hard fork
- Can always fall back to hard fork if needed

**Next steps:**
1. Start as hard fork (get to MVP fast)
2. Validate architecture
3. Approach Crush team about package exports
4. Migrate to structured sync when mature

**Risk mitigation:**
- Comprehensive test suite
- Version pinning
- Integration tests
- Quarterly review cycle

This gives you the best of both worlds: independence to innovate + automatic improvements from upstream.
