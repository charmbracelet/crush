package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

// MemoryID is the identifier for the memory inspect dialog.
const MemoryID = "memory"

type memoryMode uint8

const (
	memoryModeNormal memoryMode = iota
	memoryModeDeleting
)

// MemoryEntry holds one memory file's metadata for the dialog list.
type MemoryEntry struct {
	slug        string
	description string
	size        int64
	path        string
}

// Memory is a dialog that shows all saved memories and allows
// deleting or opening them.
type Memory struct {
	com        *common.Common
	help       help.Model
	list       *list.FilterableList
	input      textinput.Model
	entries    []MemoryEntry
	memoryMode memoryMode
	memoryDir  string

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		UpDown        key.Binding
		Delete        key.Binding
		ConfirmDelete key.Binding
		CancelDelete  key.Binding
		Close         key.Binding
	}
}

var _ Dialog = (*Memory)(nil)

// NewMemory creates a new Memory dialog from an already-scanned entry list.
// The caller scans off the Update loop (see ScanMemoryDir) because reading
// every memory file is disk I/O and Update must never block on it.
func NewMemory(com *common.Common, entries []MemoryEntry) (*Memory, error) {
	m := &Memory{
		com:        com,
		memoryMode: memoryModeNormal,
	}

	dataDir := com.Config().Options.DataDirectory
	m.memoryDir = filepath.Join(dataDir, "memory")
	m.entries = entries

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	m.help = help

	m.list = list.NewFilterableList(memoryItems(com.Styles, memoryModeNormal, entries...)...)
	m.list.Focus()
	if len(entries) > 0 {
		m.list.SelectFirst()
		m.list.ScrollToTop()
	}

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑↓", "choose"),
	)
	m.keyMap.Delete = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "delete"),
	)
	m.keyMap.ConfirmDelete = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "delete"),
	)
	m.keyMap.CancelDelete = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	m.keyMap.Close = CloseKey

	return m, nil
}

// ShortHelp implements help.KeyMap.
func (m *Memory) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		m.keyMap.Select,
		m.keyMap.Delete,
		m.keyMap.Close,
	}
	if m.memoryMode == memoryModeDeleting {
		bindings = []key.Binding{
			m.keyMap.ConfirmDelete,
			m.keyMap.CancelDelete,
		}
	}
	return bindings
}

// FullHelp implements help.KeyMap.
func (m *Memory) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.keyMap.Select, m.keyMap.Delete},
		{m.keyMap.Previous, m.keyMap.Next},
		{m.keyMap.Close},
	}
}

// ID implements Dialog.
func (m *Memory) ID() string {
	return MemoryID
}

// HandleMsg implements Dialog.
func (m *Memory) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.memoryMode {
		case memoryModeDeleting:
			switch {
			case key.Matches(msg, m.keyMap.ConfirmDelete):
				action := m.confirmDelete()
				m.list.SetItems(memoryItems(m.com.Styles, memoryModeNormal, m.entries...)...)
				m.list.SelectFirst()
				m.list.ScrollToSelected()
				return action
			case key.Matches(msg, m.keyMap.CancelDelete):
				m.memoryMode = memoryModeNormal
				m.list.SetItems(memoryItems(m.com.Styles, memoryModeNormal, m.entries...)...)
			}
		default:
			switch {
			case key.Matches(msg, m.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, m.keyMap.Delete):
				m.memoryMode = memoryModeDeleting
				m.list.SetItems(memoryItems(m.com.Styles, memoryModeDeleting, m.entries...)...)
			case key.Matches(msg, m.keyMap.Previous):
				m.list.Focus()
				if m.list.IsSelectedFirst() {
					m.list.SelectLast()
				} else {
					m.list.SelectPrev()
				}
				m.list.ScrollToSelected()
			case key.Matches(msg, m.keyMap.Next):
				m.list.Focus()
				if m.list.IsSelectedLast() {
					m.list.SelectFirst()
				} else {
					m.list.SelectNext()
				}
				m.list.ScrollToSelected()
			case key.Matches(msg, m.keyMap.Select):
				if item := m.list.SelectedItem(); item != nil {
					if mi, ok := item.(*MemoryItem); ok {
						return mi.action()
					}
				}
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				value := m.input.Value()
				m.list.SetFilter(value)
				m.list.ScrollToTop()
				m.list.SetSelected(0)
				return ActionCmd{cmd}
			}
		}
	}
	return nil
}

// Cursor implements Dialog.
func (m *Memory) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// Draw implements Dialog.
func (m *Memory) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	m.input.SetWidth(dialogInputTextWidth(t, m.input, innerWidth))
	listHeight, listTotalHeight, _ := sizeDialogList(t, m.list, innerWidth, height)

	var cur *tea.Cursor
	rc := NewRenderContext(t, width)
	rc.Title = "Memories"
	switch m.memoryMode {
	case memoryModeDeleting:
		rc.TitleStyle = t.Dialog.Sessions.DeletingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.DeletingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.DeletingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.DeletingView
		rc.AddPart(t.Dialog.Sessions.DeletingMessage.Render("Delete this memory?"))
	default:
		inputView := t.Dialog.InputPrompt.Render(m.input.View())
		cur = m.Cursor()
		rc.AddPart(inputView)
	}

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	view := rc.Render()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *Memory) selectedEntry() *MemoryEntry {
	if item := m.list.SelectedItem(); item != nil {
		if mi, ok := item.(*MemoryItem); ok {
			// Walk entries to find by slug.
			for i := range m.entries {
				if m.entries[i].slug == mi.slug {
					return &m.entries[i]
				}
			}
		}
	}
	return nil
}

func (m *Memory) confirmDelete() Action {
	m.memoryMode = memoryModeNormal
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}

	// Delete through the tool's locked writer. Doing the remove and the
	// index rebuild here would race a concurrent agent-side memory_write,
	// and whichever rename landed last would silently discard the other's
	// index update.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tools.DeleteMemory(ctx, m.com.Config().Options.DataDirectory, entry.slug); err != nil {
		return ActionCmd{Cmd: func() tea.Msg {
			return util.NewErrorMsg(fmt.Errorf("delete memory %s: %w", entry.slug, err))
		}}
	}

	// Remove from entries slice.
	var remaining []MemoryEntry
	for _, e := range m.entries {
		if e.slug != entry.slug {
			remaining = append(remaining, e)
		}
	}
	m.entries = remaining

	return ActionDeleteMemory{Slug: entry.slug}
}

// ScanMemoryDir reads the memory directory and returns entries sorted by
// slug. It returns no error for a missing directory: that is just an empty
// memory store. Call it from a tea.Cmd, never from Update.
func ScanMemoryDir(memoryDir string) ([]MemoryEntry, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var mems []MemoryEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "MEMORY.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		slug := strings.TrimSuffix(name, ".md")
		path := filepath.Join(memoryDir, name)
		fi, err := entry.Info()
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		desc := tools.ExtractMemoryDescription(string(b), slug)
		mems = append(mems, MemoryEntry{
			slug:        slug,
			description: desc,
			size:        fi.Size(),
			path:        path,
		})
	}

	slices.SortFunc(mems, func(a, b MemoryEntry) int {
		return strings.Compare(a.slug, b.slug)
	})
	return mems, nil
}

// MemoryItem implements list.FilterableItem for a memory entry.
type MemoryItem struct {
	*list.Versioned
	slug        string
	description string
	sizeText    string
	t           *styles.Styles
	m           fuzzy.Match
	cache       map[int]string
	focused     bool
	hideInfo    bool
}

var (
	_ list.FilterableItem = (*MemoryItem)(nil)
	_ list.Focusable      = (*MemoryItem)(nil)
	_ list.MatchSettable  = (*MemoryItem)(nil)
	_ infoColumnItem      = (*MemoryItem)(nil)
)

func (mi *MemoryItem) Finished() bool { return true }

// Filter returns searchable text for the fuzzy filter.
func (mi *MemoryItem) Filter() string {
	return mi.slug + " " + mi.description
}

// ID returns the slug as the unique identifier.
func (mi *MemoryItem) ID() string {
	return mi.slug
}

func (mi *MemoryItem) SetFocused(focused bool) {
	if mi.focused != focused {
		mi.focused = focused
		mi.cache = nil
		mi.Bump()
	}
}

func (mi *MemoryItem) SetMatch(m fuzzy.Match) {
	if !sameFuzzyMatch(mi.m, m) {
		mi.cache = nil
		mi.m = m
		mi.Bump()
	}
}

// InfoText returns the file size as secondary info.
func (mi *MemoryItem) InfoText() string {
	return mi.sizeText
}

// SetHideInfo implements infoColumnItem.
func (mi *MemoryItem) SetHideInfo(v bool) {
	if mi.hideInfo == v {
		return
	}
	mi.cache = nil
	mi.hideInfo = v
	mi.Bump()
}

// Render draws the list item line.
func (mi *MemoryItem) Render(width int) string {
	title := mi.slug + ": " + mi.description
	info := mi.sizeText
	if mi.hideInfo {
		info = ""
	}
	styles := ListItemStyles{
		ItemBlurred:     mi.t.Dialog.NormalItem,
		ItemFocused:     mi.t.Dialog.SelectedItem,
		InfoTextBlurred: mi.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: mi.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, title, info, mi.focused, width, mi.cache, &mi.m)
}

// action returns the action to take when this item is selected.
func (mi *MemoryItem) action() Action {
	return ActionOpenMemory{Slug: mi.slug}
}

// memoryItems builds a slice of FilterableItem from memory entries.
func memoryItems(t *styles.Styles, mode memoryMode, entries ...MemoryEntry) []list.FilterableItem {
	items := make([]list.FilterableItem, 0, len(entries))
	for _, e := range entries {
		sizeText := formatSize(e.size)
		items = append(items, &MemoryItem{
			Versioned:   list.NewVersioned(),
			slug:        e.slug,
			description: e.description,
			sizeText:    sizeText,
			t:           t,
		})
	}
	return items
}

func formatSize(n int64) string {
	switch {
	case n > 1024:
		return fmt.Sprintf("%dKB", n/1024)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
