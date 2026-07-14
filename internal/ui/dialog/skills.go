package dialog

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

// SkillsID is the identifier for the skills dialog.
const SkillsID = "skills"

// SkillItem wraps a skill catalog entry and its state as a list item.
type SkillItem struct {
	*list.Versioned
	entry    skills.CatalogEntry
	state    *skills.SkillState
	disabled bool
	t        *styles.Styles
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var _ ListItem = &SkillItem{Versioned: list.NewVersioned()}

// NewSkillItem creates a new SkillItem.
func NewSkillItem(t *styles.Styles, entry skills.CatalogEntry, state *skills.SkillState, disabled bool) *SkillItem {
	return &SkillItem{
		Versioned: list.NewVersioned(),
		t:         t,
		entry:     entry,
		state:     state,
		disabled:  disabled,
	}
}

// Finished implements list.Item.
func (s *SkillItem) Finished() bool { return true }

// Filter implements ListItem.
func (s *SkillItem) Filter() string {
	return s.entry.Name + " " + s.entry.Description
}

// ID implements ListItem.
func (s *SkillItem) ID() string {
	return s.entry.ID
}

// SetFocused implements ListItem.
func (s *SkillItem) SetFocused(focused bool) {
	if s.focused == focused {
		return
	}
	s.cache = nil
	s.focused = focused
	if s.Versioned != nil {
		s.Bump()
	}
}

// SetMatch implements ListItem.
func (s *SkillItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(s.m, match) {
		return
	}
	s.cache = nil
	s.m = match
	if s.Versioned != nil {
		s.Bump()
	}
}

// Render implements ListItem.
func (s *SkillItem) Render(width int) string {
	itemStyles := ListItemStyles{
		ItemBlurred:     s.t.Dialog.NormalItem,
		ItemFocused:     s.t.Dialog.SelectedItem,
		InfoTextBlurred: s.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: s.t.Dialog.ListItem.InfoFocused,
	}

	// Build the info text: source, state indicator, user-invocable flag.
	var parts []string
	parts = append(parts, string(s.entry.Source))

	if s.disabled {
		parts = append(parts, "off")
	} else if s.state != nil && s.state.State == skills.StateError {
		parts = append(parts, "error")
	} else {
		parts = append(parts, "on")
	}

	if s.entry.UserInvocable {
		parts = append(parts, "user")
	}

	info := strings.Join(parts, " ")
	return renderItem(itemStyles, s.entry.Name, info, s.focused, width, s.cache, &s.m)
}

// Skills is a dialog that lists discovered skills and provides reload/toggle actions.
type Skills struct {
	com    *common.Common
	help   help.Model
	list   *list.FilterableList
	input  textinput.Model
	ws     workspace.Workspace
	keyMap struct {
		Reload,
		Toggle,
		Next,
		Previous,
		UpDown,
		Close key.Binding
	}
}

var _ Dialog = (*Skills)(nil)

// NewSkills creates a new skills dialog.
func NewSkills(com *common.Common, ws workspace.Workspace) *Skills {
	d := &Skills{
		com: com,
		ws:  ws,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	d.help = help

	d.list = list.NewFilterableList(d.skillItems()...)
	d.list.Focus()
	d.list.SetSelected(0)

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type to filter"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	d.keyMap.Reload = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "reload"),
	)
	d.keyMap.Toggle = key.NewBinding(
		key.WithKeys("enter", "ctrl+d"),
		key.WithHelp("enter", "toggle"),
	)
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "close")
	d.keyMap.Close = closeKey

	return d
}

// skillItems builds the list items from current skill catalog entries and states.
func (d *Skills) skillItems() []list.FilterableItem {
	entries, err := d.ws.ListSkills(context.Background())
	if err != nil || len(entries) == 0 {
		return nil
	}

	states := d.ws.GetSkillStates()
	stateMap := make(map[string]*skills.SkillState, len(states))
	for i := range states {
		stateMap[states[i].Name] = states[i]
	}

	// Get the config's disabled skills list to check which skills are disabled.
	disabledSet := make(map[string]bool)
	if cfg := d.com.Config(); cfg != nil && cfg.Options != nil {
		for _, name := range cfg.Options.DisabledSkills {
			disabledSet[name] = true
		}
	}

	// Also include disabled skills from all skills, not just active catalog.
	// The catalog only returns active skills, so we also need to include
	// disabled ones from the states list.
	// Track which skills are already listed by NAME. CatalogEntry.ID is the
	// skill's file path, not its name, whereas SkillState.Name is the name —
	// so the disabled-skill dedup below must key on name, or every active
	// skill would be re-appended as "off".
	seenNames := make(map[string]bool, len(entries))
	items := make([]list.FilterableItem, 0, len(entries))
	for _, entry := range entries {
		seenNames[entry.Name] = true
		state := stateMap[entry.Name]
		items = append(items, NewSkillItem(d.com.Styles, entry, state, disabledSet[entry.Name]))
	}

	// Add skills that are discovered but disabled (not in the active catalog).
	for _, st := range states {
		if seenNames[st.Name] {
			continue
		}
		entry := skills.CatalogEntry{
			ID:          st.Name,
			Name:        st.Name,
			Description: "",
			Source:      skills.SourceProject,
		}
		items = append(items, NewSkillItem(d.com.Styles, entry, st, true))
	}

	return items
}

// ID implements Dialog.
func (d *Skills) ID() string { return SkillsID }

// HandleMsg implements Dialog.
func (d *Skills) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Previous):
			d.list.Focus()
			if d.list.IsSelectedFirst() {
				d.list.SelectLast()
			} else {
				d.list.SelectPrev()
			}
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Next):
			d.list.Focus()
			if d.list.IsSelectedLast() {
				d.list.SelectFirst()
			} else {
				d.list.SelectNext()
			}
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Reload):
			return ActionSkillsReload{}
		case key.Matches(msg, d.keyMap.Toggle):
			if item := d.selectedSkill(); item != nil {
				return ActionSkillToggle{SkillName: item.entry.Name}
			}
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			value := d.input.Value()
			d.list.SetFilter(value)
			d.list.ScrollToTop()
			d.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// selectedSkill returns the currently selected SkillItem, or nil.
func (d *Skills) selectedSkill() *SkillItem {
	item := d.list.SelectedItem()
	if item == nil {
		return nil
	}
	if si, ok := item.(*SkillItem); ok {
		return si
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (d *Skills) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *Skills) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))

	listHeight := height - heightOffset
	listWidth := max(0, innerWidth-3)
	d.list.SetSize(listWidth, listHeight)
	d.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Skills"
	inputView := t.Dialog.InputPrompt.Render(d.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(d.list.Height()).Render(d.list.Render())
	scrollbar := common.Scrollbar(t, listHeight, d.list.TotalHeight(), listHeight, d.list.Offset())
	if scrollbar != "" {
		listView = lipgloss.JoinHorizontal(lipgloss.Top, listView, scrollbar)
	}
	rc.AddPart(listView)
	rc.Help = d.help.View(d)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *Skills) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.UpDown,
		d.keyMap.Toggle,
		d.keyMap.Reload,
		d.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (d *Skills) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.Toggle, d.keyMap.Reload, d.keyMap.UpDown, d.keyMap.Close},
	}
}

// Refresh reloads the skill items from the workspace. Call after a reload
// or toggle action to update the list.
func (d *Skills) Refresh() {
	d.list.SetItems(d.skillItems()...)
	d.list.SetFilter("")
	d.list.ScrollToTop()
	d.list.SetSelected(0)
	d.input.SetValue("")
}
