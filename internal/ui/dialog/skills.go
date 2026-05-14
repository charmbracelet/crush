package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

const SkillsID = "skills"

type Skills struct {
	com     *common.Common
	input   textinput.Model
	list    *list.FilterableList
	spinner spinner.Model
	loading bool
	help    help.Model
	keyMap  struct {
		Select, UpDown, Next, Previous, Close key.Binding
	}

	allSkills []skills.CatalogEntry
}

var _ Dialog = (*Skills)(nil)

func NewSkills(com *common.Common, entries []skills.CatalogEntry) *Skills {
	s := &Skills{com: com, allSkills: entries}
	s.help = help.New()
	s.help.Styles = com.Styles.DialogHelpStyles()
	s.list = list.NewFilterableList()
	s.list.Focus()
	s.list.SetSelected(0)
	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.Placeholder = "Type to filter"
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Focus()
	s.keyMap.Select = key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "confirm"))
	s.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	s.keyMap.Next = key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next item"))
	s.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous item"))
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	s.keyMap.Close = closeKey
	s.spinner = spinner.New()
	s.spinner.Spinner = spinner.Dot
	s.spinner.Style = com.Styles.Dialog.Spinner
	s.setAllSkillItems()
	return s
}

func (s *Skills) ID() string { return SkillsID }

func (s *Skills) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		if s.loading {
			if key.Matches(msg, s.keyMap.Close) {
				return ActionClose{}
			}
			return nil
		}
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
			} else {
				s.list.SelectPrev()
			}
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
			} else {
				s.list.SelectNext()
			}
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			if selectedItem := s.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*SkillItem); ok && item != nil {
					return item.Action()
				}
			}
		default:
			prevValue := s.input.Value()
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			if newValue := s.input.Value(); newValue != prevValue {
				s.list.SetFilter(newValue)
				s.list.ScrollToTop()
				s.list.SetSelected(0)
			}
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (s *Skills) Cursor() *tea.Cursor { return InputCursor(s.com.Styles, s.input.Cursor()) }

func (s *Skills) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - s.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	s.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	s.list.SetSize(innerWidth, max(0, height-heightOffset))
	s.help.SetWidth(innerWidth)
	rc := NewRenderContext(t, width)
	rc.Title = "Skills"
	rc.TitleInfo = ""
	rc.AddPart(t.Dialog.InputPrompt.Render(s.input.View()))
	rc.AddPart(t.Dialog.List.Height(s.list.Height()).Render(s.list.Render()))
	if s.loading {
		rc.Help = s.spinner.View() + " Loading skills..."
	} else {
		rc.Help = s.help.View(s)
	}
	view := rc.Render()
	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *Skills) ShortHelp() []key.Binding {
	return []key.Binding{s.keyMap.UpDown, s.keyMap.Select, s.keyMap.Close}
}

func (s *Skills) FullHelp() [][]key.Binding {
	return [][]key.Binding{{s.keyMap.Select, s.keyMap.Next, s.keyMap.Previous}, {s.keyMap.Close}}
}

func (s *Skills) SetSkills(entries []skills.CatalogEntry) {
	s.allSkills = entries
	s.setAllSkillItems()
}

func (s *Skills) setAllSkillItems() {
	items := make([]list.FilterableItem, 0, len(s.allSkills))
	for _, entry := range s.allSkills {
		action := ActionAttachSkill{ID: entry.ID, Name: entry.Name}
		items = append(items, NewSkillItem(s.com.Styles, entry, action))
	}
	query := s.input.Value()
	s.list.SetItems(items...)
	s.list.SetFilter(query)
	s.list.ScrollToTop()
	s.list.SetSelected(0)
}

func (s *Skills) StartLoading() tea.Cmd {
	if s.loading {
		return nil
	}
	s.loading = true
	return s.spinner.Tick
}

func (s *Skills) StopLoading() {
	s.loading = false
}
