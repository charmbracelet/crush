package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// sidebarTestWorkspace is a minimal [workspace.Workspace] stub used to
// exercise drawSidebar, following the same idiom as testWorkspace in
// ui_test.go and getSessionWorkspace in session_test.go.
type sidebarTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
	dir string
}

func (w *sidebarTestWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *sidebarTestWorkspace) WorkingDir() string {
	return w.dir
}

// AgentIsReady is overridden (rather than left to the nil embedded
// workspace.Workspace) because drawSidebar renders model info via
// modelInfo, which calls selectedLargeModel, which calls
// Workspace.AgentIsReady before touching AgentModel. Leaving this
// unimplemented would panic on the nil embedded interface.
func (w *sidebarTestWorkspace) AgentIsReady() bool {
	return false
}

// LSPGetStates and LSPGetDiagnosticCounts are overridden (rather than left to
// the nil embedded workspace.Workspace) because lspInfo always pulls fresh
// state from the workspace. Leaving these unimplemented would panic on the
// nil embedded interface.
func (w *sidebarTestWorkspace) LSPGetStates() map[string]workspace.LSPClientInfo {
	return nil
}

func (w *sidebarTestWorkspace) LSPGetDiagnosticCounts(string) lsp.DiagnosticCounts {
	return lsp.DiagnosticCounts{}
}

// newSidebarTestUI builds a *UI suitable for rendering drawSidebar, wiring
// up the minimal workspace/session/layout state it needs.
func newSidebarTestUI(t *testing.T, runningSubagents []workspace.RunningSubagentInfo) *UI {
	t.Helper()

	u := newTestUI()
	u.com.Workspace = &sidebarTestWorkspace{
		// Options must be non-nil: skillStatusItems dereferences
		// cfg.Options.DisabledSkills while building the sidebar's
		// skills section.
		cfg: &config.Config{Options: &config.Options{}},
		dir: "/tmp/test-workspace",
	}
	u.session = &session.Session{ID: "s1", Title: "Test Session"}
	u.runningSubagents = runningSubagents
	u.updateLayoutAndSize()

	return u
}

// renderSidebar draws the sidebar into a fresh screen buffer and returns
// the ANSI-stripped rendered text.
func renderSidebar(t *testing.T, u *UI) string {
	t.Helper()

	w := u.layout.sidebar.Dx()
	h := u.layout.sidebar.Dy()
	require.Positive(t, w, "sidebar width must be positive")
	require.Positive(t, h, "sidebar height must be positive")

	scr := uv.ScreenBuffer{
		RenderBuffer: uv.NewRenderBuffer(w, h),
		Method:       ansi.GraphemeWidth,
	}

	// drawSidebar reads cached scroll state (m.sidebarContent and friends)
	// populated by updateSidebarScrollState; the real update loop always
	// calls it before drawing.
	u.updateSidebarScrollState()

	// drawSidebar only relies on area.Dx()/Dy() for sizing but draws
	// relative to area's origin; since our screen buffer is allocated
	// at (0,0) with exactly the sidebar's width/height, we draw into a
	// zero-origin rect of the same dimensions (mirroring the pattern in
	// chat_draw_cache_test.go, which draws into uv.Rect(0, 0, w, h)
	// rather than the real, possibly offset, layout rectangle).
	u.drawSidebar(scr, uv.Rect(0, 0, w, h))

	return stripANSI(scr.Render())
}

// TestDrawSidebar_ShowsSubagentsWhenPresent asserts that the full sidebar
// renders an "Active subagents" style section (reusing subagentsInfo) when
// there are running subagents, mirroring the compact-mode behavior in
// drawSessionDetails.
func TestDrawSidebar_ShowsSubagentsWhenPresent(t *testing.T) {
	t.Parallel()

	u := newSidebarTestUI(t, []workspace.RunningSubagentInfo{
		{Name: "test-agent", Color: "blue"},
	})

	out := renderSidebar(t, u)

	require.Contains(t, out, "Subagents")
	require.Contains(t, out, "test-agent")
}

// TestDrawSidebar_HidesSubagentsWhenEmpty asserts that the full sidebar
// does not render any "Subagents" section when there are no running
// subagents.
func TestDrawSidebar_HidesSubagentsWhenEmpty(t *testing.T) {
	t.Parallel()

	u := newSidebarTestUI(t, nil)

	out := renderSidebar(t, u)

	require.NotContains(t, out, "Subagents")
}
