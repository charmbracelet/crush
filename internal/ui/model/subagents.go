package model

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
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

// subAgentIdleRetention is how long a finished sub-agent stays in the
// sidebar. Without it the section only ever grows, since every dispatch
// leaves a row behind for the rest of the session. A retired sub-agent is
// hidden, not gone: agent_send can still reach it.
const subAgentIdleRetention = 5 * time.Minute

// subAgentStatuses reads the dispatched sub-agents out of the chat itself.
// Every dispatch is a tool call item in the transcript, and the sub-agent's
// tool calls already stream in as that item's nested tools, so the chat
// already knows both who is running and what they are doing.
//
// Sub-agents that finished more than [subAgentIdleRetention] ago are left
// out. Nothing repaints on a timer once every sub-agent is done, so a row
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

// runningSubAgents counts the sub-agents still working.
func (m *UI) runningSubAgents() int {
	var n int
	for _, s := range m.subAgentStatuses() {
		if s.Running {
			n++
		}
	}
	return n
}

// subAgentRow renders one sub-agent's line: a spinner and elapsed time while
// it works, so a glance distinguishes it from a static "online" dot.
func (m *UI) subAgentRow(status chat.DispatchStatus, width int) string {
	t := m.com.Styles

	if !status.Running {
		return common.Status(t, common.StatusOpts{
			Icon:        t.Resource.OfflineIcon.String(),
			Title:       t.Resource.Name.Render(status.Label),
			Description: "done",
		}, width)
	}

	description := status.Activity
	if description == "" {
		description = "starting"
	}
	if !status.Started.IsZero() {
		description = fmt.Sprintf("%s · %s", description, time.Since(status.Started).Round(time.Second))
	}
	return common.Status(t, common.StatusOpts{
		Icon:        t.Resource.WorkingIcon.Render(subAgentSpinner(time.Now())),
		Title:       t.Resource.Name.Render(status.Label),
		Description: description,
	}, width)
}

// subAgentsInfo renders the dispatched sub-agent section. It renders empty
// when nothing has been dispatched, so it costs no sidebar space until it has
// something to say.
func (m *UI) subAgentsInfo(width int, isSection bool) string {
	statuses := m.subAgentStatuses()
	if len(statuses) == 0 {
		return ""
	}

	t := m.com.Styles
	title := t.Resource.Heading.Render("Sub-agents")
	if isSection {
		title = common.Section(t, title, width)
	}

	rows := make([]string, 0, len(statuses))
	for _, status := range statuses {
		rows = append(rows, m.subAgentRow(status, width))
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf(
		"%s\n\n%s", title, lipgloss.JoinVertical(lipgloss.Left, rows...),
	))
}
