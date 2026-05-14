// internal/ui/dialog/mcp_toggle.go
package dialog

import (
	"sort"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const MCPToggleID = "mcp_toggle"

type MCPToggle struct {
	com    *common.Common
	help   help.Model
	list   *list.FilterableList
	keyMap struct {
		Toggle,
		ToggleAll,
		Next,
		Previous,
		Close key.Binding
	}
}

type MCPItem struct {
	name    string
	enabled bool
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

func NewMCPToggle(com *common.Common) *MCPToggle {
	m := &MCPToggle{com: com}
	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("enter", " ", "ctrl+y"),
		key.WithHelp("enter/space", "toggle"),
	)
	m.keyMap.ToggleAll = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle all"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "next"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "prev"),
	)
	m.keyMap.Close = CloseKey

	m.RefreshItems()
	return m
}

func (m *MCPToggle) ID() string { return MCPToggleID }

func (m *MCPToggle) RefreshItems() {
	cfg := m.com.Config()
	var items []list.FilterableItem

	// Sort keys for stable display
	keys := make([]string, 0, len(cfg.MCP))
	for name := range cfg.MCP {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	for _, name := range keys {
		c := cfg.MCP[name]
		items = append(items, &MCPItem{
			name:    name,
			enabled: !c.Disabled,
			t:       m.com.Styles,
		})
	}
	m.list.SetItems(items...)
}

func (m *MCPToggle) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Next):
			m.list.SelectNext()
		case key.Matches(msg, m.keyMap.Previous):
			m.list.SelectPrev()
		case key.Matches(msg, m.keyMap.Toggle):
			if item, ok := m.list.SelectedItem().(*MCPItem); ok {
				return ActionToggleMCPServer{Name: item.name, Enabled: !item.enabled}
			}
		case key.Matches(msg, m.keyMap.ToggleAll):
			anyEnabled := false
			for _, item := range m.list.FilteredItems() {
				if i, ok := item.(*MCPItem); ok && i.enabled {
					anyEnabled = true
					break
				}
			}
			return ActionToggleAllMCPServers{Enable: !anyEnabled}
		}
	}
	return nil
}

func (m *MCPToggle) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := min(60, area.Dx()-4)
	height := min(15, area.Dy()-4)

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	m.list.SetSize(innerWidth, height-6)

	rc := NewRenderContext(t, width)
	rc.Title = "Toggle MCP Servers"
	rc.AddPart(t.Dialog.List.Render(m.list.Render()))
	rc.Help = m.help.View(m)

	DrawCenter(scr, area, rc.Render())
	return nil
}

func (m *MCPToggle) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Toggle, m.keyMap.ToggleAll, m.keyMap.Close}
}

func (m *MCPToggle) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Next, m.keyMap.Previous, m.keyMap.Toggle, m.keyMap.ToggleAll, m.keyMap.Close}}
}

// MCPItem implementation
func (i *MCPItem) ID() string             { return i.name }
func (i *MCPItem) Filter() string         { return i.name }
func (i *MCPItem) SetFocused(f bool)      { i.focused = f; i.cache = nil }
func (i *MCPItem) SetMatch(m fuzzy.Match) { i.m = m; i.cache = nil }
func (i *MCPItem) Render(width int) string {
	status := i.t.Resource.StatusText.Render("Disabled")
	if i.enabled {
		status = i.t.Tool.IconSuccess.Render("Enabled")
	}

	styles := ListItemStyles{
		ItemBlurred: i.t.Dialog.NormalItem,
		ItemFocused: i.t.Dialog.SelectedItem,
	}
	return renderItem(styles, i.name, status, i.focused, width, i.cache, &i.m)
}
