package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
)

const (
	// observeMinTokens is the least amount of new conversation worth a
	// distillation call. Below it the observer would spend a model call per
	// trivial turn to record nothing durable, which is the cost this feature
	// is least able to justify.
	observeMinTokens = 800
	// observeMaxTokens bounds the transcript handed to the observer. It is a
	// cheap model reading for facts, not a reasoner reconciling a whole
	// session, so a long backlog is observed across several turns instead.
	observeMaxTokens = 24_000
	// observeTimeout bounds one distillation. Local models on modest hardware
	// are slow; this is deliberately generous because nothing waits on it.
	observeTimeout = 4 * time.Minute
)

const observeSystemPrompt = `You distill a coding session into durable memory.

Record only what a future reader could not recover from the transcript itself:

- decisions and the reason for them, including approaches that were rejected
- constraints, preferences, and deadlines the user stated
- what the session is trying to achieve, when it is stated or changes
- bugs traced to a cause, migrations applied, work validated

Do not record routine steps. "Read a file", "ran tests", "listed a directory"
are recoverable and worthless as memory. Do not speculate: if the session did
not establish it, it is not an observation.

Answer with a JSON array and nothing else. Each element:

  {"kind": "observation"|"reflection", "text": "...", "relevance": 0|1|2,
   "sources": ["<message id>", ...]}

kind "reflection" is for a durable conclusion drawn from several observations,
citing their message ids; "observation" is for one concrete thing.
relevance 2 is for a decision, a rejected approach, or a standing constraint,
1 for a notable fact, 0 for background.
sources must name the message ids the entry rests on, copied exactly.
Prefer a few high-value entries over many. An empty array is a valid answer.`

// observeTurn records what the ledger has not yet seen. It is fired from the
// single terminal path for a turn and returns immediately, because a
// distillation is slow and nothing about the turn should wait for it.
//
// Every failure here is a no-op by design: the messages stay unobserved and the
// next turn picks them up. Memory that occasionally lags is a smaller problem
// than a turn that fails because bookkeeping did.
func (a *sessionAgent) observeTurn(complete notify.RunComplete) {
	if a.ledger == nil || !a.observeMemory || a.isSubAgent {
		return
	}
	// The terminal payload names the session, and it is the only field
	// guaranteed to be set: reading the session off the call instead would
	// silently observe nothing when a caller left it empty.
	sessionID := complete.SessionID
	if complete.Cancelled || sessionID == "" {
		return
	}

	// Detached from the run: the turn is over, and its context is about to be
	// cancelled. A bounded, cancellable context keeps a slow local model from
	// outliving the session's usefulness.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), observeTimeout)

	go func() {
		defer cancel()
		defer log.RecoverPanic("agent.observeTurn", func() {})

		if err := a.distill(ctx, sessionID); err != nil {
			slog.Debug("Memory observation skipped", "session", sessionID, "err", err)
		}
	}()
}

// distill gathers the unseen conversation and appends what the observer makes
// of it.
func (a *sessionAgent) distill(ctx context.Context, sessionID string) error {
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("list messages: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}

	current, err := a.ledger.Ledger(ctx, sessionID)
	if err != nil {
		return err
	}

	// Unobserved rather than "messages after a position": an entry cites the
	// messages it rests on, so coverage is recorded by what was actually read.
	// A turn whose distillation failed stays visible and is retried.
	ids := make([]string, 0, len(msgs))
	byID := make(map[string]message.Message, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.ID)
		byID[m.ID] = m
	}
	unseen := current.Unobserved(ids)
	if len(unseen) == 0 {
		return nil
	}

	// Only the newest unseen messages are read at once; the rest wait for
	// later turns so one enormous backlog cannot blow the observer's budget.
	selected := tailWithinTokens(unseen, byID, observeMaxTokens)
	if compaction.EstimateAll(selected) < observeMinTokens && len(unseen) == len(selected) {
		return nil
	}

	transcript := renderForObservation(selected)
	if strings.TrimSpace(transcript) == "" {
		return nil
	}

	raw, err := a.askObserver(ctx, sessionID, transcript)
	if err != nil {
		return err
	}
	entries, err := parseObservations(raw, byID)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		// A batch the observer found nothing durable in still has to be marked
		// read. Without this the same messages look unobserved on every later
		// turn and the observer pays a model call each time to reach the same
		// conclusion.
		entries = []compaction.Entry{{
			Kind:    compaction.KindDrop,
			Text:    "observed; nothing durable to record",
			Sources: idsOf(selected),
		}}
	}

	// Reading a message and citing it are different things: the observer reads
	// a whole batch and cites only what it used. Anything read but not cited is
	// marked covered, or the next turn reads it again to reach the same
	// conclusion - the same wasted call, one batch at a time.
	if uncited := uncitedOf(selected, entries); len(uncited) > 0 {
		entries = append(entries, compaction.Entry{
			Kind:    compaction.KindDrop,
			Text:    "observed; nothing durable to record",
			Sources: uncited,
		})
	}

	stored, err := a.ledger.Append(ctx, sessionID, entries)
	if err != nil {
		return err
	}
	slog.Debug("Memory observed", "session", sessionID, "entries", len(stored))
	return nil
}

// uncitedOf lists the messages a pass read that no entry claims to rest on.
func uncitedOf(selected []message.Message, entries []compaction.Entry) []string {
	cited := make(map[string]bool)
	for _, e := range entries {
		for _, id := range e.Sources {
			cited[id] = true
		}
	}
	var uncited []string
	for _, m := range selected {
		if !cited[m.ID] {
			uncited = append(uncited, m.ID)
		}
	}
	return uncited
}

// askObserver runs the distillation on the small model. Unlike title
// generation there is no large-model fallback: a bad or failed observation is
// retried next turn for free, so escalating to an expensive model to avoid a
// delay nobody is waiting on would be the wrong trade.
func (a *sessionAgent) askObserver(ctx context.Context, sessionID, transcript string) (string, error) {
	model := a.smallModel.Get()
	if model.Model == nil {
		return "", fmt.Errorf("no small model configured")
	}

	agent := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(observeSystemPrompt+"\n /no_think"),
		fantasy.WithMaxOutputTokens(2048),
		fantasy.WithUserAgent(userAgent),
	)

	resp, err := agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt:  transcript,
		Headers: sessionHeaders(sessionID),
		PrepareStep: func(callCtx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			return callCtx, prepared, nil
		},
	})
	if err != nil {
		return "", fmt.Errorf("observe: %w", err)
	}
	if resp.Response.FinishReason == fantasy.FinishReasonLength {
		return "", fmt.Errorf("observe: model hit its output limit")
	}
	return resp.Response.Content.Text(), nil
}

// observation is one entry as the model is asked to write it.
type observation struct {
	Kind      string   `json:"kind"`
	Text      string   `json:"text"`
	Relevance int      `json:"relevance"`
	Sources   []string `json:"sources"`
}

// parseObservations reads the model's array, tolerating the code fences and
// trailing prose models add despite being asked for bare JSON.
func parseObservations(raw string, known map[string]message.Message) ([]compaction.Entry, error) {
	body := extractJSONArray(raw)
	if body == "" {
		return nil, nil
	}
	var parsed []observation
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, fmt.Errorf("observe: unreadable entries: %w", err)
	}

	entries := make([]compaction.Entry, 0, len(parsed))
	for _, o := range parsed {
		text := strings.TrimSpace(o.Text)
		if text == "" {
			continue
		}
		kind := compaction.KindObservation
		if o.Kind == string(compaction.KindReflection) {
			kind = compaction.KindReflection
		}
		// Drop sources the session does not have. A cited id that never
		// existed would make the ledger unverifiable against the transcript,
		// which is the one property that keeps it trustworthy.
		var sources []string
		for _, id := range o.Sources {
			if _, ok := known[id]; ok {
				sources = append(sources, id)
			}
		}
		entries = append(entries, compaction.Entry{
			Kind:      kind,
			Text:      text,
			Relevance: clampRelevance(o.Relevance),
			Sources:   sources,
		})
	}
	return entries, nil
}

func clampRelevance(r int) compaction.Relevance {
	switch {
	case r >= int(compaction.RelevanceDecision):
		return compaction.RelevanceDecision
	case r <= int(compaction.RelevanceContext):
		return compaction.RelevanceContext
	default:
		return compaction.RelevanceNotable
	}
}

// extractJSONArray returns the outermost bracketed region of the model's
// answer, or "" when there is none.
func extractJSONArray(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

// tailWithinTokens selects the newest messages that fit a budget, so a long
// backlog is observed over several turns instead of truncating in the middle
// of the work it describes.
func tailWithinTokens(ids []string, byID map[string]message.Message, budget int64) []message.Message {
	var (
		selected []message.Message
		total    int64
	)
	for i := len(ids) - 1; i >= 0; i-- {
		msg, ok := byID[ids[i]]
		if !ok {
			continue
		}
		cost := compaction.Estimate(msg)
		if total+cost > budget && len(selected) > 0 {
			break
		}
		total += cost
		selected = append(selected, msg)
	}
	// Collected newest-first for budgeting; read oldest-first.
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected
}

// renderForObservation writes the conversation for the observer, citing each
// message's id so entries can point back at what they rest on. It is terser
// than the checkpoint transcript: this reader is looking for facts, and tool
// output is where a cheap model wastes its attention.
func renderForObservation(msgs []message.Message) string {
	const maxPartBytes = 2_000
	var b strings.Builder
	for _, msg := range msgs {
		fmt.Fprintf(&b, "--- id=%s role=%s\n", msg.ID, msg.Role)
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case message.TextContent:
				writeCapped(&b, p.Text, maxPartBytes)
			case message.ToolCall:
				fmt.Fprintf(&b, "tool_call %s: ", p.Name)
				writeCapped(&b, p.Input, maxPartBytes)
			case message.ToolResult:
				fmt.Fprintf(&b, "tool_result %s", p.Name)
				if p.IsError {
					b.WriteString(" (error)")
				}
				b.WriteString(": ")
				writeCapped(&b, p.Content, maxPartBytes)
			case message.ShellCommand:
				fmt.Fprintf(&b, "shell %q exit=%d: ", p.Command, p.ExitCode)
				writeCapped(&b, p.Output, maxPartBytes)
			}
		}
	}
	return b.String()
}

// idsOf lists the messages an observation pass read, so a batch that produced
// no entries can still be recorded as covered.
func idsOf(msgs []message.Message) []string {
	ids := make([]string, 0, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	return ids
}

func writeCapped(b *strings.Builder, s string, limit int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if len(s) > limit {
		s = s[:limit] + "… [truncated]"
	}
	b.WriteString(s)
	b.WriteByte('\n')
}
