package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/message"

	"charm.land/fantasy/providers/openai"
	"github.com/stretchr/testify/require"
)

func TestExtractPrismModel(t *testing.T) {
	t.Parallel()

	prismMetadata := func(header http.Header) fantasy.ProviderMetadata {
		metadata := &openai.ProviderMetadata{}
		hyper.HeaderFunc(header, metadata)
		return fantasy.ProviderMetadata{openai.Name: metadata}
	}

	t.Run("returns the routed model ID and name", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		header.Set(hyper.PrismModelIDHeader, "prism-42")
		header.Set(hyper.PrismModelNameHeader, "GPT-5.2 Codex Max")
		modelID, modelName := extractPrismModel(prismMetadata(header))
		require.Equal(t, "prism-42", modelID)
		require.Equal(t, "GPT-5.2 Codex Max", modelName)
	})

	t.Run("returns empty when not routed through Prism", func(t *testing.T) {
		t.Parallel()
		modelID, modelName := extractPrismModel(fantasy.ProviderMetadata{})
		require.Empty(t, modelID)
		require.Empty(t, modelName)

		metadata := &openai.ProviderMetadata{
			ExtraFields: map[string]json.RawMessage{"remaining": json.RawMessage(`{"hypercredits": 10}`)},
		}
		modelID, modelName = extractPrismModel(fantasy.ProviderMetadata{openai.Name: metadata})
		require.Empty(t, modelID)
		require.Empty(t, modelName)
	})

	t.Run("persisted on the assistant message", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		header.Set(hyper.PrismModelIDHeader, "prism-42")
		header.Set(hyper.PrismModelNameHeader, "GPT-5.2 Codex Max")

		msg := message.Message{}
		msg.PrismModelID, msg.PrismModelName = extractPrismModel(prismMetadata(header))
		require.Equal(t, "prism-42", msg.PrismModelID)
		require.Equal(t, "GPT-5.2 Codex Max", msg.PrismModelName)
	})
}

func TestSummarizeCapturesPrismModel(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set(hyper.PrismModelIDHeader, "prism-42")
	header.Set(hyper.PrismModelNameHeader, "GPT-5.2 Codex Max")
	header.Set(hyper.PrismHypercreditSavingsHeader, "1.5")
	metadata := &openai.ProviderMetadata{}
	hyper.HeaderFunc(header, metadata)

	model := &fakePrismLanguageModel{metadata: fantasy.ProviderMetadata{openai.Name: metadata}}

	env := testEnv(t)
	agent := testSessionAgent(env, model, model, "test prompt")

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)

	require.NoError(t, agent.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)

	summary, err := env.messages.Get(ctx, updated.SummaryMessageID)
	require.NoError(t, err)
	require.True(t, summary.IsSummaryMessage)
	require.Equal(t, "prism-42", summary.PrismModelID)
	require.Equal(t, "GPT-5.2 Codex Max", summary.PrismModelName)
	require.NotNil(t, summary.PrismHypercreditSavings)
	require.Equal(t, 1.5, *summary.PrismHypercreditSavings)
	require.Nil(t, summary.PrismDollarSavings)
}

type fakePrismLanguageModel struct {
	metadata fantasy.ProviderMetadata
}

func (f *fakePrismLanguageModel) Provider() string { return hyper.Name }
func (f *fakePrismLanguageModel) Model() string    { return "prism" }

func (f *fakePrismLanguageModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (f *fakePrismLanguageModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		for _, part := range []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "A summary."},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"},
			{
				Type:             fantasy.StreamPartTypeFinish,
				FinishReason:     fantasy.FinishReasonStop,
				Usage:            fantasy.Usage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110},
				ProviderMetadata: f.metadata,
			},
		} {
			if !yield(part) {
				return
			}
		}
	}, nil
}

func (f *fakePrismLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakePrismLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func TestExtractPrismSavings(t *testing.T) {
	t.Parallel()

	prismMetadata := func(header http.Header) fantasy.ProviderMetadata {
		metadata := &openai.ProviderMetadata{}
		hyper.HeaderFunc(header, metadata)
		return fantasy.ProviderMetadata{openai.Name: metadata}
	}

	t.Run("returns both savings figures when reported", func(t *testing.T) {
		t.Parallel()
		header := http.Header{}
		header.Set(hyper.PrismHypercreditSavingsHeader, "1.5")
		header.Set(hyper.PrismDollarSavingsHeader, "0.002")
		hypercredits, dollars := extractPrismSavings(prismMetadata(header))
		require.NotNil(t, hypercredits)
		require.Equal(t, 1.5, *hypercredits)
		require.NotNil(t, dollars)
		require.Equal(t, 0.002, *dollars)
	})

	t.Run("returns nil when not reported", func(t *testing.T) {
		t.Parallel()
		hypercredits, dollars := extractPrismSavings(fantasy.ProviderMetadata{})
		require.Nil(t, hypercredits)
		require.Nil(t, dollars)
	})

	t.Run("returns nil when malformed", func(t *testing.T) {
		t.Parallel()
		metadata := &openai.ProviderMetadata{
			ExtraFields: map[string]json.RawMessage{
				hyper.PrismHypercreditSavingsField: json.RawMessage(`"not-a-number"`),
			},
		}
		hypercredits, dollars := extractPrismSavings(fantasy.ProviderMetadata{openai.Name: metadata})
		require.Nil(t, hypercredits)
		require.Nil(t, dollars)
	})
}
