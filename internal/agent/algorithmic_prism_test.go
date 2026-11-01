package agent

import (
	"net/http"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openai"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// An algorithmic summary goes through the one summarize path, so it records
// the model Prism routed it to and reads only the extraction.
func TestAlgorithmicSummarizeCapturesPrismModel(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set(hyper.PrismModelIDHeader, "glm-5.3")
	header.Set(hyper.PrismModelNameHeader, "GLM 5.3")
	metadata := &openai.ProviderMetadata{}
	hyper.HeaderFunc(header, metadata)
	model := &fakePrismLanguageModel{metadata: fantasy.ProviderMetadata{openai.Name: metadata}}

	env := testEnv(t)
	agent := testSessionAgent(env, model, model, "test prompt")
	agent.(*sessionAgent).compactSummaries = true
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	for _, part := range []message.CreateMessageParams{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "fix the login bug"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Looking into it."}, message.Finish{Reason: message.FinishReasonEndTurn}}},
	} {
		_, err = env.messages.Create(ctx, sess.ID, part)
		require.NoError(t, err)
	}

	require.NoError(t, agent.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)
	summary, err := env.messages.Get(ctx, updated.SummaryMessageID)
	require.NoError(t, err)
	require.Equal(t, "GLM 5.3", summary.PrismModelName)
	require.Equal(t, "glm-5.3", summary.PrismModelID)
}
