package model

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// newHostThemeTestUI builds a UI with every component a theme switch
// restyles, over a stub workspace serving cfg.
func newHostThemeTestUI(t *testing.T, cfg *config.Config) *UI {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}
	}
	com := common.DefaultCommon(&themeTestWorkspace{testWorkspace: testWorkspace{cfg: cfg}})
	keyMap := DefaultKeyMap()
	u := &UI{
		com:      com,
		keyMap:   keyMap,
		dialog:   dialog.NewOverlay(),
		header:   newHeader(com),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		completions: completions.New(
			com.Styles.Completions.Normal,
			com.Styles.Completions.Focused,
			com.Styles.Completions.Match,
		),
		attachments: attachments.New(attachments.NewRenderer(com.Styles.Attachments), attachments.Keymap{
			DeleteMode: keyMap.Editor.AttachmentDeleteMode,
			DeleteAll:  keyMap.Editor.DeleteAllAttachments,
			Escape:     keyMap.Editor.Escape,
		}),
		themeKey:      "default",
		isTransparent: cfg.Options.TUI.IsTransparent(),
	}
	u.status = NewStatus(com, u)
	return u
}

// themeTestWorkspace is a [testWorkspace] whose config writes land in its
// config, the way they do once a real workspace reloads.
type themeTestWorkspace struct {
	testWorkspace
	setFields []string
}

func (w *themeTestWorkspace) SetConfigField(_ config.Scope, key string, value any) error {
	w.setFields = append(w.setFields, fmt.Sprintf("%s=%v", key, value))
	switch key {
	case "options.tui.active_theme":
		w.cfg.Options.TUI.ActiveTheme = value.(string)
	case "options.tui.transparent":
		w.cfg.Options.TUI.Transparent = new(value.(bool))
	}
	return nil
}

// actionDialog is a dialog that answers any message with its action, so a
// test can hand the UI a dialog action the way a real dialog would.
type actionDialog struct {
	action dialog.Action
}

func (*actionDialog) ID() string                               { return "test-action" }
func (d *actionDialog) HandleMsg(tea.Msg) dialog.Action        { return d.action }
func (*actionDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

// runDialogAction hands m action as if an open dialog produced it and
// returns the messages the resulting command produces.
func runDialogAction(m *UI, action dialog.Action) []tea.Msg {
	d := &actionDialog{action: action}
	m.dialog.OpenDialog(d)
	defer m.dialog.CloseDialog(d.ID())
	return runCmd(m.handleDialogMsg(struct{}{}))
}

// isolateThemeFiles points the user theme directory at an empty temporary
// one, so tests that save, revert or delete themes never touch the real
// one.
func isolateThemeFiles(t *testing.T) {
	t.Helper()
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
}

// deliverHostTheme hands m a host theme event and, when it asks for styles
// to be built, runs that command and hands the result back, the way the
// program would.
func deliverHostTheme(t *testing.T, m *UI, theme *proto.HostTheme) {
	t.Helper()
	cmd := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: theme})
	if cmd == nil {
		return
	}
	msg, ok := cmd().(hostThemeStylesMsg)
	require.True(t, ok)
	require.Nil(t, m.handleHostThemeStyles(msg))
}

func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// primaryOf reports the active theme's primary color.
func primaryOf(m *UI) string {
	return hexOf(m.com.Styles.WorkingGradFromColor)
}

func hostTheme(primary string) *proto.HostTheme {
	return &proto.HostTheme{
		Name:    "host-" + primary,
		Source:  "test",
		Palette: map[string]string{"primary": primary},
		Syntax:  map[string]string{"keyword": "#123456"},
	}
}

func TestHostTheme_AppliesInPlace(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)
	stylesPtr := m.com.Styles

	deliverHostTheme(t, m, hostTheme("#112233"))

	require.Same(t, stylesPtr, m.com.Styles, "the theme must be swapped in place")
	require.Equal(t, "#112233", primaryOf(m))
	require.Equal(t, "#123456", *m.com.Styles.Markdown.CodeBlock.Chroma.Keyword.Color)
	require.True(t, m.hostTheme.active)
	require.False(t, m.isTransparent, "no override keeps the config's transparency")
}

func TestHostTheme_DedupesReplays(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	deliverHostTheme(t, m, hostTheme("#112233"))
	// A replay of the same theme (a reconnect) asks for nothing.
	require.Nil(t, m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#112233")}))

	// The user picks a theme in Crush; a replay must not undo that.
	m.applyTheme(styles.CharmtonePantera())
	m.userAppliedTheme()
	require.False(t, m.hostTheme.active)
	require.Nil(t, m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#112233")}))
	require.Equal(t, "#6b50ff", primaryOf(m))

	// The host's next change wins again.
	deliverHostTheme(t, m, hostTheme("#445566"))
	require.Equal(t, "#445566", primaryOf(m))
	require.True(t, m.hostTheme.active)

	// A clear first, then the first theme again, is a change.
	deliverHostTheme(t, m, nil)
	deliverHostTheme(t, m, hostTheme("#445566"))
	require.Equal(t, "#445566", primaryOf(m))
}

func TestHostTheme_ClearRevertsToConfiguredTheme(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, &config.Config{Options: &config.Options{
		TUI: &config.TUIOptions{ActiveTheme: "gruvbox-dark"},
	}})
	require.Equal(t, "#fabd2f", primaryOf(m))

	deliverHostTheme(t, m, hostTheme("#112233"))
	require.Equal(t, "#112233", primaryOf(m))

	deliverHostTheme(t, m, nil)
	require.Equal(t, "#fabd2f", primaryOf(m))
	require.False(t, m.hostTheme.active)
}

func TestHostTheme_ClearRevertsToProviderTheme(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	deliverHostTheme(t, m, hostTheme("#112233"))
	deliverHostTheme(t, m, nil)
	require.Equal(t, hexOf(styles.ThemeForKey("default").WorkingGradFromColor), primaryOf(m))
	require.False(t, m.hostTheme.active)
}

func TestHostTheme_ClearKeepsUserTheme(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)
	stylesPtr := m.com.Styles

	deliverHostTheme(t, m, hostTheme("#112233"))
	own := styles.CharmtonePantera()
	own.WorkingGradFromColor = lipgloss.Color("#abcdef")
	m.applyTheme(own)
	m.userAppliedTheme()

	// The host clears its theme while the user's own theme is on screen:
	// nothing is rebuilt and the user's theme stays.
	require.Nil(t, m.handleHostThemeEvent(workspace.HostThemeEvent{}))
	require.Nil(t, m.hostTheme.last)
	require.Same(t, stylesPtr, m.com.Styles)
	require.Equal(t, "#abcdef", primaryOf(m))
}

func TestHostTheme_ProviderSwitchDoesNotOverride(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	deliverHostTheme(t, m, hostTheme("#112233"))
	m.applyThemeForProvider("hyper")
	require.Equal(t, "#112233", primaryOf(m), "a provider switch must not replace the host theme")
	require.Equal(t, "hyper", m.themeKey, "the provider's theme is still recorded")

	// Once the host clears its theme, the provider's theme applies.
	deliverHostTheme(t, m, nil)
	require.Equal(t, hexOf(styles.ThemeForKey("hyper").WorkingGradFromColor), primaryOf(m))
}

func TestHostTheme_StaleBuildsAreDropped(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	first := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#111111")})
	second := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#222222")})
	require.Nil(t, m.handleHostThemeStyles(second().(hostThemeStylesMsg)))
	require.Nil(t, m.handleHostThemeStyles(first().(hostThemeStylesMsg)))
	require.Equal(t, "#222222", primaryOf(m))

	// A theme the user applies while a host build is in flight wins.
	third := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#333333")})
	m.applyTheme(styles.CharmtonePantera())
	m.userAppliedTheme()
	require.Nil(t, m.handleHostThemeStyles(third().(hostThemeStylesMsg)))
	require.Equal(t, "#6b50ff", primaryOf(m))
}

func TestHostTheme_PreviewRevertsToHostTheme(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	// A theme preview is open: its snapshot is the theme esc restores.
	saved := m.com.Styles.Clone()
	m.preThemeStyles = &saved
	m.previewTheme(styles.ThemeForKey("hyper"))

	deliverHostTheme(t, m, hostTheme("#112233"))
	require.NotEqual(t, "#112233", primaryOf(m), "the preview stays on screen")
	require.NotNil(t, m.preThemeStyles)
	require.Equal(t, "#112233", hexOf(m.preThemeStyles.WorkingGradFromColor))
	require.True(t, m.hostTheme.active)
}

func TestHostTheme_Transparency(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	theme := hostTheme("#112233")
	theme.Transparent = new(true)
	deliverHostTheme(t, m, theme)
	require.True(t, m.isTransparent)

	// A host theme without an override hands it back to the config.
	deliverHostTheme(t, m, hostTheme("#445566"))
	require.False(t, m.isTransparent)

	deliverHostTheme(t, m, theme)
	require.True(t, m.isTransparent)
	deliverHostTheme(t, m, nil)
	require.False(t, m.isTransparent, "a clear restores the config's transparency")

	// The user's own toggle wins over the host's override, and a clear
	// leaves it alone.
	deliverHostTheme(t, m, theme)
	_, _ = m.Update(transparencyToggledMsg{transparent: false})
	require.False(t, m.isTransparent)
	require.False(t, m.hostTheme.transparent)
	m.isTransparent = true // As if the toggle went the other way.
	deliverHostTheme(t, m, nil)
	require.True(t, m.isTransparent)
}

func TestHostTheme_InvalidThemeReportsError(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	cmd := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: &proto.HostTheme{
		Name:    "broken",
		Palette: map[string]string{"primary": "#6b50ffff"},
	}})
	require.NotNil(t, cmd)
	report := m.handleHostThemeStyles(cmd().(hostThemeStylesMsg))
	require.NotNil(t, report)
	info, ok := report().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeError, info.Type)
	require.Contains(t, info.Msg, "broken")
	require.Equal(t, "#6b50ff", primaryOf(m), "the current theme stays")
	require.False(t, m.hostTheme.active)
}

// TestHostTheme_RoutedThroughUpdate checks that Update wires both messages:
// the event asks for styles to be built and the built styles are applied.
func TestHostTheme_RoutedThroughUpdate(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)

	msgs := runCmd(m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#112233")}))
	require.Len(t, msgs, 1)
	_, _ = m.Update(msgs[0])
	require.Equal(t, "#112233", primaryOf(m))

	_, cmd := m.Update(workspace.HostThemeEvent{Theme: hostTheme("#445566")})
	var built []tea.Msg
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(hostThemeStylesMsg); ok {
			built = append(built, msg)
		}
	}
	require.Len(t, built, 1)
	_, _ = m.Update(built[0])
	require.Equal(t, "#445566", primaryOf(m))
}

// runCmd runs cmd and returns the messages it produces, flattening
// batches.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, runCmd(c)...)
	}
	return out
}

// TestHostTheme_UserThemeActionsWin drives each theme action that applies a
// theme inside Crush while a host theme with a transparency override is on
// screen, a theme preview is open and another host theme is still being
// built. The user's theme wins: the host theme goes inactive, transparency
// follows the config again, the host build in flight is dropped, and esc no
// longer brings back the styles from before the action.
func TestHostTheme_UserThemeActionsWin(t *testing.T) {
	writeTheme := func(t *testing.T, name, primary string) {
		t.Helper()
		path, err := styles.ThemePath(name)
		require.NoError(t, err)
		require.NoError(t, styles.SaveThemeFile(path, &styles.ThemeFile{
			Base:    "charmtone-panther",
			Palette: styles.Palette{Primary: primary},
		}))
	}

	for _, tc := range []struct {
		name   string
		active string
		setup  func(t *testing.T)
		action dialog.Action
		want   string
	}{
		{
			name:   "switch",
			action: dialog.ActionSwitchTheme{Theme: "gruvbox-dark"},
			want:   "#fabd2f",
		},
		{
			name:   "save",
			action: dialog.ActionSaveThemePalette{Base: "charmtone-panther", Palette: styles.Palette{Primary: "#abcdef"}},
			want:   "#abcdef",
		},
		{
			name:   "revert",
			active: "gruvbox-dark",
			setup:  func(t *testing.T) { writeTheme(t, "gruvbox-dark", "#abcdef") },
			action: dialog.ActionRevertOverriddenTheme{Name: "gruvbox-dark"},
			want:   "#fabd2f",
		},
		{
			name:   "delete",
			active: "mine",
			setup:  func(t *testing.T) { writeTheme(t, "mine", "#abcdef") },
			action: dialog.ActionDeleteTheme{Name: "mine"},
			want:   "#6b50ff",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateThemeFiles(t)
			if tc.setup != nil {
				tc.setup(t)
			}
			m := newHostThemeTestUI(t, &config.Config{Options: &config.Options{
				TUI: &config.TUIOptions{ActiveTheme: tc.active},
			}})

			theme := hostTheme("#112233")
			theme.Transparent = new(true)
			deliverHostTheme(t, m, theme)
			require.True(t, m.isTransparent)
			runDialogAction(m, dialog.ActionPreviewTheme{Theme: "keanu"})
			require.NotNil(t, m.preThemeStyles)
			inFlight := m.handleHostThemeEvent(workspace.HostThemeEvent{Theme: hostTheme("#445566")})
			require.NotNil(t, inFlight)

			runDialogAction(m, tc.action)
			require.Equal(t, tc.want, primaryOf(m))
			require.False(t, m.hostTheme.active)
			require.False(t, m.isTransparent, "the config's transparency applies again")
			require.Nil(t, m.preThemeStyles)

			require.Nil(t, m.handleHostThemeStyles(inFlight().(hostThemeStylesMsg)))
			require.Equal(t, tc.want, primaryOf(m), "the host build in flight must be dropped")
			runDialogAction(m, dialog.ActionRevertThemePreview{})
			require.Equal(t, tc.want, primaryOf(m), "esc must not bring back the styles from before the action")
		})
	}
}

// TestHostTheme_TransparencyToggle checks the toggle flips what is on
// screen, which the host overrode, and that only Update changes it.
func TestHostTheme_TransparencyToggle(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)
	ws := m.com.Workspace.(*themeTestWorkspace)

	theme := hostTheme("#112233")
	theme.Transparent = new(true)
	deliverHostTheme(t, m, theme)
	require.True(t, m.isTransparent)
	// The palette offers what the toggle does, although the config still
	// has the background color on.
	require.Equal(t, "Enable Background Color", transparencyLabel(t, m))

	msgs := runDialogAction(m, dialog.ActionToggleTransparentBackground{})
	require.Equal(t, []tea.Msg{transparencyToggledMsg{transparent: false}}, msgs)
	require.Equal(t, []string{"options.tui.transparent=false"}, ws.setFields)
	require.True(t, m.isTransparent, "the command must leave the UI state to Update")
	require.True(t, m.hostTheme.transparent)

	_, _ = m.Update(msgs[0])
	require.False(t, m.isTransparent)
	require.False(t, m.hostTheme.transparent)
	require.True(t, m.hostTheme.active, "the host's colors stay")
	require.Equal(t, "Disable Background Color", transparencyLabel(t, m))
}

// transparencyLabel opens the command palette, filters it down to the
// background color toggle and returns the label it shows.
func transparencyLabel(t *testing.T, m *UI) string {
	t.Helper()
	_ = m.openCommandsDialog()
	defer m.dialog.CloseDialog(dialog.CommandsID)
	d := m.dialog.Dialog(dialog.CommandsID)
	require.NotNil(t, d)
	for _, r := range "background color" {
		d.HandleMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	const w, h = 100, 30
	scr := uv.NewScreenBuffer(w, h)
	d.Draw(scr, image.Rect(0, 0, w, h))
	out := ansi.Strip(scr.Render())
	for _, label := range []string{"Enable Background Color", "Disable Background Color"} {
		if strings.Contains(out, label) {
			return label
		}
	}
	require.Failf(t, "no background color toggle in the palette", "%s", out)
	return ""
}

// restyleProbe is a dialog and an inline editor that counts how often it
// was asked to pick up the current styles.
type restyleProbe struct {
	actionDialog
	refreshed int
}

func (p *restyleProbe) RefreshStyles()                          { p.refreshed++ }
func (*restyleProbe) HandleKey(tea.KeyPressMsg) (bool, tea.Cmd) { return false, nil }
func (*restyleProbe) ShortHelp() []key.Binding                  { return nil }
func (*restyleProbe) Height(int) int                            { return 1 }
func (*restyleProbe) HeightChanged() bool                       { return false }
func (*restyleProbe) SetFocused(bool)                           {}
func (*restyleProbe) Draw(uv.Screen, uv.Rectangle) *tea.Cursor  { return nil }

// TestHostTheme_RestylesOpenDialogs checks a host theme reaches the open
// dialogs and the inline editor, which copy styles when they are built.
func TestHostTheme_RestylesOpenDialogs(t *testing.T) {
	t.Parallel()
	m := newHostThemeTestUI(t, nil)
	open, inline := &restyleProbe{}, &restyleProbe{}
	m.dialog.OpenDialog(open)
	m.activeInline = inline

	deliverHostTheme(t, m, hostTheme("#112233"))
	require.Equal(t, 1, open.refreshed)
	require.Equal(t, 1, inline.refreshed)
}
