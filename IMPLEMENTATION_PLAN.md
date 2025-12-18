# Karigor Implementation Plan

**Version:** 1.0
**Status:** Ready for Implementation
**Based on:** PRD.md v1.0
**Created:** 2025-12-18

---

## Executive Summary

This document provides a detailed, phase-by-phase implementation plan for transforming Crush into Karigor. The plan consists of 6 sequential phases with approximately 40 discrete tasks, organized to minimize risk and maintain the ability to merge upstream changes.

**Estimated Effort:** Medium complexity (2-3 days for experienced Go developer)
**Risk Level:** Low-Medium (non-destructive, mostly text changes)
**Testing Requirements:** High (must verify all user-facing changes)

---

## Implementation Strategy

### Approach

- **Sequential Phases:** Complete phases in order (1→6) to manage dependencies
- **Task Parallelization:** Within phases, independent tasks can run in parallel
- **Incremental Commits:** Commit after each phase for easy rollback
- **Testing Gates:** Test after each phase before proceeding
- **Branch Strategy:** Work in `feat/karigor-rebrand` branch

### Dependencies

```
Phase 1 (Foundation)
    ↓
Phase 2 (Provider)
    ↓
Phase 3 (Branding) ← Can work in parallel → Phase 4 (ASCII)
    ↓                                              ↓
Phase 5 (Schema & Docs) ←─────────────────────────┘
    ↓
Phase 6 (Testing & Validation)
```

---

## Phase 1: Foundation - Constants & Configuration

**Goal:** Update core constants and configuration infrastructure
**Duration:** 2-3 hours
**Priority:** CRITICAL - Everything depends on this

### Task 1.1: Update Core Constants

**File:** `internal/config/config.go`

**Changes:**
```go
// Line ~30
const (
    appName              = "karigor"  // was "crush"
    defaultDataDirectory = ".karigor" // was ".crush"
    defaultInitializeAs  = "KARIGOR.md" // was "AGENTS.md" (or keep AGENTS.md?)
)

// Line ~35 - Add to defaultContextPaths
var defaultContextPaths = []string{
    ".github/copilot-instructions.md",
    ".cursorrules",
    ".cursor/rules/",
    "CLAUDE.md",
    "CLAUDE.local.md",
    "GEMINI.md",
    "gemini.md",
    "karigor.md",           // ADD
    "karigor.local.md",     // ADD
    "Karigor.md",           // ADD
    "Karigor.local.md",     // ADD
    "KARIGOR.md",           // ADD
    "KARIGOR.local.md",     // ADD
    "AGENTS.md",
    "agents.md",
    "Agents.md",
}
```

**Acceptance Criteria:**
- [ ] `appName` constant = "karigor"
- [ ] `defaultDataDirectory` constant = ".karigor"
- [ ] KARIGOR.md paths added to defaultContextPaths
- [ ] Code compiles without errors

---

### Task 1.2: Update Config File Resolution

**File:** `internal/config/load.go`

**Changes:**
Search for all references to "crush.json" and ".crush.json", update to:
- `.crush.json` → `.karigor.json`
- `crush.json` → `karigor.json`
- `~/.config/crush/crush.json` → `~/.config/karigor/karigor.json`

**Specific Locations:**
- Config file search order function
- Path construction logic
- Error messages referencing config paths

**Acceptance Criteria:**
- [ ] Config loads from `.karigor.json` first
- [ ] Config loads from `karigor.json` second
- [ ] Global config loaded from `~/.config/karigor/karigor.json`
- [ ] Error messages reference correct file names

---

### Task 1.3: Update Environment Variables

**File:** `internal/env/env.go`

**Changes:**
Map old environment variables to new ones:
```go
// Old → New mappings
CRUSH_DISABLE_METRICS              → KARIGOR_DISABLE_METRICS
CRUSH_DISABLE_PROVIDER_AUTO_UPDATE → KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE
CRUSH_PROFILE                      → KARIGOR_PROFILE
```

**Add new:**
```go
KARIGOR_API_KEY // For ZAI API key
```

**Acceptance Criteria:**
- [ ] All CRUSH_* env vars replaced with KARIGOR_*
- [ ] KARIGOR_API_KEY environment variable supported
- [ ] Old CRUSH_* variables no longer work (clean break)
- [ ] Code compiles without errors

---

### Task 1.4: Update Data Directory References

**Files:**
- `internal/cmd/dirs.go`
- `internal/cmd/root.go` (createDotCrushDir function)
- Any file creating/accessing `.crush/` directory

**Changes:**
- `.crush/` → `.karigor/`
- `.crush/logs/` → `.karigor/logs/`
- `~/.local/share/crush/` → `~/.local/share/karigor/`

**Acceptance Criteria:**
- [ ] All directory paths use `.karigor/` naming
- [ ] Log files written to `.karigor/logs/`
- [ ] Database stored in `.karigor/`
- [ ] No references to `.crush/` in code

---

### Task 1.5: Update Schema Constants

**File:** `internal/cmd/schema.go`

**Changes:**
Update schema URL in generated schema:
```go
// Change schema URL output
"$schema": "https://charm.land/karigor.json"  // was crush.json
```

**Note:** This is display-only. Actual validation will use embedded schema (don't fetch from remote).

**Acceptance Criteria:**
- [ ] Generated schema.json has karigor.json URL
- [ ] Schema generation command works
- [ ] Output is valid JSON

---

## Phase 2: Provider Simplification

**Goal:** Remove all providers except ZAI, hardcode single provider
**Duration:** 2-3 hours
**Priority:** CRITICAL - UI depends on this

### Task 2.1: Hardcode ZAI Provider

**File:** `internal/config/provider.go`

**Changes:**
Create function to return hardcoded ZAI-only provider list:

```go
// Add new function
func getKarigorProvider() catwalk.Provider {
    return catwalk.Provider{
        ID:      catwalk.InferenceProviderZAI,
        Name:    "Karigor Chintok",
        BaseURL: "https://open.bigmodel.cn/api/paas/v4",
        Type:    catwalk.TypeOpenAICompat,
        Models: []catwalk.Model{
            {
                ID:                 "glm-4.6",
                Name:               "Karigor Chintok",
                ContextWindow:      204800,
                DefaultMaxTokens:   131072,
                CanReason:          true,
                ReasoningLevels:    []string{"low", "medium", "high"},
                DefaultReasoningEffort: "medium",
            },
        },
    }
}

// Modify Providers() function to return only ZAI
func (c *Config) Providers() []catwalk.Provider {
    return []catwalk.Provider{getKarigorProvider()}
}
```

**Acceptance Criteria:**
- [ ] Providers() returns exactly one provider
- [ ] Provider ID is "zai"
- [ ] Provider name is "Karigor Chintok"
- [ ] Model name is "Karigor Chintok"
- [ ] BaseURL points to ZAI API

---

### Task 2.2: Disable Provider Auto-Update

**File:** `internal/config/catwalk.go`

**Changes:**
```go
// Set default to disable auto-updates
func Init(...) {
    ...
    if cfg.Options.DisableProviderAutoUpdate == nil {
        disabled := true
        cfg.Options.DisableProviderAutoUpdate = &disabled
    }
    ...
}

// Override update function to no-op
func UpdateProviders() error {
    // Do nothing - providers are hardcoded
    return nil
}
```

**Acceptance Criteria:**
- [ ] DisableProviderAutoUpdate defaults to true
- [ ] UpdateProviders function returns immediately
- [ ] No network calls to Catwalk on startup

---

### Task 2.3: Remove Provider Selection UI

**Files:**
- `internal/tui/components/dialogs/models/` (model picker)
- Find provider selection dialog components

**Changes:**
Option A: Remove completely (recommended)
- Delete provider selection dialog
- Remove from dialog router

Option B: Simplify to read-only display
- Show "Karigor Chintok" as current model
- Disable switching

**Acceptance Criteria:**
- [ ] No provider selection UI available
- [ ] No model switching UI available
- [ ] Current model displays as "Karigor Chintok"

---

### Task 2.4: Set Default Model

**File:** `internal/config/config.go`

**Changes:**
```go
// Auto-set selected model to ZAI if not specified
func (c *Config) GetModelByType(modelType SelectedModelType) *catwalk.Model {
    // If no model selected, default to ZAI
    if c.SelectedModels == nil || c.SelectedModels[modelType].Provider == "" {
        provider := getKarigorProvider()
        return &provider.Models[0]
    }
    ...
}
```

**Acceptance Criteria:**
- [ ] First run automatically selects ZAI model
- [ ] No model selection prompt shown
- [ ] Status bar shows "Karigor Chintok"

---

## Phase 3: Branding & UI Text

**Goal:** Replace all "Crush" with "Karigor" in user-facing text
**Duration:** 3-4 hours
**Priority:** HIGH - User experience

### Task 3.1: Update Root Command

**File:** `internal/cmd/root.go`

**Changes:**
```go
// Line ~52
var rootCmd = &cobra.Command{
    Use:   "karigor",  // was "crush"
    Short: "Terminal-based AI assistant for software development",
    Long: `Karigor is a powerful terminal-based AI assistant that helps with software development tasks.
It provides an interactive chat interface with AI capabilities, code analysis, and LSP integration
to assist developers in writing, debugging, and understanding code directly from the terminal.`,
    Example: `
# Run in interactive mode
karigor

# Run with debug logging
karigor -d

# Run with debug logging in a specific directory
karigor -d -c /path/to/project

# Run with custom data directory
karigor -D /path/to/custom/.karigor

# Print version
karigor -v

# Run a single non-interactive prompt
karigor run "Explain the use of context in Go"

# Run in dangerous mode (auto-accept all permissions)
karigor -y
  `,
    RunE: func(cmd *cobra.Command, args []string) error {
        ...
        // Line ~104 - Update error message
        return errors.New("Karigor crashed. If metrics are enabled, we were notified about it. If you'd like to report it, please copy the stacktrace above and open an issue at https://github.com/charmbracelet/crush/issues/new?template=bug.yml")
        ...
    },
}
```

**Search and replace:**
- All "Crush" → "Karigor"
- All "crush" → "karigor" (in examples/usage)

**Acceptance Criteria:**
- [ ] `karigor --help` shows Karigor branding
- [ ] All examples use `karigor` command
- [ ] Error messages reference Karigor

---

### Task 3.2: Update CLI Commands

**Files:**
- `internal/cmd/run.go`
- `internal/cmd/logs.go`
- `internal/cmd/projects.go`
- `internal/cmd/login.go`
- `internal/cmd/update_providers.go`
- `internal/cmd/schema.go`

**Changes for each file:**
Search for:
- "Crush" → "Karigor"
- "crush" → "karigor"
- ".crush" → ".karigor"
- References to log paths, config paths

**Example (logs.go):**
```go
// Help text
Short: "View Karigor logs",
Long:  "Display logs from the Karigor application...",

// Log path
logPath := filepath.Join(cfg.Options.DataDirectory, "logs", "karigor.log")
```

**Acceptance Criteria:**
- [ ] All command help text uses Karigor
- [ ] Log commands reference correct paths
- [ ] No "Crush" in command output

---

### Task 3.3: Update TUI Window Title

**File:** `internal/tui/tui.go`

**Changes:**
Find window title setting, update to "Karigor"

**Acceptance Criteria:**
- [ ] Terminal window title shows "Karigor"
- [ ] No "Crush" in window title

---

### Task 3.4: Update TUI Components

**Files:** (Search all files in `internal/tui/`)
- `internal/tui/components/chat/`
- `internal/tui/components/dialogs/`
- `internal/tui/components/core/status/`
- `internal/tui/page/chat/`

**Strategy:**
```bash
# Find all "Crush" references in TUI
grep -r "Crush\|crush" internal/tui/ --include="*.go"
```

Replace user-facing text:
- Welcome messages
- Status bar text
- Dialog titles
- Help text
- Error messages

**Acceptance Criteria:**
- [ ] No "Crush" visible in TUI interface
- [ ] Status bar uses Karigor
- [ ] Dialog titles use Karigor

---

### Task 3.5: Update Git Commit Attribution

**Files:** (Search for "Generated with" and attribution)
- `internal/app/`
- Anywhere git commits are created

**Changes:**
```go
// Old
"💘 Generated with Crush"
"Co-Authored-By: Crush <crush@charm.land>"

// New
"Generated with Karigor"
"Assisted-by: Karigor Chintok <karigor@example.com>"
```

**Acceptance Criteria:**
- [ ] Git commits say "Generated with Karigor"
- [ ] Co-author attribution references Karigor
- [ ] No emoji in commit messages (see Phase 4)

---

### Task 3.6: Update PR Description Templates

**Files:** (Search for pull request creation code)

**Changes:**
```markdown
<!-- Old -->
💘 Generated with Crush

<!-- New -->
Generated with Karigor
```

**Acceptance Criteria:**
- [ ] PR descriptions reference Karigor
- [ ] No emoji in PR text

---

### Task 3.7: Update Error Messages

**Strategy:**
```bash
# Find all error messages
grep -r "fmt.Errorf\|errors.New" internal/ --include="*.go" | grep -i crush
```

**Changes:**
Replace any user-facing error messages mentioning Crush

**Acceptance Criteria:**
- [ ] Error messages use Karigor where appropriate
- [ ] Stack traces can still say "crush" (internal)

---

### Task 3.8: Update Splash Screen

**File:** `internal/tui/components/chat/splash/`

**Changes:**
Update welcome message, tips, any branding text

**Acceptance Criteria:**
- [ ] Splash screen shows Karigor branding
- [ ] Tips reference karigor commands

---

### Task 3.9: Update Help Text

**Files:** All files with help/usage text

**Strategy:**
Search for help strings, update examples

**Acceptance Criteria:**
- [ ] All help examples use `karigor` command
- [ ] Configuration examples show `.karigor.json`

---

### Task 3.10: Update Permission Dialog

**File:** `internal/tui/components/dialogs/permissions/`

**Changes:**
Update any text in permission prompts

**Acceptance Criteria:**
- [ ] Permission dialogs use Karigor branding

---

### Task 3.11: Update Session Management

**Files:** Session-related components

**Changes:**
Update any "Crush session" references to "Karigor session"

**Acceptance Criteria:**
- [ ] Session names/labels use Karigor

---

### Task 3.12: Update About/Version Info

**Files:** Version display code

**Changes:**
```go
// Version output
"Karigor version x.x.x"
```

**Acceptance Criteria:**
- [ ] `karigor --version` shows Karigor branding

---

## Phase 4: ASCII-Only Conversion

**Goal:** Remove all emoji and special unicode characters
**Duration:** 1-2 hours
**Priority:** MEDIUM - User experience

### Task 4.1: Find All Emoji

**Strategy:**
```bash
# Search for emoji
grep -r "💘\|❤️\|💖\|🎉\|✨\|🚀\|⚡\|🔥\|💡\|📝\|📋\|✅\|❌\|⚠️\|ℹ️" --include="*.go" .
```

**Files to check:**
- Commit message templates
- Attribution text
- Status indicators
- Error messages
- TUI decorations

**Acceptance Criteria:**
- [ ] Grep for emoji returns zero results in user-facing code
- [ ] Visual inspection shows no emoji in UI

---

### Task 4.2: Replace Commit Message Emoji

**Changes:**
```go
// Old
"💘 Generated with Crush"

// New
"Generated with Karigor"
```

**Acceptance Criteria:**
- [ ] No emoji in git commit templates
- [ ] Commits use plain text only

---

### Task 4.3: Replace Status Indicators

**Files:** Status bar, progress indicators

**Changes:**
Replace fancy characters with ASCII:
- ✅ → [OK]
- ❌ → [ERROR]
- ⚠️ → [WARNING]
- ℹ️ → [INFO]

**Acceptance Criteria:**
- [ ] All status indicators use ASCII
- [ ] No unicode symbols in status bar

---

### Task 4.4: Replace TUI Decorations

**Files:** TUI component decoration

**Changes:**
- Remove fancy bullets (•, ▸, etc.) → use `-`, `*`, `>`
- Remove unicode box drawing → use ASCII `-`, `|`, `+`

**Acceptance Criteria:**
- [ ] All decorative elements use standard ASCII
- [ ] UI renders correctly on basic terminals

---

## Phase 5: Schema & Documentation

**Goal:** Update schema and documentation files
**Duration:** 2 hours
**Priority:** MEDIUM - External resources

### Task 5.1: Generate Updated Schema

**Command:**
```bash
task schema
# or
go run main.go schema > schema.json
```

**Verify:**
- [ ] schema.json has "$schema": "https://charm.land/karigor.json"
- [ ] Schema structure is valid JSON
- [ ] No references to "crush" in schema

---

### Task 5.2: Update README.md

**File:** `README.md`

**Changes:**
Full rebrand of README:
- Title: Karigor
- Replace all Crush references
- Update installation commands
- Update configuration examples
- Update command examples

**Strategy:**
Since this is a fork/rebrand, consider:
- Keep original README for reference
- Create new KARIGOR_README.md
- Or fully replace (if complete fork)

**Acceptance Criteria:**
- [ ] README uses Karigor branding
- [ ] Examples show `.karigor.json`
- [ ] Commands use `karigor`

---

### Task 5.3: Update CLAUDE.md

**File:** `CLAUDE.md`

**Changes:**
- Update project name references
- Update file path examples
- Update command examples
- Keep technical architecture details (mostly unchanged)

**Acceptance Criteria:**
- [ ] CLAUDE.md references Karigor
- [ ] Build commands updated
- [ ] File paths updated

---

### Task 5.4: Create KARIGOR.md Context File

**File:** Create new `KARIGOR.md`

**Content:**
```markdown
# Karigor Configuration

This file is automatically loaded by Karigor as project context.

## About Karigor

Karigor is powered by ZAI's GLM-4.6 model, providing intelligent coding assistance.

## Project Information

[Your project-specific information here]
```

**Acceptance Criteria:**
- [ ] KARIGOR.md exists in repo root
- [ ] File is in defaultContextPaths

---

### Task 5.5: Update Build Configuration

**Files:**
- `Taskfile.yaml`
- `.goreleaser.yml` (if exists)
- GitHub Actions workflows

**Changes:**
```yaml
# Taskfile.yaml
build:
  desc: Build karigor
  cmds:
    - go build -o karigor .  # was crush

install:
  desc: Install karigor
  cmds:
    - go install .
```

**Acceptance Criteria:**
- [ ] Build produces `karigor` binary
- [ ] Install command works
- [ ] Binary is named correctly

---

## Phase 6: Testing & Validation

**Goal:** Verify all changes work correctly
**Duration:** 3-4 hours
**Priority:** CRITICAL - Quality assurance

### Task 6.1: Unit Tests

**Command:**
```bash
task test
# or
go test ./...
```

**Expected:**
- Some tests may fail due to path changes
- Update test fixtures if needed
- Update golden files if needed

**Acceptance Criteria:**
- [ ] All unit tests pass
- [ ] Test data updated for new paths
- [ ] No failing tests

---

### Task 6.2: Build Test

**Commands:**
```bash
task build
./karigor --version
./karigor --help
```

**Acceptance Criteria:**
- [ ] Binary builds successfully
- [ ] Binary named `karigor`
- [ ] Version shows Karigor
- [ ] Help text shows Karigor

---

### Task 6.3: Configuration Loading Test

**Test:**
```bash
# Create test config
mkdir -p ~/.config/karigor
cat > ~/.config/karigor/karigor.json <<EOF
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "zai": {
      "api_key": "test-key"
    }
  }
}
EOF

# Run karigor
./karigor
```

**Acceptance Criteria:**
- [ ] Config loads from ~/.config/karigor/
- [ ] No errors about missing config
- [ ] Correct paths displayed in debug output

---

### Task 6.4: Provider Test

**Test:**
Launch Karigor and check:
- Provider list shows only "Karigor Chintok"
- No other providers visible
- Model shows "Karigor Chintok"

**Acceptance Criteria:**
- [ ] Only one provider available
- [ ] Provider named "Karigor Chintok"
- [ ] Model selection UI removed/simplified

---

### Task 6.5: Branding Audit

**Manual Test:**
Run through entire TUI and check:
- Welcome screen
- Status bar
- Help text
- Error messages
- Dialogs
- Settings

**Acceptance Criteria:**
- [ ] No "Crush" visible anywhere
- [ ] All text says "Karigor"
- [ ] Screenshots look correct

---

### Task 6.6: ASCII Compliance Test

**Manual Test:**
Run Karigor and verify:
- No emoji displayed
- All characters render on basic terminal
- Test on multiple terminal emulators

**Acceptance Criteria:**
- [ ] No emoji in any UI element
- [ ] All characters are standard ASCII
- [ ] Renders correctly on xterm, Terminal.app, Windows Terminal

---

### Task 6.7: Environment Variable Test

**Test:**
```bash
export KARIGOR_API_KEY="test-key"
export KARIGOR_DISABLE_METRICS=1
./karigor
```

**Acceptance Criteria:**
- [ ] KARIGOR_API_KEY is recognized
- [ ] KARIGOR_DISABLE_METRICS works
- [ ] Old CRUSH_* variables don't work

---

### Task 6.8: End-to-End Test

**Test Scenario:**
1. Fresh install (no existing config)
2. Run `karigor`
3. Enter ZAI API key
4. Start chat session
5. Send test message
6. Create git commit
7. Check commit message

**Acceptance Criteria:**
- [ ] First-run experience works
- [ ] API key saved correctly
- [ ] Chat works with ZAI API
- [ ] Commit message says "Generated with Karigor"
- [ ] No errors in logs

---

## Implementation Checklist

### Pre-Implementation

- [ ] Create feature branch: `git checkout -b feat/karigor-rebrand`
- [ ] Review PRD.md thoroughly
- [ ] Set up test environment
- [ ] Backup current working state

### Phase Completion

- [ ] Phase 1: Foundation completed and tested
- [ ] Phase 2: Provider completed and tested
- [ ] Phase 3: Branding completed and tested
- [ ] Phase 4: ASCII completed and tested
- [ ] Phase 5: Schema & Docs completed and tested
- [ ] Phase 6: Testing & Validation completed

### Post-Implementation

- [ ] All tests passing
- [ ] No "Crush" in user-facing text
- [ ] No emoji in UI
- [ ] Binary named `karigor`
- [ ] Config uses `.karigor.json`
- [ ] Documentation updated
- [ ] README updated
- [ ] Create PR for review

---

## Validation Commands

### Quick Check Commands

```bash
# Check for Crush references (should return 0 user-facing matches)
grep -ri "crush" internal/cmd internal/tui --include="*.go" | grep -v "// " | grep -v "package"

# Check for emoji (should return 0)
grep -r "💘\|❤️\|💖\|🎉" --include="*.go" .

# Check environment variables (should return 0 CRUSH_)
grep -r "CRUSH_" --include="*.go" internal/

# Check config paths (should all be karigor)
grep -r "\.crush\|/crush/" --include="*.go" internal/config internal/cmd

# Verify binary name
ls -la karigor
./karigor --version
```

### Full Validation Script

```bash
#!/bin/bash
echo "Karigor Validation Script"
echo "========================="

# Build
echo "Building..."
task build || exit 1

# Check binary exists
if [ ! -f "./karigor" ]; then
    echo "❌ Binary not found"
    exit 1
fi
echo "✓ Binary exists"

# Check version output
if ./karigor --version | grep -q "Karigor"; then
    echo "✓ Version shows Karigor"
else
    echo "❌ Version shows wrong branding"
    exit 1
fi

# Check for Crush in user-facing code
CRUSH_COUNT=$(grep -ri "crush" internal/cmd internal/tui --include="*.go" | grep -v "// " | grep -v "package" | wc -l)
if [ "$CRUSH_COUNT" -eq 0 ]; then
    echo "✓ No Crush references in UI code"
else
    echo "❌ Found $CRUSH_COUNT Crush references"
    exit 1
fi

# Check for emoji
EMOJI_COUNT=$(grep -r "💘\|❤️\|💖" --include="*.go" . | wc -l)
if [ "$EMOJI_COUNT" -eq 0 ]; then
    echo "✓ No emoji in code"
else
    echo "❌ Found $EMOJI_COUNT emoji"
    exit 1
fi

# Run tests
echo "Running tests..."
task test || exit 1
echo "✓ All tests pass"

echo ""
echo "========================="
echo "✓ All validation checks passed!"
```

---

## Risk Mitigation

### Identified Risks

1. **Breaking Changes:** Config file path changes break existing users
   - **Mitigation:** This is a fork/rebrand, clean break is acceptable

2. **Incomplete Rebranding:** Missing some Crush references
   - **Mitigation:** Use comprehensive grep searches, manual testing

3. **Provider API Issues:** ZAI API might not work as expected
   - **Mitigation:** Test API connectivity early in Phase 2

4. **Emoji Removal Impact:** Some UI elements might look worse without emoji
   - **Mitigation:** Use clear ASCII alternatives, test on multiple terminals

5. **Schema Validation Errors:** Users' editors might complain about karigor.json schema
   - **Mitigation:** Consider hosting karigor.json schema OR disable remote validation

### Rollback Plan

If critical issues found:
```bash
# Rollback to main
git checkout main

# Or rollback specific phase
git revert <phase-commit-hash>
```

---

## Success Metrics

### Functional Success
- [ ] 100% of "Crush" replaced in user-facing text
- [ ] Config files work with .karigor.json
- [ ] Single ZAI provider successfully connects
- [ ] Zero emoji in UI
- [ ] Schema displays correctly

### Quality Metrics
- [ ] All unit tests pass
- [ ] Manual testing reveals no branding issues
- [ ] Performance unchanged from original Crush
- [ ] All features work identically

### User Success
- [ ] First-run setup completes in < 2 minutes
- [ ] Clear, consistent branding throughout
- [ ] Works on all major terminals (tested)

---

## Next Steps After Implementation

1. **Documentation:**
   - Create user migration guide (if needed)
   - Update any API documentation
   - Create contributor guide for Karigor fork

2. **Distribution:**
   - Set up release pipeline
   - Create installation packages
   - Update package manager configs

3. **Maintenance:**
   - Decide on upstream merge strategy
   - Set up CI/CD for Karigor
   - Plan for version numbering

4. **Community:**
   - Create Karigor repository (if separate from Crush)
   - Set up issue templates
   - Create contribution guidelines

---

**Document Status:** Ready for Implementation
**Last Updated:** 2025-12-18
**Approved By:** [Pending]
