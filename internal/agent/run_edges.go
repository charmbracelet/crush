package agent

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// maxRepairAttempts bounds the shared repair-turn budget: the number of
// harness-initiated follow-up turns permitted after the original turn.
// Every firing edge draws from this one counter — unbudgeted escalation
// is nagging. The loop detector is not a second bound: it resets per Run
// and sees a different signature each retry anyway.
const maxRepairAttempts = 2

// edgeInput is the finished-run state every edge's scan inspects. It is
// assembled once at the run boundary — after Stream returns, before the
// queue dequeues.
type edgeInput struct {
	result           *fantasy.AgentResult
	currentAssistant *message.Message
	// stalled records that the run ended because the loop detector's
	// StopWhen fired — the signature repetition signal the edge layer
	// turns into a replan/escalation turn instead of a silent stop.
	stalled bool
}

// edgeTrigger is one edge's scan output: the evidence a retry prompt or
// exhaustion note renders from. The verification edge fills the check
// fields, the todos edge fills todos, and fire marks whether the edge
// still wants a repair turn after resolve ran.
type edgeTrigger struct {
	failed   []gateCheckOutcome
	pending  []gateCheckOutcome
	observed []observedBash
	todos    []session.Todo
	fire     bool
}

// A runEdge is a deterministic transition evaluated at the run boundary.
// scan inspects the finished run and returns a trigger candidate, or nil
// when the edge cannot apply. resolve performs the edge's deterministic
// work — running checks, persisting outcomes — and runs even when the
// trigger will not produce a retry (resolved evidence must land
// regardless); it may clear t.fire when resolution shows nothing to
// repair. prompt renders the retry-prompt section; note renders the
// budget-exhausted terminal line.
type runEdge struct {
	name    string
	scan    func(ctx context.Context, call SessionAgentCall, in edgeInput) *edgeTrigger
	resolve func(ctx context.Context, call SessionAgentCall, t *edgeTrigger)
	prompt  func(t *edgeTrigger) string
	note    func(t *edgeTrigger, attempts int) string
}

// runEdgeSet is the evaluated order: verification and todos are the
// original gate halves; stall converts a loop-detector stop into a
// replan turn. All firing edges merge into ONE retry prompt — two edges
// each enqueueing a turn would double every repair.
func (a *sessionAgent) runEdgeSet() []runEdge {
	return []runEdge{
		{
			name:    "verification",
			scan:    a.scanVerificationEdge,
			resolve: a.resolveVerificationEdge,
			prompt:  verificationRetrySection,
			note:    verificationExhaustNote,
		},
		{
			name:   "todos",
			scan:   a.scanTodosEdge,
			prompt: todosRetrySection,
			note:   todosExhaustNote,
		},
		{
			name:   "stall",
			scan:   a.scanStallEdge,
			prompt: a.stallRetrySection,
			note:   stallExhaustNote,
		},
	}
}

// runEdges evaluates every edge at the run boundary. Firing edges merge
// their evidence into one budgeted retry call, prepended ahead of queued
// prompts. Returns true when a retry was queued so the caller can
// suppress the finished notification.
func (a *sessionAgent) runEdges(ctx context.Context, call SessionAgentCall, in edgeInput) bool {
	if a.configStore == nil || in.result == nil || len(in.result.Steps) == 0 {
		return false
	}

	var prompts, notes []string
	for _, edge := range a.runEdgeSet() {
		t := edge.scan(ctx, call, in)
		if t == nil {
			continue
		}
		if edge.resolve != nil {
			edge.resolve(ctx, call, t)
			if ctx.Err() != nil {
				// Cancelled mid-resolution: enqueue nothing. Partial
				// outcomes are already persisted; pending entries stay
				// pending rather than recording verdicts for checks
				// that never completed.
				return false
			}
		}
		if !t.fire {
			continue
		}
		if p := edge.prompt(t); p != "" {
			prompts = append(prompts, p)
		}
		if edge.note != nil {
			if n := edge.note(t, call.RepairAttempts); n != "" {
				notes = append(notes, n)
			}
		}
	}
	if len(prompts) == 0 {
		return false
	}

	if call.RepairAttempts >= maxRepairAttempts {
		// Budget exhausted: surface the terminal state on the final
		// assistant message — it is the last assistant message of the
		// run, so the text reaches RunComplete.Text for `crush run`.
		a.writeRepairExhaustion(ctx, call, in.currentAssistant, notes)
		return false
	}

	// Clone the caller's call — ProviderOptions, sampling params,
	// NonInteractive, and OnAuthRefresh all carry through — with the
	// prompt replaced and the budget incremented. The same RunID keeps
	// the retry non-foldable and suppresses the premature RunComplete;
	// Accepted/acceptSeq are cleared so a cancel mark drops it.
	retry := call
	retry.Prompt = strings.Join(prompts, "\n")
	retry.RepairAttempts++
	retry.Accepted = nil
	retry.acceptSeq = 0

	mu := a.sessionMu(call.SessionID)
	mu.Lock()
	existing, _ := a.messageQueue.Get(call.SessionID)
	a.messageQueue.Set(call.SessionID, append([]SessionAgentCall{retry}, existing...))
	mu.Unlock()
	return true
}

// cleanStop reports whether the run ended on a genuine completion claim:
// the terminal step finished with reason stop, and no tool result halted
// the turn (a hook halt, permission denial, or question-tool answer is
// not a completion claim and is never gated).
func cleanStop(in edgeInput) bool {
	if in.result == nil || len(in.result.Steps) == 0 {
		return false
	}
	terminal := in.result.Steps[len(in.result.Steps)-1]
	if terminal.FinishReason != fantasy.FinishReasonStop {
		return false
	}
	for _, tr := range terminal.Content.ToolResults() {
		if tr.StopTurn {
			return false
		}
	}
	return true
}

// writeRepairExhaustion appends the merged exhaustion notes to the
// final assistant message so the terminal state is visible in the TUI
// and reaches RunComplete.Text for `crush run`.
func (a *sessionAgent) writeRepairExhaustion(ctx context.Context, call SessionAgentCall, currentAssistant *message.Message, notes []string) {
	if currentAssistant == nil || len(notes) == 0 {
		return
	}
	currentAssistant.AppendContent("\n\nVerification: " + strings.Join(notes, " "))
	if err := a.messages.Update(ctx, *currentAssistant); err != nil {
		slog.Error("Failed to record verification exhaustion", "error", err, "session_id", call.SessionID)
	} else if err := a.messages.FlushAll(ctx); err != nil {
		// Same flush race as the outcome writes: a fast notebook
		// goroutine would generate this turn's entries without
		// the exhaustion line.
		slog.Error("Failed to flush verification exhaustion", "error", err, "session_id", call.SessionID)
	}
}

// --- verification edge ---

// scanVerificationEdge collects this run's write-tool verification
// entries. Pending checks start unfired: resolve decides whether any of
// them resolve to failed.
func (a *sessionAgent) scanVerificationEdge(_ context.Context, _ SessionAgentCall, in edgeInput) *edgeTrigger {
	if !cleanStop(in) {
		return nil
	}
	failed, pending, observed := scanVerification(in.result.Steps)
	if len(failed) == 0 && len(pending) == 0 {
		return nil
	}
	return &edgeTrigger{
		failed:   failed,
		pending:  pending,
		observed: observed,
		fire:     len(failed) > 0,
	}
}

// resolveVerificationEdge runs the pending checks harness-side,
// persists the resolved outcomes onto the stored tool results, and
// refires the edge when any pending check resolved to failed.
func (a *sessionAgent) resolveVerificationEdge(ctx context.Context, call SessionAgentCall, t *edgeTrigger) {
	if len(t.pending) == 0 {
		return
	}
	unique := map[string]bool{}
	for _, p := range t.pending {
		unique[p.check.Check] = true
	}
	a.notifyVerifying(call, len(unique))
	resolved := a.runGateChecks(ctx, a.configStore.WorkingDir(), t.pending, t.observed)
	if ctx.Err() != nil {
		// Cancelled mid-gate: leave pending entries pending (the
		// notebook maps them to unverified) rather than writing
		// failed verdicts for checks that never completed.
		return
	}
	for i := range t.pending {
		out, ok := resolved[t.pending[i].check.Check]
		if !ok {
			continue
		}
		t.pending[i].check.State = out.state
		t.pending[i].check.Detail = out.detail
		t.pending[i].output = out.output
	}
	// Persist the resolved states onto the originating results, then
	// flush: the post-run goroutine's List reads storage directly and
	// would otherwise miss a debounced metadata-only update.
	a.writeVerificationOutcomes(ctx, call.SessionID, t.pending)
	if err := a.messages.FlushAll(ctx); err != nil {
		slog.Error("Failed to flush verification outcomes", "error", err, "session_id", call.SessionID)
	}
	for _, p := range t.pending {
		if p.check.State == message.VerificationFailed {
			t.failed = append(t.failed, p)
		}
	}
	t.fire = len(t.failed) > 0
}

// verificationRetrySection renders the failed checks' raw output
// (truncated to the tool-result cap) — evidence the turn claimed done
// prematurely.
func verificationRetrySection(t *edgeTrigger) string {
	if len(t.failed) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Verification failed. The following check(s) did not pass — fix the underlying issue; do not restate success.\n")
	for _, f := range t.failed {
		fmt.Fprintf(&b, "\n<check name=%q>\n", f.check.Check)
		out := f.output
		if out == "" {
			out = f.check.Detail
		}
		b.WriteString(tools.TruncateOutput(out))
		b.WriteString("\n</check>\n")
	}
	return b.String()
}

// verificationExhaustNote renders the budget-exhausted line for checks
// that never went green.
func verificationExhaustNote(t *edgeTrigger, attempts int) string {
	if len(t.failed) == 0 {
		return ""
	}
	headline := t.failed[0].check.Detail
	if headline == "" {
		headline = firstLine(t.failed[0].output)
	}
	return fmt.Sprintf("%d check(s) still failing after %d attempt(s). Last failure: %s",
		len(t.failed), attempts, headline)
}

// --- todos edge ---

// scanTodosEdge fires when a clean stop leaves session todos open — the
// model's own declared scope says the turn is not done.
func (a *sessionAgent) scanTodosEdge(ctx context.Context, call SessionAgentCall, in edgeInput) *edgeTrigger {
	if !cleanStop(in) {
		return nil
	}
	open := a.incompleteTodos(ctx, call.SessionID)
	if len(open) == 0 {
		return nil
	}
	return &edgeTrigger{todos: open, fire: true}
}

// todosRetrySection renders the open todo items left behind by a turn
// that reported done.
func todosRetrySection(t *edgeTrigger) string {
	if len(t.todos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("The todo list still has incomplete item(s) — a turn is not done while its declared tasks are open:\n")
	const maxListedTodos = 20
	for i, todo := range t.todos {
		if i >= maxListedTodos {
			fmt.Fprintf(&b, "- … and %d more\n", len(t.todos)-maxListedTodos)
			break
		}
		fmt.Fprintf(&b, "- [%s] %s\n", todo.Status, todo.Content)
	}
	b.WriteString("Finish the remaining work, or reconcile the list with the todos tool (mark genuinely done items completed; drop abandoned ones). Do not report the task finished while the list says otherwise.\n")
	return b.String()
}

// todosExhaustNote renders the budget-exhausted line for open items.
func todosExhaustNote(t *edgeTrigger, attempts int) string {
	if len(t.todos) == 0 {
		return ""
	}
	return fmt.Sprintf("%d todo item(s) still incomplete after %d attempt(s).",
		len(t.todos), attempts)
}

// --- stall edge ---

// scanStallEdge fires when the loop detector stopped the run — the
// repeated-signature signal is evidence of no progress, and a silent
// stop is the wrong action for it. The escalation is opt-in via
// options.ambiguity_clarification: one structured escalation per
// blocker, drawn from the shared repair budget.
func (a *sessionAgent) scanStallEdge(_ context.Context, _ SessionAgentCall, in edgeInput) *edgeTrigger {
	if !in.stalled || !a.ambiguityClarification {
		return nil
	}
	return &edgeTrigger{fire: true}
}

// stallRetrySection renders the escalation prompt for a loop-stopped
// run. When the question tool is available the escalation is structured
// — what was tried, what's blocking, options with tradeoffs; in runs
// without it (non-interactive, sub-agents) the turn replans instead:
// state the blocker hypothesis and take a materially different path.
func (a *sessionAgent) stallRetrySection(_ *edgeTrigger) string {
	if a.hasTool(tools.QuestionToolName) {
		return "The previous attempt was stopped: the same tool calls repeated without making progress. " +
			"Do not retry the same approach. Escalate with ONE question-tool call — a single_choice question " +
			"naming what you tried and what is blocking, with each choice describing the tradeoff of that " +
			"way forward. If the user cannot answer, proceed with your stated-best option."
	}
	return "The previous attempt was stopped: the same tool calls repeated without making progress. " +
		"You cannot ask the user in this run. State your best hypothesis about the blocker in one line, " +
		"then take a materially different approach — a different command, search term, tool, or scope. " +
		"If no approach remains, report what you tried, what is blocking, and the minimal external action required."
}

// stallExhaustNote renders the budget-exhausted line for a run that
// stalled again after its repair turns.
func stallExhaustNote(_ *edgeTrigger, attempts int) string {
	return fmt.Sprintf("Stopped after repeating identical tool calls; %d repair attempt(s) exhausted.", attempts)
}

// hasTool reports whether the agent's current toolset includes the
// named tool — the check for whether a run can actually ask questions.
func (a *sessionAgent) hasTool(name string) bool {
	return slices.ContainsFunc(a.tools.Copy(), func(t fantasy.AgentTool) bool {
		return t.Info().Name == name
	})
}
