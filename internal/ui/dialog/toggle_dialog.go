package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// toggleRow is one row of a toggles dialog.
type toggleRow struct {
	// mark is the styled glyph left of the name; plainMark is the same glyph unstyled, for the selected row.
	mark, plainMark string
	name, status    string
	statusStyle     lipgloss.Style
}

// toggleSource supplies a toggles dialog's rows and flips one of them.
type toggleSource interface {
	toggleRows(scope MCPToggleScope) []toggleRow
	toggleAt(i int, scope MCPToggleScope) Action
}

// toggleDialog is a scrolling list of on/off rows with a Local/Global scope switch.
type toggleDialog struct {
	toggleWindow
	com      *common.Common
	src      toggleSource
	title    string
	emptyMsg string
	width    int
	scope    MCPToggleScope
	help     help.Model
	keyMap   struct {
		UpDown key.Binding
		Up     key.Binding
		Down   key.Binding
		Toggle key.Binding
		Scope  key.Binding
		Close  key.Binding
	}
}

// init sets up d in place, so src can already reach d's fields.
func (d *toggleDialog) init(com *common.Common, src toggleSource, title, emptyMsg string) {
	*d = toggleDialog{
		toggleWindow: toggleWindow{maxRows: maxVisibleToggleRows},
		com:          com,
		src:          src,
		title:        title,
		emptyMsg:     emptyMsg,
	}
	d.help = help.New()
	d.help.Styles = com.Styles.DialogHelpStyles()
	d.keyMap.UpDown = upDownKey("choose")
	d.keyMap.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)
	d.keyMap.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)
	d.keyMap.Toggle = key.NewBinding(
		key.WithKeys("enter", " ", "space"),
		key.WithHelp("enter", "toggle"),
	)
	d.keyMap.Scope = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch scope"),
	)
	d.keyMap.Close = CloseKey
	d.syncKeys()
}

// syncKeys disables the row keys while there are no rows.
func (d *toggleDialog) syncKeys() {
	hasRows := len(d.src.toggleRows(d.scope)) > 0
	d.keyMap.Toggle.SetEnabled(hasRows)
	d.keyMap.UpDown.SetEnabled(hasRows)
	d.keyMap.Scope.SetEnabled(hasRows)
}

// Scope returns the selected toggle scope.
func (d *toggleDialog) Scope() MCPToggleScope {
	return d.scope
}

// HandleMsg implements Dialog.
func (d *toggleDialog) HandleMsg(msg tea.Msg) Action {
	msgKey, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	count := len(d.src.toggleRows(d.scope))
	switch {
	case key.Matches(msgKey, d.keyMap.Up):
		d.cursorUp(count)
	case key.Matches(msgKey, d.keyMap.Down):
		d.cursorDown(count)
	case key.Matches(msgKey, d.keyMap.Scope):
		if d.scope == MCPToggleScopeLocal {
			d.scope = MCPToggleScopeGlobal
		} else {
			d.scope = MCPToggleScopeLocal
		}
	case key.Matches(msgKey, d.keyMap.Toggle):
		if d.cursor < 0 || d.cursor >= count {
			return nil
		}
		return d.src.toggleAt(d.cursor, d.scope)
	case key.Matches(msgKey, d.keyMap.Close):
		return ActionClose{}
	}
	return nil
}

// Draw implements Dialog.
func (d *toggleDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	rows := d.src.toggleRows(d.scope)
	d.width = max(0, min(d.requiredWidth(t, rows), area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	DrawCenter(scr, area, d.fit(len(rows), area.Dy(), func() string { return d.dialogContent(rows) }))
	return nil
}

// requiredWidth fits the widest row (mark, name, a space and status) on one line, plus row padding and the frame.
func (d *toggleDialog) requiredWidth(t *styles.Styles, rows []toggleRow) int {
	widest := minToggleDialogWidth
	for _, r := range rows {
		w := 2 + lipgloss.Width(r.name)
		if r.status != "" {
			w += 1 + lipgloss.Width(r.status)
		}
		widest = max(widest, w)
	}
	return widest + 2 + t.Dialog.View.GetHorizontalFrameSize()
}

func (d *toggleDialog) dialogContent(rows []toggleRow) string {
	t := d.com.Styles
	rc := NewRenderContext(t, d.width)
	rc.Title = d.title
	rc.TitleInfo = d.scopeRadioView(t)
	rc.AddPart(d.innerContent(rows))
	rc.Help = renderDialogHelp(t, &d.help, d, d.width-t.Dialog.View.GetHorizontalFrameSize())
	return rc.Render()
}

// scopeRadioView renders the Local/Global radio selector for the title line.
func (d *toggleDialog) scopeRadioView(t *styles.Styles) string {
	radio := func(s MCPToggleScope) string {
		bullet := t.Radio.Off
		if s == d.scope {
			bullet = t.Radio.On
		}
		return bullet.Render() + t.Radio.Label.Padding(0, 1).Render(s.String())
	}
	return " " + radio(MCPToggleScopeLocal) + " " + radio(MCPToggleScopeGlobal)
}

// innerContent renders the visible window of rows with a scrollbar, or the empty message.
func (d *toggleDialog) innerContent(rows []toggleRow) string {
	t := d.com.Styles
	innerWidth := d.width - t.Dialog.View.GetHorizontalFrameSize()
	if len(rows) == 0 {
		return toggleEmptyState(t, d.emptyMsg, innerWidth)
	}

	first, visible := d.span(len(rows))
	last := min(len(rows), first+visible)
	// Rows are padded one column each side; reserve the scrollbar column when scrolling.
	rowWidth := max(0, innerWidth-2)
	if len(rows) > visible {
		rowWidth = max(0, rowWidth-scrollbarColumnWidth)
	}

	lines := make([]string, 0, visible)
	for i := first; i < last; i++ {
		r := rows[i]
		name, status := fitToggleRow(r.name, r.status, rowWidth)
		gap := strings.Repeat(" ", max(1, rowWidth-2-lipgloss.Width(name)-lipgloss.Width(status)))
		if i == d.cursor {
			// Plain text: styled parts would reset the selection background mid-row.
			lines = append(lines, t.Dialog.SelectedItem.Render(r.plainMark+" "+name+gap+status))
			continue
		}
		line := r.mark + " " + t.Dialog.NormalItem.UnsetPadding().Render(name) + gap + r.statusStyle.Render(status)
		lines = append(lines, lipgloss.NewStyle().Padding(0, 1).Render(line))
	}
	return joinToggleRows(t, lines, len(rows), visible, first)
}

// FullHelp implements help.KeyMap.
func (d *toggleDialog) FullHelp() [][]key.Binding {
	return [][]key.Binding{d.ShortHelp()}
}

// ShortHelp implements help.KeyMap.
func (d *toggleDialog) ShortHelp() []key.Binding {
	return dialogHelp(d.keyMap.Toggle, d.keyMap.Close, d.keyMap.UpDown, d.keyMap.Scope)
}
