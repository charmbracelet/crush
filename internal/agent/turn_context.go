package agent

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

const (
	// turnContextWorkingSetLimit bounds the working-set section of the
	// per-turn blob. The read set is cumulative — unbounded it
	// degenerates to "every file ever touched".
	turnContextWorkingSetLimit = 10
	// vaguePromptMaxWords bounds the vagueness pre-filter: prompts
	// longer than this carry enough of their own context that a
	// missing referent is unlikely.
	vaguePromptMaxWords = 12
)

// vagueReferentRe matches prompts that lean on a definite or anaphoric
// referent whose target context must supply — "the bug", "it", "this
// crash". The noun list is deliberately referent-shaped: "run the
// tests" stays actionable on its own, "fix the bug" does not.
var vagueReferentRe = regexp.MustCompile(`(?i)\b(it|its|this|that|them|they)\b|` +
	`\bthe\s+(bug|bugfix|crash|error|errors|failure|fail|issue|problem|panic|regression|leak|typo|warnings?|` +
	`config|configuration|test|tests|spec|endpoint|handler|route|feature|changes?|fix|workaround|hack|todo|fixme)\b`)

// isVaguePrompt reports whether the prompt is underspecified in the way
// the pre-filter cares about: short enough to carry no context of its
// own, naming no explicit file paths, and leaning on a referent.
func isVaguePrompt(prompt string) bool {
	n := len(strings.Fields(prompt))
	if n == 0 || n > vaguePromptMaxWords {
		return false
	}
	if len(extractExplicitFilePaths(prompt)) > 0 {
		return false
	}
	return vagueReferentRe.MatchString(prompt)
}

// turnTailMessages returns the ephemeral per-turn tail messages — the
// turn-context blob and, when the vagueness pre-filter fires, the
// clarify directive. They are computed once per Run and appended to
// prepared.Messages inside PrepareStep so they survive the notebook
// rebuild and stay byte-stable across the turn's steps: the history
// prefix still cache-reads and the blob rides in the tail that is new
// anyway.
func (a *sessionAgent) turnTailMessages(ctx context.Context, call SessionAgentCall, msgs []message.Message) []fantasy.Message {
	var out []fantasy.Message
	if blob := a.turnContextBlob(ctx, call); blob != "" {
		out = append(out, fantasy.NewSystemMessage(blob))
	}
	if directive := a.ambiguityDirective(ctx, call, msgs); directive != "" {
		out = append(out, fantasy.NewSystemMessage(directive))
	}
	if len(out) > 0 {
		var bytes int
		for _, m := range out {
			for _, p := range m.Content {
				if tp, ok := p.(fantasy.TextPart); ok {
					bytes += len(tp.Text)
				}
			}
		}
		slog.Debug("Turn tail augmentation",
			"session_id", call.SessionID,
			"sections", len(out),
			"bytes", bytes,
		)
	}
	return out
}

// turnContextBlob renders the <turn_context> blob for the session and
// semantic tiers — deterministic session signals, each in a labeled
// section, appended at the request tail. The semantic tier currently
// resolves to the same signals; meaning-based retrieval plugs in here
// when it lands. Returns "" when the tier is off, the agent is a
// sub-agent, or no signal has content.
func (a *sessionAgent) turnContextBlob(ctx context.Context, call SessionAgentCall) string {
	if (a.turnContext != "session" && a.turnContext != "semantic") || a.isSubAgent {
		return ""
	}
	var b strings.Builder

	if a.filetracker != nil {
		if files, err := a.filetracker.ListRecentReadFiles(ctx, call.SessionID, turnContextWorkingSetLimit); err == nil && len(files) > 0 {
			b.WriteString("<working_set>\nRecently read or edited files — the most likely referents for \"the file\", \"the bug\", and similar:\n")
			for _, f := range files {
				fmt.Fprintf(&b, "- %s\n", a.relWorkdir(f))
			}
			b.WriteString("</working_set>\n")
		}
	}

	if a.sessions != nil {
		if sess, err := a.sessions.Get(ctx, call.SessionID); err == nil {
			var open []session.Todo
			for _, t := range sess.Todos {
				if t.Status != session.TodoStatusCompleted {
					open = append(open, t)
				}
			}
			if len(open) > 0 {
				b.WriteString("<open_todos>\nDeclared work items still open:\n")
				const maxListedTodos = 10
				for i, t := range open {
					if i >= maxListedTodos {
						fmt.Fprintf(&b, "- … and %d more\n", len(open)-maxListedTodos)
						break
					}
					fmt.Fprintf(&b, "- [%s] %s\n", t.Status, t.Content)
				}
				b.WriteString("</open_todos>\n")
			}
		}
	}

	if b.Len() == 0 {
		return ""
	}
	return "<turn_context>\n" + b.String() + "</turn_context>"
}

// relWorkdir renders p relative to the working directory when possible,
// keeping the blob short and prompt-portable.
func (a *sessionAgent) relWorkdir(p string) string {
	if a.configStore == nil {
		return p
	}
	if rel, err := filepath.Rel(a.configStore.WorkingDir(), p); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

// ambiguityDirective implements the turn-zero vagueness pre-filter: a
// short, referent-leaning prompt with no explicit paths and nothing in
// session context to resolve against takes the forced
// clarify-or-state-assumptions path. The gate fires only when no
// resolvable signal exists — a working set or earlier user text means
// the referent has candidates — and stays opt-in behind
// options.ambiguity_clarification.
func (a *sessionAgent) ambiguityDirective(ctx context.Context, call SessionAgentCall, msgs []message.Message) string {
	if !a.ambiguityClarification || a.isSubAgent || !isVaguePrompt(call.Prompt) {
		return ""
	}
	// Earlier user text in the session can supply the referent.
	if hasUserTextMessage(msgs) {
		return ""
	}
	// A non-empty working set gives the referent candidates.
	if a.filetracker != nil {
		if files, err := a.filetracker.ListRecentReadFiles(ctx, call.SessionID, 1); err == nil && len(files) > 0 {
			return ""
		}
	}
	if a.hasTool(tools.QuestionToolName) {
		return `<ambiguity_gate>
The user's request appears underspecified: it names no files and this
session has no working set or earlier context to resolve the referent
from. Resolve it before executing:

- If a quick search can identify the referent, do so and state the
  assumption in one line.
- Otherwise call the question tool ONCE with a single focused
  clarifying question — single_choice with per-choice tradeoffs, or
  free_text. If the user cannot answer, proceed with your stated-best
  option.
</ambiguity_gate>`
	}
	// Headless degradation: no question tool means the clause collapses
	// to "state assumptions, proceed" — an unanswerable question must
	// degrade, never stall.
	return `<ambiguity_gate>
The user's request appears underspecified: it names no files and this
session has no working set or earlier context to resolve the referent
from. You cannot ask the user in this run — make the most reasonable
assumption, state it in one line, and proceed.
</ambiguity_gate>`
}
