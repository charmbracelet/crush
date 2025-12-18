# Detailed Implementation Guide for Karigor Rebrand

**Reference:** Explains the WHERE, HOW, and WHY for each change
**Use with:** TASKS.md for checkbox tracking

---

## Phase 1: Foundation - Detailed Breakdown

### P1.1: Core Constants (internal/config/config.go)

**Location:** `internal/config/config.go` lines 30-52

**Current Code:**
```go
const (
    appName              = "crush"
    defaultDataDirectory = ".crush"
    defaultInitializeAs  = "AGENTS.md"
)

var defaultContextPaths = []string{
    ".github/copilot-instructions.md",
    ".cursorrules",
    ".cursor/rules/",
    "CLAUDE.md",
    "CLAUDE.local.md",
    // ... existing paths ...
    "AGENTS.md",
    "agents.md",
    "Agents.md",
}
```

**New Code:**
```go
const (
    appName              = "karigor"
    defaultDataDirectory = ".karigor"
    defaultInitializeAs  = "KARIGOR.md"
)

var defaultContextPaths = []string{
    ".github/copilot-instructions.md",
    ".cursorrules",
    ".cursor/rules/",
    "CLAUDE.md",
    "CLAUDE.local.md",
    "GEMINI.md",
    "gemini.md",
    "karigor.md",           // NEW
    "karigor.local.md",     // NEW
    "Karigor.md",           // NEW
    "Karigor.local.md",     // NEW
    "KARIGOR.md",           // NEW
    "KARIGOR.local.md",     // NEW
    "AGENTS.md",
    "agents.md",
    "Agents.md",
}
```

**Why:** These constants are used throughout the codebase for:
- Application identification
- Default config directory naming
- Context file discovery

**Test:**
```bash
go build ./internal/config
# Should compile without errors
```

---

### P1.2: Config File Resolution (internal/config/load.go)

**Location:** `internal/config/load.go` (search for all config path strings)

**Find and Replace:**
```go
// Pattern 1: Hidden config file
".crush.json" → ".karigor.json"

// Pattern 2: Visible config file
"crush.json" → "karigor.json"

// Pattern 3: Global config directory
filepath.Join(home, ".config", "crush", "crush.json")
→
filepath.Join(home, ".config", "karigor", "karigor.json")

// Pattern 4: Error messages
"could not find crush.json"
→
"could not find karigor.json"
```

**Specific Functions to Check:**
- `Load()` - Main config loading function
- `configPaths()` - Returns list of config file paths
- `findConfig()` - Searches for config files
- Any error messages mentioning config files

**Why:** Config file discovery is the first thing that happens on startup. If this doesn't work, nothing else will.

**Test:**
```bash
# Create test config
mkdir -p ~/.config/karigor
echo '{"test": true}' > ~/.config/karigor/karigor.json

# Build and test
go build .
./karigor -d 2>&1 | grep -i "config"
# Should show loading from karigor paths
```

---

### P1.3: Environment Variables (internal/env/env.go)

**Location:** `internal/env/env.go`

**Current Code Pattern:**
```go
// Example locations where env vars are referenced
os.Getenv("CRUSH_DISABLE_METRICS")
os.Getenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE")
os.Getenv("CRUSH_PROFILE")
```

**New Code:**
```go
os.Getenv("KARIGOR_DISABLE_METRICS")
os.Getenv("KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE")
os.Getenv("KARIGOR_PROFILE")

// Add new:
os.Getenv("KARIGOR_API_KEY")
```

**Additional Locations:**
```bash
# Find all env var references
grep -r "CRUSH_" --include="*.go" internal/
grep -r "ANTHROPIC_API_KEY\|OPENAI_API_KEY" --include="*.go" internal/config/

# Should also check:
internal/cmd/root.go         # shouldEnableMetrics()
internal/config/config.go    # API key loading
internal/app/app.go          # Metrics initialization
```

**Why:** Environment variables control runtime behavior. Old vars must not work (clean break).

**Test:**
```bash
export KARIGOR_DISABLE_METRICS=1
./karigor
# Check metrics are disabled

export CRUSH_DISABLE_METRICS=1  # Old var
./karigor
# Should NOT disable metrics (verify old vars don't work)
```

---

### P1.4: Data Directory References

**Location:** Multiple files

**Files to Update:**
1. `internal/cmd/root.go` line ~277
```go
// Old function name
func createDotCrushDir(dir string) error {
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return fmt.Errorf("failed to create data directory: %q %w", dir, err)
    }

    gitIgnorePath := filepath.Join(dir, ".gitignore")
    // ...
}

// New function name
func createDotKarigorDir(dir string) error {
    // Same implementation, just renamed
}

// Update call site (line ~200)
if err := createDotKarigorDir(cfg.Options.DataDirectory); err != nil {
    return nil, err
}
```

2. `internal/cmd/logs.go`
```go
// Old
logPath := filepath.Join(cfg.Options.DataDirectory, "logs", "crush.log")

// New
logPath := filepath.Join(cfg.Options.DataDirectory, "logs", "karigor.log")
```

3. `internal/cmd/dirs.go`
```go
// Search for any .crush references
// Replace with .karigor
```

**Global Search Commands:**
```bash
# Find all .crush/ directory references
grep -r "\.crush/" --include="*.go" internal/

# Find all /crush/ path references
grep -r "/crush/" --include="*.go" internal/

# Find ~/.local/share/crush references
grep -r "\.local/share/crush" --include="*.go" internal/
```

**Why:** Data directories store logs, databases, and runtime state. Must use new naming.

**Test:**
```bash
./karigor
ls -la ~/.karigor/
ls -la ~/.local/share/karigor/
# Directories should exist with karigor naming
```

---

### P1.5: Schema URL

**Location:** `internal/cmd/schema.go`

**Current Code:**
```go
// In schema generation code
schema := &jsonschema.Schema{
    // ...
    ID: "https://charm.land/crush.json",
}
```

**New Code:**
```go
schema := &jsonschema.Schema{
    // ...
    ID: "https://charm.land/karigor.json",
}
```

**Generate Schema:**
```bash
task schema
# or
go run main.go schema > schema.json
```

**Verify:**
```bash
cat schema.json | jq '."$schema"'
# Should output: "https://charm.land/karigor.json"
```

**Why:** VSCode and other editors use $schema for validation. While we can't control charm.land, we change what we output.

---

## Phase 2: Provider Simplification - Detailed Breakdown

### P2.1: Hardcode ZAI Provider (internal/config/provider.go)

**Location:** `internal/config/provider.go`

**Add New Function:**
```go
// getKarigorProvider returns the single hardcoded ZAI provider
// configured as "Karigor Chintok" for the Karigor fork.
func getKarigorProvider() catwalk.Provider {
    return catwalk.Provider{
        ID:      catwalk.InferenceProviderZAI,  // "zai"
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
                ReasoningLevels:        []string{"low", "medium", "high"},
                DefaultReasoningEffort: "medium",
                SupportsAttachments:    false,
                CostPer1MIn:            0.6,
                CostPer1MOut:           2.2,
                CostPer1MInCached:      0.11,
                Options:                map[string]any{},
            },
        },
    }
}
```

**Modify Existing Function:**
```go
// Providers returns all available providers.
// In Karigor, this is hardcoded to return only ZAI.
func (c *Config) Providers() []catwalk.Provider {
    // Original code might load from Catwalk, config, etc.
    // Replace entire implementation:
    return []catwalk.Provider{getKarigorProvider()}
}
```

**Why:**
- ZAI provider (catwalk.InferenceProviderZAI) already exists in the codebase
- Special handling for tool_stream is already in coordinator.go:737
- We just need to return ONLY this provider, labeled as "Karigor Chintok"

**Test:**
```bash
go build .
./karigor -d 2>&1 | grep -i "provider"
# Should show only one provider: "Karigor Chintok"
```

---

### P2.2: Disable Auto-Update (internal/config/catwalk.go)

**Location:** `internal/config/catwalk.go`

**Find Init or Similar:**
```go
// Look for initialization code that sets up config
func Init(cwd, dataDir string, debug bool) (*Config, error) {
    // ... existing code ...

    // Add this section:
    if cfg.Options.DisableProviderAutoUpdate == nil {
        disabled := true
        cfg.Options.DisableProviderAutoUpdate = &disabled
    }

    // ... rest of code ...
}
```

**Override Update Function:**
```go
// UpdateProviders fetches latest provider list from Catwalk.
// In Karigor, this is disabled as providers are hardcoded.
func UpdateProviders() error {
    // Original might have:
    // - Network call to Catwalk
    // - JSON parsing
    // - Config updates

    // Replace with:
    slog.Debug("Provider auto-update disabled in Karigor (hardcoded ZAI provider)")
    return nil
}
```

**Alternative - Make it a no-op:**
```go
func (c *Config) updateProvidersFromCatwalk() error {
    // Just return immediately
    return nil
}
```

**Why:** Catwalk auto-update would override our hardcoded ZAI provider. Must disable.

**Test:**
```bash
# Run with debug logging
./karigor -d 2>&1 | grep -i "catwalk\|provider.*update"
# Should NOT see any network calls to Catwalk
```

---

### P2.3: Remove Provider Selection UI

**Location:** `internal/tui/components/dialogs/models/`

**Strategy 1: Remove Dialog Entirely**
```go
// Find dialog registration (likely in internal/tui/tui.go or dialog router)

// Old:
dialogs := map[DialogType]DialogComponent{
    DialogTypeModelPicker: newModelPickerDialog(),
    // ... other dialogs
}

// New:
dialogs := map[DialogType]DialogComponent{
    // DialogTypeModelPicker: removed
    // ... other dialogs
}
```

**Strategy 2: Make Read-Only**
```go
// In model picker component
func (m ModelPicker) View() string {
    // Instead of showing selection UI:
    return "Current Model: Karigor Chintok\n(Model switching disabled)"
}

// Disable key bindings
func (m ModelPicker) Update(msg tea.Msg) (ModelPicker, tea.Cmd) {
    // Don't handle selection keys
    return m, nil
}
```

**Files to Check:**
```bash
# Find model/provider selection UI
grep -r "model.*picker\|provider.*select" --include="*.go" internal/tui/
grep -r "switch.*model\|change.*provider" --include="*.go" internal/tui/
```

**Why:** With only one model, selection UI is unnecessary and confusing.

**Test:**
```bash
./karigor
# Try to open model picker (usually Ctrl+M or similar)
# Should either not exist or show read-only message
```

---

### P2.4: Auto-Select Default Model

**Location:** `internal/config/config.go`

**Modify GetModelByType:**
```go
func (c *Config) GetModelByType(modelType SelectedModelType) *catwalk.Model {
    // Original code checks c.SelectedModels[modelType]

    // Add default fallback:
    if c.SelectedModels == nil || c.SelectedModels[modelType].Provider == "" {
        // Auto-select ZAI provider's first model
        provider := getKarigorProvider()
        if len(provider.Models) > 0 {
            return &provider.Models[0]
        }
    }

    // ... rest of original logic
}
```

**Alternative - Set on Init:**
```go
func Init(...) (*Config, error) {
    // ... existing code ...

    // Auto-configure selected models
    if cfg.SelectedModels == nil {
        cfg.SelectedModels = make(map[SelectedModelType]SelectedModel)
    }

    if cfg.SelectedModels[SelectedModelTypeLarge].Provider == "" {
        cfg.SelectedModels[SelectedModelTypeLarge] = SelectedModel{
            Provider: "zai",
            Model:    "glm-4.6",
        }
    }

    // ... rest of code
}
```

**Why:** First-run experience should "just work" without model selection.

**Test:**
```bash
rm -rf ~/.config/karigor ~/.karigor  # Fresh start
./karigor
# Should not prompt for model selection
# Should show "Karigor Chintok" in status bar
```

---

## Phase 3: Branding - Detailed Breakdown

### P3.1: Root Command (internal/cmd/root.go)

**Location:** `internal/cmd/root.go` starting line 52

**Before:**
```go
var rootCmd = &cobra.Command{
    Use:   "crush",
    Short: "Terminal-based AI assistant for software development",
    Long: `Crush is a powerful terminal-based AI assistant...`,
    Example: `
# Run in interactive mode
crush

# Run with debug logging
crush -d
    `,
}
```

**After:**
```go
var rootCmd = &cobra.Command{
    Use:   "karigor",
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
        // ... implementation ...

        // Update error message (line ~104):
        return errors.New("Karigor encountered an error. If metrics are enabled, we were notified about it. If you'd like to report it, please copy the stacktrace above and open an issue at https://github.com/charmbracelet/crush/issues/new?template=bug.yml")
    },
}
```

**Why:** Root command is the entry point. All help text flows from here.

**Test:**
```bash
./karigor --help
# Every line should say "karigor" or "Karigor", never "crush" or "Crush"
```

---

### P3.2: CLI Commands - Specific Examples

**File:** `internal/cmd/logs.go`

**Before:**
```go
var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "View Crush logs",
    Long:  `Display logs from the Crush application...`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // ...
        logPath := filepath.Join(cfg.Options.DataDirectory, "logs", "crush.log")
        // ...
    },
}
```

**After:**
```go
var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "View Karigor logs",
    Long:  `Display logs from the Karigor application. Logs are stored in the .karigor directory.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // ...
        logPath := filepath.Join(cfg.Options.DataDirectory, "logs", "karigor.log")
        // ...
    },
}
```

**File:** `internal/cmd/run.go`

**Before:**
```go
var runCmd = &cobra.Command{
    Use:   "run [prompt]",
    Short: "Run Crush with a single prompt",
    // ...
}
```

**After:**
```go
var runCmd = &cobra.Command{
    Use:   "run [prompt]",
    Short: "Run Karigor with a single prompt",
    Long:  `Run Karigor in non-interactive mode with a single prompt and exit.`,
    // ...
}
```

**Search Pattern:**
```bash
# Find all command definitions
grep -r "cobra.Command{" internal/cmd/ --include="*.go" -A 3

# For each file, search and replace:
# "Crush" → "Karigor"
# "crush" → "karigor" (in examples)
```

---

### P3.3-3.4: TUI Components

**Search Strategy:**
```bash
# Find all user-facing strings in TUI
grep -r '".*Crush.*"' internal/tui/ --include="*.go"
grep -r '`.*Crush.*`' internal/tui/ --include="*.go"

# Common locations:
internal/tui/components/chat/splash/      # Welcome screen
internal/tui/components/dialogs/*/        # Dialog titles
internal/tui/components/core/status/      # Status bar
internal/tui/page/chat/                   # Chat page
```

**Example - Splash Screen:**
```go
// File: internal/tui/components/chat/splash/splash.go

// Before:
func (s Splash) welcomeMessage() string {
    return lipgloss.NewStyle().
        Bold(true).
        Render("Welcome to Crush!")
}

// After:
func (s Splash) welcomeMessage() string {
    return lipgloss.NewStyle().
        Bold(true).
        Render("Welcome to Karigor!")
}
```

**Example - Status Bar:**
```go
// File: internal/tui/components/core/status/status.go

// Before:
func (s Status) providerInfo() string {
    return fmt.Sprintf("Crush | %s", s.currentModel)
}

// After:
func (s Status) providerInfo() string {
    return fmt.Sprintf("Karigor | %s", s.currentModel)
}
```

---

### P3.5-3.6: Git Attribution

**Search Strategy:**
```bash
# Find git-related code
grep -r "git commit\|Git commit" --include="*.go" internal/
grep -r "Generated with\|Co-Authored" --include="*.go" internal/
grep -r "pull request\|PR description" --include="*.go" internal/
```

**Example Location:** Likely in `internal/app/` or git tool code

**Before:**
```go
commitMessage := fmt.Sprintf(`%s

💘 Generated with Crush

Co-Authored-By: Crush <crush@charm.land>`, userMessage)
```

**After:**
```go
commitMessage := fmt.Sprintf(`%s

Generated with Karigor

Assisted-by: Karigor Chintok <karigor@example.com>`, userMessage)
```

**Note:** Removing emoji (💘) is part of Phase 4, but can be done here too.

---

## Phase 4: ASCII Conversion - Detailed Breakdown

### P4.1-4.2: Find and Remove Emoji

**Search Command:**
```bash
# Find all emoji (comprehensive list)
grep -r $'💘\|❤️\|💖\|🎉\|✨\|🚀\|⚡\|🔥\|💡\|📝\|📋\|✅\|❌\|⚠️\|ℹ️\|🔍\|📊\|🎯\|💻\|🌟' \
    --include="*.go" . > emoji_locations.txt
```

**Common Replacements:**
```go
// Commit messages
"💘 Generated with Crush" → "Generated with Karigor"
"✨ " → ""  // Just remove sparkles

// Status indicators
"✅" → "[OK]" or "✓" (plain ASCII checkmark)
"❌" → "[ERROR]" or "✗"
"⚠️" → "[WARNING]" or "!"
"ℹ️" → "[INFO]" or "i"

// Decorative
"🎉" → ""  // Just remove
"🚀" → ""  // Just remove
"💡" → "Tip:"
"📝" → "Note:"
```

**Manual Review Needed:** Some emoji might be in:
- Test files (can keep for test data)
- Comments (usually okay to keep)
- String constants that aren't user-facing

---

### P4.3: Status Indicators

**Location:** Likely `internal/tui/components/core/status/` or similar

**Before:**
```go
func (s Status) formatStatus(level StatusLevel, msg string) string {
    switch level {
    case StatusSuccess:
        return fmt.Sprintf("✅ %s", msg)
    case StatusError:
        return fmt.Sprintf("❌ %s", msg)
    case StatusWarning:
        return fmt.Sprintf("⚠️ %s", msg)
    case StatusInfo:
        return fmt.Sprintf("ℹ️ %s", msg)
    }
}
```

**After:**
```go
func (s Status) formatStatus(level StatusLevel, msg string) string {
    switch level {
    case StatusSuccess:
        return fmt.Sprintf("[OK] %s", msg)
    case StatusError:
        return fmt.Sprintf("[ERROR] %s", msg)
    case StatusWarning:
        return fmt.Sprintf("[WARNING] %s", msg)
    case StatusInfo:
        return fmt.Sprintf("[INFO] %s", msg)
    }
}
```

---

### P4.4: TUI Decorations

**Search for Unicode:**
```bash
# Find box drawing characters
grep -r "[\u2500-\u257F]" --include="*.go" internal/tui/

# Find fancy bullets
grep -r "•\|▸\|◆\|▪" --include="*.go" internal/tui/
```

**Replacements:**
```go
// Bullets
"• " → "- "
"▸ " → "> "
"◆ " → "* "

// Box drawing - if using lipgloss, it handles this
// But if manual:
"─" → "-"
"│" → "|"
"┌" → "+"
"└" → "+"
"┐" → "+"
"┘" → "+"
```

---

## Phase 5: Documentation - Detailed Breakdown

### P5.2: README.md

**Strategy:** Full search and replace

**Commands:**
```bash
# Backup first
cp README.md README.md.backup

# Replace in place (macOS)
sed -i '' 's/Crush/Karigor/g' README.md
sed -i '' 's/crush/karigor/g' README.md

# Replace in place (Linux)
sed -i 's/Crush/Karigor/g' README.md
sed -i 's/crush/karigor/g' README.md

# Review changes
diff README.md.backup README.md
```

**Manual Review Needed:**
- Installation instructions (package names might stay "crush")
- GitHub links (might point to original repo or fork)
- License information
- Contribution guidelines

---

### P5.3: CLAUDE.md

**Specific Sections to Update:**

1. Project Overview (line 9)
```markdown
<!-- Before -->
Crush is a terminal-based AI coding assistant written in Go.

<!-- After -->
Karigor is a terminal-based AI coding assistant written in Go, forked from Crush.
```

2. Build Commands (line 14-20)
```markdown
<!-- Before -->
- **Build**: `go build .` or `go run .`
- **Run**: `task run` or `go run . {{.CLI_ARGS}}`

<!-- After -->
- **Build**: `go build .` (produces `karigor` binary)
- **Run**: `./karigor` or `task run`
```

3. File paths throughout:
```markdown
.crush.json → .karigor.json
.crush/logs/ → .karigor/logs/
~/.config/crush/ → ~/.config/karigor/
```

---

### P5.5: Build Configuration

**Taskfile.yaml:**
```yaml
# Before
build:
  desc: Run build
  cmds:
    - go build {{.LDFLAGS}} .
  generates:
    - crush

# After
build:
  desc: Build karigor binary
  cmds:
    - go build {{.LDFLAGS}} -o karigor .
  generates:
    - karigor

install:
  desc: Install karigor
  cmds:
    - go install {{.LDFLAGS}} -v .
```

**If .goreleaser.yml exists:**
```yaml
# Before
builds:
  - binary: crush

# After
builds:
  - binary: karigor
```

---

## Phase 6: Testing - Detailed Test Cases

### P6.3: Configuration Loading Test

**Full Test Script:**
```bash
#!/bin/bash
set -e

echo "=== Configuration Loading Test ==="

# Clean state
rm -rf ~/.config/karigor ~/.karigor

# Create test config
mkdir -p ~/.config/karigor
cat > ~/.config/karigor/karigor.json <<'EOF'
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "zai": {
      "id": "zai",
      "api_key": "test-key-12345"
    }
  }
}
EOF

echo "✓ Created test config"

# Run with debug and capture output
OUTPUT=$(./karigor -d 2>&1)

# Check for correct config path
if echo "$OUTPUT" | grep -q "karigor.json"; then
    echo "✓ Config loaded from correct path"
else
    echo "✗ Config path incorrect"
    exit 1
fi

# Check for ZAI provider
if echo "$OUTPUT" | grep -q "zai"; then
    echo "✓ ZAI provider detected"
else
    echo "✗ ZAI provider not found"
    exit 1
fi

echo "=== All checks passed ==="
```

---

### P6.8: End-to-End Test

**Full E2E Test Script:**
```bash
#!/bin/bash
set -e

echo "=== Karigor E2E Test ==="

# Step 1: Clean state
echo "Cleaning previous state..."
rm -rf ~/.config/karigor ~/.karigor ~/.local/share/karigor

# Step 2: First run (interactive - manual)
echo "Run './karigor' and complete first-run setup:"
echo "  1. Enter a valid ZAI API key"
echo "  2. Verify welcome screen says 'Karigor'"
echo "  3. Check status bar shows 'Karigor Chintok'"
read -p "Press ENTER when ready to continue..."

# Step 3: Verify config created
if [ -f ~/.config/karigor/karigor.json ]; then
    echo "✓ Config file created"
else
    echo "✗ Config file not found"
    exit 1
fi

# Step 4: Verify data directory
if [ -d ~/.karigor ]; then
    echo "✓ Data directory created"
else
    echo "✗ Data directory not found"
    exit 1
fi

# Step 5: Check logs
if [ -f ~/.karigor/logs/karigor.log ]; then
    echo "✓ Log file exists"

    # Check for errors
    if grep -i "error\|fatal" ~/.karigor/logs/karigor.log | grep -v "debug"; then
        echo "⚠ Found errors in log (review manually)"
    else
        echo "✓ No errors in log"
    fi
else
    echo "✗ Log file not found"
fi

# Step 6: Test git commit (if in git repo)
if git rev-parse --git-dir > /dev/null 2>&1; then
    echo "Testing git commit..."
    echo "test" > /tmp/karigor_test_file.txt
    git add /tmp/karigor_test_file.txt

    # This would be done through karigor's git integration
    echo "Create a commit using Karigor and verify:"
    echo "  - Commit message includes 'Generated with Karigor'"
    echo "  - Commit message includes 'Assisted-by: Karigor Chintok'"
    echo "  - NO emoji in commit message"
    read -p "Press ENTER when verified..."

    # Cleanup
    git reset HEAD /tmp/karigor_test_file.txt
    rm /tmp/karigor_test_file.txt
fi

echo "=== E2E Test Complete ==="
```

---

## Common Issues and Solutions

### Issue 1: Build Fails with "undefined: catwalk.InferenceProviderZAI"

**Cause:** ZAI provider constant might be named differently

**Solution:**
```bash
# Find actual constant name
grep -r "InferenceProvider.*ZAI\|ZAI.*Provider" go.mod go.sum
grep -r "type InferenceProvider" $(go list -m -f '{{.Dir}}' github.com/charmbracelet/catwalk)

# Use actual constant name in provider.go
```

---

### Issue 2: Config Not Loading from Expected Path

**Debug:**
```bash
# Run with extreme verbosity
./karigor -d -d 2>&1 | grep -i "config\|load"

# Manually test path resolution
go run -x . -c /path/to/test 2>&1 | grep "config"

# Check file permissions
ls -la ~/.config/karigor/
ls -la ~/.karigor/
```

**Solution:** Verify `internal/config/load.go` changes were applied correctly

---

### Issue 3: Tests Fail with "fixture mismatch"

**Cause:** Golden files contain old "Crush" references

**Solution:**
```bash
# Update all golden files
go test ./... -update

# Or update specific package
go test ./internal/tui/components/core -update

# Review changes
git diff **/*.golden
```

---

### Issue 4: Provider Shows Wrong Name

**Check:**
```bash
# Verify provider config
./karigor -d 2>&1 | grep -i "provider"

# Check in code
grep -n "Karigor Chintok" internal/config/provider.go
```

**Debug:**
```go
// Add debug logging in Providers() function
func (c *Config) Providers() []catwalk.Provider {
    providers := []catwalk.Provider{getKarigorProvider()}
    slog.Debug("returning providers", "count", len(providers), "name", providers[0].Name)
    return providers
}
```

---

## Validation Checklist (Expanded)

### Automated Checks

```bash
#!/bin/bash
# validation.sh

echo "Karigor Validation Checklist"
echo "============================="

# 1. Check binary name
if [ -f "./karigor" ]; then
    echo "✓ Binary named karigor"
else
    echo "✗ Binary not named karigor"
fi

# 2. Check for Crush references in UI code
CRUSH_COUNT=$(grep -ri "crush" internal/cmd internal/tui --include="*.go" | \
              grep -v "//" | grep -v "package" | wc -l)
if [ "$CRUSH_COUNT" -eq 0 ]; then
    echo "✓ No Crush references in UI ($CRUSH_COUNT)"
else
    echo "✗ Found $CRUSH_COUNT Crush references"
    grep -ri "crush" internal/cmd internal/tui --include="*.go" | \
        grep -v "//" | grep -v "package" | head -5
fi

# 3. Check for emoji
EMOJI_COUNT=$(grep -r $'💘\|❤️\|💖\|🎉\|✨' --include="*.go" . | wc -l)
if [ "$EMOJI_COUNT" -eq 0 ]; then
    echo "✓ No emoji found ($EMOJI_COUNT)"
else
    echo "✗ Found $EMOJI_COUNT emoji"
    grep -r $'💘\|❤️\|💖' --include="*.go" . | head -3
fi

# 4. Check environment variables
ENV_COUNT=$(grep -r "CRUSH_" --include="*.go" internal/ | wc -l)
if [ "$ENV_COUNT" -eq 0 ]; then
    echo "✓ No CRUSH_ env vars ($ENV_COUNT)"
else
    echo "✗ Found $ENV_COUNT CRUSH_ env vars"
fi

# 5. Check config paths
CONFIG_CRUSH_COUNT=$(grep -r "\.crush\|/crush/" --include="*.go" internal/config internal/cmd | wc -l)
if [ "$CONFIG_CRUSH_COUNT" -eq 0 ]; then
    echo "✓ No .crush paths in config/cmd ($CONFIG_CRUSH_COUNT)"
else
    echo "✗ Found $CONFIG_CRUSH_COUNT .crush path references"
fi

# 6. Run tests
echo "Running tests..."
if go test ./... > /dev/null 2>&1; then
    echo "✓ All tests pass"
else
    echo "✗ Some tests fail"
fi

# 7. Check version output
VERSION_OUTPUT=$(./karigor --version 2>&1)
if echo "$VERSION_OUTPUT" | grep -qi "karigor"; then
    echo "✓ Version shows Karigor"
else
    echo "✗ Version doesn't show Karigor"
fi

echo "============================="
```

### Manual Verification

**Checklist:**
- [ ] Run `./karigor --help` - all text says "Karigor"
- [ ] Run `./karigor` - TUI opens, welcome says "Karigor"
- [ ] Status bar shows "Karigor Chintok" model
- [ ] No provider selection UI available
- [ ] Create chat message - works correctly
- [ ] Create git commit - message says "Generated with Karigor"
- [ ] No emoji visible anywhere in UI
- [ ] Works on xterm (or basic terminal)
- [ ] Config saved to `~/.config/karigor/karigor.json`
- [ ] Logs saved to `~/.karigor/logs/karigor.log`

---

## Performance Considerations

**No Expected Performance Impact:**
- Same codebase, just rebranded
- Single provider might slightly reduce startup time (less config parsing)
- No provider selection UI = simpler rendering

**Benchmark (Optional):**
```bash
# Before changes
time ./crush -c /tmp/test --version

# After changes
time ./karigor -c /tmp/test --version

# Should be similar or slightly faster
```

---

## Security Considerations

**No Security Impact Expected:**
- Same authentication mechanisms
- Same API key storage
- Same permission system

**Verify:**
- API keys still stored with 0o600 permissions
- No API keys in logs (even with -d flag)
- Environment variables work correctly

---

## Next Steps After Implementation

1. **Create Release Branch**
```bash
git checkout -b release/karigor-v1.0.0
```

2. **Update Version Numbers**
```bash
# internal/version/version.go
const Version = "1.0.0-karigor"
```

3. **Create Release Notes**
```markdown
# Karigor v1.0.0

Karigor is a streamlined, single-provider fork of Crush focused on the ZAI GLM-4.6 model.

## Changes from Crush
- Rebranded to Karigor
- Single provider: Karigor Chintok (ZAI GLM-4.6)
- ASCII-only UI (no emoji)
- Simplified configuration

## Breaking Changes
- Config files use `.karigor.json`
- Data directory is `.karigor/`
- Environment variables use `KARIGOR_` prefix
```

4. **Tag Release**
```bash
git tag -a v1.0.0 -m "Karigor v1.0.0 - Initial release"
git push origin v1.0.0
```

---

**Last Updated:** 2025-12-18
**Implementation Status:** Ready to Begin
**Estimated Completion:** 154 tasks, ~3-5 hours for experienced developer
