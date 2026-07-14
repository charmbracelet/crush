package dialog

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// ModesID is the identifier for the agent mode picker dialog.
	ModesID              = "modes"
	modesDialogMaxWidth  = 72
	modesDialogMaxHeight = 12
)

type modeDefinition struct {
	ID    string
	Title string
}

var modeDefinitions = []modeDefinition{
	{ID: config.AgentCoder, Title: "Task"},
	{ID: config.AgentGoal, Title: "Goal"},
	{ID: config.AgentReview, Title: "Review"},
}

var modelTypeOrder = []config.SelectedModelType{
	config.SelectedModelTypeLarge,
	config.SelectedModelTypeSmall,
	config.SelectedModelTypeSummary,
	config.SelectedModelTypeReview,
}

// Modes lets the user activate an agent mode and assign its model slot.
type Modes struct {
	com      *common.Common
	help     help.Model
	modeList *list.FilterableList

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		NextModel     key.Binding
		PreviousModel key.Binding
		Close         key.Binding
	}
}

// ModeItem is one selectable agent mode.
type ModeItem struct {
	*list.Versioned
	com       *common.Common
	mode      modeDefinition
	modelType config.SelectedModelType
	m         fuzzy.Match
	focused   bool
	active    bool
}

var (
	_ Dialog   = (*Modes)(nil)
	_ ListItem = (*ModeItem)(nil)
)

// NewModes creates the agent mode picker dialog.
func NewModes(com *common.Common) *Modes {
	m := &Modes{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	m.help = h

	m.modeList = list.NewFilterableList()
	m.modeList.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "apply"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("down", "next"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("up", "previous"),
	)
	m.keyMap.PreviousModel = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("left/right", "model slot"),
	)
	m.keyMap.NextModel = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("left/right", "model slot"),
	)
	m.keyMap.Close = CloseKey

	m.setModeItems()
	return m
}

// ID implements Dialog.
func (m *Modes) ID() string {
	return ModesID
}

// HandleMsg implements Dialog.
func (m *Modes) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, m.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, m.keyMap.Previous):
		if m.modeList.IsSelectedFirst() {
			m.modeList.SelectLast()
		} else {
			m.modeList.SelectPrev()
		}
		m.modeList.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.Next):
		if m.modeList.IsSelectedLast() {
			m.modeList.SelectFirst()
		} else {
			m.modeList.SelectNext()
		}
		m.modeList.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.Select):
		item := m.selectedModeItem()
		if item == nil {
			break
		}
		return ActionActivateAgentMode{
			AgentID:   item.mode.ID,
			ModelType: item.modelType,
		}
	case key.Matches(keyMsg, m.keyMap.PreviousModel):
		m.cycleSelectedModel(-1)
	case key.Matches(keyMsg, m.keyMap.NextModel):
		m.cycleSelectedModel(1)
	}
	return nil
}

func (m *Modes) selectedModeItem() *ModeItem {
	item, _ := m.modeList.SelectedItem().(*ModeItem)
	return item
}

func (m *Modes) cycleSelectedModel(direction int) {
	item := m.selectedModeItem()
	if item == nil {
		return
	}
	item.SetModelType(adjacentModelType(item.modelType, direction))
}

func (m *Modes) modelType(agentID string) config.SelectedModelType {
	cfg := m.com.Config()
	if cfg == nil {
		return config.SelectedModelTypeLarge
	}
	agent, ok := cfg.Agents[agentID]
	if !ok {
		return config.SelectedModelTypeLarge
	}
	return agent.Model
}

// Draw implements Dialog.
func (m *Modes) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(modesDialogMaxWidth, area.Dx()))
	height := max(0, min(modesDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() + t.Dialog.View.GetVerticalFrameSize()

	m.modeList.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)
	m.modeList.ScrollToSelected()

	rc := NewRenderContext(t, width)
	rc.Title = "Choose Mode"
	rc.AddPart(t.Dialog.List.Height(m.modeList.Height()).Render(m.modeList.Render()))
	rc.Help = m.help.View(m)

	DrawCenter(scr, area, rc.Render())
	return nil
}

// ShortHelp implements help.KeyMap.
func (m *Modes) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Select, m.keyMap.PreviousModel, m.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (m *Modes) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Select, m.keyMap.Next, m.keyMap.Previous, m.keyMap.PreviousModel, m.keyMap.Close}}
}

func (m *Modes) setModeItems() {
	items := make([]list.FilterableItem, 0, len(modeDefinitions))
	selected := 0
	current := m.com.Workspace.AgentMode()
	for i, mode := range modeDefinitions {
		items = append(items, &ModeItem{
			Versioned: list.NewVersioned(),
			com:       m.com,
			mode:      mode,
			modelType: m.modelType(mode.ID),
			active:    mode.ID == current,
		})
		if mode.ID == current {
			selected = i
		}
	}
	m.modeList.SetItems(items...)
	m.modeList.SetSelected(selected)
	m.modeList.ScrollToSelected()
}

func adjacentModelType(current config.SelectedModelType, direction int) config.SelectedModelType {
	index := 0
	for i, modelType := range modelTypeOrder {
		if modelType == current {
			index = i
			break
		}
	}
	index = (index + direction + len(modelTypeOrder)) % len(modelTypeOrder)
	return modelTypeOrder[index]
}

// Finished implements list.Item. Mode rows read live config and agent state.
func (m *ModeItem) Finished() bool {
	return false
}

// Filter implements ListItem.
func (m *ModeItem) Filter() string {
	return m.mode.Title
}

// ID implements ListItem.
func (m *ModeItem) ID() string {
	return m.mode.ID
}

// SetFocused implements ListItem.
func (m *ModeItem) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.focused = focused
	m.Bump()
}

// SetModelType updates the row's displayed model slot.
func (m *ModeItem) SetModelType(modelType config.SelectedModelType) {
	if m.modelType == modelType {
		return
	}
	m.modelType = modelType
	m.Bump()
}

// SetMatch implements ListItem.
func (m *ModeItem) SetMatch(match fuzzy.Match) {
	m.m = match
	m.Bump()
}

// Render implements ListItem.
func (m *ModeItem) Render(width int) string {
	modelName := "not configured"
	cfg := m.com.Config()
	if cfg != nil {
		if model := cfg.GetModelByType(m.modelType); model != nil {
			modelName = model.Name
			if modelName == "" {
				modelName = model.ID
			}
		}
	}

	title := m.mode.Title
	if m.active {
		title += " (active)"
	}
	info := fmt.Sprintf("Uses %s / %s", modelTypeLabel(m.modelType), modelName)
	itemStyles := ListItemStyles{
		ItemBlurred:     m.com.Styles.Dialog.NormalItem,
		ItemFocused:     m.com.Styles.Dialog.SelectedItem,
		InfoTextBlurred: m.com.Styles.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: m.com.Styles.Dialog.ListItem.InfoFocused,
	}
	return renderItem(itemStyles, title, info, m.focused, width, nil, &m.m)
}

func modelTypeLabel(modelType config.SelectedModelType) string {
	switch modelType {
	case config.SelectedModelTypeSmall:
		return "Small Task"
	case config.SelectedModelTypeSummary:
		return "Summary"
	case config.SelectedModelTypeReview:
		return "Review"
	default:
		return "Large Task"
	}
}
