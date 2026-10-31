package model

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// subAgentSpinnerFrames animates the running marker. The sidebar is rebuilt
// on every animation tick while a dispatch item is spinning, so deriving the
// frame from the clock is enough to make it move without its own ticker.
var subAgentSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// subAgentSpinner returns the frame for the current moment.
func subAgentSpinner(now time.Time) string {
	const frameEvery = 80 * time.Millisecond
	i := (now.UnixNano() / int64(frameEvery)) % int64(len(subAgentSpinnerFrames))
	return subAgentSpinnerFrames[i]
}

// subAgentIdleRetention is how long a finished subagent stays in the
// sidebar. Without it the section only ever grows, since every dispatch
// leaves a row behind for the rest of the session. A retired subagent is
// hidden, not gone: agent_send can still reach it.
const subAgentIdleRetention = 5 * time.Minute

// subAgentStatuses reads the dispatched subagents out of the chat itself.
// Every dispatch is a tool call item in the transcript, and the subagent's
// tool calls already stream in as that item's nested tools, so the chat
// already knows both who is running and what they are doing.
//
// Subagents that finished more than [subAgentIdleRetention] ago are left
// out. Nothing repaints on a timer once every subagent is done, so a row
// retires on the next render rather than exactly on the deadline.
func (m *UI) subAgentStatuses() []chat.DispatchStatus {
	var statuses []chat.DispatchStatus
	for i := range m.chat.Len() {
		item, ok := m.chat.MessageItemAt(i).(*chat.AgentToolMessageItem)
		if !ok {
			continue
		}
		status, ok := item.DispatchStatus()
		if !ok {
			continue
		}
		if !status.Running && !status.Finished.IsZero() &&
			time.Since(status.Finished) > subAgentIdleRetention {
			continue
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// runningSubAgents counts the subagents still working.
func (m *UI) runningSubAgents() int {
	var n int
	for _, s := range m.subAgentStatuses() {
		if s.Running {
			n++
		}
	}
	return n
}

// subAgentRow renders one subagent's line: a spinner and elapsed time while
// it works, so a glance distinguishes it from a static "online" dot.
func (m *UI) subAgentRow(status chat.DispatchStatus, width int) string {
	t := m.com.Styles

	icon := t.Resource.WorkingIcon.Render(subAgentSpinner(time.Now()))
	state := "starting"
	switch {
	case !status.Running:
		icon, state = t.Resource.OfflineIcon.String(), "done"
	case !status.Started.IsZero():
		state = time.Since(status.Started).Round(time.Second).String()
	}

	// The label is the description so Status truncates it, not the state.
	first, label := agent.SubagentDisplayName(status.Label)
	title := t.Tool.AgentName.Render(first)
	if first == "" {
		title, label = t.Tool.AgentLabel.Render(label), ""
	}
	return common.Status(t, common.StatusOpts{
		Icon:         icon,
		Title:        title,
		Description:  label,
		ExtraContent: t.Resource.RowDescBase.Foreground(t.Resource.DefaultDescFg).Render(state),
	}, width)
}

// subAgentsInfo renders the dispatched subagent section. It renders empty
// when nothing has been dispatched, so it costs no sidebar space until it has
// something to say.
func (m *UI) subAgentsInfo(statuses []chat.DispatchStatus, width, maxItems int, isSection bool) string {
	if len(statuses) == 0 {
		return ""
	}

	t := m.com.Styles
	title := t.Resource.Heading.Render("Subagents")
	if isSection {
		title = common.Section(t, title, width)
	}

	rows := make([]string, 0, len(statuses))
	for _, status := range statuses {
		rows = append(rows, m.subAgentRow(status, width))
	}
	rows = truncateRows(rows, maxItems, t.Resource.AdditionalText)

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf(
		"%s\n\n%s", title, lipgloss.JoinVertical(lipgloss.Left, rows...),
	))
}
