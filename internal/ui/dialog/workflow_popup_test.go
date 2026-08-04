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

// TestWorkflowPopup_DropsStaleSeq pins the out-of-order guard: a packet whose
// Seq is not greater than the last applied one must be dropped, so a stale
// agent_start cannot resurrect an agent that already reported done.
func TestWorkflowPopup_DropsStaleSeq(t *testing.T) {
	wp := newTestWorkflowPopup(t)

	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Seq:        1,
		Kind:       "agent_start",
		Index:      0,
		Running:    1,
		Total:      2,
	})
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Seq:        2,
		Kind:       "agent_done",
		Index:      0,
		Running:    0,
		Completed:  1,
		Total:      2,
	})
	require.Equal(t, "done", wp.agents[0].Status)

	// Stale packet: Seq 2 is not greater than the last applied Seq 3.
	// (Seq 1 would be treated as a new stream start, by design.)
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Seq:        3,
		Kind:       "agent_error",
		Index:      1,
		Message:    "stale-now",
		Running:    0,
		Completed:  1,
		Total:      2,
	})
	require.Equal(t, "error", wp.agents[1].Status)
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Seq:        2,
		Kind:       "agent_start",
		Index:      1,
		Message:    "stale",
		Running:    0,
		Completed:  1,
		Total:      2,
	})
	require.Equal(t, "done", wp.agents[0].Status,
		"a stale packet must not resurrect a finished agent")
	require.Equal(t, "error", wp.agents[1].Status,
		"a stale packet must not resurrect a finished agent")
	require.NotEqual(t, "running", wp.agents[1].Status,
		"a stale agent_start must not be applied")
	require.Equal(t, 1, wp.completed, "a stale packet must not rewind the counters")
}

// TestWorkflowPopup_AcceptsRestartedSeq pins the Seq==1 reset: a second
// workflow run reusing the same tool call ID restarts its sequence at 1 and
// must be accepted, not swallowed by the retained popup's lastSeq.
func TestWorkflowPopup_AcceptsRestartedSeq(t *testing.T) {
	wp := newTestWorkflowPopup(t)

	for seq := int64(1); seq <= 4; seq++ {
		wp.HandleProgress(&notify.WorkflowProgress{
			ToolCallID: "call_123",
			Seq:        seq,
			Kind:       "agent_start",
			Index:      0,
			Running:    1,
			Total:      1,
		})
	}
	require.Equal(t, int64(4), wp.lastSeq)

	// Second run on the same tool call ID: Seq restarts at 1 and must be
	// applied, not swallowed.
	wp.HandleProgress(&notify.WorkflowProgress{
		ToolCallID: "call_123",
		Seq:        1,
		Kind:       "agent_start",
		Index:      0,
		Label:      "second run",
		Running:    1,
		Total:      1,
	})
	require.Equal(t, "running", wp.agents[0].Status)
	require.Equal(t, "second run", wp.agents[0].Label)
}
