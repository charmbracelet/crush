package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/version"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	// statusLineInterval is how often the status line command re-runs.
	statusLineInterval = time.Second
	// statusLineTimeout bounds a single status line command run. Slow or
	// hung scripts leave the previous output in place instead of blocking
	// the refresh loop forever.
	statusLineTimeout = 5 * time.Second
)

// statusLineTickMsg schedules the next status line refresh.
type statusLineTickMsg struct{}

// statusLineOutputMsg carries the latest status line content. An empty
// content string hides the status line.
type statusLineOutputMsg struct {
	content string
}

// StatusLine renders the output of a user-provided status line command at
// the bottom of the TUI. The command receives a JSON payload describing the
// current session on stdin and its first output line is displayed as-is,
// truncated to the available width.
type StatusLine struct {
	com     *common.Common
	content string
	width   int
}

// NewStatus creates a status line model. It is only constructed when a
// status line command is configured.
func NewStatusLine(com *common.Common) *StatusLine {
	return &StatusLine{com: com}
}

// SetContent updates the rendered content.
func (s *StatusLine) SetContent(content string) {
	s.content = content
}

// SetWidth sets the width used to truncate the status line.
func (s *StatusLine) SetWidth(width int) {
	s.width = width
}

// Draw draws the status line onto the screen.
func (s *StatusLine) Draw(scr uv.Screen, area uv.Rectangle) {
	if s.content == "" || area.Dx() <= 0 || area.Dy() <= 0 {
		return
	}
	line := strings.TrimRight(s.content, "\r\n")
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = ansi.Truncate(line, min(area.Dx(), s.width), "…")
	uv.NewStyledString(line).Draw(scr, area)
}

// statusLineTick schedules the periodic status line refresh. The first tick
// fires after one interval so startup is not blocked on an external command.
func statusLineTick() tea.Cmd {
	return tea.Tick(statusLineInterval, func(time.Time) tea.Msg {
		return statusLineTickMsg{}
	})
}

// runStatusLine executes the configured status line command with the JSON
// payload on stdin and returns its trimmed output. On failure the command's
// stderr (or the execution error) is returned instead, so misconfiguration
// is visible instead of silently rendering nothing.
func runStatusLine(command string, payload []byte) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), statusLineTimeout)
		defer cancel()
		name, args := statusLineShellCommand(command)
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = bytes.NewReader(payload)
		out, err := cmd.Output()
		content := string(out)
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				content = string(exitErr.Stderr)
			} else {
				content = err.Error()
			}
		}
		return statusLineOutputMsg{content: strings.TrimSpace(content)}
	}
}

// statusLineShellCommand wraps the user command in a shell so pipes, env
// expansion, and quoted arguments behave the way status line scripts expect.
func statusLineShellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
}

// StatusLinePayload is the JSON document written to the status line
// command's stdin on every refresh. Field names follow the Claude Code
// status line contract where there is an equivalent, so existing scripts
// port with minimal changes.
type StatusLinePayload struct {
	SessionID string              `json:"session_id,omitempty"`
	Title     string              `json:"title,omitempty"`
	Cwd       string              `json:"cwd,omitempty"`
	Model     StatusLineModel     `json:"model"`
	Workspace StatusLineWorkspace `json:"workspace"`
	Context   StatusLineContext   `json:"context"`
	Cost      StatusLineCost      `json:"cost"`
	Version   string              `json:"version,omitempty"`
}

// StatusLineModel describes the selected model.
type StatusLineModel struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Provider    string `json:"provider,omitempty"`
}

// StatusLineWorkspace describes the workspace directories.
type StatusLineWorkspace struct {
	CurrentDir string `json:"current_dir,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
}

// StatusLineContext describes token usage relative to the context window.
type StatusLineContext struct {
	UsedTokens     int64   `json:"used_tokens,omitempty"`
	ContextWindow  int64   `json:"context_window,omitempty"`
	Percentage     float64 `json:"percentage,omitempty"`
	EstimatedUsage bool    `json:"estimated_usage,omitempty"`
}

// StatusLineCost describes the accumulated session cost.
type StatusLineCost struct {
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
}

// buildStatusLinePayload snapshots the current session state into the JSON
// payload handed to the status line command.
func (m *UI) buildStatusLinePayload() StatusLinePayload {
	payload := StatusLinePayload{
		Version: version.Version,
	}
	if m.com.Workspace != nil {
		payload.Cwd = m.com.Workspace.WorkingDir()
		payload.Workspace.CurrentDir = payload.Cwd
	}
	if m.session != nil {
		s := m.session
		payload.SessionID = s.ID
		payload.Title = s.Title
		payload.Context.UsedTokens = s.PromptTokens + s.CompletionTokens
		payload.Context.EstimatedUsage = s.EstimatedUsage
		payload.Cost.TotalCostUSD = s.Cost
	}
	if model := m.selectedLargeModel(); model != nil {
		payload.Model.ID = model.ModelCfg.Model
		payload.Model.DisplayName = model.CatwalkCfg.Name
		payload.Model.Provider = model.ModelCfg.Provider
		payload.Context.ContextWindow = model.CatwalkCfg.ContextWindow
	}
	if payload.Context.ContextWindow > 0 {
		payload.Context.Percentage = float64(payload.Context.UsedTokens) / float64(payload.Context.ContextWindow) * 100
	}
	return payload
}

// statusLineRefresh returns the command that runs the configured status
// line command with a fresh payload snapshot and re-arms the tick.
func (m *UI) statusLineRefresh() tea.Cmd {
	if m.statusLine == nil {
		return nil
	}
	cfg := config.StatusLineConfig{}
	if c := m.com.Config(); c != nil && c.Options != nil && c.Options.TUI != nil {
		cfg = *c.Options.TUI.StatusLine
	}
	payload, err := json.Marshal(m.buildStatusLinePayload())
	if err != nil {
		payload = []byte("{}")
	}
	return tea.Batch(
		runStatusLine(cfg.Command, payload),
		statusLineTick(),
	)
}
