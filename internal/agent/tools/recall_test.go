package tools

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/ledger"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func recallEnv(t *testing.T) (context.Context, ledger.Service, message.Service, string) {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, sess.ID)
	return ctx, ledger.NewService(q), messages, sess.ID
}

// TestRecallReturnsTheEntryAndItsSources is the point of the tool: a memory
// line in a checkpoint is a claim, and this is how a reader gets the evidence
// behind it rather than having to trust the summary.
func TestRecallReturnsTheEntryAndItsSources(t *testing.T) {
	t.Parallel()

	ctx, memory, messages, sessionID := recallEnv(t)

	source, err := messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "do not split the ledger across sessions, it must stay per-session"}},
	})
	require.NoError(t, err)

	stored, err := memory.Append(ctx, sessionID, []compaction.Entry{{
		Kind:      compaction.KindReflection,
		Relevance: compaction.RelevanceDecision,
		Text:      "Ledger stays session-scoped",
		Sources:   []string{source.ID},
	}})
	require.NoError(t, err)

	out := recallMemory(ctx, memory, messages, sessionID, stored[0].ID)

	require.Contains(t, out, "Ledger stays session-scoped")
	require.Contains(t, out, "active")
	require.Contains(t, out, "reflection")
	require.Contains(t, out, "it must stay per-session",
		"the exact wording must come back, not a paraphrase")
}

// TestRecallStillAnswersForARetiredEntry: a dropped entry is history, not a
// mistake. Reporting it as missing would tell a reader its evidence never
// existed, when the truth is that it no longer rides along in checkpoints.
func TestRecallStillAnswersForARetiredEntry(t *testing.T) {
	t.Parallel()

	ctx, memory, messages, sessionID := recallEnv(t)

	stored, err := memory.Append(ctx, sessionID, []compaction.Entry{
		{Kind: compaction.KindObservation, Relevance: compaction.RelevanceContext, Text: "Listed a directory"},
	})
	require.NoError(t, err)
	_, err = memory.Append(ctx, sessionID, []compaction.Entry{{
		Kind: compaction.KindDrop, Text: "decayed", Retires: []int{stored[0].Seq},
	}})
	require.NoError(t, err)

	out := recallMemory(ctx, memory, messages, sessionID, stored[0].ID)
	require.Contains(t, out, "Listed a directory")
	require.Contains(t, out, "dropped")
}

func TestRecallReportsAMissingSourceHonestly(t *testing.T) {
	t.Parallel()

	ctx, memory, messages, sessionID := recallEnv(t)

	stored, err := memory.Append(ctx, sessionID, []compaction.Entry{{
		Kind:    compaction.KindObservation,
		Text:    "Rests on a message that no longer exists",
		Sources: []string{"gone-forever"},
	}})
	require.NoError(t, err)

	out := recallMemory(ctx, memory, messages, sessionID, stored[0].ID)
	require.Contains(t, out, "Unavailable sources")
	require.Contains(t, out, "gone-forever")
}

func TestRecallSaysWhenTheIdIsUnknown(t *testing.T) {
	t.Parallel()

	ctx, memory, messages, sessionID := recallEnv(t)

	out := recallMemory(ctx, memory, messages, sessionID, "e-0000000000000000")
	require.Contains(t, out, "No memory entry with id")

	empty := recallMemory(ctx, memory, messages, sessionID, "")
	require.Contains(t, empty, "No id given")
}

func TestRecallToolIsConstructible(t *testing.T) {
	t.Parallel()

	_, memory, messages, _ := recallEnv(t)
	require.NotNil(t, NewRecallTool(memory, messages))
}
