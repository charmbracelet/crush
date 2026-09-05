package agent

import (
	"encoding/json"
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
