package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
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

// scopeGateState is the per-session gate bookkeeping for one turn — one
// entry per session ID, negligible growth. A new run stamp resets it;
// repair retries share the turn's stamp via SessionAgentCall.RunStamp
// carried through the retry clone, so each user turn gets one boundary
// check.
type scopeGateState struct {
	stamp    uint64
	explore  int
	asking   bool
	resolved bool
}

// gateVerdict is observe's tri-state: pass the call through, hold it
// while a scope question is already in flight, or confirm scope with
// the user before the write executes. gateWait is defensive — parallel
// tools (download, fetch, web_*, MCP reads) run concurrently under
// parallelSem, and a parallel-classified mutating tool like download
// could observe st.asking mid-question if the fantasy executor ever
// pipelines a step's calls; it stays as insurance.
type gateVerdict int

const (
	gatePass gateVerdict = iota
	gateWait
	gateConfirm
)

// scopeGate wraps the tool list to intercept the first mutating call of
// a run when deep exploration suggests a non-routine scope. In
// interactive runs the gate asks one structured question — proceed,
// narrow, or stop — and then resolves for the rest of the run: the
// confirmation is a checkpoint, not a toll booth. Tools the model uses
// to externalize a plan (todos, question) satisfy the gate without
// asking. In non-interactive runs the same boundary degrades to
// proceed-with-logged-assumption — a question nobody can answer must
// never stall.
//
// Known routes around the checkpoint, accepted by design: delegating
// the write to a task agent (the agent tool isn't a mutating call and
// sub-agent toolsets are unwrapped), and mutating MCP tools whose
// effects can't be classified. The gate is a heuristic checkpoint for
// scope confirmation, not a security boundary.
type scopeGate struct {
	svc         question.Service
	interactive bool
	mu          sync.Mutex
	states      map[string]*scopeGateState
}

// newScopeGate builds the gate. Returns nil when an interactive run
// has no question service to ask through.
func newScopeGate(svc question.Service, interactive bool) *scopeGate {
	if interactive && svc == nil {
		return nil
	}
	return &scopeGate{svc: svc, interactive: interactive, states: map[string]*scopeGateState{}}
}

// wrap decorates every tool so the gate sees exploration calls as well
// as writes. Only mutating calls are ever intercepted. The gate object
// is long-lived — it survives SetTools rebuilds so per-turn
// exploration bookkeeping and the resolved mark persist across wraps.
func (g *scopeGate) wrap(all []fantasy.AgentTool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(all))
	for i, tool := range all {
		out[i] = &scopeGateTool{inner: tool, gate: g}
	}
	return out
}

// mutatingBashRe matches shell commands that mutate files or git state
// — the "large, destructive, hard to reverse" calls that must not
// bypass the gate just because they arrive through bash instead of a
// write tool. Deliberately conservative in both directions: mutations
// hidden inside scripts or build targets (make, go generate) pass
// un-gated, and read-ish commands that merely touch state (git config
// --get) stay exploration. A false positive costs one confirmation
// question; a false negative skips the checkpoint.
//
// The scan runs on the raw command text — command names inside quoted
// spans still match ("bash -c 'rm -rf /'" gates) — while the redirect
// check below masks quoted spans so "echo 'a > b'" stays exploration.
var mutatingBashRe = regexp.MustCompile(`\b(rm|rmdir|mv|cp|dd|truncate|shred|chmod|chown|chgrp|ln|tee|patch|install|touch|mkdir|rsync|scp)\b|` +
	`\b(sed|perl)\s+(-\S+\s+)*(-\S*i|-i\S*|--in-place)\b|` +
	`\bgit\s+(commit|push|reset|checkout|switch|restore|clean|rebase|merge|am|apply|stash|tag|revert|cherry-pick|mv|rm|init|clone|pull|bisect|submodule|update-ref|notes|branch\s+-[dDmM])\b|` +
	`\bapt(-get)?\s+(install|remove|purge|upgrade|update|dist-upgrade)\b|` +
	`\bkubectl\s+(delete|apply|create|patch|edit|replace|scale|drain|cordon|uncordon)\b`)

// redirectTargetRe finds shell redirects and their targets; writing to
// a real file mutates it, while fd duplication and /dev/null do not.
var redirectTargetRe = regexp.MustCompile(`>>?\s*(\S+)`)

// fdDupTargetRe matches the fd-duplication redirect targets that are
// not file writes — `>&1`, `>&-` — as opposed to `>&out`, which is
// bash's stdout+stderr-to-file form and does mutate.
var fdDupTargetRe = regexp.MustCompile(`^&[-\d]`)

// quotedSpanRe masks single- and double-quoted spans before the
// redirect scan: a `>` inside a string literal must not gate, while a
// quoted *target* (`> 'out'`) still counts — masking to a placeholder
// keeps the target position occupied.
var quotedSpanRe = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// isMutatingCall classifies a call as a write for gate purposes: a
// write-tool name, a file-writing download, or a bash command whose
// text matches a mutating pattern or a file-writing redirect.
func isMutatingCall(call fantasy.ToolCall) bool {
	if tools.WriteToolNames[call.Name] || call.Name == tools.DownloadToolName {
		return true
	}
	if call.Name != "bash" {
		return false
	}
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil || params.Command == "" {
		return false
	}
	if mutatingBashRe.MatchString(params.Command) {
		return true
	}
	for _, m := range redirectTargetRe.FindAllStringSubmatch(quotedSpanRe.ReplaceAllString(params.Command, "f"), -1) {
		if m[1] != "/dev/null" && !fdDupTargetRe.MatchString(m[1]) {
			return true
		}
	}
	return false
}

// observe records one tool call against the session's run state and
// reports the gate's verdict plus the exploration count the verdict was
// reached at. A new run stamp resets the state — the boundary is per
// turn, not per session. gateConfirm claims the one in-flight question
// slot (the service supports a single pending question); a parallel
// gated write in the same step gets gateWait and is told to re-issue
// after the question resolves.
func (g *scopeGate) observe(ctx context.Context, call fantasy.ToolCall) (gateVerdict, int) {
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return gatePass, 0
	}
	stamp := tools.GetRunStampFromContext(ctx)

	g.mu.Lock()
	defer g.mu.Unlock()
	st, ok := g.states[sessionID]
	if !ok || st.stamp != stamp {
		st = &scopeGateState{stamp: stamp}
		g.states[sessionID] = st
	}
	if isMutatingCall(call) {
		switch {
		case st.resolved || st.explore < scopeGateMinExploration:
			return gatePass, st.explore
		case st.asking:
			return gateWait, st.explore
		default:
			st.asking = true
			return gateConfirm, st.explore
		}
	}
	// A declared plan or an in-flight question already externalized the
	// scope decision — the gate is satisfied for the rest of the run.
	if call.Name == tools.TodosToolName || call.Name == tools.QuestionToolName {
		st.resolved = true
		return gatePass, st.explore
	}
	st.explore++
	return gatePass, st.explore
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
// the tradeoffs live. The question service keeps a single pending
// question globally — a concurrent Ask from another session would
// clobber it; that hazard pre-dates the gate (the question tool
// exposes it too) and same-session serialization keeps it from
// firing here.
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
	verdict, explore := t.gate.observe(ctx, call)
	switch verdict {
	case gatePass:
		return t.inner.Run(ctx, call)
	case gateWait:
		return fantasy.NewTextErrorResponse(
			"Scope check in progress — re-issue this call after the pending question resolves.",
		), nil
	}

	if !t.gate.interactive {
		// Headless degrade: proceed with a logged assumption — the
		// boundary is observed and recorded, never asked.
		t.gate.resolve(ctx)
		slog.Info("Scope gate: proceeding on stated assumptions",
			"tool", call.Name,
			"session_id", tools.GetSessionFromContext(ctx),
		)
		return t.inner.Run(ctx, call)
	}

	proceed, err := t.gate.confirm(ctx, explore)
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
