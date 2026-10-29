package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
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

// backgroundLabel labels an auto-backgrounded subagent after its tool call.
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

	label := c.names.name(backgroundLabel(toolName, params.ToolCallID))
	childSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	entry := newDispatchedAgent(label, access, childSessionID, params.SessionID, params.Agent)

	// The background goroutine rolls the child's cost up itself, as a
	// dispatched subagent does, so the synchronous rollup is turned off.
	params.SkipCostRollup = true

	key := dispatchKey(params.SessionID, label)
	c.dispatchMu.Lock()
	c.dispatched.Set(key, entry)
	runCtx, gen, _ := entry.claim()
	done := entry.runInline(gen)
	c.dispatchMu.Unlock()

	c.runDispatched(runCtx, entry, gen, func(ctx context.Context) (fantasy.ToolResponse, error) {
		return c.runSubAgent(ctx, params)
	})

	// A call that finishes in the foreground leaves nothing in the registry.
	takeInline := func() (fantasy.ToolResponse, error) {
		c.dispatchMu.Lock()
		if current, ok := c.dispatched.Get(key); ok && current == entry {
			c.dispatched.Del(key)
		}
		c.dispatchMu.Unlock()
		return entry.result()
	}

	timer := time.NewTimer(c.backgroundAfter)
	defer timer.Stop()

	select {
	case <-done:
		return takeInline()
	case <-timer.C:
		if !entry.detach(gen) {
			return takeInline()
		}
		return backgroundNotice(label, access, params.Prompt, c.backgroundAfter), nil
	case <-ctx.Done():
		// Cancelled before the handoff: stop it and surface the
		// cancellation, as a blocking call would; no report is written.
		entry.stop("the parent turn was cancelled")
		<-done
		takeInline()
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
