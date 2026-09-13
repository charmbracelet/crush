package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
)

//go:embed templates/agent_status_tool.md
var agentStatusToolDescription string

const AgentStatusToolName = "agent_status"

type AgentStatusParams struct {
	Label string `json:"label,omitempty" description:"Show detail for this sub-agent only: its task, recent tool calls, and latest output. Omit to list every sub-agent in this session"`
}

// dispatchRecord is what a parent transcript says about one sub-agent label:
// the latest dispatch under it, and whether that dispatch is still waiting
// on a report.
type dispatchRecord struct {
	Label     string
	Access    string
	Task      string
	MessageID string
	CallID    string
	// Open is true while the latest dispatch or follow-up has no report.
	Open       bool
	LastReport *message.SubAgentReport
	prev       *dispatchRecord
}

// dispatchRecords indexes a transcript's dispatches by label, keeping the
// order labels were first dispatched in.
type dispatchRecords struct {
	byLabel map[string]*dispatchRecord
	order   []string
}

// dispatchHistory replays a parent transcript in order to find, for each
// label, its latest dispatch and whether it has reported since. A label can
// be reused across a long session, and a finished sub-agent can be reopened
// by a follow-up, so only replaying the events in order gets this right.
func dispatchHistory(msgs []message.Message) dispatchRecords {
	records := dispatchRecords{byLabel: map[string]*dispatchRecord{}}
	byCallID := map[string]*dispatchRecord{}
	for _, msg := range msgs {
		for _, call := range msg.ToolCalls() {
			if call.Name != AgentDispatchToolName {
				continue
			}
			var params AgentDispatchParams
			if json.Unmarshal([]byte(call.Input), &params) != nil {
				continue
			}
			label := strings.TrimSpace(params.Label)
			if label == "" {
				continue
			}
			access := params.Access
			if access == "" {
				access = accessRead
			}
			record := &dispatchRecord{
				Label:     label,
				Access:    access,
				Task:      params.Prompt,
				MessageID: msg.ID,
				CallID:    call.ID,
				Open:      true,
				prev:      records.byLabel[label],
			}
			if record.prev == nil {
				records.order = append(records.order, label)
			}
			records.byLabel[label] = record
			byCallID[call.ID] = record
		}
		for _, result := range msg.ToolResults() {
			switch result.Name {
			case AgentDispatchToolName:
				// A refused dispatch never started anything, so the label
				// still means whatever it meant before.
				record, ok := byCallID[result.ToolCallID]
				if !ok || !result.IsError {
					continue
				}
				if records.byLabel[record.Label] == record {
					if record.prev != nil {
						records.byLabel[record.Label] = record.prev
					} else {
						delete(records.byLabel, record.Label)
					}
				}
			case AgentSendToolName:
				var meta AgentSendResponseMetadata
				if result.Metadata == "" || json.Unmarshal([]byte(result.Metadata), &meta) != nil || !meta.Reopened {
					continue
				}
				if record, ok := records.byLabel[meta.Label]; ok {
					record.Open = true
				}
			}
		}
		for _, report := range msg.SubAgentReports() {
			if record, ok := records.byLabel[report.Label]; ok {
				record.Open = false
				record.LastReport = &report
			}
		}
	}
	return records
}

// reconcileDispatches tells a parent session about sub-agents that were cut
// off when Crush last exited. The transcript shows them dispatched and never
// reporting, and nothing in this process is running them, so without this the
// parent would wait forever on a report that is never coming.
//
// It runs once per session per process, only while the session is idle, so
// the report lands between turns rather than in the middle of one.
func (c *coordinator) reconcileDispatches(ctx context.Context, agent SessionAgent, sessionID string) {
	if _, done := c.reconciled.Load(sessionID); done || agent.IsSessionBusy(sessionID) {
		return
	}
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return
	}
	c.reconciled.Store(sessionID, struct{}{})

	records := dispatchHistory(msgs)
	for _, label := range records.order {
		record, ok := records.byLabel[label]
		if !ok || !record.Open {
			continue
		}
		if _, live := c.dispatched.Get(dispatchKey(sessionID, label)); live {
			continue
		}
		c.recordDispatchReport(sessionID, message.SubAgentReport{
			Label:  label,
			Output: "Interrupted: Crush exited while this sub-agent was running. Its session is kept; agent_send resumes it with its work in context.",
			Failed: true,
		})
	}
}

// agentStatusTool lets the main agent see what its sub-agents are doing.
func (c *coordinator) agentStatusTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgentStatusToolName,
		agentStatusToolDescription,
		func(ctx context.Context, params AgentStatusParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			msgs, err := c.messages.List(ctx, parentSessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("read session: %w", err)
			}
			records := dispatchHistory(msgs)

			label := strings.TrimSpace(params.Label)
			if label != "" {
				record, ok := records.byLabel[label]
				if !ok {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"no sub-agent labeled %q in this session. Known: %s",
						label, knownLabels(records),
					)), nil
				}
				return fantasy.NewTextResponse(c.describeDispatch(ctx, parentSessionID, record, true)), nil
			}

			if len(records.order) == 0 {
				return fantasy.NewTextResponse("No sub-agents have been dispatched in this session."), nil
			}
			var sb strings.Builder
			for _, label := range records.order {
				record, ok := records.byLabel[label]
				if !ok {
					continue
				}
				sb.WriteString(c.describeDispatch(ctx, parentSessionID, record, false))
				sb.WriteString("\n")
			}
			return fantasy.NewTextResponse(strings.TrimRight(sb.String(), "\n")), nil
		},
	)
}

func knownLabels(records dispatchRecords) string {
	var labels []string
	for _, label := range records.order {
		if _, ok := records.byLabel[label]; ok {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return "none"
	}
	return strings.Join(labels, ", ")
}

// describeDispatch renders one sub-agent's status. The registry is the
// authority on a sub-agent this process is running; the transcript covers
// the rest, including those cut off by a restart.
func (c *coordinator) describeDispatch(ctx context.Context, parentSessionID string, record *dispatchRecord, detail bool) string {
	state, timing := "finished", ""
	if record.LastReport != nil && record.LastReport.Failed {
		state = "failed"
	}
	if entry, ok := c.dispatched.Get(dispatchKey(parentSessionID, record.Label)); ok && entry.SessionID == c.sessions.CreateAgentToolSessionID(record.MessageID, record.CallID) {
		snap := entry.snapshot()
		state = snap.State.String()
		switch {
		case snap.State == dispatchRunning && snap.StopReason != "":
			state = "stopping"
			timing = "for " + formatElapsed(time.Since(snap.Started))
		case snap.State == dispatchRunning:
			timing = "for " + formatElapsed(time.Since(snap.Started))
		case !snap.Ended.IsZero():
			timing = formatElapsed(time.Since(snap.Ended)) + " ago"
		}
		if snap.State == dispatchStopped && snap.StopReason != "" {
			state += " (" + snap.StopReason + ")"
		}
	} else if record.Open {
		state = "interrupted (Crush exited while it was running)"
	}

	childID := c.sessions.CreateAgentToolSessionID(record.MessageID, record.CallID)
	activity := c.subAgentActivity(ctx, childID)

	var sb strings.Builder
	fmt.Fprintf(&sb, "- %s: %s", record.Label, state)
	if timing != "" {
		fmt.Fprintf(&sb, " %s", timing)
	}
	fmt.Fprintf(&sb, ", %s access, %d tool calls", record.Access, activity.toolCalls)
	if activity.cost > 0 {
		fmt.Fprintf(&sb, ", $%.2f", activity.cost)
	}
	if activity.lastCall != "" {
		fmt.Fprintf(&sb, "\n  latest: %s", activity.lastCall)
	}
	if !detail {
		return sb.String()
	}

	fmt.Fprintf(&sb, "\n\nTask:\n%s", truncateForStatus(record.Task, 1500))
	if len(activity.recentCalls) > 0 {
		sb.WriteString("\n\nRecent tool calls, oldest first:")
		for _, line := range activity.recentCalls {
			fmt.Fprintf(&sb, "\n- %s", line)
		}
	}
	if activity.lastText != "" {
		fmt.Fprintf(&sb, "\n\nLatest output:\n%s", truncateForStatus(activity.lastText, 2000))
	}
	if record.LastReport != nil {
		fmt.Fprintf(&sb, "\n\nLast report:\n%s", truncateForStatus(record.LastReport.Output, 2000))
	}
	return sb.String()
}

// subAgentProgress summarizes what a sub-agent session has done so far.
type subAgentProgress struct {
	toolCalls   int
	lastCall    string
	recentCalls []string
	lastText    string
	cost        float64
}

func (c *coordinator) subAgentActivity(ctx context.Context, sessionID string) subAgentProgress {
	var progress subAgentProgress
	if session, err := c.sessions.Get(ctx, sessionID); err == nil {
		progress.cost = session.Cost
	}
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return progress
	}

	results := map[string]message.ToolResult{}
	for _, msg := range msgs {
		for _, result := range msg.ToolResults() {
			results[result.ToolCallID] = result
		}
	}

	const recent = 8
	var calls []string
	for _, msg := range msgs {
		if msg.Role != message.Assistant {
			continue
		}
		if text := strings.TrimSpace(msg.Content().Text); text != "" {
			progress.lastText = text
		}
		for _, call := range msg.ToolCalls() {
			progress.toolCalls++
			outcome := "running"
			if result, ok := results[call.ID]; ok {
				outcome = "ok"
				if result.IsError {
					outcome = "error"
				}
			}
			calls = append(calls, fmt.Sprintf("%s %s [%s]", call.Name, truncateForStatus(compactJSON(call.Input), 120), outcome))
		}
	}
	if len(calls) > 0 {
		progress.lastCall = calls[len(calls)-1]
		progress.recentCalls = calls[max(0, len(calls)-recent):]
	}
	return progress
}

// compactJSON squeezes tool input onto one line.
func compactJSON(input string) string {
	var v any
	if json.Unmarshal([]byte(input), &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return strings.Join(strings.Fields(input), " ")
}

func truncateForStatus(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return strings.TrimSpace(s[:limit]) + "..."
}

func formatElapsed(d time.Duration) string {
	return d.Round(time.Second).String()
}

// DispatchState is where the latest dispatch under a label stands, as far as
// a parent transcript can tell.
type DispatchState struct {
	// CallID is the agent_dispatch tool call that started it.
	CallID string
	// Open is true while it has not reported since it was dispatched or
	// last reopened. An open dispatch is either still running or was cut
	// off; only the backend can say which.
	Open bool
}

// LatestDispatches returns, for each sub-agent label in a parent transcript,
// the state of its latest dispatch.
func LatestDispatches(msgs []*message.Message) map[string]DispatchState {
	plain := make([]message.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg != nil {
			plain = append(plain, *msg)
		}
	}
	records := dispatchHistory(plain)
	states := make(map[string]DispatchState, len(records.byLabel))
	for label, record := range records.byLabel {
		states[label] = DispatchState{CallID: record.CallID, Open: record.Open}
	}
	return states
}
