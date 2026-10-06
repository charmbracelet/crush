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

// SkillsTogglesID is the identifier for the skills toggles dialog.
const SkillsTogglesID = "skills_toggles"

// SkillToggleItem describes one skill in the toggles dialog.
type SkillToggleItem struct {
	Name string
	// Disabled is the repository-scoped override: when true the skill is
	// hidden from the agent in this repository.
	Disabled bool
	// ConfigDisabled is the skill's disabled state in the config, before
	// any repository-scoped override. The Global scope reads and writes
	// this flag.
	ConfigDisabled bool
	// EnabledOverride is the repository-scoped enabled override: a
	// config-disabled skill the user enabled here. Only the Local scope
	// considers it.
	EnabledOverride bool
}

// localDisabled returns the effective local state: a config-disabled
// skill stays disabled locally unless the repository enabled override
// turned it on.
func (i SkillToggleItem) localDisabled() bool {
	return i.Disabled || (i.ConfigDisabled && !i.EnabledOverride)
}

// ActionToggleSkill is sent when the user toggles a skill. Local toggles
// persist a repository-scoped override; global toggles edit the config's
// options.disabled_skills list.
type ActionToggleSkill struct {
	Name     string
	Disabled bool
	Global   bool
}

// SkillToggles lets the user enable and disable skills, either for the
// current repository (Local, the default) or in the config (Global).
type SkillToggles struct {
	com    *common.Common
	width  int
	items  []SkillToggleItem
	cursor int
	// offset is the first visible row; the window follows the cursor.
	offset int
	scope  MCPToggleScope
	help   help.Model
	keyMap struct {
		Up     key.Binding
		Down   key.Binding
		Toggle key.Binding
		Scope  key.Binding
		Close  key.Binding
	}
}

var _ Dialog = (*SkillToggles)(nil)

// NewSkillsToggles creates a new skills toggles dialog.
func NewSkillsToggles(com *common.Common, items []SkillToggleItem) *SkillToggles {
	t := com.Styles
	m := &SkillToggles{
		com:   com,
		width: 0, // Set dynamically in Draw().
		items: items,
	}

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)
	m.keyMap.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)
	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("enter", " ", "space"),
		key.WithHelp("enter", "toggle"),
	)
	m.keyMap.Scope = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch scope"),
	)
	m.keyMap.Close = CloseKey

	return m
}

// ID implements Dialog.
func (m *SkillToggles) ID() string {
	return SkillsTogglesID
}

// Items returns the current items.
func (m *SkillToggles) Items() []SkillToggleItem {
	return m.items
}

// SetItems replaces the items, keeping the cursor clamped, so an open
// dialog updates after a toggle or an overrides refresh.
func (m *SkillToggles) SetItems(items []SkillToggleItem) {
	m.items = items
	m.cursor = min(m.cursor, max(0, len(items)-1))
}

// Scope returns the selected toggle scope.
func (m *SkillToggles) Scope() MCPToggleScope {
	return m.scope
}

// HandleMsg implements Dialog.
func (m *SkillToggles) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Up):
			m.cursor = max(0, m.cursor-1)
			m.offset = toggleVisibleOffset(m.cursor, m.offset, len(m.items), maxVisibleToggleRows)
		case key.Matches(msg, m.keyMap.Down):
			m.cursor = min(len(m.items)-1, m.cursor+1)
			m.offset = toggleVisibleOffset(m.cursor, m.offset, len(m.items), maxVisibleToggleRows)
		case key.Matches(msg, m.keyMap.Scope):
			if m.scope == MCPToggleScopeLocal {
				m.scope = MCPToggleScopeGlobal
			} else {
				m.scope = MCPToggleScopeLocal
			}
		case key.Matches(msg, m.keyMap.Toggle):
			if m.cursor < 0 || m.cursor >= len(m.items) {
				return nil
			}
			item := m.items[m.cursor]
			currentlyDisabled := item.localDisabled()
			if m.scope == MCPToggleScopeGlobal {
				currentlyDisabled = item.ConfigDisabled
			}
			newState := !currentlyDisabled
			if m.scope == MCPToggleScopeGlobal {
				m.items[m.cursor].ConfigDisabled = newState
			} else {
				m.items[m.cursor].Disabled = newState
				if item.ConfigDisabled && !newState {
					m.items[m.cursor].EnabledOverride = true
				}
			}
			return ActionToggleSkill{
				Name:     item.Name,
				Disabled: newState,
				Global:   m.scope == MCPToggleScopeGlobal,
			}
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *SkillToggles) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	m.width = max(0, min(m.requiredWidth(t), area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	DrawCenter(scr, area, m.dialogContent())
	return nil
}

// requiredWidth returns the width needed to fit the widest row (status
// dot, name, at least one space, and status) on a single line, plus row
// padding and the dialog frame.
func (m *SkillToggles) requiredWidth(t *styles.Styles) int {
	widest := minToggleDialogWidth
	for _, item := range m.items {
		row := 2 /* check column */ + lipgloss.Width(item.Name)
		widest = max(widest, row)
	}
	return widest + 2 /* row padding */ + t.Dialog.View.GetHorizontalFrameSize()
}

func (m *SkillToggles) dialogContent() string {
	t := m.com.Styles
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	rc := NewRenderContext(t, m.width)
	rc.Title = "Toggle Skills"
	rc.TitleInfo = m.scopeRadioView(t)
	rc.AddPart(m.innerContent())
	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)
	return rc.Render()
}

// scopeRadioView renders the Local/Global radio selector, mirroring the
// command palette's System/User switch on the title line.
func (m *SkillToggles) scopeRadioView(t *styles.Styles) string {
	radio := func(s MCPToggleScope) string {
		bullet := t.Radio.Off
		if s == m.scope {
			bullet = t.Radio.On
		}
		return bullet.Render() + t.Radio.Label.Padding(0, 1).Render(s.String())
	}
	return " " + radio(MCPToggleScopeLocal) + " " + radio(MCPToggleScopeGlobal)
}

func (m *SkillToggles) innerContent() string {
	t := m.com.Styles
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	if len(m.items) == 0 {
		return t.Dialog.SecondaryText.
			Width(innerWidth).
			Padding(0, 1).
			Render("No skills configured.")
	}

	// Cap the visible rows so a long skill list cannot make the dialog
	// grow past the screen; the window follows the cursor.
	visible := min(maxVisibleToggleRows, len(m.items))
	first := m.visibleOffset(visible)
	last := min(len(m.items), first+visible)

	// The row style adds Padding(0, 1), so the text area is two columns
	// narrower than the dialog's inner width. When the list scrolls,
	// reserve the scrollbar column so row lines do not wrap and knock
	// the track out of alignment.
	rowWidth := max(0, innerWidth-2)
	if len(m.items) > visible {
		rowWidth = max(0, rowWidth-scrollbarColumnWidth)
	}

	rows := make([]string, 0, visible)
	for i := first; i < last; i++ {
		item := m.items[i]
		// Enabled skills get a green check in a fixed-width column so
		// names align; disabled skills show nothing in it — skills have
		// no connection state worth a status column like MCPs.
		mark := " " // Reserved check column so names align.
		if !m.itemDisabled(item) {
			mark = t.Tool.IconSuccess.Render()
		}

		if i == m.cursor {
			// The full row goes through the selection style in plain
			// text: a styled check inside the content would emit ANSI
			// resets that clear the selection background for the rest of
			// the line. Padding to rowWidth keeps the highlight spanning
			// the row and the scrollbar column at the right edge.
			// Disabled skills keep the empty gutter here too, so a check
			// never appears just because the row happens to be selected.
			glyph := styles.CheckIcon
			if m.itemDisabled(item) {
				glyph = " "
			}
			row := glyph + " " + item.Name +
				strings.Repeat(" ", max(0, rowWidth-2-lipgloss.Width(item.Name)))
			rows = append(rows, t.Dialog.SelectedItem.Render(row))
			continue
		}

		row := mark + " " +
			t.Dialog.NormalItem.UnsetPadding().Render(item.Name) +
			strings.Repeat(" ", max(0, rowWidth-2-lipgloss.Width(item.Name)))
		rows = append(rows, lipgloss.NewStyle().Padding(0, 1).Render(row))
	}

	return joinToggleRows(t, rows, len(m.items), visible, first)
}

// visibleOffset returns the stored first visible row index, clamped to
// the current item count.
func (m *SkillToggles) visibleOffset(visible int) int {
	return max(0, min(m.offset, len(m.items)-visible))
}

// itemDisabled reports whether the item shows as disabled given the
// active scope: Local reads the repository overrides, Global reads the
// config's raw disabled flag.
func (m *SkillToggles) itemDisabled(item SkillToggleItem) bool {
	if m.scope == MCPToggleScopeGlobal {
		return item.ConfigDisabled
	}
	return item.localDisabled()
}

// FullHelp implements help.KeyMap.
func (m *SkillToggles) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp implements help.KeyMap.
func (m *SkillToggles) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Up, m.keyMap.Down, m.keyMap.Toggle, m.keyMap.Scope, m.keyMap.Close}
}
