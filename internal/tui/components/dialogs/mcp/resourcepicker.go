package mcp

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ResourcePickerID = "resourcepicker"
	listHeight       = 15
)

type ResourcePickedMsg struct {
	Attachment message.Attachment
}

type ResourcePicker interface {
	dialogs.DialogModel
}

type resourceItem struct {
	clientName string
	resource   *mcp.Resource
}

func (i resourceItem) Title() string {
	return i.resource.URI
}

func (i resourceItem) Description() string {
	return cmp.Or(i.resource.Description, i.resource.Title, i.resource.Name, "(no description)")
}

func (i resourceItem) FilterValue() string {
	return i.Title() + " " + i.Description()
}

type model struct {
	wWidth  int
	wHeight int
	width   int
	list    list.Model
	keyMap  KeyMap
	help    help.Model
	loading bool
}

func NewResourcePickerCmp() ResourcePicker {
	t := styles.CurrentTheme()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(t.Secondary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(t.FgMuted)

	l := list.New([]list.Item{}, delegate, 0, listHeight)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	help := help.New()
	help.Styles = t.S().Help

	return &model{
		list:    l,
		keyMap:  DefaultKeyMap(),
		help:    help,
		loading: true,
	}
}

func (m *model) Init() tea.Cmd {
	return m.loadResources
}

func (m *model) loadResources() tea.Msg {
	resources := agent.GetMCPResources()
	items := make([]list.Item, 0, len(resources))

	for key, resource := range resources {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		clientName := parts[0]
		items = append(items, resourceItem{
			clientName: clientName,
			resource:   resource,
		})
	}

	return resourcesLoadedMsg{items: items}
}

type resourcesLoadedMsg struct {
	items []list.Item
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.width = min(80, m.wWidth-4)
		h := min(listHeight+4, m.wHeight-4)
		m.list.SetSize(m.width-4, h)
		return m, nil

	case resourcesLoadedMsg:
		m.loading = false
		cmd := m.list.SetItems(msg.items)
		if len(msg.items) == 0 {
			return m, tea.Batch(
				cmd,
				util.ReportWarn("No MCP resources available"),
				util.CmdHandler(dialogs.CloseDialogMsg{}),
			)
		}
		return m, cmd

	case tea.KeyPressMsg:
		if key.Matches(msg, m.keyMap.Close) {
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		if key.Matches(msg, m.keyMap.Select) {
			if item, ok := m.list.SelectedItem().(resourceItem); ok {
				return m, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					m.fetchResource(item),
				)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) fetchResource(item resourceItem) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		content, err := agent.GetMCPResourceContent(ctx, item.clientName, item.resource.URI)
		if err != nil {
			return util.ReportError(fmt.Errorf("failed to fetch resource: %w", err))
		}

		var textContent strings.Builder
		for _, c := range content.Contents {
			if c.Text != "" {
				textContent.WriteString(c.Text)
			} else if len(c.Blob) > 0 {
				textContent.WriteString(string(c.Blob))
			}
		}

		fileName := item.resource.Name
		if item.resource.Title != "" {
			fileName = item.resource.Title
		}

		mimeType := item.resource.MIMEType
		if mimeType == "" {
			mimeType = "text/plain"
		}

		attachment := message.Attachment{
			FileName: fileName,
			FilePath: fileName,
			MimeType: mimeType,
			Content:  []byte(textContent.String()),
		}

		return ResourcePickedMsg{Attachment: attachment}
	}
}

func (m *model) View() string {
	t := styles.CurrentTheme()

	if m.loading {
		return m.style().Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Loading MCP Resources...", m.width-4)),
			),
		)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Add MCP Resource", m.width-4)),
		m.list.View(),
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)

	return m.style().Render(content)
}

func (m *model) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *model) Position() (int, int) {
	x := (m.wWidth - m.width) / 2
	y := (m.wHeight - listHeight - 6) / 2
	return y, x
}

func (m *model) ID() dialogs.DialogID {
	return ResourcePickerID
}
