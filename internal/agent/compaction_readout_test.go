package agent

import (
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/stretchr/testify/require"
)

// TestCompactionReadoutCountsTheCheckpointOnce guards the number the user sees
// immediately after a compaction. Every readout — the header percentage, the
// sidebar context line, and `crush session` — adds PromptTokens and
// CompletionTokens and calls the sum the context in use. A compaction puts the
// next request's size into PromptTokens, and that request already contains the
// checkpoint, so reporting the checkpoint a second time as a completion makes
// the session look barely lighter than it was, which reads as a readout that
// did not update. The figure corrects itself on the following step once the
// provider reports real usage, so the symptom is intermittent by construction.
func TestCompactionReadoutCountsTheCheckpointOnce(t *testing.T) {
	env := testEnv(t)

	// A checkpoint big enough that a second copy of it is unmissable.
	checkpoint := strings.Repeat("c", 40_000)
	model := &recordingModel{text: checkpoint}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)

	// The checkpoint plus the retained tail is exactly what the next request
	// carries. No provider usage has been reported in this session, so the
	// measured overhead is zero and the two figures should agree.
	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	nextRequest := compaction.EstimateAll(view)
	require.Positive(t, nextRequest)

	checkpointTokens := compaction.Tokens(checkpoint)
	require.Greater(t, checkpointTokens, int64(1000),
		"the checkpoint must be large enough for this test to mean anything")

	shown := updated.PromptTokens + updated.CompletionTokens
	// One percent of slack absorbs rounding in the two estimators; a second
	// copy of the checkpoint is far larger than that.
	bound := nextRequest + nextRequest/100
	require.LessOrEqual(t, shown, bound,
		"the readout must describe one request, not a request plus a second copy of its checkpoint")
}
