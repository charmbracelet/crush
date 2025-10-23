# CRUSH Enhanced – Implementation Guide

This document packages everything the coding agent (Kilocode) needs to ship the CRUSH Enhanced fork: themes, file explorer, plan mode, and narrator agent.

---

## 1. Project Snapshot
- **Goal:** add a left-pane file explorer, right-pane narrator, theme system (4 presets), and 5-step plan mode to CRUSH.
- **Timeline:** 2–3 weeks, phased rollout.
- **Tech:** Go 1.21+, Cobra CLI, Bubble Tea + Bubbles, Lipgloss, Ollama API, fsnotify.
- **Constraints:** explorer is read-only; narrator is pull-based (no always-on streaming); plans limited to five steps; no VS Code integration.

### Success Criteria
- Users can browse the workspace tree while coding.
- Narrator explains at least 80% of triggered actions clearly without freezing the UI.
- Theme changes apply instantly and persist across restarts.
- Generated plans complete 90% of tasks within five steps.

---

## 2. Release Phases

| Phase | Window | Deliverable | Key Files |
|-------|--------|-------------|-----------|
| 1 | Day 1 | Theme presets, loader, CLI plumbing | `internal/themes/*`, `cmd/crush/theme.go`, `.crush/config.yaml` |
| 2 | Days 2–3 | Read-only explorer pane w/ watcher | `internal/explorer/*`, `internal/tui/layout.go` |
| 3 | Days 4–5 | Plan mode (5 steps max) with CLI entrypoint | `internal/planner/*`, `cmd/crush/plan.go` |
| 4 | Days 6–10 | Narrator agent with streaming support | `internal/narrator/*`, `internal/tui/layout.go` |
| 5 | Days 11–14 | Integrated 3-pane TUI polish, QA, docs | `internal/tui/*`, configs, README updates |

Commit after each phase, run smoke tests, then push to `feature/enhanced-ui`.

---

## 3. Repository Structure

```
crush-enhanced/
├── cmd/crush/
│   ├── main.go
│   ├── explore.go
│   ├── plan.go
│   ├── narrate.go
│   └── theme.go
├── configs/
│   └── themes/
│       ├── default.yaml
│       ├── nord.yaml
│       ├── dracula.yaml
│       └── monokai.yaml
├── internal/
│   ├── explorer/
│   │   ├── tree.go
│   │   └── watcher.go
│   ├── narrator/
│   │   ├── agent.go
│   │   └── explainer.go
│   ├── planner/
│   │   ├── decomposer.go
│   │   └── executor.go
│   ├── themes/
│   │   ├── manager.go
│   │   └── presets.go
│   └── tui/
│       └── layout.go
└── .crush/config.yaml
```

---

## 4. Architecture Overview

### UI Layout (ASCII)
```
[ Explorer 20% ] [ Main Chat 50% ] [ Narrator 30% ]
```
- Explorer pane hosts the read-only tree and navigation bindings.
- Chat pane remains the existing CRUSH conversational view.
- Narrator pane streams explanations when triggered.

### Component Relationships
```
cmd/crush/*  -> orchestrates CLI commands (theme, plan, narrate, default run)
internal/tui -> layout model wires explorer, chat, narrator Bubble Tea models
internal/themes -> manages theme presets and runtime styling
internal/explorer -> builds tree model + fsnotify watcher events
internal/planner -> LLM-backed plan generation and executor
internal/narrator -> Ollama-backed explanation service with streaming
```

---

## 5. Detailed Component Specs

### 5.1 Themes
- **Presets:** defined in `internal/themes/presets.go` and mirrored under `configs/themes/*.yaml`.
- **Manager API (`internal/themes/manager.go`):**
  ```go
  func LoadTheme(name string) (*Theme, error)
  func ListThemes() []string
  func (t *Theme) Apply(*lipgloss.Renderer) Styles // wire into Bubble Tea models
  ```
- **CLI (`cmd/crush/theme.go`):**
  - `crush theme list` prints available themes in alphabetical order.
  - `crush theme set <name>` validates preset, writes to `.crush/config.yaml`, and re-renders.
- **Persistence:** config stored in `$HOME/.crush/config.yaml`; ensure helper functions to read/write YAML with graceful defaults.

### 5.2 File Explorer
- **Tree Model (`tree.go`):** lazy directory expansion, sorted with dirs first, hidden file toggle from config.
- **Watcher (`watcher.go`):** fsnotify-based watcher that debounces events and notifies the narrator/explorer models on change.
- **Bubble Tea Model:** supports arrow navigation, expand/collapse on Enter, highlights selected node, exposes `SetSize`, `Update`, `View`.
- **Integration:** `layout.go` instantiates explorer model and routes keyboard messages.

### 5.3 Plan Mode
- **Planner API:**
  ```go
  func NewPlanner(client llm.Client, opts Options) *Planner
  func (p *Planner) CreatePlan(ctx context.Context, goal string) (*Plan, error)
  func (p *Planner) Execute(ctx context.Context, plan *Plan) error
  func (p *Planner) Progress(plan *Plan) (completed, total int)
  ```
- **Options:** max steps (default 5), auto-execute flag from config, retry limit.
- **LLM Interaction:** send structured prompt to Ollama (or configured client), expect JSON payload; include resilience (validate JSON, fallback to template).
- **CLI (`cmd/crush/plan.go`):** `crush plan "<goal>"` prints steps, optionally executes sequentially with progress bar.

### 5.4 Narrator Agent
- **Context (`explainer.go`):** builds summaries from git diffs, recent commands, or file watcher events.
- **Agent (`agent.go`):** wraps Ollama HTTP calls. Provide both blocking `Explain` and streaming `Stream` variants. Handle timeouts and offline mode by returning friendly fallback messages.
- **Trigger Points:** file change watcher, completed plan steps, user-invoked `--narrator` flag.
- **UI Model:** separate Bubble Tea model with loading spinner, scrollback, and theme-aware styling.

### 5.5 TUI Layout
- Unified layout model recomputes pane widths on `tea.WindowSizeMsg`.
- Routes messages to explorer/chat/narrator models.
- Applies current theme styles to each pane before rendering.

---

## 6. Configuration & Data Contracts

### `.crush/config.yaml`
```yaml
theme: nord

narrator:
  enabled: true
  ollama_url: http://localhost:11434
  model: llama2
  timeout_seconds: 30

explorer:
  show_hidden: false
  max_depth: 5

planner:
  max_steps: 5
  auto_execute: false
```

### Plan JSON Template
```json
{
  "goal": "Add authentication",
  "steps": [
    { "id": 1, "description": "Create user model", "status": "pending" }
  ]
}
```

### Narrator Context Payload
```json
{
  "action": "file_changed",
  "files": ["auth.go"],
  "diff_summary": "Added password hashing"
}
```

---

## 7. External Interfaces

### Ollama API
- **Endpoint:** `POST /api/generate` for blocking calls, `POST /api/generate` with streamed chunks for SSE.
- **Headers:** `Content-Type: application/json`.
- **Request Body:** include `prompt`, `model`, `stream` flags.
- **Timeout Handling:** configure context deadlines (`timeout_seconds` from config) and surface user-friendly errors in narrator pane.

### fsnotify Watcher
- Watch workspace root.
- Ignore temporary files and `.git`.
- Debounce bursts (suggest 150–250ms window) to avoid flooding narrator.

---

## 8. Implementation Steps for Kilocode

1. **Prep**
   ```bash
   gh repo fork charmbracelet/crush --clone
   cd crush
   git checkout -b feature/enhanced-ui
   gofmt -w .
   ```
2. **Create directories** following the structure in Section 3.
3. **Update `go.mod`** with required dependencies (Section 9).
4. **Phase Execution**
   - Implement theme presets/manager, register Cobra commands, and wire config persistence.
   - Build explorer tree + watcher, integrate into layout; ensure keybindings tested.
   - Implement planner with LLM client, CLI, and progress reporting.
   - Build narrator agent with streaming; connect triggers and UI pane.
   - Polish layout, apply theme styling across panes, finalize tests/docs.
5. **Testing** after each phase using checklist in Section 10.
6. **Commits & Push**
   ```bash
   git add <phase files>
   git commit -m "feat: add <component>"
   git push origin feature/enhanced-ui
   ```

---

## 9. Dependencies (add to `go.mod`)
```
github.com/charmbracelet/bubbletea v0.25.0
github.com/charmbracelet/bubbles v0.18.0
github.com/charmbracelet/lipgloss v0.9.1
github.com/fsnotify/fsnotify v1.7.0
github.com/spf13/cobra v1.7.0
gopkg.in/yaml.v3 v3.0.1
```

---

## 10. Testing Checklist

**Themes**
- `crush theme list` prints `default`, `dracula`, `monokai`, `nord`.
- `crush theme set nord` applies colors immediately.
- Restart and verify persisted theme.

**Explorer**
- Launch `crush`; left pane shows current directory tree.
- Arrow keys move selection; Enter toggles expand/collapse.
- Active file highlight follows editor events (if available) or defaults to selection.

**Plan Mode**
- `crush plan "Add error handling"` outputs exactly five actionable steps.
- Progress bar advances per step; failure sets status to `failed` with message.
- Auto-execute off by default; respect config when enabled.

**Narrator**
- Run `crush --narrator`; right pane shows “Waiting for events…” baseline.
- On file change, narrator streams 2–3 sentence explanation.
- Handles Ollama offline by showing fallback notice without crashing.

**Regression**
- Existing chat functionality unaffected.
- Theme styles applied across panes consistently.

---

## 11. Build & Smoke Tests
```bash
# Build binary
go build -o crush-enhanced ./cmd/crush

# Themes
./crush-enhanced theme list
./crush-enhanced theme set dracula

# Explorer + narrator
./crush-enhanced --narrator

# Plan mode
./crush-enhanced plan "Refactor logging"
```

Automate smoke tests via a simple Go or shell script once all features land.

---

## 12. Hand-off Notes
- Use consistent module import path (replace `your-repo/internal/...` with actual module name, e.g., `github.com/username/crush-enhanced/internal/...`).
- Avoid destructive git commands; the repo may include user-local changes.
- Maintain ASCII-only diagrams for compatibility with various terminals.
- Document any deviations from this guide in `docs/CHANGELOG.md`.
- After delivery, bundle a short README update summarizing new features.

---

Give this guide to Kilocode with the instruction: **“Implement each phase sequentially, running the checklist after every phase before proceeding.”**

