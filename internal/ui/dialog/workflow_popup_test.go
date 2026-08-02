package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestWorkflowPopup(t *testing.T) *WorkflowPopup {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return NewWorkflowPopup(com, "call_123")
}

func TestWorkflowPopup_ID(t *testing.T) {
	wp := newTestWorkflowPopup(t)
	require.Equal(t, WorkflowPopupID, wp.ID())
	require.Equal(t, "call_123", wp.ToolCallID())
}

func TestWorkflowPopup_ProgressTracking(t *testing.T) {
	wp := newTestWorkflowPopup(t)

	// Send agent_start
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_start",
		Index:      0,
		Label:      "Check Imports",
		Running:    1,
		Completed:  0,
		Total:      2,
	})

	require.Equal(t, 1, wp.running)
	require.Equal(t, 0, wp.completed)
	require.Equal(t, 2, wp.total)
	require.Len(t, wp.agents, 2)
	require.Equal(t, "running", wp.agents[0].Status)
	require.Equal(t, "Check Imports", wp.agents[0].Label)
	require.Equal(t, "queued", wp.agents[1].Status)

	// Send log for agent 0
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "log",
		Index:      0,
		Message:    "Scanning files...",
		Running:    1,
		Completed:  0,
		Total:      2,
	})
	require.Equal(t, "Scanning files...", wp.agents[0].LastLog)
	require.Contains(t, wp.agents[0].Logs, "Scanning files...")

	// Send agent_done for agent 0
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_done",
		Index:      0,
		Label:      "Check Imports",
		Running:    0,
		Completed:  1,
		Total:      2,
	})
	require.Equal(t, "done", wp.agents[0].Status)
	require.Equal(t, 1, wp.completed)

	// Send agent_error for agent 1
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_error",
		Index:      1,
		Label:      "Lint Code",
		Message:    "syntax error in line 4",
		Running:    0,
		Completed:  2,
		Total:      2,
	})
	require.Equal(t, "error", wp.agents[1].Status)
	require.Equal(t, "syntax error in line 4", wp.agents[1].Error)
	require.Equal(t, 2, wp.completed)
}

func TestWorkflowPopup_KeyHandling(t *testing.T) {
	wp := newTestWorkflowPopup(t)
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_start",
		Index:      0,
		Label:      "Agent A",
		Running:    2,
		Completed:  0,
		Total:      2,
	})
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_start",
		Index:      1,
		Label:      "Agent B",
		Running:    2,
		Completed:  0,
		Total:      2,
	})

	require.Equal(t, 0, wp.selectedIndex)

	// Move down with 'j'
	wp.HandleMsg(tea.KeyPressMsg{Code: 'j', Text: "j"})
	require.Equal(t, 1, wp.selectedIndex)

	// Move up with 'k'
	wp.HandleMsg(tea.KeyPressMsg{Code: 'k', Text: "k"})
	require.Equal(t, 0, wp.selectedIndex)

	// Close with 'esc'
	action := wp.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.IsType(t, ActionClose{}, action)
	require.True(t, wp.IsDismissed())
}

func TestWorkflowPopup_Draw(t *testing.T) {
	wp := newTestWorkflowPopup(t)
	wp.SetDescription("Optimize DB queries")
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Kind:       "agent_start",
		Index:      0,
		Label:      "Query Optimizer",
		Running:    1,
		Completed:  0,
		Total:      1,
	})

	scr := uv.NewScreenBuffer(80, 24)
	cur := wp.Draw(scr, scr.Bounds())
	require.Nil(t, cur)
}
