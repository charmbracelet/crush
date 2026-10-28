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
	Label string `json:"label,omitempty" description:"The name of one subagent, to show only its detail: its task, recent tool calls, and latest output. Omit to list every subagent in this session"`
}

// dispatchRecord is what a parent transcript says about one subagent label:
// the latest dispatch under it, and whether that dispatch is still waiting
// on a report.
type dispatchRecord struct {
	Label     string
	Access    string
	Task      string
	MessageID string
	CallID    string
	// Tool is the tool call that started this run: agent_dispatch, or the
	// agent or agentic_fetch tool when a blocking call was moved to the
	// background. It picks which agent a follow-up reopens into.
	Tool string
	// The model override as asked for; the result fills in the provider.
	Model     string
	Provider  string
	Reasoning string
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
// be reused across a long session, and a finished subagent can be reopened
// by a follow-up, so only replaying the events in order gets this right.
func dispatchHistory(msgs []message.Message) dispatchRecords {
	records := dispatchRecords{byLabel: map[string]*dispatchRecord{}}
	byCallID := map[string]*dispatchRecord{}
	// bgCallMsg records the message a blocking subagent call lives in, so a
	// later "moved to background" result can derive its child session from
	// the same IDs the run used.
	bgCallMsg := map[string]string{}
	for _, msg := range msgs {
		for _, call := range msg.ToolCalls() {
			switch call.Name {
			case AgentToolName, tools.AgenticFetchToolName:
				bgCallMsg[call.ID] = msg.ID
				continue
			case AgentDispatchToolName:
			default:
				continue
			}
			var params AgentDispatchParams
			if json.Unmarshal([]byte(call.Input), &params) != nil {
				continue
			}
			label := SubagentName(params.Label)
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
				Tool:      AgentDispatchToolName,
				Model:     strings.TrimSpace(params.Model),
				Provider:  strings.TrimSpace(params.Provider),
				Reasoning: strings.TrimSpace(params.Reasoning),
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
				record, ok := byCallID[result.ToolCallID]
				if !ok {
					continue
				}
				if result.IsError {
					// A refused dispatch never started anything, so the label
					// still means whatever it meant before.
					if records.byLabel[record.Label] == record {
						if record.prev != nil {
							records.byLabel[record.Label] = record.prev
						} else {
							delete(records.byLabel, record.Label)
						}
					}
					continue
				}
				// A started dispatch records the model override it resolved,
				// so a restart rehydrates on the provider the dispatch
				// actually used rather than re-deriving it from whatever
				// model the session has by then.
				var meta AgentDispatchResponseMetadata
				if result.Metadata == "" || json.Unmarshal([]byte(result.Metadata), &meta) != nil {
					continue
				}
				record.Model = meta.Model
				record.Provider = meta.Provider
				record.Reasoning = meta.Reasoning
			case AgentSendToolName:
				var meta AgentSendResponseMetadata
				if result.Metadata == "" || json.Unmarshal([]byte(result.Metadata), &meta) != nil || !meta.Reopened {
					continue
				}
				if record, ok := records.byLabel[meta.Label]; ok {
					record.Open = true
				}
			case AgentToolName, tools.AgenticFetchToolName:
				// A blocking subagent that outran the threshold and was
				// moved to the background behaves from here on exactly
				// like a dispatch: its result carries the label it took,
				// and its report lands later under that label.
				var meta AgentBackgroundMetadata
				if result.Metadata == "" || json.Unmarshal([]byte(result.Metadata), &meta) != nil || !meta.Backgrounded {
					continue
				}
				label := strings.TrimSpace(meta.Label)
				if label == "" {
					continue
				}
				access := meta.Access
				if access == "" {
					access = accessRead
				}
				record := &dispatchRecord{
					Label:     label,
					Access:    access,
					Task:      meta.Task,
					MessageID: bgCallMsg[result.ToolCallID],
					CallID:    result.ToolCallID,
					Tool:      result.Name,
					Open:      true,
					prev:      records.byLabel[label],
				}
				if record.prev == nil {
					records.order = append(records.order, label)
				}
				records.byLabel[label] = record
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

// reconcileDispatches tells a parent session about subagents that were cut
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
			Output: "Interrupted: Crush exited while this subagent was running. Its session is kept; agent_send resumes it with its work in context.",
			Failed: true,
		})
	}
}

// agentStatusTool lets the main agent see what its subagents are doing.
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

			label := SubagentName(params.Label)
			if label != "" {
				record, ok := records.byLabel[label]
				if !ok {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"no subagent %q; known: %s",
						label, knownLabels(records),
					)), nil
				}
				text, sum := c.describeDispatch(ctx, parentSessionID, record, true)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(text),
					AgentStatusResponseMetadata{Agents: []AgentStatusSummary{sum}}), nil
			}

			if len(records.order) == 0 {
				return fantasy.NewTextResponse("No subagents dispatched."), nil
			}
			var (
				lines []string
				meta  AgentStatusResponseMetadata
			)
			for _, label := range records.order {
				record, ok := records.byLabel[label]
				if !ok {
					continue
				}
				text, sum := c.describeDispatch(ctx, parentSessionID, record, false)
				lines = append(lines, text)
				meta.Agents = append(meta.Agents, sum)
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(strings.Join(lines, "\n")), meta), nil
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

// AgentStatusResponseMetadata lets the UI draw agent_status results as rows.
type AgentStatusResponseMetadata struct {
	Agents []AgentStatusSummary `json:"agents"`
}

// AgentStatusSummary is one subagent's state, as agent_status reports it.
type AgentStatusSummary struct {
	Label     string        `json:"label"`
	State     SubagentState `json:"state"`
	Detail    string        `json:"detail,omitempty"` // Stop reason.
	Timing    string        `json:"timing,omitempty"` // "for 2m" or "5m ago".
	Access    string        `json:"access"`
	ToolCalls int           `json:"tool_calls"`
	Cost      float64       `json:"cost,omitempty"`
}

// summarizeDispatch reads one subagent's state. The registry is the
// authority on a subagent this process is running; the transcript covers
// the rest, including those cut off by a restart.
func (c *coordinator) summarizeDispatch(parentSessionID string, record *dispatchRecord, activity subAgentProgress) AgentStatusSummary {
	sum := AgentStatusSummary{
		Label:     record.Label,
		State:     SubagentFinished,
		Access:    record.Access,
		ToolCalls: activity.toolCalls,
		Cost:      activity.cost,
	}
	if record.LastReport != nil && record.LastReport.Failed {
		sum.State = SubagentFailed
	}
	childID := c.sessions.CreateAgentToolSessionID(record.MessageID, record.CallID)
	entry, ok := c.dispatched.Get(dispatchKey(parentSessionID, record.Label))
	if !ok || entry.SessionID != childID {
		if record.Open {
			sum.State, sum.Detail = SubagentInterrupted, "Crush exited"
		}
		return sum
	}
	snap := entry.snapshot()
	sum.State = snap.State.reported()
	switch {
	case snap.State == dispatchRunning:
		if snap.StopReason != "" {
			sum.State = SubagentStopping
		}
		sum.Timing = "for " + formatElapsed(time.Since(snap.Started))
	case !snap.Ended.IsZero():
		sum.Timing = formatElapsed(time.Since(snap.Ended)) + " ago"
	}
	if snap.State == dispatchStopped {
		sum.Detail = snap.StopReason
	}
	return sum
}

// describeDispatch renders one subagent's status for the model.
func (c *coordinator) describeDispatch(ctx context.Context, parentSessionID string, record *dispatchRecord, detail bool) (string, AgentStatusSummary) {
	activity := c.subAgentActivity(ctx, c.sessions.CreateAgentToolSessionID(record.MessageID, record.CallID))
	sum := c.summarizeDispatch(parentSessionID, record, activity)

	var sb strings.Builder
	fmt.Fprintf(&sb, "- %s: %s", sum.Label, sum.State)
	if sum.Detail != "" {
		fmt.Fprintf(&sb, " (%s)", sum.Detail)
	}
	if sum.Timing != "" {
		fmt.Fprintf(&sb, " %s", sum.Timing)
	}
	fmt.Fprintf(&sb, ", %s access, %d tool calls", sum.Access, sum.ToolCalls)
	if sum.Cost > 0 {
		fmt.Fprintf(&sb, ", $%.2f", sum.Cost)
	}
	if activity.lastCall != "" {
		fmt.Fprintf(&sb, "\n  latest: %s", activity.lastCall)
	}
	if !detail {
		return sb.String(), sum
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
	return sb.String(), sum
}

// subAgentProgress summarizes what a subagent session has done so far.
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

// LatestDispatches returns, for each subagent label in a parent transcript,
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
