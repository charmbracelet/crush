package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/ledger"
	"github.com/charmbracelet/crush/internal/message"
)

const RecallToolName = "recall"

//go:embed recall.md.tpl
var recallDescription string

// RecallParams is the call a model makes to follow a memory back to its
// sources.
type RecallParams struct {
	// ID is the bracketed id printed beside a memory entry in a checkpoint.
	ID string `json:"id" description:"The memory entry id, exactly as printed in brackets beside the entry, e.g. e-1f2a3b4c5d6e7f80"`
}

// NewRecallTool reads recorded session memory and the messages behind it. It is
// only registered when a session actually records memory, so a session without
// it never pays for the description in its prompt.
func NewRecallTool(memory ledger.Service, messages message.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RecallToolName,
		recallDescription,
		func(ctx context.Context, params RecallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session in context"), nil
			}
			return fantasy.NewTextResponse(recallMemory(ctx, memory, messages, sessionID, params.ID)), nil
		},
	)
}

// recallMemory renders one entry and the sources it rests on. Everything a
// caller could be misled by is stated rather than omitted: whether the entry is
// still active, and which of its sources can no longer be read.
func recallMemory(ctx context.Context, memory ledger.Service, messages message.Service, sessionID, id string) string {
	wanted := strings.TrimSpace(id)
	if wanted == "" {
		return "No id given. Pass the bracketed id printed beside a memory entry."
	}

	current, err := memory.Ledger(ctx, sessionID)
	if err != nil {
		return fmt.Sprintf("Could not read recorded memory: %v", err)
	}

	entry, active := findEntry(current, wanted)
	if entry == nil {
		return fmt.Sprintf("No memory entry with id %q in this session. Ids are printed in brackets beside each entry of a checkpoint.", wanted)
	}

	var b strings.Builder
	status := "active"
	if !active {
		status = "dropped"
	}
	fmt.Fprintf(&b, "Memory %s (%s, %s, relevance %d)\n\n%s\n",
		entry.ID, status, entry.Kind, entry.Relevance, entry.Text)

	if len(entry.Sources) == 0 {
		b.WriteString("\nNo sources recorded for this entry.\n")
		return b.String()
	}

	// The transcript is the only place an exact quote, path, or error string
	// still exists, so the sources are quoted rather than summarized.
	b.WriteString("\nSources:\n")
	var missing []string
	for _, sourceID := range entry.Sources {
		msg, err := messages.Get(ctx, sourceID)
		if err != nil {
			missing = append(missing, sourceID)
			continue
		}
		fmt.Fprintf(&b, "\n--- %s (%s)\n%s\n", sourceID, msg.Role, recallSourceText(msg))
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\nUnavailable sources (gone from the transcript): %s\n",
			strings.Join(missing, ", "))
	}
	return b.String()
}

// findEntry locates an entry by id, including one that has been retired. A
// dropped entry is still evidence: the reader is told it is dropped, not that
// it never existed.
func findEntry(l compaction.Ledger, id string) (*compaction.Entry, bool) {
	retired := make(map[int]bool)
	for _, e := range l.Entries {
		if e.Kind != compaction.KindDrop {
			continue
		}
		for _, seq := range e.Retires {
			retired[seq] = true
		}
	}
	for i := range l.Entries {
		if l.Entries[i].ID != id {
			continue
		}
		return &l.Entries[i], !retired[l.Entries[i].Seq]
	}
	return nil, false
}

// recallSourceText flattens a message to what a reader needs, capping tool
// output that could otherwise be enormous.
func recallSourceText(msg message.Message) string {
	const maxPart = 4_000
	var b strings.Builder
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			writeRecallPart(&b, p.Text, maxPart)
		case message.ToolCall:
			fmt.Fprintf(&b, "tool_call %s: ", p.Name)
			writeRecallPart(&b, p.Input, maxPart)
		case message.ToolResult:
			fmt.Fprintf(&b, "tool_result %s", p.Name)
			if p.IsError {
				b.WriteString(" (error)")
			}
			b.WriteString(": ")
			writeRecallPart(&b, p.Content, maxPart)
		case message.ShellCommand:
			fmt.Fprintf(&b, "shell %q exit=%d: ", p.Command, p.ExitCode)
			writeRecallPart(&b, p.Output, maxPart)
		case message.ReasoningContent:
			writeRecallPart(&b, p.Thinking, maxPart)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeRecallPart(b *strings.Builder, s string, limit int) {
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
