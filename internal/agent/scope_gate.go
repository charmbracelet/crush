package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/question"
)

// scopeGateMinExploration is the explore→execute boundary: a first
// mutating call that arrives only after this many exploration calls
// marks a task whose scope is worth confirming. Routine-size tasks —
// a handful of reads before the edit — pass un-gated.
const scopeGateMinExploration = 8

// scopeGateState is the per-session gate bookkeeping for one run. A new
// run stamp resets it: each user turn gets one boundary check.
type scopeGateState struct {
	stamp    uint64
	explore  int
	asking   bool
	resolved bool
}

// gateVerdict is observe's tri-state: pass the call through, hold it
// while a scope question is already in flight, or confirm scope with
// the user before the write executes.
type gateVerdict int

const (
	gatePass gateVerdict = iota
	gateWait
	gateConfirm
)

// scopeGate wraps the tool list to intercept the first mutating call of
// a run when deep exploration suggests a non-routine scope. The gate
// asks one structured question — proceed, narrow, or stop — and then
// resolves for the rest of the run: the confirmation is a checkpoint,
// not a toll booth. Tools the model uses to externalize a plan (todos,
// question) satisfy the gate without asking.
//
// Headless runs build no gate: a question nobody can answer must
// degrade, never stall.
type scopeGate struct {
	svc    question.Service
	mu     sync.Mutex
	states map[string]*scopeGateState
}

// wrapToolsWithScopeGate wraps every tool so the gate sees exploration
// calls as well as writes. Only mutating calls are ever intercepted.
// Returns the slice unchanged when the service is nil (non-interactive
// runs, tests) or the flag is off.
func wrapToolsWithScopeGate(all []fantasy.AgentTool, svc question.Service) []fantasy.AgentTool {
	if svc == nil {
		return all
	}
	g := &scopeGate{svc: svc, states: map[string]*scopeGateState{}}
	out := make([]fantasy.AgentTool, len(all))
	for i, tool := range all {
		out[i] = &scopeGateTool{inner: tool, gate: g}
	}
	return out
}

// observe records one tool call against the session's run state and
// reports the gate's verdict. A new run stamp resets the state — the
// boundary is per turn, not per session. gateConfirm claims the one
// in-flight question slot (the service supports a single pending
// question); a parallel gated write in the same step gets gateWait and
// is told to re-issue after the question resolves.
func (g *scopeGate) observe(ctx context.Context, toolName string) gateVerdict {
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return gatePass
	}
	stamp := tools.GetRunStampFromContext(ctx)

	g.mu.Lock()
	defer g.mu.Unlock()
	st, ok := g.states[sessionID]
	if !ok || st.stamp != stamp {
		st = &scopeGateState{stamp: stamp}
		g.states[sessionID] = st
	}
	if writeToolNames[toolName] {
		switch {
		case st.resolved || st.explore < scopeGateMinExploration:
			return gatePass
		case st.asking:
			return gateWait
		default:
			st.asking = true
			return gateConfirm
		}
	}
	// A declared plan or an in-flight question already externalized the
	// scope decision — the gate is satisfied for the rest of the run.
	if toolName == tools.TodosToolName || toolName == tools.QuestionToolName {
		st.resolved = true
		return gatePass
	}
	st.explore++
	return gatePass
}

// resolve marks the session's current run gated — the checkpoint fired
// once, whatever the answer.
func (g *scopeGate) resolve(ctx context.Context) {
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if st, ok := g.states[sessionID]; ok {
		st.asking = false
		st.resolved = true
	}
}

// confirm asks the scope question. The choice descriptions are where
// the tradeoffs live.
func (g *scopeGate) confirm(ctx context.Context, explore int) (proceed bool, err error) {
	answers, err := g.svc.Ask(ctx, question.Request{
		SessionID: tools.GetSessionFromContext(ctx),
		Questions: []question.Question{{
			Type:  question.TypeSingleChoice,
			Label: "Scope check",
			Text:  fmt.Sprintf("This task has explored %d steps without a declared plan. Confirm scope before the first write?", explore),
			Description: "The exploration so far suggests a non-routine change. " +
				"Confirming means the plan is worth a checkpoint; you can also narrow the scope or stop to restate the task.",
			Choices: []question.Choice{
				{ID: "proceed", Label: "Proceed", Description: "Scope looks right — run the planned writes."},
				{ID: "narrow", Label: "Narrow scope", Description: "Do less: the agent restates a smaller plan before writing."},
				{ID: "stop", Label: "Stop", Description: "End the turn so you can restate the task."},
			},
		}},
	})
	if err != nil {
		return false, err
	}
	for _, a := range answers {
		for _, id := range a.SelectedIDs {
			if id == "proceed" {
				return true, nil
			}
			if id == "stop" {
				return false, question.ErrCancelled
			}
		}
	}
	return false, nil
}

// scopeGateTool is the decorator half of scopeGate: it feeds calls to
// observe and blocks the gated write on the user's answer.
type scopeGateTool struct {
	inner fantasy.AgentTool
	gate  *scopeGate
}

// Unwrap returns the wrapped tool, matching hookedTool's convention.
func (t *scopeGateTool) Unwrap() fantasy.AgentTool {
	return t.inner
}

func (t *scopeGateTool) Info() fantasy.ToolInfo {
	return t.inner.Info()
}

func (t *scopeGateTool) ProviderOptions() fantasy.ProviderOptions {
	return t.inner.ProviderOptions()
}

func (t *scopeGateTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.inner.SetProviderOptions(opts)
}

func (t *scopeGateTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	switch t.gate.observe(ctx, call.Name) {
	case gatePass:
		return t.inner.Run(ctx, call)
	case gateWait:
		return fantasy.NewTextErrorResponse(
			"Scope check in progress — re-issue this call after the pending question resolves.",
		), nil
	}

	proceed, err := t.gate.confirm(ctx, scopeGateMinExploration)
	t.gate.resolve(ctx)
	switch {
	case err == nil && proceed:
		return t.inner.Run(ctx, call)
	case errors.Is(err, question.ErrCancelled):
		resp := fantasy.NewTextErrorResponse("User stopped at the scope check")
		resp.StopTurn = true
		return resp, nil
	case err != nil:
		// A degraded question path must not stall the run: treat the
		// failure as "you decide" and let the write through.
		return t.inner.Run(ctx, call)
	default:
		return fantasy.NewTextErrorResponse(
			"Scope check: the user asked to narrow the plan. Restate a smaller scope " +
				"with the todos tool, then re-issue the write.",
		), nil
	}
}
