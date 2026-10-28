package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
)

// subAgentBackgroundAfter is how long a blocking subagent tool call runs in
// the foreground before it is moved to the background, mirroring the bash
// tool's auto-background threshold.
const subAgentBackgroundAfter = 60 * time.Second

// AgentBackgroundMetadata marks a blocking subagent tool result as handed
// off to the background. The parent transcript carries it so the subagent
// can be replayed as a running dispatch: shown by agent_status, reconciled
// after a restart, and reached by agent_send and agent_stop.
type AgentBackgroundMetadata struct {
	Label        string `json:"label"`
	Backgrounded bool   `json:"backgrounded"`
	Access       string `json:"access,omitempty"`
	Task         string `json:"task,omitempty"`
}

// backgroundLabel names an auto-backgrounded subagent after its tool call.
// The call ID is unique within a session and stays in the transcript, so the
// label is stable across a restart without a counter to keep.
func backgroundLabel(toolName, callID string) string {
	prefix := "agent"
	if toolName == tools.AgenticFetchToolName {
		prefix = "search"
	}
	id := callID
	if i := strings.LastIndexAny(id, "_-"); i >= 0 && i+1 < len(id) {
		id = id[i+1:]
	}
	if len(id) > 6 {
		id = id[len(id)-6:]
	}
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	}
	return prefix + "-" + id
}

// runSubAgentMaybeBackground runs a blocking subagent tool call, but moves it
// to the background if it outruns the threshold. A headless run has no later
// turn to deliver a report into, so there the call always blocks, exactly as
// before. In an interactive run a long call is handed to the dispatch
// registry: the tool returns a notice, the result arrives later as a report,
// and agent_status, agent_send, and agent_stop can reach it meanwhile.
func (c *coordinator) runSubAgentMaybeBackground(
	ctx context.Context,
	toolName, access string,
	params subAgentParams,
) (fantasy.ToolResponse, error) {
	if !c.interactive {
		return c.runSubAgent(ctx, params)
	}
	if access == "" {
		access = accessRead
	}

	label := backgroundLabel(toolName, params.ToolCallID)
	childSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	entry := newDispatchedAgent(label, access, childSessionID, params.SessionID, params.Agent)

	// The background goroutine rolls the child's cost up itself, as a
	// dispatched subagent does, so the synchronous rollup is turned off.
	params.SkipCostRollup = true

	key := dispatchKey(params.SessionID, label)
	c.dispatchMu.Lock()
	c.dispatched.Set(key, entry)
	runCtx, gen, _ := entry.claim()
	c.dispatchMu.Unlock()

	type outcome struct {
		resp fantasy.ToolResponse
		err  error
	}
	var (
		mu           sync.Mutex
		backgrounded bool
		result       *outcome
	)
	done := make(chan struct{})

	// The run uses the detached claim context, so moving it to the
	// background outlives the turn that started it.
	go func() {
		resp, err := c.runSubAgent(runCtx, params)
		c.rollUpDispatchCost(context.Background(), entry)
		stopReason := entry.settle(gen, err != nil || resp.IsError)

		mu.Lock()
		result = &outcome{resp: resp, err: err}
		reportIt := backgrounded
		mu.Unlock()
		close(done)

		if !reportIt {
			// The foreground is still waiting, took the result, or
			// cancelled: it owns the outcome and no report is written.
			return
		}
		switch stopReason {
		case "":
			c.reportDispatchResult(entry, resp, err)
		case stopForShutdown:
		default:
			c.recordDispatchReport(entry.ParentSessionID, message.SubAgentReport{
				Label:  entry.Label,
				Output: "Stopped: " + stopReason + ".",
				Failed: true,
			})
		}
	}()

	drop := func() {
		c.dispatchMu.Lock()
		c.dispatched.Del(key)
		c.dispatchMu.Unlock()
	}
	take := func() (fantasy.ToolResponse, error) {
		mu.Lock()
		out := *result
		mu.Unlock()
		return out.resp, out.err
	}

	timer := time.NewTimer(c.backgroundAfter)
	defer timer.Stop()

	select {
	case <-done:
		// Finished within the threshold: behave like a blocking call and
		// leave nothing in the registry.
		drop()
		return take()
	case <-timer.C:
		mu.Lock()
		if result != nil {
			// Finished right at the threshold; prefer the real result over
			// a background notice.
			mu.Unlock()
			drop()
			return take()
		}
		backgrounded = true
		mu.Unlock()
		return backgroundNotice(label, access, params.Prompt, c.backgroundAfter), nil
	case <-ctx.Done():
		// The parent turn was cancelled before the handoff. Stop the
		// subagent and surface the cancellation, as a blocking call would;
		// the goroutine sees backgrounded still false and writes no report.
		entry.stop("the parent turn was cancelled")
		<-done
		drop()
		return fantasy.ToolResponse{}, ctx.Err()
	}
}

// backgroundNotice is what the model gets when a subagent is moved to the
// background. It names the label so the model can follow up on it.
func backgroundNotice(label, access, prompt string, after time.Duration) fantasy.ToolResponse {
	notice := fmt.Sprintf(
		"Still working after %s, so this subagent moved to the background as %q. "+
			"Keep going; its report arrives under that label when it finishes. "+
			"Check on it with %s, steer it with %s, or stop it with %s.",
		after, label, AgentStatusToolName, AgentSendToolName, AgentStopToolName,
	)
	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse(notice),
		AgentBackgroundMetadata{
			Label:        label,
			Backgrounded: true,
			Access:       access,
			Task:         prompt,
		},
	)
}
