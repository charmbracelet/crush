package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// maxVerificationAttempts bounds the gate's retry loop — the number of
// verification-repair turns permitted after the original turn. It is the
// only bound: the loop detector resets per Run and sees a different
// signature each retry anyway.
const maxVerificationAttempts = 2

// gateCheckOutcome pairs a verification check with the tool call that
// recorded it, so a resolved outcome lands back on the originating
// result's stored metadata. stepIndex records where in the run the entry
// was seen, for observed-bash postdating.
type gateCheckOutcome struct {
	toolCallID string
	stepIndex  int
	check      message.VerificationCheck
	output     string
}

// observedBash is a bash tool run seen in this run's steps, used to
// satisfy a pending check without re-running it.
type observedBash struct {
	stepIdx int
	command string
	isError bool
	output  string
}

// resolvedCheck is a gate-run check's terminal outcome.
type resolvedCheck struct {
	state  string
	detail string
	output string
}

// runVerificationGate implements the end-of-turn verification gate. When
// the run ended on a clean stop with failed or pending checks — or left
// session todos open — it resolves the checks, lands outcomes on stored
// tool-result metadata, and — within budget — prepends a retry call
// carrying the check output and open items. Returns true when a retry
// was queued so the caller can suppress the finished notification.
func (a *sessionAgent) runVerificationGate(ctx context.Context, call SessionAgentCall, result *fantasy.AgentResult, currentAssistant *message.Message) bool {
	if a.configStore == nil || result == nil || len(result.Steps) == 0 {
		return false
	}
	terminal := result.Steps[len(result.Steps)-1]
	if terminal.Response.FinishReason != fantasy.FinishReasonStop {
		return false
	}
	// A StopTurn ending — hook halt, permission denial, question tool —
	// is not a completion claim; do not gate it.
	for _, tr := range terminal.Content.ToolResults() {
		if tr.StopTurn {
			return false
		}
	}

	failed, pending, observed := scanVerification(result.Steps)

	if len(pending) > 0 {
		unique := map[string]bool{}
		for _, p := range pending {
			unique[p.check.Check] = true
		}
		a.notifyVerifying(call, len(unique))
		resolved := a.runGateChecks(ctx, a.configStore.WorkingDir(), pending, observed)
		if ctx.Err() != nil {
			// Cancelled mid-gate: leave pending entries pending (the
			// notebook maps them to unverified) rather than writing
			// failed verdicts for checks that never completed.
			return false
		}
		for i := range pending {
			out, ok := resolved[pending[i].check.Check]
			if !ok {
				continue
			}
			pending[i].check.State = out.state
			pending[i].check.Detail = out.detail
			pending[i].output = out.output
		}
		// Persist the resolved states onto the originating results, then
		// flush: the post-run goroutine's List reads storage directly and
		// would otherwise miss a debounced metadata-only update.
		a.writeVerificationOutcomes(ctx, call.SessionID, pending)
		if err := a.messages.FlushAll(ctx); err != nil {
			slog.Error("Failed to flush verification outcomes", "error", err, "session_id", call.SessionID)
		}
		for _, p := range pending {
			if p.check.State == message.VerificationFailed {
				failed = append(failed, p)
			}
		}
	}

	// The session's todo list is the model's own declared scope: a
	// clean stop that leaves items open is the same premature-done
	// claim a failed check is — reconcile before the turn counts.
	openTodos := a.incompleteTodos(ctx, call.SessionID)

	if len(failed) == 0 && len(openTodos) == 0 {
		return false
	}

	if call.VerificationAttempts >= maxVerificationAttempts {
		// Budget exhausted: surface the terminal state on the final
		// assistant message — it is the last assistant message of the
		// run, so the text reaches RunComplete.Text for `crush run`.
		if currentAssistant != nil {
			var note strings.Builder
			if len(failed) > 0 {
				headline := failed[0].check.Detail
				if headline == "" {
					headline = firstLine(failed[0].output)
				}
				fmt.Fprintf(&note, "%d check(s) still failing after %d attempt(s). Last failure: %s",
					len(failed), call.VerificationAttempts, headline)
			}
			if len(openTodos) > 0 {
				if note.Len() > 0 {
					note.WriteString(" ")
				}
				fmt.Fprintf(&note, "%d todo item(s) still incomplete after %d attempt(s).",
					len(openTodos), call.VerificationAttempts)
			}
			currentAssistant.AppendContent("\n\nVerification: " + note.String())
			if err := a.messages.Update(ctx, *currentAssistant); err != nil {
				slog.Error("Failed to record verification exhaustion", "error", err, "session_id", call.SessionID)
			} else if err := a.messages.FlushAll(ctx); err != nil {
				// Same flush race as the outcome writes: a fast notebook
				// goroutine would generate this turn's entries without
				// the exhaustion line.
				slog.Error("Failed to flush verification exhaustion", "error", err, "session_id", call.SessionID)
			}
		}
		return false
	}

	// Clone the caller's call — ProviderOptions, sampling params,
	// NonInteractive, and OnAuthRefresh all carry through — with the
	// prompt replaced and the budget incremented. The same RunID keeps
	// the retry non-foldable and suppresses the premature RunComplete;
	// Accepted/acceptSeq are cleared so a cancel mark drops it.
	retry := call
	retry.Prompt = gateRetryPrompt(failed, openTodos)
	retry.VerificationAttempts++
	retry.Accepted = nil
	retry.acceptSeq = 0

	mu := a.sessionMu(call.SessionID)
	mu.Lock()
	existing, _ := a.messageQueue.Get(call.SessionID)
	a.messageQueue.Set(call.SessionID, append([]SessionAgentCall{retry}, existing...))
	mu.Unlock()
	return true
}

// scanVerification walks the run's steps collecting write-tool
// verification entries by state plus observed bash runs for
// satisfy-from-observed matching.
func scanVerification(steps []fantasy.StepResult) (failed, pending []gateCheckOutcome, observed []observedBash) {
	// jobLaunches maps a background job's shell_id to the step the bash
	// tool launched it on. A job_output poll is not when the command
	// ran — the launch step is what a poll verdict must postdate.
	jobLaunches := map[string]int{}
	for stepIdx, step := range steps {
		bashCmds := map[string]string{}
		for _, tc := range step.Content.ToolCalls() {
			if tc.ToolName == tools.BashToolName {
				bashCmds[tc.ToolCallID] = gjson.Get(tc.Input, "command").String()
			}
		}
		for _, tr := range step.Content.ToolResults() {
			switch {
			case tr.ToolName == tools.BashToolName || tr.ToolName == tools.JobOutputToolName:
				shellID := gjson.Get(tr.ClientMetadata, "shell_id").String()
				done := gjson.Get(tr.ClientMetadata, "done").Bool()
				if tr.ToolName == tools.BashToolName && shellID != "" && !done {
					// Still-running background start: not a verdict —
					// record the launch step for a later job_output.
					jobLaunches[shellID] = stepIdx
					continue
				}
				// The verdict lives in metadata: a non-zero exit is a
				// text response, never an error result. `done` marks a
				// completed run — a still-running command or a poll of
				// one is not a verdict.
				if !done &&
					(tr.Result == nil || tr.Result.GetType() != fantasy.ToolResultContentTypeError) {
					continue
				}
				cmd := bashCmds[tr.ToolCallID]
				verdictStep := stepIdx
				if tr.ToolName == tools.JobOutputToolName {
					// JobOutput's input is a shell_id; the command
					// comes from its response metadata, and the step
					// the verdict postdates from is the job's launch.
					cmd = gjson.Get(tr.ClientMetadata, "command").String()
					launch, ok := jobLaunches[shellID]
					if !ok {
						// Launched in an earlier run — predates every
						// write this run made.
						verdictStep = -1
					} else {
						verdictStep = launch
					}
				}
				if cmd == "" {
					continue
				}
				isErr := tr.Result != nil && tr.Result.GetType() == fantasy.ToolResultContentTypeError
				if ec := gjson.Get(tr.ClientMetadata, "exit_code"); ec.Exists() {
					isErr = ec.Int() != 0
				}
				observed = append(observed, observedBash{
					stepIdx: verdictStep,
					command: cmd,
					isError: isErr,
					output:  toolResultText(tr),
				})
			case writeToolNames[tr.ToolName] && tr.ClientMetadata != "":
				var meta struct {
					Verification []message.VerificationCheck `json:"verification"`
				}
				if err := json.Unmarshal([]byte(tr.ClientMetadata), &meta); err != nil {
					continue
				}
				for _, chk := range meta.Verification {
					outcome := gateCheckOutcome{
						toolCallID: tr.ToolCallID,
						stepIndex:  stepIdx,
						check:      chk,
						// Carry the result text so a retry prompt is
						// self-contained — a decorator-failed check's
						// detail is only "N new error(s)"; the errors
						// themselves live on the result.
						output: toolResultText(tr),
					}
					switch chk.State {
					case message.VerificationFailed:
						failed = append(failed, outcome)
					case message.VerificationPending:
						pending = append(pending, outcome)
					}
				}
			}
		}
	}
	return failed, pending, observed
}

// runGateChecks dedups pending checks by identity, satisfies each from
// an observed bash run of the exact command when one postdates the
// pending, and otherwise executes it harness-side under a per-check
// timeout derived from the run context.
func (a *sessionAgent) runGateChecks(ctx context.Context, workingDir string, pending []gateCheckOutcome, observed []observedBash) map[string]resolvedCheck {
	type uniqueCheck struct {
		check      message.VerificationCheck
		latestStep int
	}
	var uniques []uniqueCheck
	seen := map[string]int{}
	for _, p := range pending {
		if i, ok := seen[p.check.Check]; ok {
			// An observed run must postdate the LAST write the check
			// covers — keep the latest step the pending was recorded on.
			if p.stepIndex > uniques[i].latestStep {
				uniques[i].latestStep = p.stepIndex
			}
			continue
		}
		seen[p.check.Check] = len(uniques)
		uniques = append(uniques, uniqueCheck{check: p.check, latestStep: p.stepIndex})
	}

	out := map[string]resolvedCheck{}
	for _, uc := range uniques {
		cmd := uc.check.Command
		if cmd == "" {
			out[uc.check.Check] = resolvedCheck{state: message.VerificationUnverified, detail: "no command to run"}
			continue
		}
		satisfied := false
		for _, b := range observed {
			// Exact command match only — a prefix match invites
			// `cmd && rm -rf` trickery — and the run must postdate the
			// writes it covers.
			if b.command == cmd && b.stepIdx > uc.latestStep {
				st := message.VerificationPassed
				if b.isError {
					st = message.VerificationFailed
				}
				out[uc.check.Check] = resolvedCheck{state: st, detail: "observed via bash run", output: b.output}
				satisfied = true
				break
			}
		}
		if satisfied {
			continue
		}

		timeout := time.Duration(uc.check.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 120 * time.Second
		}
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		res, err := shell.RunAndCapture(checkCtx, shell.RunOptions{
			Command: cmd,
			Cwd:     workingDir,
		})
		cancel()
		switch {
		case ctx.Err() != nil:
			// The run was cancelled mid-check — stop resolving and leave
			// the rest pending; a cancelled run must not record a failed
			// verdict for a check that never completed.
			return out
		case checkCtx.Err() == context.DeadlineExceeded:
			// The deadline fired — a kill may surface as a bare exit
			// code with no error, so check the context first.
			out[uc.check.Check] = resolvedCheck{
				state:  message.VerificationFailed,
				detail: fmt.Sprintf("timed out after %s", timeout),
				output: res.Output,
			}
		case err != nil:
			out[uc.check.Check] = resolvedCheck{
				state:  message.VerificationFailed,
				detail: err.Error(),
				output: res.Output,
			}
		case res.ExitCode != 0:
			out[uc.check.Check] = resolvedCheck{
				state:  message.VerificationFailed,
				detail: fmt.Sprintf("exit code %d", res.ExitCode),
				output: res.Output,
			}
		default:
			out[uc.check.Check] = resolvedCheck{state: message.VerificationPassed, output: res.Output}
		}
	}
	return out
}

// writeVerificationOutcomes resolves pending entries on the stored
// tool-result messages, using the mergeSupersededMarks read-modify-write
// so a concurrent flagging update cannot clobber the verdicts.
func (a *sessionAgent) writeVerificationOutcomes(ctx context.Context, sessionID string, resolved []gateCheckOutcome) {
	if len(resolved) == 0 {
		return
	}
	byCall := map[string][]message.VerificationCheck{}
	for _, o := range resolved {
		byCall[o.toolCallID] = append(byCall[o.toolCallID], o.check)
	}
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		slog.Error("Failed to list messages for verification write-back", "error", err, "session_id", sessionID)
		return
	}
	for i := range msgs {
		m := &msgs[i]
		if m.Role != message.Tool {
			continue
		}
		changed := false
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok {
				continue
			}
			checks, ok := byCall[tr.ToolCallID]
			if !ok {
				continue
			}
			tr.Metadata = mergeVerificationResolved(tr.Metadata, checks)
			m.Parts[j] = tr
			changed = true
		}
		if changed {
			a.mergeSupersededMarks(ctx, m)
			if err := a.messages.Update(ctx, *m); err != nil {
				slog.Error("Failed to write verification outcome", "error", err, "session_id", sessionID)
			}
		}
	}
}

// mergeVerificationResolved replaces pending entries in an existing
// verification list by check identity, preserving unrelated metadata keys.
func mergeVerificationResolved(existing string, updates []message.VerificationCheck) string {
	var checks []message.VerificationCheck
	if raw := gjson.Get(existing, "verification"); raw.Exists() {
		_ = json.Unmarshal([]byte(raw.Raw), &checks)
	}
	for _, u := range updates {
		replaced := false
		for i := range checks {
			if checks[i].Check == u.Check {
				checks[i] = u
				replaced = true
			}
		}
		if !replaced {
			checks = append(checks, u)
		}
	}
	data, err := json.Marshal(checks)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "verification", string(data))
	if err != nil {
		return existing
	}
	return merged
}

// unionToolMetadata merges stored metadata keys the incoming copy lacks —
// stored first, incoming wins on conflicts — so a whole-message rewrite
// built from a pre-verification snapshot cannot drop the key. The
// "verification" list merges state-aware: a stored terminal verdict
// (passed/failed/unverified) does not regress to pending when the
// incoming copy was snapshotted before the gate resolved it.
func unionToolMetadata(stored, incoming string) string {
	if stored == "" {
		return incoming
	}
	if incoming == "" {
		return stored
	}
	merged := stored
	for k, v := range gjson.Parse(incoming).Map() {
		out, err := sjson.SetRaw(merged, k, v.Raw)
		if err == nil {
			merged = out
		}
	}
	if checks := unionVerificationChecks(
		gjson.Get(stored, "verification"),
		gjson.Get(incoming, "verification"),
	); checks != "" {
		if out, err := sjson.SetRaw(merged, "verification", checks); err == nil {
			merged = out
		}
	}
	return merged
}

// unionVerificationChecks merges two verification lists by check
// identity. An entry present only on one side is kept; where both sides
// carry the same check, a resolved stored state beats a stale pending —
// the race this guards is a whole-message rewrite built from a pre-gate
// snapshot, which would otherwise regress the verdict.
func unionVerificationChecks(stored, incoming gjson.Result) string {
	if !stored.Exists() {
		return ""
	}
	order := []string{}
	merged := map[string]message.VerificationCheck{}
	for _, s := range []gjson.Result{stored, incoming} {
		for _, raw := range s.Array() {
			var chk message.VerificationCheck
			if json.Unmarshal([]byte(raw.Raw), &chk) != nil || chk.Check == "" {
				continue
			}
			prev, ok := merged[chk.Check]
			if !ok {
				order = append(order, chk.Check)
				merged[chk.Check] = chk
				continue
			}
			// Terminal states stick; otherwise the later (incoming) copy
			// wins so a re-flag can update detail/output.
			if prev.State != message.VerificationPending {
				continue
			}
			merged[chk.Check] = chk
		}
	}
	checks := make([]message.VerificationCheck, 0, len(order))
	for _, name := range order {
		checks = append(checks, merged[name])
	}
	data, err := json.Marshal(checks)
	if err != nil {
		return ""
	}
	return string(data)
}

// notifyVerifying publishes the verify-in-progress notification so the
// TUI can say why the session is still busy while checks run.
func (a *sessionAgent) notifyVerifying(call SessionAgentCall, n int) {
	if call.NonInteractive || a.notify == nil {
		return
	}
	a.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		SessionID: call.SessionID,
		RunID:     call.RunID,
		Type:      notify.TypeVerifying,
		Message:   fmt.Sprintf("Running %d verification check(s)", n),
	})
}

// incompleteTodos returns the session's open todo items — the model's
// own declared checklist. Returns nil when the toolset is unknown (nil)
// or lacks the todos tool: a model that cannot write the list cannot
// reconcile it, and the retry would be a guaranteed thrash.
func (a *sessionAgent) incompleteTodos(ctx context.Context, sessionID string) []session.Todo {
	if a.sessions == nil || a.tools == nil {
		return nil
	}
	if !slices.ContainsFunc(a.tools.Copy(), func(t fantasy.AgentTool) bool {
		return t.Info().Name == tools.TodosToolName
	}) {
		return nil
	}
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		slog.Error("Failed to load session todos for verification gate", "error", err, "session_id", sessionID)
		return nil
	}
	var open []session.Todo
	for _, t := range sess.Todos {
		if t.Status != session.TodoStatusCompleted {
			open = append(open, t)
		}
	}
	return open
}

// gateRetryPrompt builds the retry prompt from the failed checks' raw
// output (truncated to the tool-result cap) plus any todo items left
// open — both are evidence the turn claimed done prematurely.
func gateRetryPrompt(failed []gateCheckOutcome, openTodos []session.Todo) string {
	var b strings.Builder
	if len(failed) > 0 {
		b.WriteString("Verification failed. The following check(s) did not pass — fix the underlying issue; do not restate success.\n")
		for _, f := range failed {
			fmt.Fprintf(&b, "\n<check name=%q>\n", f.check.Check)
			out := f.output
			if out == "" {
				out = f.check.Detail
			}
			b.WriteString(tools.TruncateOutput(out))
			b.WriteString("\n</check>\n")
		}
	}
	if len(openTodos) > 0 {
		b.WriteString("\nThe todo list still has incomplete item(s) — a turn is not done while its declared tasks are open:\n")
		const maxListedTodos = 20
		for i, t := range openTodos {
			if i >= maxListedTodos {
				fmt.Fprintf(&b, "- … and %d more\n", len(openTodos)-maxListedTodos)
				break
			}
			fmt.Fprintf(&b, "- [%s] %s\n", t.Status, t.Content)
		}
		b.WriteString("Finish the remaining work, or reconcile the list with the todos tool (mark genuinely done items completed; drop abandoned ones). Do not report the task finished while the list says otherwise.\n")
	}
	return b.String()
}

// toolResultText extracts text from a fantasy tool result for check
// output purposes.
func toolResultText(tr fantasy.ToolResultContent) string {
	switch out := tr.Result.(type) {
	case fantasy.ToolResultOutputContentText:
		return out.Text
	case fantasy.ToolResultOutputContentError:
		if out.Error != nil {
			return out.Error.Error()
		}
	}
	return ""
}

// firstLine returns the first line of s, for failure headlines.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
