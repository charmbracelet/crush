package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
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

const prismCostTestExpectedCost = 0.0002

func prismCostTestMetadata(modelID string) fantasy.ProviderMetadata {
	metadata := &openai.ProviderMetadata{}
	header := http.Header{}
	if modelID != "" {
		header.Set(hyper.PrismModelIDHeader, modelID)
	}
	hyper.HeaderFunc(header, metadata)
	return fantasy.ProviderMetadata{openai.Name: metadata}
}

func prismCostTestModel(languageModel fantasy.LanguageModel) (Model, *config.ConfigStore) {
	selected := catwalk.Model{
		ID:               "prism",
		ContextWindow:    200000,
		DefaultMaxTokens: 10000,
		Pricing: catwalk.Pricing{
			CacheCreate: 20,
			CacheHit:    10,
			Input:       100,
			Output:      1000,
		},
	}
	providers := csync.NewMapFrom(map[string]config.ProviderConfig{
		hyper.Name: {
			ID: hyper.Name,
			Models: []catwalk.Model{
				selected,
				{ID: "prism-42", Pricing: catwalk.Pricing{CacheCreate: 0.2, CacheHit: 0.1, Input: 1, Output: 10}},
				{ID: "prism-84", Pricing: catwalk.Pricing{CacheCreate: 0.4, CacheHit: 0.2, Input: 2, Output: 20}},
			},
		},
	})
	store := config.NewTestStore(&config.Config{Providers: providers})
	model := Model{
		Model:        languageModel,
		CatwalkCfg:   selected,
		ModelCfg:     config.SelectedModel{Provider: hyper.Name, Model: selected.ID},
		ProviderName: hyper.DisplayName,
	}
	return model, store
}

func newPrismCostTestAgent(env fakeEnv, languageModel fantasy.LanguageModel) *sessionAgent {
	model, store := prismCostTestModel(languageModel)
	return NewSessionAgent(SessionAgentOptions{
		Models:       fixedModels(model, model),
		SystemPrompt: "test prompt",
		Sessions:     env.sessions,
		Messages:     env.messages,
		Cfg:          store,
	}).(*sessionAgent)
}

func TestModelForStepCost(t *testing.T) {
	t.Parallel()

	languageModel := &fakePrismLanguageModel{}
	model, store := prismCostTestModel(languageModel)
	model.FlatRate = true
	agent := &sessionAgent{cfg: store}

	t.Run("uses the routed model pricing without changing selection", func(t *testing.T) {
		t.Parallel()
		actual := agent.modelForStepCost(model, prismCostTestMetadata("prism-42"))
		require.Equal(t, "prism-42", actual.CatwalkCfg.ID)
		require.Equal(t, 1.0, actual.CatwalkCfg.Pricing.Input)
		require.Equal(t, model.ModelCfg, actual.ModelCfg)
		require.Same(t, languageModel, actual.Model)
		require.True(t, actual.FlatRate)
		require.Equal(t, hyper.DisplayName, actual.ProviderName)
	})

	t.Run("falls back without routed model metadata", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, model, agent.modelForStepCost(model, prismCostTestMetadata("")))
	})

	t.Run("falls back for an unknown routed model", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, model, agent.modelForStepCost(model, prismCostTestMetadata("unknown")))
	})

	t.Run("falls back without config", func(t *testing.T) {
		t.Parallel()
		agentWithoutConfig := &sessionAgent{}
		require.Equal(t, model, agentWithoutConfig.modelForStepCost(model, prismCostTestMetadata("prism-42")))
	})

	t.Run("ignores Prism metadata for another provider", func(t *testing.T) {
		t.Parallel()
		nonHyper := model
		nonHyper.ModelCfg.Provider = "openai"
		require.Equal(t, nonHyper, agent.modelForStepCost(nonHyper, prismCostTestMetadata("prism-42")))
	})
}

func TestHyperStepsCostUsesEachRoutedModel(t *testing.T) {
	t.Parallel()

	model, store := prismCostTestModel(nil)
	agent := &sessionAgent{cfg: store}
	steps := []fantasy.StepResult{
		{Response: fantasy.Response{
			Usage:            fantasy.Usage{InputTokens: 1_000_000, OutputTokens: 2_000_000},
			ProviderMetadata: prismCostTestMetadata("prism-42"),
		}},
		{Response: fantasy.Response{
			Usage:            fantasy.Usage{InputTokens: 3_000_000},
			ProviderMetadata: prismCostTestMetadata("prism-84"),
		}},
		{Response: fantasy.Response{
			Usage: fantasy.Usage{OutputTokens: 1_000_000},
		}},
	}

	cost := agent.hyperStepsCost(model, steps)
	require.NotNil(t, cost)
	require.Equal(t, 1027.0, *cost)
}

func TestUsageCostIncludesEveryTokenBucket(t *testing.T) {
	t.Parallel()

	pricing := catwalk.Pricing{CacheCreate: 10, CacheHit: 20, Input: 30, Output: 40}
	usage := fantasy.Usage{
		CacheCreationTokens: 1_000_000,
		CacheReadTokens:     2_000_000,
		InputTokens:         3_000_000,
		OutputTokens:        4_000_000,
	}
	require.Equal(t, 300.0, usageCost(pricing, usage))
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
	agent := newPrismCostTestAgent(env, model)

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
	require.InDelta(t, prismCostTestExpectedCost, updated.Cost, 1e-12)

	summary, err := env.messages.Get(ctx, updated.SummaryMessageID)
	require.NoError(t, err)
	require.True(t, summary.IsSummaryMessage)
	require.Equal(t, "prism-42", summary.PrismModelID)
	require.Equal(t, "GPT-5.2 Codex Max", summary.PrismModelName)
	require.NotNil(t, summary.PrismHypercreditSavings)
	require.Equal(t, 1.5, *summary.PrismHypercreditSavings)
	require.Nil(t, summary.PrismDollarSavings)
}

func TestRunUsesPrismRoutedModelCost(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set(hyper.PrismModelIDHeader, "prism-42")
	header.Set(hyper.PrismModelNameHeader, "GPT-5.2 Codex Max")
	metadata := &openai.ProviderMetadata{}
	hyper.HeaderFunc(header, metadata)

	model := &fakePrismLanguageModel{metadata: fantasy.ProviderMetadata{openai.Name: metadata}}
	env := testEnv(t)
	agent := newPrismCostTestAgent(env, model)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "existing prompt"}},
	})
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello"})
	require.NoError(t, err)
	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, prismCostTestExpectedCost, updated.Cost, 1e-12)
}

func TestGenerateTitleUsesPrismRoutedModelCost(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	header.Set(hyper.PrismModelIDHeader, "prism-42")
	header.Set(hyper.PrismModelNameHeader, "GPT-5.2 Codex Max")
	metadata := &openai.ProviderMetadata{}
	hyper.HeaderFunc(header, metadata)

	model := &fakePrismLanguageModel{metadata: fantasy.ProviderMetadata{openai.Name: metadata}}
	env := testEnv(t)
	agent := newPrismCostTestAgent(env, model)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	agent.GenerateTitle(t.Context(), sess.ID, "hello")
	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "A summary.", updated.Title)
	require.InDelta(t, prismCostTestExpectedCost, updated.Cost, 1e-12)
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
