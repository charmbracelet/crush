package dialog

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

// AgentsID is the identifier for the agents dialog.
const AgentsID = "agents"

// Agents represents a dialog that shows available subagents.
type Agents struct {
	com    *common.Common
	keyMap struct {
		Select,
		UpDown,
		Next,
		Previous,
		Close key.Binding
	}

	help  help.Model
	input textinput.Model
	list  *list.FilterableList

	agents []*subagent.Subagent

	// showingDetail indicates if we're showing agent details.
	showingDetail bool
	// selectedAgent is the agent whose details are being shown.
	selectedAgent *subagent.Subagent
}

var _ Dialog = (*Agents)(nil)

// NewAgents creates a new agents dialog.
func NewAgents(com *common.Common) (*Agents, error) {
	a := &Agents{
		com: com,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	a.help = help

	a.list = list.NewFilterableList()
	a.list.Focus()
	a.list.SetSelected(0)

	a.input = textinput.New()
	a.input.SetVirtualCursor(false)
	a.input.Placeholder = "Type to filter"
	a.input.SetStyles(com.Styles.TextInput)
	a.input.Focus()

	a.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "select"),
	)
	a.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	a.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next item"),
	)
	a.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "back/cancel")
	a.keyMap.Close = closeKey

	// Discover agents.
	if err := a.discoverAgents(); err != nil {
		return nil, err
	}

	return a, nil
}

// discoverAgents finds all available subagents.
func (a *Agents) discoverAgents() error {
	homeDir, _ := os.UserHomeDir()
	workingDir, _ := os.Getwd()

	paths := subagent.DefaultDiscoveryPaths(homeDir, workingDir)
	agents, err := subagent.Discover(paths)
	if err != nil {
		return err
	}

	a.agents = agents
	a.setAgentItems()
	return nil
}

// setAgentItems sets the list items from discovered agents.
func (a *Agents) setAgentItems() {
	items := make([]list.FilterableItem, len(a.agents))
	for i, agent := range a.agents {
		items[i] = &AgentItem{
			Agent: agent,
			t:     a.com.Styles,
		}
	}
	a.list.SetItems(items...)
	a.list.SetFilter("")
	a.list.ScrollToTop()
	a.list.SetSelected(0)
	a.input.SetValue("")
}

// ID implements Dialog.
func (a *Agents) ID() string {
	return AgentsID
}

// HandleMsg implements [Dialog].
func (a *Agents) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keyMap.Close):
			if a.showingDetail {
				a.showingDetail = false
				a.selectedAgent = nil
				return nil
			}
			return ActionClose{}
		case key.Matches(msg, a.keyMap.Previous):
			if a.showingDetail {
				return nil
			}
			a.list.Focus()
			if a.list.IsSelectedFirst() {
				a.list.SelectLast()
				a.list.ScrollToBottom()
				break
			}
			a.list.SelectPrev()
			a.list.ScrollToSelected()
		case key.Matches(msg, a.keyMap.Next):
			if a.showingDetail {
				return nil
			}
			a.list.Focus()
			if a.list.IsSelectedLast() {
				a.list.SelectFirst()
				a.list.ScrollToTop()
				break
			}
			a.list.SelectNext()
			a.list.ScrollToSelected()
		case key.Matches(msg, a.keyMap.Select):
			if a.showingDetail {
				return nil
			}
			if selectedItem := a.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*AgentItem); ok && item != nil {
					a.selectedAgent = item.Agent
					a.showingDetail = true
				}
			}
		default:
			if a.showingDetail {
				return nil
			}
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			value := a.input.Value()
			a.list.SetFilter(value)
			a.list.ScrollToTop()
			a.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (a *Agents) Cursor() *tea.Cursor {
	if a.showingDetail {
		return nil
	}
	return InputCursor(a.com.Styles, a.input.Cursor())
}

// Draw implements [Dialog].
func (a *Agents) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if a.showingDetail {
		return a.drawDetail(scr, area)
	}
	return a.drawList(scr, area)
}

// drawList draws the agent list view.
func (a *Agents) drawList(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := a.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()))
	height := max(0, min(defaultDialogHeight, area.Dy()))

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	a.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)

	listHeight := min(height-heightOffset, a.list.Len())
	a.list.SetSize(innerWidth, listHeight)
	a.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Agents"

	if len(a.agents) == 0 {
		rc.AddPart(t.Dialog.NormalItem.Render("No agents found"))
		rc.AddPart(t.Subtle.Render("Create agents in ~/.config/crush/agents/"))
	} else {
		inputView := t.Dialog.InputPrompt.Render(a.input.View())
		rc.AddPart(inputView)
		listView := t.Dialog.List.Height(a.list.Height()).Render(a.list.Render())
		rc.AddPart(listView)
	}
	rc.Help = a.help.View(a)

	view := rc.Render()

	cur := a.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// drawDetail draws the agent detail view.
func (a *Agents) drawDetail(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := a.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()))

	rc := NewRenderContext(t, width)
	rc.Title = a.selectedAgent.Name
	rc.Gap = 1

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	// Use TitleAccent for bold labels.
	labelStyle := t.Dialog.TitleAccent

	// Description.
	rc.AddPart(labelStyle.Render("Description"))
	desc := a.selectedAgent.Description
	if len(desc) > innerWidth-4 {
		desc = desc[:innerWidth-7] + "..."
	}
	rc.AddPart(t.Dialog.NormalItem.Render(desc))

	// Path.
	rc.AddPart(labelStyle.Render("Path"))
	path := a.selectedAgent.Path
	// Shorten path if too long.
	if len(path) > innerWidth-4 {
		path = "..." + path[len(path)-innerWidth+7:]
	}
	rc.AddPart(t.Dialog.NormalItem.Render(path))

	// Model.
	rc.AddPart(labelStyle.Render("Model"))
	rc.AddPart(t.Dialog.NormalItem.Render(a.selectedAgent.Model))

	// Tools.
	if len(a.selectedAgent.Tools) > 0 {
		rc.AddPart(labelStyle.Render("Tools"))
		tools := strings.Join(a.selectedAgent.Tools, ", ")
		if len(tools) > innerWidth-4 {
			tools = tools[:innerWidth-7] + "..."
		}
		rc.AddPart(t.Dialog.NormalItem.Render(tools))
	}

	// Allowed Tools.
	if len(a.selectedAgent.AllowedTools) > 0 {
		rc.AddPart(labelStyle.Render("Allowed Tools (auto-approved)"))
		allowed := strings.Join(a.selectedAgent.AllowedTools, ", ")
		if len(allowed) > innerWidth-4 {
			allowed = allowed[:innerWidth-7] + "..."
		}
		rc.AddPart(t.Dialog.NormalItem.Render(allowed))
	}

	// Yolo Mode.
	if a.selectedAgent.YoloMode {
		rc.AddPart(labelStyle.Render("Yolo Mode"))
		rc.AddPart(t.Dialog.NormalItem.Render("Enabled (all tools auto-approved)"))
	}

	// Max Steps.
	if a.selectedAgent.MaxSteps > 0 {
		rc.AddPart(labelStyle.Render("Max Steps"))
		rc.AddPart(t.Dialog.NormalItem.Render(fmt.Sprintf("%d", a.selectedAgent.MaxSteps)))
	}

	// Note about invocation logs.
	rc.AddPart("")
	rc.AddPart(t.Subtle.Render("Invocation logs are stored as task sessions (ctrl+s)"))

	a.help.SetWidth(innerWidth)
	rc.Help = a.help.View(a)

	view := rc.Render()

	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (a *Agents) ShortHelp() []key.Binding {
	if a.showingDetail {
		return []key.Binding{
			a.keyMap.Close,
		}
	}
	return []key.Binding{
		a.keyMap.UpDown,
		a.keyMap.Select,
		a.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (a *Agents) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{a.keyMap.Select, a.keyMap.Next, a.keyMap.Previous},
		{a.keyMap.Close},
	}
}
