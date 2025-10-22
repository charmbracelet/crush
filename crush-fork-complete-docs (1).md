# CRUSH Fork Implementation Package

## Project Overview

**Name:** CRUSH Enhanced  
**Goal:** Add file explorer, narrator agent, themes, and plan mode to CRUSH  
**Timeline:** 2-3 weeks  
**Tech Stack:** Go, Bubble Tea, Lipgloss, Ollama

---

## 1. PRD (Product Requirements)

### Features Priority
1. File Explorer (browse-only)
2. Narrator Agent (mentor/teacher mode)
3. Color Themes (4 presets)
4. Plan Mode (5-step max)
5. Voice Dictation (future)

### Success Metrics
- Can browse files while coding
- Narrator explains 80% of actions clearly
- Themes apply without restart
- Plans complete 90%+ of 5-step tasks

### Non-Goals
- File editing in explorer
- Real-time narrator (on-demand only)
- Plan mode >5 steps
- VS Code integration

---

## 2. Technical Architecture

### Layout
```
┌──────────┬─────────────────┬──────────┐
│ Files    │ Main Chat       │ Narrator │
│ (20%)    │ (50%)           │ (30%)    │
│          │                 │          │
│ tree     │ user: prompt    │ teaching │
│ view     │ ai: response    │ mode     │
└──────────┴─────────────────┴──────────┘
```

### Component Architecture
```
internal/
├── explorer/
│   ├── tree.go          # File tree model
│   └── watcher.go       # File change detection
├── narrator/
│   ├── agent.go         # Narrator LLM client
│   └── explainer.go     # Context builder
├── planner/
│   ├── decomposer.go    # Task → steps
│   └── executor.go      # Step runner
├── themes/
│   ├── manager.go       # Theme loader
│   └── presets.go       # Built-in themes
└── tui/
    └── layout.go        # 3-pane layout manager
```

---

## 3. File Structure

```
my-crush-fork/
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
├── cmd/crush/
│   ├── main.go
│   ├── explore.go
│   ├── narrate.go
│   ├── plan.go
│   └── theme.go
└── configs/
    └── themes/
        ├── default.yaml
        ├── nord.yaml
        ├── dracula.yaml
        └── monokai.yaml
```

---

## 4. API Specifications

### File Explorer API

```go
package explorer

type FileTree struct {
    Root     *Node
    Selected *Node
}

type Node struct {
    Name     string
    Path     string
    IsDir    bool
    Children []*Node
}

// Public API
func NewFileTree(rootPath string) (*FileTree, error)
func (t *FileTree) Expand(node *Node) error
func (t *FileTree) Collapse(node *Node)
func (t *FileTree) Select(node *Node)
func (t *FileTree) Watch() <-chan Event
```

### Narrator Agent API

```go
package narrator

type Narrator struct {
    client OllamaClient
}

type Context struct {
    Action      string   // "file_changed", "git_commit", "error"
    Files       []string
    DiffSummary string
}

// Public API
func New(ollamaURL string) *Narrator
func (n *Narrator) Explain(ctx Context) (string, error)
func (n *Narrator) Stream(ctx Context) <-chan string
```

### Planner API

```go
package planner

type Plan struct {
    Goal  string
    Steps []Step
}

type Step struct {
    ID          int
    Description string
    Status      string // pending, running, done, failed
    Result      string
}

// Public API
func NewPlanner(llmClient interface{}) *Planner
func (p *Planner) CreatePlan(goal string) (*Plan, error)
func (p *Planner) Execute(plan *Plan) error
func (p *Planner) GetProgress() (current, total int)
```

### Themes API

```go
package themes

type Theme struct {
    Name       string
    Background string
    Foreground string
    Accent     string
    Error      string
    Success    string
}

// Public API
func LoadTheme(name string) (*Theme, error)
func ListThemes() []string
func (t *Theme) Apply() error
func (t *Theme) ToLipgloss() lipgloss.Style
```

---

## 5. Implementation Phases

### Phase 1: Themes (Day 1)
**Goal:** Working theme system  
**Files:**
- `internal/themes/manager.go`
- `internal/themes/presets.go`
- `cmd/crush/theme.go`

**Test:**
```bash
crush theme list
crush theme set nord
# Verify colors change
```

### Phase 2: File Explorer (Days 2-3)
**Goal:** Read-only file browser  
**Files:**
- `internal/explorer/tree.go`
- `internal/explorer/watcher.go`
- `internal/tui/layout.go` (add left pane)

**Test:**
```bash
crush
# See file tree in left pane
# Arrow keys navigate
# Shows current file highlighted
```

### Phase 3: Plan Mode (Days 4-5)
**Goal:** 5-step task decomposition  
**Files:**
- `internal/planner/decomposer.go`
- `internal/planner/executor.go`
- `cmd/crush/plan.go`

**Test:**
```bash
crush plan "Add error handling to auth.go"
# Shows 5 steps
# Executes sequentially
# Displays progress
```

### Phase 4: Narrator Agent (Days 6-10)
**Goal:** Side-by-side teaching mode  
**Files:**
- `internal/narrator/agent.go`
- `internal/narrator/explainer.go`
- `internal/tui/layout.go` (add right pane)

**Test:**
```bash
crush --narrator
# Main chat on left
# Narrator explanations on right
# Triggers on file changes
```

---

## 6. Data Contracts

### Theme YAML Format
```yaml
name: Nord
background: "#2E3440"
foreground: "#D8DEE9"
accent: "#88C0D0"
error: "#BF616A"
success: "#A3BE8C"
warning: "#EBCB8B"
```

### Plan JSON Format
```json
{
  "goal": "Add authentication",
  "steps": [
    {
      "id": 1,
      "description": "Create user model",
      "status": "pending"
    }
  ]
}
```

### Narrator Context Format
```json
{
  "action": "file_changed",
  "files": ["auth.go"],
  "diff_summary": "Added password hashing"
}
```

---

## 7. Bubble Tea Models

### Layout Model
```go
type LayoutModel struct {
    explorer  explorer.Model
    chat      chat.Model
    narrator  narrator.Model
    theme     themes.Theme
    width     int
    height    int
}

func (m LayoutModel) Init() tea.Cmd
func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m LayoutModel) View() string
```

### Explorer Model
```go
type Model struct {
    tree     *FileTree
    viewport viewport.Model
    cursor   int
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string
```

### Narrator Model
```go
type Model struct {
    content  string
    viewport viewport.Model
    loading  bool
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string
```

---

## 8. Configuration

### .crush/config.yaml
```yaml
theme: nord

narrator:
  enabled: true
  ollama_url: http://localhost:11434
  model: llama2

explorer:
  show_hidden: false
  max_depth: 5

planner:
  max_steps: 5
  auto_execute: false
```

---

## 9. Implementation Guide for Kilocode

### Step-by-Step Instructions

**Phase 1: Setup**
```bash
# Fork CRUSH
gh repo fork charmbracelet/crush --clone

# Create branch
git checkout -b feature/enhanced-ui

# Create directory structure
mkdir -p internal/{explorer,narrator,planner,themes,tui}
mkdir -p configs/themes
```

**Phase 2: Themes (Implement First)**

*File: `internal/themes/presets.go`*
```go
package themes

var Presets = map[string]Theme{
    "default": {
        Name:       "Default",
        Background: "#000000",
        Foreground: "#FFFFFF",
        Accent:     "#00FFFF",
        Error:      "#FF5555",
        Success:    "#50FA7B",
    },
    "nord": {
        Name:       "Nord",
        Background: "#2E3440",
        Foreground: "#D8DEE9",
        Accent:     "#88C0D0",
        Error:      "#BF616A",
        Success:    "#A3BE8C",
    },
    "dracula": {
        Name:       "Dracula",
        Background: "#282A36",
        Foreground: "#F8F8F2",
        Accent:     "#BD93F9",
        Error:      "#FF5555",
        Success:    "#50FA7B",
    },
    "monokai": {
        Name:       "Monokai",
        Background: "#272822",
        Foreground: "#F8F8F2",
        Accent:     "#66D9EF",
        Error:      "#F92672",
        Success:    "#A6E22E",
    },
}
```

*File: `internal/themes/manager.go`*
```go
package themes

import (
    "fmt"
    "github.com/charmbracelet/lipgloss"
)

type Theme struct {
    Name       string
    Background string
    Foreground string
    Accent     string
    Error      string
    Success    string
}

func LoadTheme(name string) (*Theme, error) {
    theme, ok := Presets[name]
    if !ok {
        return nil, fmt.Errorf("theme not found: %s", name)
    }
    return &theme, nil
}

func ListThemes() []string {
    names := make([]string, 0, len(Presets))
    for name := range Presets {
        names = append(names, name)
    }
    return names
}

func (t *Theme) AccentStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
}

func (t *Theme) ErrorStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error))
}

func (t *Theme) SuccessStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success))
}
```

*File: `cmd/crush/theme.go`*
```go
package main

import (
    "fmt"
    "github.com/spf13/cobra"
    "your-repo/internal/themes"
)

var themeCmd = &cobra.Command{
    Use:   "theme",
    Short: "Manage color themes",
}

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List available themes",
    Run: func(cmd *cobra.Command, args []string) {
        for _, name := range themes.ListThemes() {
            fmt.Println(name)
        }
    },
}

var setCmd = &cobra.Command{
    Use:   "set [name]",
    Short: "Set active theme",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        theme, err := themes.LoadTheme(args[0])
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        // Save to config
        fmt.Printf("Theme set to: %s\n", theme.Name)
    },
}

func init() {
    themeCmd.AddCommand(listCmd, setCmd)
    rootCmd.AddCommand(themeCmd)
}
```

**Phase 3: File Explorer**

*File: `internal/explorer/tree.go`*
```go
package explorer

import (
    "os"
    "path/filepath"
    "sort"
)

type Node struct {
    Name     string
    Path     string
    IsDir    bool
    Expanded bool
    Children []*Node
}

type FileTree struct {
    Root     *Node
    Selected *Node
}

func NewFileTree(rootPath string) (*FileTree, error) {
    info, err := os.Stat(rootPath)
    if err != nil {
        return nil, err
    }
    
    root := &Node{
        Name:  filepath.Base(rootPath),
        Path:  rootPath,
        IsDir: info.IsDir(),
    }
    
    if root.IsDir {
        loadChildren(root)
    }
    
    return &FileTree{Root: root, Selected: root}, nil
}

func loadChildren(node *Node) error {
    entries, err := os.ReadDir(node.Path)
    if err != nil {
        return err
    }
    
    node.Children = make([]*Node, 0, len(entries))
    
    for _, entry := range entries {
        if entry.Name()[0] == '.' {
            continue // Skip hidden
        }
        
        child := &Node{
            Name:  entry.Name(),
            Path:  filepath.Join(node.Path, entry.Name()),
            IsDir: entry.IsDir(),
        }
        node.Children = append(node.Children, child)
    }
    
    sort.Slice(node.Children, func(i, j int) bool {
        if node.Children[i].IsDir != node.Children[j].IsDir {
            return node.Children[i].IsDir
        }
        return node.Children[i].Name < node.Children[j].Name
    })
    
    return nil
}

func (t *FileTree) Expand(node *Node) error {
    if !node.IsDir {
        return nil
    }
    if len(node.Children) == 0 {
        if err := loadChildren(node); err != nil {
            return err
        }
    }
    node.Expanded = true
    return nil
}

func (t *FileTree) Collapse(node *Node) {
    node.Expanded = false
}

func (t *FileTree) Flatten() []*Node {
    var result []*Node
    var walk func(*Node, int)
    
    walk = func(node *Node, depth int) {
        result = append(result, node)
        if node.IsDir && node.Expanded {
            for _, child := range node.Children {
                walk(child, depth+1)
            }
        }
    }
    
    walk(t.Root, 0)
    return result
}
```

**Phase 4: Plan Mode**

*File: `internal/planner/decomposer.go`*
```go
package planner

import (
    "context"
    "encoding/json"
    "fmt"
)

type Planner struct {
    llmClient interface{} // Your LLM client
}

type Plan struct {
    Goal  string
    Steps []Step
}

type Step struct {
    ID          int
    Description string
    Status      string
    Result      string
}

func NewPlanner(client interface{}) *Planner {
    return &Planner{llmClient: client}
}

func (p *Planner) CreatePlan(ctx context.Context, goal string) (*Plan, error) {
    prompt := fmt.Sprintf(`Break this task into exactly 5 steps:

Goal: %s

Return JSON only:
{
  "steps": [
    {"id": 1, "description": "step description"},
    ...
  ]
}

Maximum 5 steps. Be specific and actionable.`, goal)
    
    // Call LLM (pseudo-code - integrate with your client)
    response := "..." // LLM response
    
    var result struct {
        Steps []struct {
            ID          int    `json:"id"`
            Description string `json:"description"`
        } `json:"steps"`
    }
    
    if err := json.Unmarshal([]byte(response), &result); err != nil {
        return nil, err
    }
    
    plan := &Plan{Goal: goal}
    for _, s := range result.Steps {
        plan.Steps = append(plan.Steps, Step{
            ID:          s.ID,
            Description: s.Description,
            Status:      "pending",
        })
    }
    
    return plan, nil
}
```

*File: `internal/planner/executor.go`*
```go
package planner

import "context"

func (p *Planner) Execute(ctx context.Context, plan *Plan) error {
    for i := range plan.Steps {
        plan.Steps[i].Status = "running"
        
        // Execute step via LLM
        result, err := p.executeStep(ctx, plan.Steps[i])
        if err != nil {
            plan.Steps[i].Status = "failed"
            plan.Steps[i].Result = err.Error()
            return err
        }
        
        plan.Steps[i].Status = "done"
        plan.Steps[i].Result = result
    }
    
    return nil
}

func (p *Planner) executeStep(ctx context.Context, step Step) (string, error) {
    // Send step description to LLM
    // Return result
    return "completed", nil
}

func (p *Planner) GetProgress(plan *Plan) (current, total int) {
    total = len(plan.Steps)
    for _, step := range plan.Steps {
        if step.Status == "done" {
            current++
        }
    }
    return
}
```

**Phase 5: Narrator Agent**

*File: `internal/narrator/agent.go`*
```go
package narrator

import (
    "context"
    "fmt"
)

type Narrator struct {
    ollamaURL string
}

type Context struct {
    Action      string
    Files       []string
    DiffSummary string
}

func New(ollamaURL string) *Narrator {
    return &Narrator{ollamaURL: ollamaURL}
}

func (n *Narrator) Explain(ctx context.Context, c Context) (string, error) {
    prompt := n.buildPrompt(c)
    
    // Call Ollama API
    response, err := n.callOllama(ctx, prompt)
    if err != nil {
        return "", err
    }
    
    return response, nil
}

func (n *Narrator) Stream(ctx context.Context, c Context) <-chan string {
    ch := make(chan string)
    
    go func() {
        defer close(ch)
        
        prompt := n.buildPrompt(c)
        // Stream from Ollama
        // Send chunks to channel
    }()
    
    return ch
}

func (n *Narrator) buildPrompt(c Context) string {
    return fmt.Sprintf(`You are a coding teacher explaining what just happened.

Action: %s
Files: %v
Changes: %s

Explain in 2-3 sentences what the coder did and why. Be encouraging and educational.`,
        c.Action, c.Files, c.DiffSummary)
}

func (n *Narrator) callOllama(ctx context.Context, prompt string) (string, error) {
    // HTTP POST to Ollama API
    // Return response
    return "", nil
}
```

**Phase 6: 3-Pane Layout**

*File: `internal/tui/layout.go`*
```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "your-repo/internal/explorer"
    "your-repo/internal/narrator"
)

type LayoutModel struct {
    explorer  explorer.Model
    chat      ChatModel // Your existing chat
    narrator  narrator.Model
    width     int
    height    int
}

func (m LayoutModel) Init() tea.Cmd {
    return nil
}

func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        
        // Allocate space
        explorerWidth := m.width * 20 / 100
        narratorWidth := m.width * 30 / 100
        chatWidth := m.width - explorerWidth - narratorWidth
        
        // Update children
        m.explorer.SetSize(explorerWidth, m.height)
        m.chat.SetSize(chatWidth, m.height)
        m.narrator.SetSize(narratorWidth, m.height)
    }
    
    // Route messages to appropriate pane
    return m, nil
}

func (m LayoutModel) View() string {
    explorer := m.explorer.View()
    chat := m.chat.View()
    narrator := m.narrator.View()
    
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        explorer,
        chat,
        narrator,
    )
}
```

---

## 10. Testing Checklist

**Themes:**
- [ ] `crush theme list` shows 4 themes
- [ ] `crush theme set nord` changes colors
- [ ] Theme persists across restarts

**File Explorer:**
- [ ] Shows current directory tree
- [ ] Arrow keys navigate
- [ ] Enter expands/collapses directories
- [ ] Highlights active file

**Plan Mode:**
- [ ] `crush plan "task"` generates 5 steps
- [ ] Steps execute sequentially
- [ ] Progress bar updates
- [ ] Handles failures gracefully

**Narrator:**
- [ ] Right pane shows explanations
- [ ] Streams from Ollama
- [ ] Triggers on file changes
- [ ] Doesn't block main chat

---

## 11. Dependencies

Add to `go.mod`:
```
github.com/charmbracelet/bubbletea v0.25.0
github.com/charmbracelet/lipgloss v0.9.1
github.com/charmbracelet/bubbles v0.18.0
github.com/fsnotify/fsnotify v1.7.0
```

---

## 12. Configuration Files

**configs/themes/nord.yaml:**
```yaml
name: Nord
background: "#2E3440"
foreground: "#D8DEE9"
accent: "#88C0D0"
error: "#BF616A"
success: "#A3BE8C"
warning: "#EBCB8B"
```

---

## 13. Build & Run

```bash
# Build
go build -o crush-fork ./cmd/crush

# Test themes
./crush-fork theme list
./crush-fork theme set nord

# Test with all features
./crush-fork --narrator

# Test plan mode
./crush-fork plan "Add error handling"
```

---

## 14. Git Workflow

```bash
# Commit after each phase
git add internal/themes
git commit -m "feat: add color themes"

git add internal/explorer
git commit -m "feat: add file explorer"

git add internal/planner
git commit -m "feat: add plan mode"

git add internal/narrator
git commit -m "feat: add narrator agent"

# Push
git push origin feature/enhanced-ui
```

---

## End of Implementation Package

All files, APIs, and instructions provided. Give this document to Kilocode with instruction: "Implement all phases sequentially, testing after each phase."