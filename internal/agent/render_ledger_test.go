package agent

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/stretchr/testify/require"
)

// attachMemory turns a test agent into one that renders checkpoints from the
// ledger, the state a real session reaches with observe_memory and
// render_from_ledger both enabled.
func attachMemory(sa SessionAgent, env fakeEnv) *sessionAgent {
	a := sa.(*sessionAgent)
	a.ledger = env.ledger
	a.renderFromLedger = true
	return a
}

// TestRenderedCheckpointSkipsTheModel is the load-bearing claim of the ledger:
// when memory holds the session's decisions, a compaction builds its checkpoint
// from them without a model call, so the moment that used to pause the session
// for a rewrite costs one string concat instead.
func TestRenderedCheckpointSkipsTheModel(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "a summary nobody should pay for"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	_, err = env.ledger.Append(ctx, sess.ID, []compaction.Entry{
		{
			Kind: compaction.KindObservation, Relevance: compaction.RelevanceDecision,
			Text: "User rejected splitting the ledger across sessions",
		},
		{
			Kind: compaction.KindReflection, Relevance: compaction.RelevanceDecision,
			Text: "Compaction renders from recorded memory so rationale survives rewrites",
		},
	})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	require.Empty(t, model.calls, "a render must not reach the model at all")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)

	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.True(t, view[0].IsSummaryMessage, "the checkpoint leads the session")

	text := view[0].Content().Text
	require.Contains(t, text, "rejected splitting the ledger")
	require.Contains(t, text, "rationale survives rewrites")
	require.Contains(t, text, `rendered="2"`,
		"the footer records that this checkpoint came from memory")
	info, ok := compaction.ParseInfo(text)
	require.True(t, ok)
	require.Equal(t, 2, info.Rendered)
}

// TestRenderFallsBackWhenMemoryIsEmpty pins the failure mode: an empty ledger
// behaves exactly as compaction did before any of this existed. An observer
// that never ran cannot break the session's ability to compact.
func TestRenderFallsBackWhenMemoryIsEmpty(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the prose checkpoint"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	require.NotEmpty(t, model.calls, "an empty ledger must fall back to the model")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.Contains(t, view[0].Content().Text, "the prose checkpoint")
	require.NotContains(t, view[0].Content().Text, `rendered=`,
		"a model-written checkpoint carries no render provenance")
}

// TestEveryMemoryRetiredFallsBackToo covers the subtle one: the ledger exists
// and has rows, but a Drop has retired all of them. Rows exist is not the same
// as memory exists.
func TestEveryMemoryRetiredFallsBackToo(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the prose checkpoint"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	_, err = env.ledger.Append(ctx, sess.ID, []compaction.Entry{
		{Kind: compaction.KindObservation, Text: "soon forgotten"},
	})
	require.NoError(t, err)
	_, err = env.ledger.Append(ctx, sess.ID, []compaction.Entry{
		{Kind: compaction.KindDrop, Text: "retired", Retires: []int{1}},
	})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))
	require.NotEmpty(t, model.calls, "retired memory is no memory")
}

// TestRenderedCheckpointPreviewsForFree checks the composition with staging:
// previewing a render should also skip the model, because a checkpoint that
// costs nothing to build can be shown before it is adopted without paying
// for it twice.
func TestRenderedCheckpointPreviewsForFree(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unused"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	_, err = env.ledger.Append(ctx, sess.ID, []compaction.Entry{
		{
			Kind: compaction.KindObservation, Relevance: compaction.RelevanceDecision,
			Text: "The migration landed and was validated",
		},
	})
	require.NoError(t, err)

	preview, err := sa.SummarizePreview(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, preview.CheckpointID)
	require.Empty(t, model.calls)

	// The session must not have adopted it: staging is still the only thing
	// that keeps a checkpoint reversible.
	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, updated.SummaryMessageID)

	require.NoError(t, sa.ConfirmSummarize(ctx, sess.ID, preview))
	updated, err = env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, preview.CheckpointID, updated.SummaryMessageID)
}

// TestObserverRecordsMemoryAtTurnEnd exercises the observer against a scripted
// small model, asserting the turn's messages are cited so the next observation
// does not redo them.
func TestObserverRecordsMemoryAtTurnEnd(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unused"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	sa.observeMemory = true
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 2_000)

	// The observer cites what it read, so script it with the ids that are
	// actually in the session rather than ones the test invents.
	seeded, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, seeded)
	ids := make([]string, 0, len(seeded))
	for _, m := range seeded {
		ids = append(ids, fmt.Sprintf("%q", m.ID))
	}
	observer := &recordingModel{text: fmt.Sprintf(
		`[{"kind":"observation","text":"The user chose append-only storage","relevance":2,"sources":[%s]}]`,
		strings.Join(ids, ","))}
	sa.smallModel.Set(Model{Model: observer, CatwalkCfg: catwalk.Model{
		ContextWindow: 200_000, DefaultMaxTokens: 10_000,
	}})

	// The observer is fired from the turn's terminal event and runs on a
	// detached goroutine, so drive it directly and wait for the effect.
	require.NoError(t, sa.distill(ctx, sess.ID))

	led, err := env.ledger.Ledger(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, led.Entries, 1)
	require.Equal(t, compaction.KindObservation, led.Entries[0].Kind)
	require.Equal(t, compaction.RelevanceDecision, led.Entries[0].Relevance)
	require.Equal(t, "The user chose append-only storage", led.Entries[0].Text)
	require.NotEmpty(t, led.Entries[0].Sources,
		"entries must cite the messages they were distilled from, or coverage cannot be tracked")

	// A second pass over the same turn finds nothing left to observe.
	require.NoError(t, sa.distill(ctx, sess.ID))
	led, err = env.ledger.Ledger(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, led.Entries, 1, "fully cited messages are not re-observed")
	require.Len(t, observer.calls, 1, "the model is not asked again either")
}

// TestObserverDropsUnknownSources keeps the ledger checkable: a model citing a
// message that never existed must not record a citation nobody can follow.
func TestObserverDropsUnknownSources(t *testing.T) {
	env := testEnv(t)
	observer := &recordingModel{
		text: `[{"kind":"observation","text":"hallucinated provenance","relevance":1,"sources":["no-such-message"]}]`,
	}
	sa := testSessionAgent(env, &recordingModel{text: "unused"}, observer, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	sa.observeMemory = true
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 2_000)

	require.NoError(t, sa.distill(ctx, sess.ID))

	led, err := env.ledger.Ledger(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, led.Entries, 1)
	require.Empty(t, led.Entries[0].Sources)
}
