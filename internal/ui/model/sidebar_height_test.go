package model

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// sidebarHeightTestWorkspace implements just enough of [workspace.Workspace]
// for drawSidebar.
type sidebarHeightTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *sidebarHeightTestWorkspace) Config() *config.Config { return w.cfg }
func (w *sidebarHeightTestWorkspace) WorkingDir() string     { return "/tmp/project" }
func (w *sidebarHeightTestWorkspace) AgentIsReady() bool     { return false }
func (w *sidebarHeightTestWorkspace) AgentReadyErr() error   { return nil }
func (w *sidebarHeightTestWorkspace) LSPGetStates() map[string]workspace.LSPClientInfo {
	return nil
}

func (w *sidebarHeightTestWorkspace) LSPGetDiagnosticCounts(string) lsp.DiagnosticCounts {
	return lsp.DiagnosticCounts{}
}

// newSidebarHeightTestUI builds a UI whose sidebar has every section
// populated with more items than the dynamic limits will allow, so the
// height budget arithmetic — not the item counts — decides what is visible.
func newSidebarHeightTestUI(t *testing.T) *UI {
	t.Helper()

	mcps := config.MCPs{}
	mcpStates := map[string]mcp.ClientInfo{}
	for i := range 4 {
		name := fmt.Sprintf("mcp-%d", i)
		mcps[name] = config.MCPConfig{}
		mcpStates[name] = mcp.ClientInfo{Name: name, State: mcp.StateConnected}

		chName := fmt.Sprintf("chan-%d", i)
		mcps[chName] = config.MCPConfig{}
		mcpStates[chName] = mcp.ClientInfo{Name: chName, State: mcp.StateConnected, Channel: true}
	}

	var files []SessionFile
	for i := range 4 {
		files = append(files, SessionFile{
			FirstVersion: history.File{Path: fmt.Sprintf("/tmp/project/file-%d.go", i)},
			Additions:    1,
		})
	}

	lspStates := map[string]workspace.LSPClientInfo{}
	for i := range 4 {
		name := fmt.Sprintf("lsp-%d", i)
		lspStates[name] = workspace.LSPClientInfo{Name: name}
	}

	var skillStates []*skills.SkillState
	for i := range 4 {
		name := fmt.Sprintf("skill-%d", i)
		skillStates = append(skillStates, &skills.SkillState{
			Name: name,
			Path: fmt.Sprintf("/skills/%s/SKILL.md", name),
		})
	}

	s := styles.CharmtonePantera()
	return &UI{
		com: &common.Common{
			Workspace: &sidebarHeightTestWorkspace{cfg: &config.Config{MCP: mcps, Options: &config.Options{}}},
			Styles:    &s,
		},
		state:        uiChat,
		focus:        uiFocusEditor, // unfocused sidebar: dynamic limits apply
		session:      &session.Session{ID: "s1", Title: "Test Session"},
		sessionFiles: files,
		lspStates:    lspStates,
		mcpStates:    mcpStates,
		skillStates:  skillStates,
	}
}

// TestSidebarAllSectionTitlesVisibleAtTightHeight pins the sidebar height
// budget: at a height where every section holds only its minimum items, all
// five section titles (Modified Files, LSPs, MCPs, Skills, Channels) must be
// visible. The old budget subtracted an overhead constant sized for four
// sections, so the item allocation overshot the space and the MaxHeight clip
// silently swallowed the bottom (Channels) section.
func TestSidebarAllSectionTitlesVisibleAtTightHeight(t *testing.T) {
	t.Parallel()

	m := newSidebarHeightTestUI(t)

	// Header is 7 lines (logo, title, blank, cwd, blank, model info, blank).
	// The five sections at minimum need 2 title+blank lines each, 4 blank
	// separators, and 2 item lines each: 7 + (5*2 + 4) + 5*2 = 31.
	const width, height = 32, 31
	m.layout.sidebar = uv.Rect(0, 0, width, height)

	scr := uv.NewScreenBuffer(width, height)
	m.drawSidebar(scr, m.layout.sidebar)
	out := ansi.Strip(scr.Render())

	for _, title := range []string{"Modified Files", "LSPs", "MCPs", "Skills", "Channels"} {
		require.Contains(t, out, title,
			"section title %q must be visible at height %d", title, height)
	}
}
