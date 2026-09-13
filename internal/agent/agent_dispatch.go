package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
)

//go:embed templates/agent_dispatch_tool.md
var agentDispatchToolDescription string

//go:embed templates/agent_send_tool.md
var agentSendToolDescription string

const (
	AgentDispatchToolName = "agent_dispatch"
	AgentSendToolName     = "agent_send"
)

type AgentDispatchParams struct {
	Prompt string `json:"prompt" description:"The task for the sub-agent to perform"`
	Label  string `json:"label" description:"A short name for this sub-agent, used to address it and to label its report"`
	Access string `json:"access,omitempty" description:"\"read\" (default) for a sub-agent that can only inspect the codebase, or \"write\" for one that can also edit files"`
}

// AgentSendResponseMetadata tells the UI which sub-agent a message went to,
// and whether it restarted a finished one, so the transcript can show that
// sub-agent as working again.
type AgentSendResponseMetadata struct {
	Label    string `json:"label"`
	Reopened bool   `json:"reopened,omitempty"`
}

type AgentSendParams struct {
	Label   string `json:"label" description:"The label of the running sub-agent to send to"`
	Message string `json:"message" description:"The message to deliver to that sub-agent"`
}

// dispatchedAgent tracks one detached sub-agent run.
type dispatchedAgent struct {
	Label           string
	SessionID       string
	ParentSessionID string
	// agent is the SessionAgent instance actually running this sub-agent.
	// Busy state and the message queue live on that instance, not on the
	// coordinator's main agent, so a message meant for a sub-agent has to
	// be addressed to it directly.
	agent  SessionAgent
	cancel context.CancelFunc
	done   atomic.Bool
	// rolledUpCost is the child session's cumulative cost already added to
	// the parent. A reopened sub-agent rolls up only what it spent since,
	// since the child's own total is cumulative across every turn.
	rolledUpCost atomic.Uint64
}

// awaitDeliverable waits until the sub-agent's session is actually running
// and can accept a message, and reports whether it can. A dispatch returns
// before its goroutine has started the run, so a message sent right after
// dispatching arrives while the session is neither busy nor done; treating
// that window as "finished" would drop the message. Waiting closes the gap
// without blocking indefinitely on a sub-agent that died on startup.
func (d *dispatchedAgent) awaitDeliverable(ctx context.Context) bool {
	const (
		pollEvery = 50 * time.Millisecond
		giveUp    = 10 * time.Second
	)
	deadline := time.NewTimer(giveUp)
	defer deadline.Stop()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()

	for {
		if d.done.Load() {
			return false
		}
		if d.agent.IsSessionBusy(d.SessionID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

// dispatchKey namespaces a label to its parent session, so two sessions can
// use the same label without colliding.
func dispatchKey(parentSessionID, label string) string {
	return parentSessionID + "\x00" + label
}

// agentDispatchTool launches a sub-agent that runs detached from this tool
// call. The tool returns as soon as the sub-agent starts; the result is
// delivered later as a message into the parent session.
func (c *coordinator) agentDispatchTool(ctx context.Context, parentAgentID string) (fantasy.AgentTool, error) {
	readOnly := readOnlyParent(parentAgentID)
	agents, err := c.taskAgents(ctx, readOnly)
	if err != nil {
		return nil, err
	}

	return fantasy.NewParallelAgentTool(
		AgentDispatchToolName,
		agentDispatchToolDescription,
		func(ctx context.Context, params AgentDispatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}
			label := strings.TrimSpace(params.Label)
			if label == "" {
				return fantasy.NewTextErrorResponse("label is required"), nil
			}

			agent, refusal := resolveSubAgent(agents, params.Access, readOnly)
			if refusal != "" {
				return fantasy.NewTextErrorResponse(refusal), nil
			}

			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			key := dispatchKey(parentSessionID, label)
			if existing, ok := c.dispatched.Get(key); ok && !existing.done.Load() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("a sub-agent labeled %q is already running; pick another label", label)), nil
			}

			// Detached from the tool call's context: this run must outlive
			// the turn that started it. Cancellation comes from the
			// registry instead.
			runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			// The sub-session ID derives from the bare tool call ID so the
			// UI can walk back from a child-session event to this tool's
			// chat item and nest the sub-agent's tool calls under it.
			toolCallID := call.ID
			entry := &dispatchedAgent{
				Label:           label,
				agent:           agent,
				SessionID:       c.sessions.CreateAgentToolSessionID(agentMessageID, toolCallID),
				ParentSessionID: parentSessionID,
				cancel:          cancel,
			}
			c.dispatched.Set(key, entry)

			go func() {
				defer cancel()
				defer entry.done.Store(true)
				resp, err := c.runSubAgent(runCtx, subAgentParams{
					Agent:          agent,
					SessionID:      parentSessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     toolCallID,
					Prompt:         params.Prompt,
					SessionTitle:   "Sub-agent: " + label,
				})
				entry.done.Store(true)
				if child, err := c.sessions.Get(runCtx, entry.SessionID); err == nil {
					entry.rolledUpCost.Store(math.Float64bits(child.Cost))
				}
				c.reportDispatchResult(runCtx, entry, resp, err)
			}()

			return fantasy.NewTextResponse(fmt.Sprintf(
				"Sub-agent %q started. It reports back on its own; carry on with other work rather than waiting for it.",
				label,
			)), nil
		},
	), nil
}

// reopen runs another turn on a finished sub-agent's existing session, so it
// answers with its whole task still in context. This is what makes a
// sub-agent answerable rather than a one-shot: the session is real and its
// history is intact, so a follow-up is just the next turn on it.
//
// Returns false if the sub-agent is not idle, meaning somebody else claimed
// it first or it is still working.
func (c *coordinator) reopen(entry *dispatchedAgent, prompt string) bool {
	// Claim the idle sub-agent. The CAS makes two concurrent follow-ups
	// resolve to one reopen rather than two turns racing on one session.
	if !entry.done.CompareAndSwap(true, false) {
		return false
	}

	agentCall, _, _, err := c.subAgentCall(entry.agent, entry.SessionID, prompt)
	if err != nil {
		entry.done.Store(true)
		return false
	}

	runCtx, cancel := context.WithCancel(context.Background())
	entry.cancel = cancel

	go func() {
		defer cancel()
		defer entry.done.Store(true)

		result, err := entry.agent.Run(runCtx, agentCall)
		entry.done.Store(true)

		var resp fantasy.ToolResponse
		switch {
		case err != nil:
			resp = fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to generate response: %s", err))
		default:
			if output := subAgentOutput(result); output != "" {
				resp = fantasy.NewTextResponse(output)
			}
		}
		c.rollUpDispatchCost(runCtx, entry)
		c.reportDispatchResult(runCtx, entry, resp, err)
	}()
	return true
}

// rollUpDispatchCost adds what a reopened sub-agent spent since the last
// roll-up to its parent session.
func (c *coordinator) rollUpDispatchCost(ctx context.Context, entry *dispatchedAgent) {
	child, err := c.sessions.Get(ctx, entry.SessionID)
	if err != nil {
		return
	}
	previous := math.Float64frombits(entry.rolledUpCost.Load())
	delta := child.Cost - previous
	if delta <= 0 {
		return
	}
	entry.rolledUpCost.Store(math.Float64bits(child.Cost))

	parent, err := c.sessions.Get(ctx, entry.ParentSessionID)
	if err != nil {
		return
	}
	parent.Cost += delta
	if _, err := c.sessions.Save(ctx, parent); err != nil {
		slog.Warn("Failed to roll up sub-agent cost", "label", entry.Label, "error", err)
	}
}

// reportDispatchResult delivers a finished sub-agent's output back to the
// parent session. If the parent is mid-turn the message folds into it at the
// next step; if the parent is idle it starts a new turn. Both branches are
// already handled by sessionAgent.Run.
func (c *coordinator) reportDispatchResult(ctx context.Context, entry *dispatchedAgent, resp fantasy.ToolResponse, err error) {
	var (
		body   string
		failed bool
	)
	switch {
	case err != nil:
		body, failed = fmt.Sprintf("Sub-agent failed: %s", err), true
	case resp.Content == "":
		body, failed = "Sub-agent finished without producing any output.", true
	default:
		body = resp.Content
	}

	report := message.SubAgentReport{
		Label:  entry.Label,
		Output: body,
		Failed: failed,
	}
	// Prompt mirrors what the part renders into for the model; the part is
	// what the UI draws.
	prompt := fmt.Sprintf("<sub-agent-report label=%q>\n%s\n</sub-agent-report>", report.Label, report.Output)
	if _, err := c.runWithParts(
		context.WithoutCancel(ctx),
		entry.ParentSessionID,
		prompt,
		[]message.ContentPart{report},
	); err != nil {
		slog.Error(
			"Failed to deliver sub-agent report",
			"label", entry.Label,
			"parent_session", entry.ParentSessionID,
			"error", err,
		)
	}
}

// agentSendTool delivers a message to a running detached sub-agent. The
// message is queued on the sub-agent's session, where sessionAgent folds it
// into the sub-agent's turn at its next step.
func (c *coordinator) agentSendTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgentSendToolName,
		agentSendToolDescription,
		func(ctx context.Context, params AgentSendParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			label := strings.TrimSpace(params.Label)
			if label == "" {
				return fantasy.NewTextErrorResponse("label is required"), nil
			}
			if params.Message == "" {
				return fantasy.NewTextErrorResponse("message is required"), nil
			}

			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			entry, ok := c.dispatched.Get(dispatchKey(parentSessionID, label))
			if !ok {
				// Not in the registry, which after a restart is every
				// sub-agent. The transcript still records the dispatch and
				// the child session still holds the work, so rebuild the
				// entry from those rather than refusing.
				entry, err := c.rehydrateDispatch(ctx, parentSessionID, label)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"no sub-agent labeled %q. Running sub-agents: %s",
						label, c.runningDispatchLabels(parentSessionID),
					)), nil
				}
				if !c.reopen(entry, params.Message) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"could not reach sub-agent %q. Dispatch a new one instead.", label,
					)), nil
				}
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
					"Sub-agent %q had finished, so it picked this up as a follow-up with its earlier work still in context. Its answer arrives as a new report.",
					label,
				)), AgentSendResponseMetadata{Label: label, Reopened: true}), nil
			}
			if !entry.awaitDeliverable(ctx) {
				// Finished, not gone: its session still holds everything
				// it did, so a follow-up reopens it rather than failing.
				if c.reopen(entry, params.Message) {
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
						"Sub-agent %q had finished, so it picked this up as a follow-up with its earlier work still in context. Its answer arrives as a new report.",
						label,
					)), AgentSendResponseMetadata{Label: label, Reopened: true}), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"could not reach sub-agent %q. Dispatch a new one instead.", label,
				)), nil
			}

			// Addressed to the sub-agent's own SessionAgent and queued
			// without a RunID, so it folds into that sub-agent's active
			// turn rather than starting a turn of its own.
			agentCall, _, _, err := c.subAgentCall(entry.agent, entry.SessionID, params.Message)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("could not deliver the message: %s", err)), nil
			}
			if _, err := entry.agent.Run(context.WithoutCancel(ctx), agentCall); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("could not deliver the message: %s", err)), nil
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
				"Delivered to %q. It picks the message up at its next step and folds the answer into its final report.",
				label,
			)), AgentSendResponseMetadata{Label: label}), nil
		},
	)
}

// runningDispatchLabels lists the sub-agents still running for a session.
func (c *coordinator) runningDispatchLabels(parentSessionID string) string {
	var labels []string
	for _, entry := range c.dispatched.Seq2() {
		if entry.ParentSessionID == parentSessionID && !entry.done.Load() {
			labels = append(labels, entry.Label)
		}
	}
	if len(labels) == 0 {
		return "none"
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

// rehydrateDispatch rebuilds a sub-agent's registry entry from what is on
// disk. The registry is in-memory, so a restart empties it, but nothing that
// matters was in memory to begin with: the parent's transcript records the
// dispatch call, the child session ID derives from that call's IDs, and the
// child session holds the whole conversation. The entry is reconstructed as a
// finished sub-agent, which is what it is, so the caller reopens it.
func (c *coordinator) rehydrateDispatch(ctx context.Context, parentSessionID, label string) (*dispatchedAgent, error) {
	messages, err := c.messages.List(ctx, parentSessionID)
	if err != nil {
		return nil, err
	}

	// Rehydration replays the access level recorded in the transcript, so
	// both variants have to be available to look it up. A read-only parent
	// could never have recorded a write dispatch in the first place.
	agents, err := c.taskAgents(ctx, false)
	if err != nil {
		return nil, err
	}

	// Walk backwards: a label can be reused across a long session, and the
	// most recent dispatch is the one being addressed.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		for _, call := range msg.ToolCalls() {
			if call.Name != AgentDispatchToolName {
				continue
			}
			var params AgentDispatchParams
			if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
				continue
			}
			if strings.TrimSpace(params.Label) != label {
				continue
			}
			access := params.Access
			if access == "" {
				access = accessRead
			}
			agent, ok := agents[access]
			if !ok {
				continue
			}

			sessionID := c.sessions.CreateAgentToolSessionID(msg.ID, call.ID)
			child, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				// The dispatch was recorded but its session is gone, so
				// there is no history to reopen into.
				continue
			}

			entry := &dispatchedAgent{
				Label:           label,
				agent:           agent,
				SessionID:       sessionID,
				ParentSessionID: parentSessionID,
				cancel:          func() {},
			}
			entry.done.Store(true)
			// Seed the roll-up with what the child has already cost. The
			// parent was billed for all of it during the original run, so
			// starting from zero would bill the whole total a second time.
			entry.rolledUpCost.Store(math.Float64bits(child.Cost))

			c.dispatched.Set(dispatchKey(parentSessionID, label), entry)
			return entry, nil
		}
	}
	return nil, fmt.Errorf("no dispatch recorded for label %q", label)
}

// cancelDispatched stops every sub-agent dispatched from a session. Cancelling
// a turn must not leave detached children running behind it.
//
// The registry entry stays. Cancelling ends the run in flight, not the session
// behind it, whose history is still intact, so the sub-agent stays addressable
// and a later agent_send reopens it rather than failing.
func (c *coordinator) cancelDispatched(parentSessionID string) {
	for _, entry := range c.dispatched.Seq2() {
		if entry.ParentSessionID != parentSessionID {
			continue
		}
		entry.cancel()
		entry.done.Store(true)
	}
}
