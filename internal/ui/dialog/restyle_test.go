package dialog

import (
	"image"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// Every dialog and inline editor that copies styles or caches renders must
// be restyled when the theme changes in place.
var (
	_ StyleRefresher = (*Permissions)(nil)
	_ StyleRefresher = (*Commands)(nil)
	_ StyleRefresher = (*Models)(nil)
	_ StyleRefresher = (*Session)(nil)
	_ StyleRefresher = (*Reasoning)(nil)
	_ StyleRefresher = (*Notifications)(nil)
	_ StyleRefresher = (*Arguments)(nil)
	_ StyleRefresher = (*Theme)(nil)
	_ StyleRefresher = (*ThemeEditor)(nil)
	_ StyleRefresher = (*ThemeNew)(nil)
	_ StyleRefresher = (*APIKeyInput)(nil)
	_ StyleRefresher = (*AuthMethod)(nil)
	_ StyleRefresher = (*AWSSSO)(nil)
	_ StyleRefresher = (*MCPAuth)(nil)
	_ StyleRefresher = (*MCPToggles)(nil)
	_ StyleRefresher = (*OAuth)(nil)
	_ StyleRefresher = (*FilePicker)(nil)
	_ StyleRefresher = (*QuestionForm)(nil)
	_ StyleRefresher = (*SingleChoice)(nil)
	_ StyleRefresher = (*MultiChoice)(nil)
	_ StyleRefresher = (*YesNo)(nil)
	_ StyleRefresher = (*FreeText)(nil)
	_ StyleRefresher = (*PlanHandoffInline)(nil)
)

// restyleTheme returns a theme whose colors differ from the default
// throughout, built without reading theme files.
func restyleTheme(t *testing.T) styles.Styles {
	t.Helper()
	s, err := styles.HostThemeStyles("", map[string]string{
		"primary":          "#fabd2f",
		"secondary":        "#b8bb26",
		"accent":           "#83a598",
		"fg_base":          "#ebdbb2",
		"fg_subtle":        "#d5c4a1",
		"fg_more_subtle":   "#bdae93",
		"fg_most_subtle":   "#a89984",
		"bg_base":          "#282828",
		"bg_least_visible": "#32302f",
		"bg_less_visible":  "#3c3836",
		"bg_most_visible":  "#504945",
		"separator":        "#665c54",
		"success":          "#98971a",
		"destructive":      "#fb4934",
	}, map[string]string{"keyword": "#fe8019", "text": "#ebdbb2"})
	require.NoError(t, err)
	return s
}

// drawn renders d into a screen of the given size.
func drawn(d Dialog, w, h int) string {
	scr := uv.NewScreenBuffer(w, h)
	d.Draw(scr, image.Rect(0, 0, w, h))
	return scr.Render()
}

// requireRestyled checks that a dialog built under the default theme,
// drawn, then restyled through the overlay after the theme was swapped in
// place, draws exactly like one built under the new theme.
func requireRestyled(t *testing.T, build func(*common.Common) Dialog) {
	t.Helper()
	const w, h = 100, 40

	old := styles.CharmtonePantera()
	com := &common.Common{Styles: &old}
	d := build(com)
	before := drawn(d, w, h)

	// Swap the theme the way the UI does: in place, then drop the shared
	// caches keyed by the styles pointer.
	*com.Styles = restyleTheme(t)
	common.InvalidateStyleCaches()
	overlay := NewOverlay(d)
	overlay.RefreshStyles()
	after := drawn(d, w, h)

	fresh := restyleTheme(t)
	want := drawn(build(&common.Common{Styles: &fresh, Workspace: com.Workspace}), w, h)
	require.NotEqual(t, before, want, "the themes must look different")
	require.Equal(t, want, after)
}

func TestRefreshStyles_PermissionsDiff(t *testing.T) {
	t.Parallel()
	requireRestyled(t, func(com *common.Common) Dialog {
		return NewPermissions(com, permission.PermissionRequest{
			ID:         "perm",
			ToolCallID: "call",
			ToolName:   tools.EditToolName,
			Params: tools.EditPermissionsParams{
				FilePath:   "main.go",
				OldContent: "package main\n\nfunc main() {}\n",
				NewContent: "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
			},
		})
	})
}

func TestRefreshStyles_Sessions(t *testing.T) {
	t.Parallel()
	ws := &sessionMouseWorkspace{sessions: []session.Session{
		{ID: "s1", Title: "First session"},
		{ID: "s2", Title: "Second session"},
	}}
	requireRestyled(t, func(com *common.Common) Dialog {
		com.Workspace = ws
		d, err := NewSessions(com, "s1")
		require.NoError(t, err)
		return d
	})
}

// TestRefreshStyles_QuestionForm checks an inline question form restyles
// its text areas.
func TestRefreshStyles_QuestionForm(t *testing.T) {
	t.Parallel()
	old := styles.CharmtonePantera()
	form := NewQuestionForm(&old, question.Request{})
	single := NewSingleChoice(&old, question.Question{})
	form.questions = append(form.questions, single, NewFreeText(&old, question.Question{}))

	fresh := restyleTheme(t)
	old = fresh
	form.RefreshStyles()

	want := questionTextareaStyles(&fresh)
	for name, got := range map[string]textarea.Styles{
		"fill-in":   single.fillIn.Styles(),
		"note":      single.noteEditor.Styles(),
		"free text": form.questions[1].(*FreeText).editor.Styles(),
	} {
		require.Equal(t, want.Cursor.Color, got.Cursor.Color, name)
		require.Equal(t, want.Focused.Text.GetForeground(), got.Focused.Text.GetForeground(), name)
		require.Equal(t, want.Focused.Placeholder.GetForeground(), got.Focused.Placeholder.GetForeground(), name)
	}
	require.NotEqual(t, questionTextareaStyles(new(styles.CharmtonePantera())).Focused.Text.GetForeground(),
		want.Focused.Text.GetForeground(), "the themes must look different")
}
