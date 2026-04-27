package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	SearchEnginesID              = "search_engines"
	searchEnginesDialogMaxWidth  = 58
	searchEnginesDialogMaxHeight = 10
)

type SearchEngines struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Edit     key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

type SearchEngineItem struct {
	engine    config.SearchEngine
	title     string
	info      string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*SearchEngines)(nil)
	_ ListItem = (*SearchEngineItem)(nil)
)

func NewSearchEngines(com *common.Common) (*SearchEngines, error) {
	s := &SearchEngines{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	s.help = help

	s.list = list.NewFilterableList()
	s.list.Focus()

	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.Placeholder = "Type to filter"
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Focus()

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	s.keyMap.Edit = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "edit"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	s.keyMap.Close = CloseKey

	s.setSearchEngineItems()

	return s, nil
}

func (s *SearchEngines) ID() string {
	return SearchEnginesID
}

func (s *SearchEngines) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
				s.list.ScrollToBottom()
				break
			}
			s.list.SelectPrev()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
				s.list.ScrollToTop()
				break
			}
			s.list.SelectNext()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select, s.keyMap.Edit):
			selectedItem := s.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			searchEngineItem, ok := selectedItem.(*SearchEngineItem)
			if !ok {
				break
			}
			isEdit := key.Matches(msg, s.keyMap.Edit)
			if isEdit && !searchEngineItem.canEdit() {
				break
			}
			return ActionSelectSearchEngine{Engine: searchEngineItem.engine, Edit: isEdit}
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			value := s.input.Value()
			s.list.SetFilter(value)
			s.list.ScrollToTop()
			s.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

func (s *SearchEngines) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

func (s *SearchEngines) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(searchEnginesDialogMaxWidth, area.Dx()))
	height := max(0, min(searchEnginesDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	s.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Select Search Engine"
	inputView := t.Dialog.InputPrompt.Render(s.input.View())
	rc.AddPart(inputView)

	visibleCount := len(s.list.FilteredItems())
	if s.list.Height() >= visibleCount {
		s.list.ScrollToTop()
	} else {
		s.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()

	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *SearchEngines) ShortHelp() []key.Binding {
	h := []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
	}
	if s.isSelectedEditable() {
		h = append(h, s.keyMap.Edit)
	}
	h = append(h, s.keyMap.Close)
	return h
}

func (s *SearchEngines) FullHelp() [][]key.Binding {
	return [][]key.Binding{s.ShortHelp()}
}

func (s *SearchEngines) isSelectedEditable() bool {
	selectedItem := s.list.SelectedItem()
	if selectedItem == nil {
		return false
	}
	searchEngineItem, ok := selectedItem.(*SearchEngineItem)
	return ok && searchEngineItem.canEdit()
}

func (s *SearchEngines) setSearchEngineItems() {
	cfg := s.com.Config()
	currentEngine := config.SearchEngineDuckDuckGo
	if cfg != nil {
		currentEngine = cfg.Tools.WebSearch.Engine()
	}

	engines := []struct {
		engine config.SearchEngine
		title  string
		info   string
	}{
		{engine: config.SearchEngineDuckDuckGo, title: "DuckDuckGo", info: "free"},
		{engine: config.SearchEngineKagi, title: "Kagi", info: "API key required"},
	}

	items := make([]list.FilterableItem, 0, len(engines))
	selectedIndex := 0
	for i, engine := range engines {
		item := &SearchEngineItem{
			engine:    engine.engine,
			title:     engine.title,
			info:      engine.info,
			isCurrent: engine.engine == currentEngine,
			t:         s.com.Styles,
		}
		items = append(items, item)
		if engine.engine == currentEngine {
			selectedIndex = i
		}
	}

	s.list.SetItems(items...)
	s.list.SetSelected(selectedIndex)
	s.list.ScrollToSelected()
}

func (s *SearchEngineItem) Filter() string {
	return strings.Join([]string{s.title, s.info, s.engine.String()}, " ")
}

func (s *SearchEngineItem) ID() string {
	return s.engine.String()
}

func (s *SearchEngineItem) canEdit() bool {
	return s.engine == config.SearchEngineKagi
}

func (s *SearchEngineItem) SetFocused(focused bool) {
	if s.focused != focused {
		s.cache = nil
	}
	s.focused = focused
}

func (s *SearchEngineItem) SetMatch(m fuzzy.Match) {
	s.cache = nil
	s.m = m
}

func (s *SearchEngineItem) Render(width int) string {
	info := s.info
	if s.isCurrent {
		info = "current"
	}
	styles := ListItemStyles{
		ItemBlurred:     s.t.Dialog.NormalItem,
		ItemFocused:     s.t.Dialog.SelectedItem,
		InfoTextBlurred: s.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: s.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, s.title, info, s.focused, width, s.cache, &s.m)
}
