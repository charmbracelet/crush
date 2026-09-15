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
			case tools.WriteToolNames[tr.ToolName] && tr.ClientMetadata != "":
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
