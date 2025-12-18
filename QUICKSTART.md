# Karigor Implementation Quick Start

**TL;DR:** Step-by-step guide to transform Crush → Karigor

---

## Prerequisites

- Go 1.25.5 or later
- Git
- Task (optional, but recommended): `brew install go-task`
- Working Crush codebase

---

## Quick Start (5 Steps)

### Step 1: Prepare Environment (5 min)

```bash
# Create feature branch
git checkout -b feat/karigor-rebrand

# Verify current state works
task build
task test

# Backup (optional)
git tag backup-pre-karigor
```

### Step 2: Phase 1 - Foundation (30 min)

**Critical files to edit:**

1. `internal/config/config.go` (Line ~30)
```go
const (
    appName              = "karigor"
    defaultDataDirectory = ".karigor"
    defaultInitializeAs  = "KARIGOR.md"
)

var defaultContextPaths = []string{
    // ... existing paths ...
    "karigor.md",
    "KARIGOR.md",
    // ... etc
}
```

2. `internal/config/load.go`
```bash
# Find and replace
.crush.json → .karigor.json
crush.json → karigor.json
~/.config/crush/ → ~/.config/karigor/
```

3. `internal/env/env.go`
```bash
# Replace all occurrences
CRUSH_ → KARIGOR_
```

4. Search for `.crush/` globally
```bash
# Find all references
grep -r "\.crush/" internal/ --include="*.go"

# Replace with .karigor/
```

**Test:**
```bash
task build
# Should compile successfully
```

### Step 3: Phase 2 - Single Provider (30 min)

**Edit:** `internal/config/provider.go`

Add hardcoded provider function:
```go
func getKarigorProvider() catwalk.Provider {
    return catwalk.Provider{
        ID:      catwalk.InferenceProviderZAI,
        Name:    "Karigor Chintok",
        BaseURL: "https://open.bigmodel.cn/api/paas/v4",
        Type:    catwalk.TypeOpenAICompat,
        Models: []catwalk.Model{
            {
                ID:                     "glm-4.6",
                Name:                   "Karigor Chintok",
                ContextWindow:          204800,
                DefaultMaxTokens:       131072,
                CanReason:              true,
                DefaultReasoningEffort: "medium",
            },
        },
    }
}

func (c *Config) Providers() []catwalk.Provider {
    return []catwalk.Provider{getKarigorProvider()}
}
```

**Disable auto-update:** `internal/config/catwalk.go`
```go
func Init(...) {
    disabled := true
    cfg.Options.DisableProviderAutoUpdate = &disabled
}
```

**Test:**
```bash
task build
# Should compile, only ZAI provider available
```

### Step 4: Phase 3+4 - Branding & ASCII (60 min)

**Automated search & replace:**
```bash
# Find all "Crush" in user-facing code
grep -ri "crush" internal/cmd internal/tui --include="*.go" \
  | grep -v "// " \
  | grep -v "package" \
  > crush_references.txt

# Review and replace each occurrence with "Karigor"
```

**Key files:**
- `internal/cmd/root.go` - Command descriptions
- `internal/cmd/*.go` - All command help text
- `internal/tui/**/*.go` - All UI text

**Remove emoji:**
```bash
# Find emoji
grep -r "💘\|❤️\|💖\|🎉" --include="*.go" . > emoji_list.txt

# Replace with plain text
# Example: "💘 Generated with Crush" → "Generated with Karigor"
```

**Test:**
```bash
task build
./karigor --help  # Should show Karigor branding
```

### Step 5: Phase 5+6 - Docs & Testing (45 min)

**Update docs:**
```bash
# Generate new schema
task schema

# Update README.md (search/replace Crush → Karigor)

# Update CLAUDE.md (paths and commands)
```

**Run full test suite:**
```bash
# Unit tests
task test

# Build and manual test
task build
./karigor --version
./karigor --help

# Create test config
mkdir -p ~/.config/karigor
cat > ~/.config/karigor/karigor.json <<'EOF'
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "zai": {
      "api_key": "your-zai-key-here"
    }
  }
}
EOF

# Test run
./karigor
```

**Validation:**
```bash
# No Crush in UI code
grep -ri "crush" internal/cmd internal/tui --include="*.go" | grep -v "// " | wc -l
# → Should be 0

# No emoji
grep -r "💘\|❤️\|💖" --include="*.go" . | wc -l
# → Should be 0

# No old env vars
grep -r "CRUSH_" --include="*.go" internal/ | wc -l
# → Should be 0
```

---

## File Modification Priority

### MUST CHANGE (Priority 1)
```
internal/config/config.go        → Constants
internal/config/load.go          → File paths
internal/config/provider.go      → Single provider
internal/env/env.go              → Environment variables
internal/cmd/root.go             → CLI root command
```

### SHOULD CHANGE (Priority 2)
```
internal/cmd/*.go                → All CLI commands
internal/tui/**/*.go             → All TUI components
internal/config/catwalk.go       → Disable auto-update
internal/cmd/schema.go           → Schema URL
```

### NICE TO CHANGE (Priority 3)
```
README.md                        → Documentation
CLAUDE.md                        → Documentation
Taskfile.yaml                    → Build config
```

---

## Common Search/Replace Patterns

### Case-Sensitive Replacements
```bash
# User-facing text
"Crush" → "Karigor"
"crush" → "karigor" (in examples/commands)

# File paths
".crush" → ".karigor"
".crush/" → ".karigor/"
"/crush/" → "/karigor/"

# Environment variables
"CRUSH_" → "KARIGOR_"

# URLs
"crush.json" → "karigor.json"
```

### Files to Edit with Regex
```bash
# Find all user-facing "Crush" strings
grep -r '".*[Cc]rush.*"' internal/cmd internal/tui --include="*.go"

# Find all path references
grep -r '\.crush\|/crush/' --include="*.go" internal/
```

---

## Testing Strategy

### 1. After Phase 1 (Foundation)
```bash
task build
# Binary should compile
```

### 2. After Phase 2 (Provider)
```bash
task build
./karigor
# Should show only Karigor Chintok provider
```

### 3. After Phase 3 (Branding)
```bash
./karigor --help
# All text should say "Karigor"
```

### 4. After Phase 4 (ASCII)
```bash
./karigor
# No emoji should be visible in TUI
```

### 5. After Phase 5 (Docs)
```bash
task schema
# schema.json should reference karigor
```

### 6. After Phase 6 (Full Test)
```bash
# Full E2E test
rm -rf ~/.config/karigor  # Fresh start
./karigor
# Enter API key, test chat, create commit
```

---

## Troubleshooting

### Build Fails
```bash
# Check for syntax errors
go build .

# Verbose output
go build -v .

# Check specific package
go build ./internal/config
```

### Tests Fail
```bash
# Run specific test
go test ./internal/config -v

# Update golden files
go test ./... -update

# Skip failing tests temporarily
go test ./... -skip=TestFailingTest
```

### Can't Find Config
```bash
# Debug config loading
./karigor -d 2>&1 | grep -i config

# Check search paths
ls -la ~/.config/karigor/
ls -la .karigor.json
```

### Provider Not Working
```bash
# Check provider list
./karigor -d 2>&1 | grep -i provider

# Verify ZAI endpoint
curl -X POST https://open.bigmodel.cn/api/paas/v4/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-4.6","messages":[{"role":"user","content":"test"}]}'
```

---

## Commit Strategy

### Recommended Commits
```bash
# After Phase 1
git add internal/config internal/env internal/cmd/dirs.go
git commit -m "feat: update foundation - constants, paths, env vars"

# After Phase 2
git add internal/config/provider.go internal/config/catwalk.go
git commit -m "feat: simplify to single ZAI provider"

# After Phase 3
git add internal/cmd internal/tui
git commit -m "feat: rebrand UI to Karigor"

# After Phase 4
git add .
git commit -m "feat: remove emoji, use ASCII only"

# After Phase 5
git add README.md CLAUDE.md schema.json Taskfile.yaml
git commit -m "docs: update documentation and schema"

# After Phase 6
git add .
git commit -m "test: update tests and validation"
```

---

## Validation Checklist (Quick)

Before opening PR, verify:

```bash
✓ Binary named karigor
  ./karigor --version

✓ Config uses .karigor.json
  ls ~/.config/karigor/karigor.json

✓ Only ZAI provider
  ./karigor # Check UI

✓ No "Crush" in UI
  grep -ri "crush" internal/cmd internal/tui --include="*.go" | grep -v "//"

✓ No emoji
  grep -r "💘\|❤️" --include="*.go" .

✓ All tests pass
  task test

✓ Manual E2E works
  ./karigor # Full walkthrough
```

---

## Quick Reference Commands

```bash
# Build
task build

# Run
./karigor

# Test
task test

# Format
task fmt

# Lint
task lint:fix

# Schema
task schema

# Search for Crush
grep -ri "crush" internal/ --include="*.go" | less

# Search for emoji
grep -r "💘\|❤️\|💖" --include="*.go" .

# Count changes needed
grep -ri "crush" internal/cmd internal/tui --include="*.go" | wc -l
```

---

## Time Estimates

| Phase | Tasks | Estimated Time |
|-------|-------|----------------|
| 1. Foundation | 5 | 30-45 min |
| 2. Provider | 4 | 30-45 min |
| 3. Branding | 12 | 60-90 min |
| 4. ASCII | 4 | 15-30 min |
| 5. Docs | 5 | 30-45 min |
| 6. Testing | 8 | 45-60 min |
| **Total** | **38** | **3-5 hours** |

*Times are for an experienced Go developer familiar with the codebase*

---

## Next Steps

1. Read IMPLEMENTATION_PLAN.md for detailed instructions
2. Review TASKS.md for checklist tracking
3. Start with Phase 1 (Foundation)
4. Test after each phase
5. Commit after each phase
6. Open PR when all phases complete

---

**Good luck! 🚀 (Wait, no emoji in Karigor! Good luck!)**
