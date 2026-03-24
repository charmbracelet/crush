package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

const SkillsID = "skills"

type Skills struct {
	com      *common.Common
	selected SkillSourceType
	input    textinput.Model
	list     *list.FilterableList
	help     help.Model
	keyMap   struct {
		Select, UpDown, Next, Previous, Tab, ShiftTab, Close key.Binding
	}

	systemSkills []SkillEntry
	userSkills   []SkillEntry
	windowWidth  int
}

var _ Dialog = (*Skills)(nil)

func NewSkills(com *common.Common, systemSkills, userSkills []SkillEntry) *Skills {
	s := &Skills{com: com, systemSkills: systemSkills, userSkills: userSkills, selected: defaultSkillTab(systemSkills, userSkills)}
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
	s.keyMap.Tab = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch selection"))
	s.keyMap.ShiftTab = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "switch selection prev"))
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	s.keyMap.Close = closeKey
	s.setSkillItems(s.selected)
	return s
}

func defaultSkillTab(systemSkills []SkillEntry, _ []SkillEntry) SkillSourceType {
	if len(systemSkills) > 0 {
		return SystemSkills
	}
	return UserSkills
}

func (s *Skills) ID() string { return SkillsID }

func (s *Skills) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
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
		case key.Matches(msg, s.keyMap.Tab):
			if s.hasMultipleTabs() {
				s.selected = s.nextSkillType()
				s.setSkillItems(s.selected)
			}
		case key.Matches(msg, s.keyMap.ShiftTab):
			if s.hasMultipleTabs() {
				s.selected = s.previousSkillType()
				s.setSkillItems(s.selected)
			}
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			value := s.input.Value()
			s.list.SetFilter(value)
			s.list.ScrollToTop()
			s.list.SetSelected(0)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (s *Skills) Cursor() *tea.Cursor { return InputCursor(s.com.Styles, s.input.Cursor()) }

func skillsRadioView(sty *styles.Styles, selected SkillSourceType, hasSystemSkills bool, hasUserSkills bool) string {
	if !hasSystemSkills && !hasUserSkills {
		return ""
	}
	selectedFn := func(t SkillSourceType) string {
		if t == selected {
			return sty.RadioOn.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
		}
		return sty.RadioOff.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
	}
	parts := []string{}
	if hasSystemSkills {
		parts = append(parts, selectedFn(SystemSkills))
	}
	if hasUserSkills {
		parts = append(parts, selectedFn(UserSkills))
	}
	return strings.Join(parts, " ")
}

func (s *Skills) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	if area.Dx() != s.windowWidth {
		s.windowWidth = area.Dx()
	}
	innerWidth := width - s.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	s.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)
	rc := NewRenderContext(t, width)
	rc.Title = "Skills"
	rc.TitleInfo = skillsRadioView(t, s.selected, len(s.systemSkills) > 0, len(s.userSkills) > 0)
	rc.AddPart(t.Dialog.InputPrompt.Render(s.input.View()))
	rc.AddPart(t.Dialog.List.Height(s.list.Height()).Render(s.list.Render()))
	rc.Help = s.help.View(s)
	view := rc.Render()
	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *Skills) ShortHelp() []key.Binding {
	return []key.Binding{s.keyMap.Tab, s.keyMap.UpDown, s.keyMap.Select, s.keyMap.Close}
}

func (s *Skills) FullHelp() [][]key.Binding {
	return [][]key.Binding{{s.keyMap.Select, s.keyMap.Next, s.keyMap.Previous, s.keyMap.Tab}, {s.keyMap.Close}}
}

func (s *Skills) hasMultipleTabs() bool { return len(s.systemSkills) > 0 && len(s.userSkills) > 0 }

func (s *Skills) nextSkillType() SkillSourceType {
	switch s.selected {
	case SystemSkills:
		if len(s.userSkills) > 0 {
			return UserSkills
		}
		return SystemSkills
	case UserSkills:
		if len(s.systemSkills) > 0 {
			return SystemSkills
		}
		return UserSkills
	default:
		return defaultSkillTab(s.systemSkills, s.userSkills)
	}
}

func (s *Skills) previousSkillType() SkillSourceType { return s.nextSkillType() }

func (s *Skills) setSkillItems(skillType SkillSourceType) {
	s.selected = skillType
	items := []list.FilterableItem{}
	for _, entry := range s.entriesFor(skillType) {
		action := ActionAttachSkill{Path: entry.Path, Name: entry.Name}
		items = append(items, NewSkillItem(s.com.Styles, entry, action))
	}
	s.list.SetItems(items...)
	s.list.SetFilter("")
	s.list.ScrollToTop()
	s.list.SetSelected(0)
	s.input.SetValue("")
}

func (s *Skills) entriesFor(skillType SkillSourceType) []SkillEntry {
	switch skillType {
	case SystemSkills:
		return s.systemSkills
	case UserSkills:
		return s.userSkills
	default:
		return nil
	}
}

func (s *Skills) StartLoading() tea.Cmd { return nil }
func (s *Skills) StopLoading()          {}
