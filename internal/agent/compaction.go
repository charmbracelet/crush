package agent

import (
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compact"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// algorithmicCompaction reports whether summaries are written from an
// algorithmic extraction of the conversation instead of the whole of it.
func algorithmicCompaction() bool {
	return os.Getenv("CRUSH_ALGORITHMIC_COMPACT") == "1"
}

// summaryInput is what the summary model reads: an algorithmic extraction of
// the conversation when that is enabled and finds anything, else the whole
// conversation.
func (a *sessionAgent) summaryInput(sess session.Session, msgs []message.Message, model Model, thinking bool) (string, []fantasy.Message) {
	if algorithmicCompaction() {
		var previous string
		history := msgs
		if sess.SummaryMessageID != "" && len(msgs) > 0 && msgs[0].ID == sess.SummaryMessageID {
			previous, history = msgs[0].Content().Text, msgs[1:]
		}
		if extracted := compact.Compact(compact.Input{Messages: history, PreviousSummary: previous}); extracted != "" {
			return buildSummaryPrompt(extracted, sess.Todos, thinking), nil
		}
	}
	aiMsgs, _ := a.preparePrompt(msgs, model.CatwalkCfg.Capabilities.Vision, model.CatwalkCfg.Capabilities.Audio)
	return buildSummaryPrompt("", sess.Todos, thinking), aiMsgs
}

// analysisRouter routes streamed <analysis>...</analysis> text to the
// reasoning display and the rest to content. Tags can span deltas, so a
// possible partial tag at the end is held back until the next one.
type analysisRouter struct {
	onReasoning, onContent func(string) error
	inAnalysis             bool
	pending                string
}

func newAnalysisRouter(onReasoning, onContent func(string) error) *analysisRouter {
	return &analysisRouter{onReasoning: onReasoning, onContent: onContent}
}

func (r *analysisRouter) side() (tag string, emit func(string) error) {
	if r.inAnalysis {
		return "</analysis>", r.onReasoning
	}
	return "<analysis>", r.onContent
}

func (r *analysisRouter) write(delta string) error {
	r.pending += delta
	for {
		tag, emit := r.side()
		idx := strings.Index(r.pending, tag)
		if idx < 0 {
			keep := partialTagLen(r.pending, tag)
			if err := emitText(emit, r.pending[:len(r.pending)-keep]); err != nil {
				return err
			}
			r.pending = r.pending[len(r.pending)-keep:]
			return nil
		}
		if err := emitText(emit, r.pending[:idx]); err != nil {
			return err
		}
		if r.inAnalysis {
			// Separate one reasoning block from the next.
			if err := r.onReasoning("\n"); err != nil {
				return err
			}
		}
		r.pending = r.pending[idx+len(tag):]
		r.inAnalysis = !r.inAnalysis
	}
}

// flush emits whatever is still held back.
func (r *analysisRouter) flush() error {
	_, emit := r.side()
	text := r.pending
	r.pending = ""
	return emitText(emit, text)
}

func emitText(emit func(string) error, text string) error {
	if text == "" {
		return nil
	}
	return emit(text)
}

// partialTagLen is the length of the longest suffix of s that starts tag.
func partialTagLen(s, tag string) int {
	for n := min(len(s), len(tag)-1); n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}

// buildSummaryPrompt asks for a summary of the conversation above, or of
// extracted when it is set.
func buildSummaryPrompt(extracted string, todos []session.Todo, thinking bool) string {
	var sb strings.Builder
	if extracted == "" {
		sb.WriteString("Summarize our conversation above following the instructions in the system prompt.\n")
	} else {
		sb.WriteString("Below is a structured extraction from our conversation. ")
		sb.WriteString("Use it as your sole source of truth to write a handoff summary following the system prompt instructions.\n\n")
		sb.WriteString("## Extracted Context\n\n")
		sb.WriteString(extracted)
		sb.WriteString("\n\n")
	}
	if thinking {
		sb.WriteString("Use your reasoning/thinking block to analyze what to preserve before writing the summary.")
	} else {
		sb.WriteString("Wrap your analysis in <analysis> tags before writing the summary. These will be displayed as thinking.")
	}
	if len(todos) > 0 {
		sb.WriteString("\n\n## Current Todo List\n\n")
		for _, t := range todos {
			fmt.Fprintf(&sb, "- [%s] %s\n", t.Status, t.Content)
		}
		sb.WriteString("\nInclude these tasks and their statuses in your summary. ")
		sb.WriteString("Instruct the resuming assistant to use the `todos` tool to continue tracking progress on these tasks.")
	}
	return sb.String()
}
