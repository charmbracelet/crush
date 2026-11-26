package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// attachMemory turns a test agent into one that records session memory, the
// state a real session reaches with observe_memory set.
func attachMemory(sa SessionAgent, env fakeEnv) *sessionAgent {
	a := sa.(*sessionAgent)
	a.ledger = env.ledger
	return a
}

// TestEmptyMemoryAddsNothingToTheCheckpoint: an observer that never ran must
// not change what a checkpoint looks like. The summary is the checkpoint, and
// memory adds nothing to it.
func TestEmptyMemoryAddsNothingToTheCheckpoint(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the prose checkpoint"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))
	require.NotEmpty(t, model.calls, "the summary is always written by the model")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)

	text := view[0].Content().Text
	require.Contains(t, text, "the prose checkpoint")
	require.NotContains(t, text, `observed=`,
		"a checkpoint with no memory behind it claims none")
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

	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))
	require.NotEmpty(t, model.calls, "the summary is still written")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.NotContains(t, view[0].Content().Text, `observed=`,
		"retired memory is no memory, so the checkpoint claims none")
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
	observerModel := Model{Model: observer, CatwalkCfg: catwalk.Model{
		ContextWindow: 200_000, DefaultMaxTokens: 10_000,
	}}
	sa.models = fixedModels(sa.models().Large, observerModel)

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
	require.Len(t, led.Entries, 2, "the entry plus the pass's coverage marker")
	require.Empty(t, led.Entries[0].Sources, "the invented citation is dropped")

	// The marker says the batch was read without inventing anything about it.
	require.Equal(t, compaction.KindDrop, led.Entries[1].Kind)
	require.NotEmpty(t, led.Entries[1].Sources, "the batch is marked covered")
}

// TestAppendixAccompaniesTheSummary is the whole behavior now: the model writes
// the narrative, and recorded memory rides along with it. That split is what
// lets memory persist across compactions without discarding the summarizer's
// ability to compress what was never observed.
func TestAppendixAccompaniesTheSummary(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the session built a ledger and then used it"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 3_000)

	_, err = env.ledger.Append(ctx, sess.ID, []compaction.Entry{
		{
			Kind: compaction.KindObservation, Relevance: compaction.RelevanceDecision,
			Text: "User rejected splitting the ledger across sessions",
		},
	})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))

	require.NotEmpty(t, model.calls, "the default keeps the summarizing model")
	require.Contains(t, model.calls[0], "<recorded_memory>",
		"memory guides the summary rather than only decorating it")
	require.Contains(t, model.calls[0], "do not restate them",
		"the prompt must divide the labor, or the appendix is paid for twice")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	text := view[0].Content().Text

	require.Contains(t, text, "the session built a ledger and then used it",
		"the prose summary is still the checkpoint")
	require.Contains(t, text, "rejected splitting the ledger",
		"memory accompanies it verbatim")
	require.Contains(t, text, "- [e-",
		"rendered memory entries print their bracketed ids so recall can use them")
	require.Contains(t, text, `observed="1"`)
	require.NotContains(t, text, `rendered="`,
		"a summarized checkpoint must not claim to have been rendered")
}

// TestObserverFiresOnTheTerminalEventTheCoordinatorUses is the regression test
// for a wiring bug that made this whole feature inert: the observer was called
// after an early return that every ordinary interactive turn takes, because the
// coordinator always supplies an OnComplete hook. The tests above all passed
// while nothing was ever observed, since they drove the distillation directly
// instead of the event that is supposed to trigger it.
func TestObserverFiresOnTheTerminalEventTheCoordinatorUses(t *testing.T) {
	env := testEnv(t)
	observer := &recordingModel{text: `[{"kind":"reflection","text":"Memory must be observed from the turn event, not called directly","relevance":2,"sources":[]}]`}
	sa := testSessionAgent(env, &recordingModel{text: "unused"}, observer, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	sa.observeMemory = true
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 2_000)

	// This is the shape an interactive turn arrives in: the coordinator always
	// sets OnComplete so it can coalesce retries.
	var hookRan bool
	sa.publishRunComplete(ctx, SessionAgentCall{
		SessionID:  sess.ID,
		OnComplete: func(notify.RunComplete) { hookRan = true },
	}, notify.RunComplete{SessionID: sess.ID})

	require.True(t, hookRan, "the caller's hook still runs")

	require.Eventually(t, func() bool {
		led, err := env.ledger.Ledger(ctx, sess.ID)
		return err == nil && len(led.Entries) > 0
	}, 10*time.Second, 20*time.Millisecond,
		"a completed turn must be observed even when the caller supplied a hook")
}

// TestObserverSkipsFailedAndCancelledTurns: a turn that errored or was
// cancelled is not memory. Skipping costs nothing, because its messages stay
// unobserved and a later turn picks them up.
func TestObserverSkipsFailedAndCancelledTurns(t *testing.T) {
	env := testEnv(t)
	observer := &recordingModel{text: `[{"kind":"observation","text":"should never be recorded","relevance":2}]`}
	sa := testSessionAgent(env, &recordingModel{text: "unused"}, observer, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	sa.observeMemory = true
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 2_000)

	sa.publishRunComplete(ctx, SessionAgentCall{SessionID: sess.ID},
		notify.RunComplete{SessionID: sess.ID, Error: "provider exploded"})
	sa.publishRunComplete(ctx, SessionAgentCall{SessionID: sess.ID},
		notify.RunComplete{SessionID: sess.ID, Cancelled: true})

	require.Never(t, func() bool {
		led, err := env.ledger.Ledger(ctx, sess.ID)
		return err == nil && len(led.Entries) > 0
	}, 750*time.Millisecond, 50*time.Millisecond,
		"a failed or cancelled turn is not memory")
	require.Empty(t, observer.calls)
}

type blockingObserverModel struct {
	recordingModel
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (m *blockingObserverModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.once.Do(func() {
		close(m.entered)
		<-m.release
	})
	return m.recordingModel.Stream(ctx, call)
}

type countingMessages struct {
	message.Service
	listCalls        int
	fromSummaryCalls int
	lastSummaryID    string
}

func (c *countingMessages) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	c.listCalls++
	return c.Service.List(ctx, sessionID)
}

func (c *countingMessages) ListFromSummary(ctx context.Context, sessionID, summaryMessageID string) ([]message.Message, error) {
	c.fromSummaryCalls++
	c.lastSummaryID = summaryMessageID
	return c.Service.ListFromSummary(ctx, sessionID, summaryMessageID)
}

func TestDistillSerializesConcurrentPassesAndUsesAnchor(t *testing.T) {
	env := testEnv(t)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 2_000)

	observer := &blockingObserverModel{
		recordingModel: recordingModel{text: "[]"},
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	msgs := &countingMessages{Service: env.messages}
	sa := testSessionAgent(env, &recordingModel{text: "unused"}, observer, "test prompt").(*sessionAgent)
	sa.messages = msgs
	sa.ledger = env.ledger
	sa.observeMemory = true

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, sa.distill(ctx, sess.ID))
	}()
	<-observer.entered
	go func() {
		defer wg.Done()
		require.NoError(t, sa.distill(ctx, sess.ID))
	}()
	close(observer.release)
	wg.Wait()

	require.Len(t, observer.calls, 1, "concurrent passes for one session must not make duplicate model calls")
	require.Zero(t, msgs.listCalls, "distill must read via ListFromSummary instead of loading full List")
	require.Equal(t, 2, msgs.fromSummaryCalls)
	require.NotEmpty(t, msgs.lastSummaryID, "second pass must anchor on the last observed source message")
}

func TestDistillDrainsBacklogChronologicallyAndSkipsCheckpoints(t *testing.T) {
	env := testEnv(t)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// Seed a backlog exceeding observeMaxTokens (24k) plus a checkpoint row
	// in the middle so we verify both chronological draining and skipping
	// summary rows.
	pad := strings.Repeat("a", 16_000*4) // ~16k tokens per message
	m1, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "oldest-backlog-msg " + pad}},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		IsSummaryMessage: true,
		Parts:            []message.ContentPart{message.TextContent{Text: "checkpoint-must-not-be-observed " + pad}},
	})
	require.NoError(t, err)
	m2, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "newer-backlog-msg " + pad}},
	})
	require.NoError(t, err)

	observer := &recordingModel{text: "[]"}
	sa := testSessionAgent(env, &recordingModel{text: "unused"}, observer, "test prompt").(*sessionAgent)
	sa.ledger = env.ledger
	sa.observeMemory = true

	// Pass 1 must read the oldest message first (not jump to the newest) and
	// ignore the checkpoint row.
	require.NoError(t, sa.distill(ctx, sess.ID))
	require.Len(t, observer.calls, 1)
	require.Contains(t, observer.calls[0], "oldest-backlog-msg")
	require.NotContains(t, observer.calls[0], "newer-backlog-msg")
	require.NotContains(t, observer.calls[0], "checkpoint-must-not-be-observed")

	// Pass 2 advances from the anchor to the newer unread message and still
	// skips the checkpoint row.
	require.NoError(t, sa.distill(ctx, sess.ID))
	require.Len(t, observer.calls, 2)
	require.Contains(t, observer.calls[1], "newer-backlog-msg")
	require.NotContains(t, observer.calls[1], "checkpoint-must-not-be-observed")

	led, err := env.ledger.Ledger(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, led.Unobserved([]string{m1.ID, m2.ID}),
		"both older and newer messages must be covered across consecutive passes")
}
