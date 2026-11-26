package model

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sync"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	joyeditor "charm.land/superjoy/editor"
)

// PromptInfo describes a prompt line, as the prompt functions receive it.
type PromptInfo struct {
	LineNumber int
	Focused    bool
}

// promptEditor adapts joy's editor model to the prompt surface the UI was
// built around: it grows with its content between TextareaMinHeight and
// TextareaMaxHeight, renders the mode prompt through a gutter, and keeps
// the textarea-style API the rest of the UI calls.
type promptEditor struct {
	joyeditor.Model
	state *promptState
}

// promptState is shared by every copy of a promptEditor: joy's Update
// returns copies, and the gutter closure must see the latest prompt.
type promptState struct {
	placeholder string
	promptFn    func(PromptInfo) string
	focused     bool
}

var joySetupOnce sync.Once

// newPromptEditor creates the prompt editor with joy's file-oriented
// features off and the keys crush handles itself unbound.
func newPromptEditor() promptEditor {
	joySetupOnce.Do(func() {
		// crush owns quit, the clipboard and its help; the prompt has no
		// file to save and no joy config to open.
		_ = joyeditor.Rebind(map[string][]string{
			"select-all":  {"ctrl+shift+a"},
			"copy":        {},
			"paste":       {},
			"cut":         {},
			"quit":        {},
			"save":        {},
			"open-config": {},
			"help":        {},
			"go-to-line":  {},
		})
		// crush paints its own background.
		joyeditor.SetThemeBackground(false)
	})

	m := joyeditor.New("")
	m.HideLineNumber(true)
	m.ShowStatusBar(false)
	m.ShowGutter(true)
	m.SetGutterBasedOnHeight(true)
	m.SetLineWrapping(true)
	m.SetHighlight(false)
	m.SetSpellCheck(false)
	m.SetAutoPairs(false)
	m.DisableCompletions()
	m.SetAutoHover(false, 0)
	m.SetInlineDiagnostics(false)
	m.SetGitIntegration(false)
	m.SetStickyScroll(false)
	m.SetSymbolHighlight(false)
	m.SetSemanticHighlighting(false)
	m.SetInlayHints(false)
	m.SetBreadcrumbs(false)
	m.SetCursorShape(tea.CursorBlock)
	m.SetCursorBlink(true)
	m.SetHeight(TextareaMinHeight)

	p := promptEditor{Model: m, state: &promptState{focused: true}}
	st := p.state
	p.SetGutter(func(ctx joyeditor.LineContext) string {
		if st.promptFn == nil {
			return "> "
		}
		line := ctx.Number
		if ctx.IsWrapped {
			line = 1 // continuation rows show the dots, not the icon
		}
		return st.promptFn(PromptInfo{LineNumber: line, Focused: st.focused})
	})
	return p
}

// ensure initializes a zero-value promptEditor, as a UI built without
// newPromptEditor may hold.
func (p *promptEditor) ensure() {
	if p.state == nil {
		*p = newPromptEditor()
	}
}

// SetPromptFunc sets the function rendering the prompt of each line.
func (p *promptEditor) SetPromptFunc(_ int, fn func(PromptInfo) string) {
	p.ensure()
	p.state.promptFn = fn
}

// SetPlaceholder sets the text shown while the prompt is empty.
func (p *promptEditor) SetPlaceholder(s string) {
	p.ensure()
	p.state.placeholder = s
	p.Model.SetPlaceholder(s)
}

// Placeholder returns the text shown while the prompt is empty.
func (p *promptEditor) Placeholder() string {
	if p.state == nil {
		return ""
	}
	return p.state.placeholder
}

// SetFocus sets the prompt's focus.
func (p *promptEditor) SetFocus(f bool) {
	p.ensure()
	p.Model.SetFocus(f)
	p.state.focused = f
}

// Focus focuses the prompt.
func (p *promptEditor) Focus() tea.Cmd {
	p.ensure()
	p.state.focused = true
	return p.Model.Focus()
}

// SetMode selects the editing mode: "vim", "helix" or "emacs" (default).
func (p *promptEditor) SetMode(mode string) {
	p.ensure()
	switch mode {
	case "vim":
		p.EnableVimMode()
	case "helix":
		p.EnableHelixMode()
	default:
		p.EnableEmacsMode()
	}
}

// Inserting reports whether typed keys insert text: always in emacs mode,
// and in vim's or Helix's insert mode. Enter sends and @ mentions only
// while inserting; the modal modes keep their other keys to themselves.
func (p *promptEditor) Inserting() bool {
	switch {
	case p.IsVimMode():
		mode := p.GetVimMode()
		return mode == joyeditor.VimModeInsert || mode == joyeditor.VimModeReplace
	case p.HelixEnabled():
		mode, _ := p.HelixMode()
		return mode == joyeditor.HelixModeInsert
	default:
		return true
	}
}

// Update processes msg and keeps the height fitted to the content.
func (p *promptEditor) Update(msg tea.Msg) (promptEditor, tea.Cmd) {
	p.ensure()
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		// joy would resize itself to the whole terminal.
		return *p, nil
	}
	nm, cmd := p.Model.Update(msg)
	p.Model = nm.(joyeditor.Model)
	p.state.focused = p.IsFocused()
	p.syncHeight()
	return *p, cmd
}

// Value returns the prompt's text.
func (p *promptEditor) Value() string {
	p.ensure()
	return p.Model.Value()
}

// Height returns the prompt's rendered height.
func (p *promptEditor) Height() int {
	p.ensure()
	return p.Model.Height()
}

// View renders the prompt.
func (p *promptEditor) View() string {
	p.ensure()
	p.state.focused = p.IsFocused()
	p.syncHeight()
	return p.Model.View().Content
}

// Cursor returns the terminal cursor, or nil when joy paints its own block
// (vim's and Helix's normal modes).
func (p *promptEditor) Cursor() *tea.Cursor {
	p.ensure()
	return p.Model.View().Cursor
}

// syncHeight fits the height to the content, within the prompt's bounds.
func (p *promptEditor) syncHeight() {
	lines, _ := p.SoftwrappedDimensions()
	h := min(max(lines, TextareaMinHeight), TextareaMaxHeight)
	if h != p.GetHeight() {
		p.SetHeight(h)
	}
}

func (p *promptEditor) SetWidth(w int) {
	p.ensure()
	p.Model.SetWidth(w)
	p.syncHeight()
}

func (p *promptEditor) SetValue(s string) {
	p.ensure()
	p.Model.SetValue(s)
	p.syncHeight()
}

func (p *promptEditor) Reset() {
	p.ensure()
	p.Model.Reset()
	p.syncHeight()
}

func (p *promptEditor) InsertString(s string) {
	p.ensure()
	p.Model.InsertString(s)
	p.syncHeight()
}

func (p *promptEditor) InsertRune(r rune) {
	p.ensure()
	p.Model.InsertRune(r)
	p.syncHeight()
}

func (p *promptEditor) Blur()         { p.SetFocus(false) }
func (p *promptEditor) Focused() bool { return p.IsFocused() }
func (p *promptEditor) MoveToEnd()    { p.ensure(); p.InputEnd() }
func (p *promptEditor) MoveToBegin()  { p.ensure(); p.InputStart() }
func (p *promptEditor) CursorStart()  { p.ensure(); p.LineStart() }
func (p *promptEditor) Length() int   { return len(p.Value()) }
func (p *promptEditor) LineCount() int {
	p.ensure()
	return p.Editor.GetLineCount()
}
func (p *promptEditor) SelectedText() string { p.ensure(); return p.SelectionText() }
func (p *promptEditor) HasSelection() bool   { return p.SelectedText() != "" }
func (p *promptEditor) ClearSelection()      { p.ensure(); p.Editor.ClearSelection() }
func (p *promptEditor) DeleteSelection()     { p.ensure(); _ = p.Editor.DeleteSelectedText() }
func (p *promptEditor) Word() string         { p.ensure(); return p.Editor.GetWordAtCursor() }

func (p *promptEditor) Line() int {
	p.ensure()
	line, _ := p.Editor.GetCursorPosition()
	return line
}

func (p *promptEditor) Column() int {
	p.ensure()
	_, col := p.Editor.GetCursorPosition()
	return col
}

func (p *promptEditor) SetCursorColumn(col int) {
	p.ensure()
	line, _ := p.Editor.GetCursorPosition()
	_ = p.Editor.SetCursorPosition(line, col)
}

// promptLineInfo carries the cursor's place in its line, as the history
// boundary checks read it.
type promptLineInfo struct {
	ColumnOffset int
	CharOffset   int
	CharWidth    int
}

func (p *promptEditor) LineInfo() promptLineInfo {
	p.ensure()
	line, col := p.Editor.GetCursorPosition()
	width, _ := p.Editor.GetLineLength(line)
	return promptLineInfo{
		ColumnOffset: p.Editor.GetVisualColumn(line, col),
		CharOffset:   col,
		CharWidth:    width,
	}
}

// joyThemeDir is where the prompt's theme, derived from crush's styles, is
// written for joy to load.
var (
	joyThemeDirOnce sync.Once
	joyThemeDir     string
)

// SetStyles restyles the prompt from crush's textarea styles, bridging
// them into a joy theme.
func (p *promptEditor) SetStyles(s textarea.Styles) {
	p.ensure()
	joyThemeDirOnce.Do(func() {
		joyThemeDir, _ = os.MkdirTemp("", "crush-joy-")
	})
	if joyThemeDir == "" {
		return
	}

	styles := map[string]any{}
	if fg := hexColor(s.Focused.Text.GetForeground()); fg != "" {
		styles["text"] = map[string]string{"fg": fg}
	}
	if fg := hexColor(s.Focused.Placeholder.GetForeground()); fg != "" {
		styles["ghost"] = map[string]string{"fg": fg}
	}
	sel := map[string]string{}
	if fg := hexColor(s.Focused.Selection.GetForeground()); fg != "" {
		sel["fg"] = fg
	}
	if bg := hexColor(s.Focused.Selection.GetBackground()); bg != "" {
		sel["bg"] = bg
	}
	if len(sel) > 0 {
		styles["selection"] = sel
	}
	if bg := hexColor(s.Cursor.Color); bg != "" {
		styles["cursor"] = map[string]string{"bg": bg}
	}

	data, err := json.Marshal(map[string]any{
		"name":     "crush",
		"inherits": "charmtone-dark",
		"dark":     true,
		"styles":   styles,
	})
	if err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(joyThemeDir, "crush.json"), data, 0o600); err != nil {
		return
	}
	t, err := joyeditor.LoadTheme("crush", joyThemeDir, true)
	if err != nil {
		return
	}
	joyeditor.SetTheme(t)
	p.ApplyTheme()
	if c := hexColor(s.Cursor.Color); c != "" {
		p.SetCursorColor(c)
	}
}

// hexColor is the "#rrggbb" form of c, empty when it has none.
func hexColor(c color.Color) string {
	if c == nil {
		return ""
	}
	if _, ok := c.(lipgloss.NoColor); ok {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
