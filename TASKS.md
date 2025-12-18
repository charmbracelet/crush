# Karigor Implementation Task Tracker

**Quick Reference:** Track implementation progress phase-by-phase

---

## Phase 1: Foundation (CRITICAL) ⬜

- [ ] **1.1** Update core constants in `internal/config/config.go`
  - [ ] Change `appName` to "karigor"
  - [ ] Change `defaultDataDirectory` to ".karigor"
  - [ ] Update `defaultContextPaths` to include KARIGOR.md paths
  - [ ] Verify code compiles

- [ ] **1.2** Update config file resolution in `internal/config/load.go`
  - [ ] Change `.crush.json` → `.karigor.json`
  - [ ] Change `crush.json` → `karigor.json`
  - [ ] Update global config path to `~/.config/karigor/`
  - [ ] Update error messages

- [ ] **1.3** Update environment variables in `internal/env/env.go`
  - [ ] Replace CRUSH_DISABLE_METRICS → KARIGOR_DISABLE_METRICS
  - [ ] Replace CRUSH_DISABLE_PROVIDER_AUTO_UPDATE → KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE
  - [ ] Replace CRUSH_PROFILE → KARIGOR_PROFILE
  - [ ] Add KARIGOR_API_KEY support

- [ ] **1.4** Update data directory references
  - [ ] Update `internal/cmd/dirs.go`
  - [ ] Update `internal/cmd/root.go` (createDotCrushDir)
  - [ ] Change all `.crush/` → `.karigor/`
  - [ ] Verify log paths use `.karigor/logs/`

- [ ] **1.5** Update schema constants in `internal/cmd/schema.go`
  - [ ] Change schema URL to "https://charm.land/karigor.json"
  - [ ] Test schema generation
  - [ ] Verify valid JSON output

**Phase 1 Complete:** ⬜

---

## Phase 2: Provider Simplification (CRITICAL) ⬜

- [ ] **2.1** Hardcode ZAI provider in `internal/config/provider.go`
  - [ ] Create `getKarigorProvider()` function
  - [ ] Modify `Providers()` to return only ZAI
  - [ ] Set provider name to "Karigor Chintok"
  - [ ] Set model name to "Karigor Chintok"
  - [ ] Verify correct ZAI API endpoint

- [ ] **2.2** Disable provider auto-update in `internal/config/catwalk.go`
  - [ ] Set `DisableProviderAutoUpdate` default to true
  - [ ] Make `UpdateProviders()` no-op
  - [ ] Verify no Catwalk network calls

- [ ] **2.3** Remove provider selection UI
  - [ ] Remove/disable model picker dialog
  - [ ] Remove provider selection dialog
  - [ ] Update TUI to show single provider only

- [ ] **2.4** Set default model in `internal/config/config.go`
  - [ ] Auto-select ZAI model on first run
  - [ ] Update `GetModelByType()` function
  - [ ] Verify status bar shows "Karigor Chintok"

**Phase 2 Complete:** ⬜

---

## Phase 3: Branding & UI (HIGH) ⬜

- [ ] **3.1** Update root command in `internal/cmd/root.go`
  - [ ] Change command name to "karigor"
  - [ ] Update Short/Long descriptions
  - [ ] Update all examples
  - [ ] Update error messages

- [ ] **3.2** Update CLI commands
  - [ ] `internal/cmd/run.go`
  - [ ] `internal/cmd/logs.go`
  - [ ] `internal/cmd/projects.go`
  - [ ] `internal/cmd/login.go`
  - [ ] `internal/cmd/update_providers.go`
  - [ ] `internal/cmd/schema.go`

- [ ] **3.3** Update TUI window title in `internal/tui/tui.go`
  - [ ] Change window title to "Karigor"

- [ ] **3.4** Update TUI components
  - [ ] `internal/tui/components/chat/`
  - [ ] `internal/tui/components/dialogs/`
  - [ ] `internal/tui/components/core/status/`
  - [ ] `internal/tui/page/chat/`

- [ ] **3.5** Update git commit attribution
  - [ ] Find commit message templates
  - [ ] Change to "Generated with Karigor"
  - [ ] Update co-author attribution

- [ ] **3.6** Update PR description templates
  - [ ] Change to "Generated with Karigor"
  - [ ] Remove emoji

- [ ] **3.7** Update error messages
  - [ ] Search for user-facing errors
  - [ ] Replace Crush references

- [ ] **3.8** Update splash screen
  - [ ] `internal/tui/components/chat/splash/`
  - [ ] Update welcome message

- [ ] **3.9** Update help text
  - [ ] All command examples use `karigor`
  - [ ] Config examples show `.karigor.json`

- [ ] **3.10** Update permission dialog
  - [ ] `internal/tui/components/dialogs/permissions/`

- [ ] **3.11** Update session management
  - [ ] Replace "Crush session" → "Karigor session"

- [ ] **3.12** Update about/version info
  - [ ] `karigor --version` shows Karigor

**Phase 3 Complete:** ⬜

---

## Phase 4: ASCII Conversion (MEDIUM) ⬜

- [ ] **4.1** Find all emoji in codebase
  - [ ] Run grep for emoji patterns
  - [ ] Document all locations

- [ ] **4.2** Replace commit message emoji
  - [ ] Remove 💘 from git templates
  - [ ] Use plain text only

- [ ] **4.3** Replace status indicators
  - [ ] ✅ → [OK]
  - [ ] ❌ → [ERROR]
  - [ ] ⚠️ → [WARNING]
  - [ ] ℹ️ → [INFO]

- [ ] **4.4** Replace TUI decorations
  - [ ] Remove fancy bullets
  - [ ] Use ASCII `-`, `*`, `>`
  - [ ] Test on basic terminal

**Phase 4 Complete:** ⬜

---

## Phase 5: Schema & Documentation (MEDIUM) ⬜

- [ ] **5.1** Generate updated schema
  - [ ] Run `task schema`
  - [ ] Verify schema.json has karigor URL
  - [ ] Validate JSON structure

- [ ] **5.2** Update README.md
  - [ ] Rebrand title and headers
  - [ ] Update installation commands
  - [ ] Update configuration examples
  - [ ] Update all command examples

- [ ] **5.3** Update CLAUDE.md
  - [ ] Update project name references
  - [ ] Update file path examples
  - [ ] Update build commands

- [ ] **5.4** Create KARIGOR.md
  - [ ] Add to repo root
  - [ ] Include project context

- [ ] **5.5** Update build configuration
  - [ ] Taskfile.yaml binary name
  - [ ] .goreleaser.yml (if exists)
  - [ ] GitHub Actions workflows

**Phase 5 Complete:** ⬜

---

## Phase 6: Testing & Validation (CRITICAL) ⬜

- [ ] **6.1** Unit tests
  - [ ] Run `task test`
  - [ ] Fix failing tests
  - [ ] Update golden files
  - [ ] All tests pass

- [ ] **6.2** Build test
  - [ ] Run `task build`
  - [ ] Verify binary named `karigor`
  - [ ] Test `./karigor --version`
  - [ ] Test `./karigor --help`

- [ ] **6.3** Configuration loading test
  - [ ] Create test config in ~/.config/karigor/
  - [ ] Verify config loads correctly
  - [ ] Check correct paths in debug

- [ ] **6.4** Provider test
  - [ ] Launch Karigor
  - [ ] Verify only "Karigor Chintok" available
  - [ ] Check model selection UI

- [ ] **6.5** Branding audit (manual)
  - [ ] Check welcome screen
  - [ ] Check status bar
  - [ ] Check help text
  - [ ] Check error messages
  - [ ] Check all dialogs
  - [ ] Take screenshots

- [ ] **6.6** ASCII compliance test
  - [ ] Run on xterm
  - [ ] Run on Terminal.app
  - [ ] Run on Windows Terminal
  - [ ] Verify no emoji visible

- [ ] **6.7** Environment variable test
  - [ ] Test KARIGOR_API_KEY
  - [ ] Test KARIGOR_DISABLE_METRICS
  - [ ] Verify old CRUSH_* don't work

- [ ] **6.8** End-to-end test
  - [ ] Fresh install
  - [ ] First-run experience
  - [ ] Enter API key
  - [ ] Start chat
  - [ ] Send test message
  - [ ] Create git commit
  - [ ] Verify commit message

**Phase 6 Complete:** ⬜

---

## Validation Checklist

### Quick Checks
- [ ] Run: `grep -ri "crush" internal/cmd internal/tui --include="*.go" | grep -v "// " | wc -l` → Should be 0
- [ ] Run: `grep -r "💘\|❤️\|💖" --include="*.go" . | wc -l` → Should be 0
- [ ] Run: `grep -r "CRUSH_" --include="*.go" internal/ | wc -l` → Should be 0
- [ ] Run: `ls -la karigor` → Binary exists
- [ ] Run: `./karigor --version` → Shows Karigor

### Full Validation
- [ ] Run validation script from IMPLEMENTATION_PLAN.md
- [ ] All checks pass

---

## Pre-Implementation

- [ ] Create feature branch: `git checkout -b feat/karigor-rebrand`
- [ ] Review PRD.md
- [ ] Review IMPLEMENTATION_PLAN.md
- [ ] Set up test environment
- [ ] Backup current state

---

## Post-Implementation

- [ ] All 6 phases complete
- [ ] All validation checks pass
- [ ] Documentation updated
- [ ] README updated
- [ ] Create PR for review
- [ ] Tag release (if applicable)

---

## Progress Summary

**Overall Progress:** 0/6 phases complete (0%)

- Phase 1 (Foundation): ⬜ Not Started
- Phase 2 (Provider): ⬜ Not Started
- Phase 3 (Branding): ⬜ Not Started
- Phase 4 (ASCII): ⬜ Not Started
- Phase 5 (Docs): ⬜ Not Started
- Phase 6 (Testing): ⬜ Not Started

---

## Notes

Use this space to track issues, blockers, or important decisions:

```
[Date] [Phase] [Note]
---
```

---

**Last Updated:** 2025-12-18
**Status:** Ready to Begin
