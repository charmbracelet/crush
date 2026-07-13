package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"
)

const (
	MemoryID              = "memory"
	memoryDialogMaxWidth  = 104
	memoryDialogMaxHeight = 30
)

var memoryViewStatuses = []memory.Status{memory.StatusActive, memory.StatusPending, ""}

// Memory presents memory settings and records in one management surface.
type Memory struct {
	com           *common.Common
	snapshot      workspace.MemorySnapshot
	help          help.Model
	list          *list.FilterableList
	statusIndex   int
	confirmForget bool

	keyMap struct {
		Next           key.Binding
		Previous       key.Binding
		NextStatus     key.Binding
		Remember       key.Binding
		Approve        key.Binding
		Pin            key.Binding
		Forget         key.Binding
		ToggleEnabled  key.Binding
		ToggleRecorder key.Binding
		ToggleRecall   key.Binding
		ToggleSession  key.Binding
		Maintain       key.Binding
		Confirm        key.Binding
		Cancel         key.Binding
		Close          key.Binding
	}
}

// MemoryItem is a selectable memory record.
type MemoryItem struct {
	*list.Versioned
	record  memory.Record
	styles  *styles.Styles
	match   fuzzy.Match
	focused bool
}

var (
	_ Dialog   = (*Memory)(nil)
	_ ListItem = (*MemoryItem)(nil)
)

func NewMemory(com *common.Common, snapshot workspace.MemorySnapshot) *Memory {
	m := &Memory{com: com, snapshot: snapshot}
	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()
	m.list = list.NewFilterableList()
	m.list.Focus()

	m.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("down", "next"))
	m.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("up", "previous"))
	m.keyMap.NextStatus = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "view"))
	m.keyMap.Remember = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "remember"))
	m.keyMap.Approve = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve"))
	m.keyMap.Pin = key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pin"))
	m.keyMap.Forget = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "forget"))
	m.keyMap.ToggleEnabled = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "memory"))
	m.keyMap.ToggleRecorder = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "recorder"))
	m.keyMap.ToggleRecall = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "recall"))
	m.keyMap.ToggleSession = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "session"))
	m.keyMap.Maintain = key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "maintain"))
	m.keyMap.Confirm = key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y", "confirm"))
	m.keyMap.Cancel = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n", "cancel"))
	m.keyMap.Close = CloseKey
	m.setItems("")
	return m
}

func (m *Memory) ID() string {
	return MemoryID
}

func (m *Memory) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	if m.confirmForget {
		switch {
		case key.Matches(keyMsg, m.keyMap.Confirm):
			m.confirmForget = false
			if item := m.selectedItem(); item != nil {
				return ActionMemorySetStatus{ID: item.record.ID, Status: memory.StatusDeleted}
			}
		case key.Matches(keyMsg, m.keyMap.Cancel):
			m.confirmForget = false
		}
		return nil
	}

	switch {
	case key.Matches(keyMsg, m.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, m.keyMap.Previous):
		if m.list.IsSelectedFirst() {
			m.list.SelectLast()
		} else {
			m.list.SelectPrev()
		}
		m.list.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.Next):
		if m.list.IsSelectedLast() {
			m.list.SelectFirst()
		} else {
			m.list.SelectNext()
		}
		m.list.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.NextStatus):
		selectedID := ""
		if item := m.selectedItem(); item != nil {
			selectedID = item.record.ID
		}
		m.statusIndex = (m.statusIndex + 1) % len(memoryViewStatuses)
		m.setItems(selectedID)
	case key.Matches(keyMsg, m.keyMap.Remember):
		return ActionOpenMemoryRemember{}
	case key.Matches(keyMsg, m.keyMap.Approve):
		if item := m.selectedItem(); item != nil && item.record.Status == memory.StatusPending {
			return ActionMemorySetStatus{ID: item.record.ID, Status: memory.StatusActive}
		}
	case key.Matches(keyMsg, m.keyMap.Pin):
		if item := m.selectedItem(); item != nil && (item.record.Status == memory.StatusActive || item.record.Status == memory.StatusPending) {
			return ActionMemorySetPinned{ID: item.record.ID, Pinned: !item.record.Pinned}
		}
	case key.Matches(keyMsg, m.keyMap.Forget):
		if m.selectedItem() != nil {
			m.confirmForget = true
		}
	case key.Matches(keyMsg, m.keyMap.ToggleEnabled):
		return ActionMemorySetFeature{Feature: workspace.MemoryFeatureEnabled, Enabled: !m.snapshot.Enabled}
	case key.Matches(keyMsg, m.keyMap.ToggleRecorder):
		return ActionMemorySetFeature{Feature: workspace.MemoryFeatureRecorder, Enabled: !m.snapshot.RecorderEnabled}
	case key.Matches(keyMsg, m.keyMap.ToggleRecall):
		return ActionMemorySetFeature{Feature: workspace.MemoryFeatureRecall, Enabled: !m.snapshot.RecallEnabled}
	case key.Matches(keyMsg, m.keyMap.ToggleSession):
		if m.snapshot.SessionID != "" {
			return ActionMemorySetSessionRecording{
				SessionID: m.snapshot.SessionID,
				Enabled:   m.snapshot.SessionMode != memory.SessionRecordingEnabled,
			}
		}
	case key.Matches(keyMsg, m.keyMap.Maintain):
		return ActionMemoryMaintain{}
	}
	return nil
}

func (m *Memory) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(memoryDialogMaxWidth, area.Dx()-2))
	height := max(0, min(memoryDialogMaxHeight, area.Dy()-2))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())

	summary := m.summary()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() + t.Dialog.View.GetVerticalFrameSize() +
		8
	available := max(3, height-heightOffset)
	listHeight := min(9, max(3, available/2))
	m.list.SetSize(innerWidth, listHeight)
	m.help.SetWidth(innerWidth)
	m.list.ScrollToSelected()

	rc := NewRenderContext(t, width)
	rc.Gap = 1
	rc.Title = "Memory"
	rc.TitleInfo = t.Dialog.ListItem.InfoBlurred.Render(" " + m.featureSummary())
	rc.AddPart(t.Dialog.Arguments.Description.Render(summary))
	if m.confirmForget {
		rc.AddPart(t.Dialog.Sessions.DeletingMessage.Render("Forget this memory?"))
	}
	rc.AddPart(t.Dialog.List.Height(listHeight).Render(m.list.Render()))
	rc.AddPart(m.selectedDetail(innerWidth, max(2, available-listHeight)))
	rc.Help = m.help.View(m)
	DrawCenter(scr, area, rc.Render())
	return nil
}

func (m *Memory) ShortHelp() []key.Binding {
	if m.confirmForget {
		return []key.Binding{m.keyMap.Confirm, m.keyMap.Cancel}
	}
	bindings := []key.Binding{
		m.keyMap.NextStatus,
		m.keyMap.Remember,
		m.keyMap.Approve,
		m.keyMap.Pin,
		m.keyMap.Forget,
		m.keyMap.ToggleEnabled,
		m.keyMap.ToggleRecorder,
		m.keyMap.ToggleRecall,
	}
	if m.snapshot.SessionID != "" {
		bindings = append(bindings, m.keyMap.ToggleSession)
	}
	return append(bindings, m.keyMap.Maintain, m.keyMap.Close)
}

func (m *Memory) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

func (m *Memory) setItems(selectedID string) {
	status := memoryViewStatuses[m.statusIndex]
	items := make([]list.FilterableItem, 0, len(m.snapshot.Records))
	selected := 0
	for _, record := range m.snapshot.Records {
		if status != "" && record.Status != status {
			continue
		}
		if record.ID == selectedID {
			selected = len(items)
		}
		items = append(items, &MemoryItem{
			Versioned: list.NewVersioned(),
			record:    record,
			styles:    m.com.Styles,
		})
	}
	m.list.SetItems(items...)
	m.list.SetSelected(selected)
	m.list.ScrollToSelected()
}

func (m *Memory) selectedItem() *MemoryItem {
	item, _ := m.list.SelectedItem().(*MemoryItem)
	return item
}

func (m *Memory) summary() string {
	if !m.snapshot.Available {
		return "Memory storage is unavailable. Settings can still be changed for the next start."
	}
	view := "active"
	if memoryViewStatuses[m.statusIndex] == memory.StatusPending {
		view = "pending"
	} else if memoryViewStatuses[m.statusIndex] == "" {
		view = "all"
	}
	summary := fmt.Sprintf("View: %s | Active %d | Pending %d | Superseded %d | Rejected %d",
		view, m.snapshot.Stats.Active, m.snapshot.Stats.Pending, m.snapshot.Stats.Superseded, m.snapshot.Stats.Rejected)
	if m.snapshot.SessionID != "" {
		summary += " | Session recording: " + string(m.snapshot.SessionMode)
	}
	return summary
}

func (m *Memory) featureSummary() string {
	return fmt.Sprintf("%s rec:%s recall:%s", onOff(m.snapshot.Enabled), onOff(m.snapshot.RecorderEnabled), onOff(m.snapshot.RecallEnabled))
}

func (m *Memory) selectedDetail(width, maxLines int) string {
	item := m.selectedItem()
	if item == nil {
		return m.com.Styles.Dialog.HelpView.Render("No memories in this view.")
	}
	record := item.record
	header := fmt.Sprintf("%s | %s/%s | confidence %.2f", record.Description, record.Scope, record.Kind, record.Confidence)
	if record.Pinned {
		header += " | pinned"
	}
	body := ansi.Wordwrap(strings.TrimSpace(record.Content), max(1, width), "")
	lines := strings.Split(body, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "...")
	}
	return m.com.Styles.Dialog.Arguments.Description.Render(header) + "\n" +
		m.com.Styles.Dialog.HelpView.Render(strings.Join(lines, "\n"))
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func (m *MemoryItem) Finished() bool {
	return true
}

func (m *MemoryItem) Filter() string {
	return m.record.Name + " " + m.record.Description + " " + string(m.record.Scope) + " " + string(m.record.Kind)
}

func (m *MemoryItem) ID() string {
	return m.record.ID
}

func (m *MemoryItem) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.focused = focused
	m.Bump()
}

func (m *MemoryItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(m.match, match) {
		return
	}
	m.match = match
	m.Bump()
}

func (m *MemoryItem) Render(width int) string {
	info := fmt.Sprintf("%s/%s %s %s", m.record.Scope, m.record.Kind, m.record.Status, humanize.Time(m.record.UpdatedAt))
	if m.record.Pinned {
		info = "pinned " + info
	}
	itemStyles := ListItemStyles{
		ItemBlurred:     m.styles.Dialog.NormalItem,
		ItemFocused:     m.styles.Dialog.SelectedItem,
		InfoTextBlurred: m.styles.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: m.styles.Dialog.ListItem.InfoFocused,
	}
	return renderItem(itemStyles, m.record.Name, info, m.focused, width, nil, &m.match)
}
