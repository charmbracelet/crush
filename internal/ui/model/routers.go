package model

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// routersChangedMsg is sent after a router config write lands so open
// router dialogs refresh from the new config.
type routersChangedMsg struct {
	info string
	err  error
}

// openRoutersDialog opens the routers list dialog.
func (m *UI) openRoutersDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.RoutersID) {
		m.dialog.BringToFront(dialog.RoutersID)
		return nil
	}
	m.dialog.OpenDialog(dialog.NewRouters(m.com))
	return nil
}

// openRouterEditor opens the router editor for name, or for a new router
// when name is empty.
func (m *UI) openRouterEditor(name string) tea.Cmd {
	cfg := m.com.Config()
	if cfg == nil {
		return util.ReportError(errors.New("configuration not found"))
	}
	m.dialog.CloseDialog(dialog.RouterEditorID)
	existing := slices.Sorted(maps.Keys(cfg.Routers))
	m.dialog.OpenDialog(dialog.NewRouterEditor(m.com, name, cfg.Routers[name], existing))
	return nil
}

// handleRouterAction handles the router dialog actions. It reports
// whether msg was a router action.
func (m *UI) handleRouterAction(msg dialog.Action) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case dialog.ActionSelectRouter:
		if m.isAgentBusy() {
			return util.ReportWarn("Agent is busy, please wait..."), true
		}
		m.dialog.CloseDialog(dialog.ModelsID)
		m.dialog.CloseDialog(dialog.RoutersID)
		m.dialog.CloseDialog(dialog.CommandsID)
		name := msg.Name
		return m.updateAgentModelCmd(func() tea.Msg {
			if err := m.com.Workspace.SetActiveRouter(config.ScopeGlobal, name); err != nil {
				return util.NewErrorMsg(err)
			}
			if err := m.com.Workspace.UpdateAgentModel(context.TODO()); err != nil {
				return util.NewErrorMsg(err)
			}
			return util.NewInfoMsg(fmt.Sprintf("Router %q enabled", name))
		}), true

	case dialog.ActionEditRouter:
		m.dialog.CloseDialog(dialog.ModelsID)
		return m.openRouterEditor(msg.Name), true

	case dialog.ActionPickRouterModel:
		picker, err := dialog.NewModelPicker(m.com, msg.Target)
		if err != nil {
			return util.ReportError(err), true
		}
		m.dialog.CloseDialog(dialog.ModelPickerID)
		m.dialog.OpenDialog(picker)
		return nil, true

	case dialog.ActionRouterModelPicked:
		m.dialog.CloseDialog(dialog.ModelPickerID)
		if editor, ok := m.dialog.Dialog(dialog.RouterEditorID).(*dialog.RouterEditor); ok {
			editor.SetPickedModel(msg.Target, msg.Model)
		}
		return nil, true

	case dialog.ActionSaveRouter:
		m.dialog.CloseDialog(dialog.RouterEditorID)
		return m.saveRouterCmd(msg), true

	case dialog.ActionDeleteRouter:
		return m.deleteRouterCmd(msg.Name), true
	}
	return nil, false
}

func (m *UI) saveRouterCmd(msg dialog.ActionSaveRouter) tea.Cmd {
	cfg := m.com.Config()
	wasActive := cfg != nil && msg.OrigName != "" && cfg.ActiveRouter == msg.OrigName
	ws := m.com.Workspace
	return func() tea.Msg {
		if err := ws.SetConfigField(config.ScopeGlobal, "routers."+msg.Name, msg.Router); err != nil {
			return routersChangedMsg{err: err}
		}
		if msg.OrigName != "" && msg.OrigName != msg.Name {
			if err := ws.RemoveConfigField(config.ScopeGlobal, "routers."+msg.OrigName); err != nil {
				return routersChangedMsg{err: err}
			}
			if wasActive {
				if err := ws.SetActiveRouter(config.ScopeGlobal, msg.Name); err != nil {
					return routersChangedMsg{err: err}
				}
			}
		}
		if wasActive {
			_ = ws.UpdateAgentModel(context.TODO())
		}
		return routersChangedMsg{info: fmt.Sprintf("Router %q saved", msg.Name)}
	}
}

func (m *UI) deleteRouterCmd(name string) tea.Cmd {
	cfg := m.com.Config()
	wasActive := cfg != nil && cfg.ActiveRouter == name
	ws := m.com.Workspace
	return func() tea.Msg {
		if wasActive {
			if err := ws.SetActiveRouter(config.ScopeGlobal, ""); err != nil {
				return routersChangedMsg{err: err}
			}
		}
		if err := ws.RemoveConfigField(config.ScopeGlobal, "routers."+name); err != nil {
			return routersChangedMsg{err: err}
		}
		if wasActive {
			_ = ws.UpdateAgentModel(context.TODO())
		}
		return routersChangedMsg{info: fmt.Sprintf("Router %q deleted", name)}
	}
}

// applyRoutersChanged refreshes open router dialogs after a config write.
func (m *UI) applyRoutersChanged(msg routersChangedMsg) tea.Cmd {
	if r, ok := m.dialog.Dialog(dialog.RoutersID).(*dialog.Routers); ok {
		r.Refresh()
	}
	cmds := []tea.Cmd{agentModelChangedCmd}
	if msg.err != nil {
		cmds = append(cmds, util.ReportError(msg.err))
	} else if msg.info != "" {
		cmds = append(cmds, util.ReportInfo(msg.info))
	}
	return tea.Batch(cmds...)
}
