package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func pricedModel(id string, context int64, input, output float64) catwalk.Model {
	return catwalk.Model{
		ID:            id,
		Name:          id,
		ContextWindow: context,
		Pricing:       catwalk.Pricing{Input: input, Output: output},
	}
}

func shortlistIDs(entries []ShortlistEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Model.ID)
	}
	return ids
}

func TestPickShortlistEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, PickShortlist(nil))
	require.Empty(t, PickShortlist([]catwalk.Model{{}, {ID: ""}}))
}

func TestPickShortlistSmallCatalogKeepsCatalogOrder(t *testing.T) {
	t.Parallel()

	models := []catwalk.Model{
		pricedModel("small", 32_000, 1, 2),
		pricedModel("large", 200_000, 15, 75),
	}
	entries := PickShortlist(models)
	require.Equal(t, []string{"small", "large"}, shortlistIDs(entries))
	for _, entry := range entries {
		require.Empty(t, entry.Tag, "a two-model provider should not be labeled")
	}
}

func TestPickShortlistWithoutPricingKeepsCatalogOrder(t *testing.T) {
	t.Parallel()

	models := []catwalk.Model{
		pricedModel("a", 8_000, 0, 0),
		pricedModel("b", 16_000, 0, 0),
		pricedModel("c", 32_000, 0, 0),
		pricedModel("d", 64_000, 0, 0),
	}
	entries := PickShortlist(models)
	require.Equal(t, []string{"a", "b", "c"}, shortlistIDs(entries))
	for _, entry := range entries {
		require.Empty(t, entry.Tag)
	}
}

func TestPickShortlistTagsTradeoffs(t *testing.T) {
	t.Parallel()

	models := []catwalk.Model{
		pricedModel("capable", 200_000, 15, 75),
		pricedModel("mid", 128_000, 3, 15),
		pricedModel("cheap", 64_000, 1, 4),
		pricedModel("fastest", 32_000, 0.5, 2),
		pricedModel("balanced", 100_000, 2, 8),
	}

	entries := PickShortlist(models)
	require.Equal(t, []string{"capable", "balanced", "fastest"}, shortlistIDs(entries))
	require.Equal(t, shortlistTagCapable, entries[0].Tag)
	require.Equal(t, shortlistTagBalanced, entries[1].Tag)
	require.Equal(t, shortlistTagFastest, entries[2].Tag)
}

func TestPickShortlistIsOrderIndependent(t *testing.T) {
	t.Parallel()

	forward := []catwalk.Model{
		pricedModel("capable", 200_000, 15, 75),
		pricedModel("mid", 128_000, 3, 15),
		pricedModel("cheap", 64_000, 1, 4),
		pricedModel("fastest", 32_000, 0.5, 2),
		pricedModel("balanced", 100_000, 2, 8),
	}
	reversed := make([]catwalk.Model, len(forward))
	for i, model := range forward {
		reversed[len(forward)-1-i] = model
	}

	require.Equal(t, shortlistIDs(PickShortlist(forward)), shortlistIDs(PickShortlist(reversed)))
}

func TestPickShortlistSkipsDuplicates(t *testing.T) {
	t.Parallel()

	models := []catwalk.Model{
		pricedModel("a", 8_000, 1, 1),
		pricedModel("a", 9_000, 2, 2),
		pricedModel("b", 16_000, 2, 2),
		pricedModel("c", 32_000, 3, 3),
		pricedModel("d", 64_000, 4, 4),
	}
	entries := PickShortlist(models)
	for _, entry := range entries {
		require.NotEqual(t, int64(9_000), entry.Model.ContextWindow, "first duplicate wins")
	}
}

func TestPickShortlistNeverRepeatsAModel(t *testing.T) {
	t.Parallel()

	// The cheapest model is also the one with the largest context, so a
	// naive ranking would offer it twice.
	models := []catwalk.Model{
		pricedModel("nano", 1_000_000, 0.1, 0.4),
		pricedModel("pro", 200_000, 10, 30),
		pricedModel("mid", 400_000, 2, 6),
		pricedModel("low", 100_000, 1, 3),
	}

	entries := PickShortlist(models)
	require.Equal(t, []string{"nano", "pro", "low"}, shortlistIDs(entries))
	require.Equal(t, shortlistTagCapable, entries[0].Tag)
	require.Equal(t, shortlistTagBalanced, entries[1].Tag)
	require.Equal(t, shortlistTagFastest, entries[2].Tag)
}

func TestShortlistModelsUsesSubscriptionCatalog(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID:     catwalk.InferenceProviderOpenAI,
		Models: []catwalk.Model{pricedModel("api-only", 100_000, 1, 1)},
	}
	cfg := testConfig()
	cfg.Providers.Set("openai", config.ProviderConfig{
		ID:         "openai",
		OAuthToken: &oauth.Token{AccessToken: "token"},
		ChatGPTModels: []catwalk.Model{
			pricedModel("gpt-5", 200_000, 2, 8),
		},
	})

	models := shortlistModels(cfg, provider)
	require.Len(t, models, 1)
	require.Equal(t, "gpt-5", models[0].ID)
}

func TestShortlistModelsAddsConfiguredModels(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID:     "custom",
		Models: []catwalk.Model{pricedModel("catalog", 100_000, 1, 1)},
	}
	cfg := testConfig()
	cfg.Providers.Set("custom", config.ProviderConfig{
		ID:     "custom",
		Models: []catwalk.Model{pricedModel("mine", 50_000, 0, 0)},
	})

	models := shortlistModels(cfg, provider)
	require.Len(t, models, 2)
	require.Equal(t, "catalog", models[0].ID)
	require.Equal(t, "mine", models[1].ID)
}

func newTestShortlist(t *testing.T, provider catwalk.Provider) *ModelShortlist {
	t.Helper()

	m, err := NewModelShortlist(testCommon(testConfig()), provider, config.SelectedModelTypeLarge)
	require.NoError(t, err)
	return m
}

func TestModelShortlistSelectsCuratedModel(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID:   "acme",
		Name: "Acme",
		Models: []catwalk.Model{
			pricedModel("capable", 200_000, 15, 75),
			pricedModel("mid", 128_000, 3, 15),
			pricedModel("cheap", 64_000, 1, 4),
			pricedModel("fastest", 32_000, 0.5, 2),
			pricedModel("balanced", 100_000, 2, 8),
		},
	}
	m := newTestShortlist(t, provider)
	require.Equal(t, ModelShortlistID, m.ID())

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectModel)
	require.True(t, ok)
	require.Equal(t, "capable", selected.Model.Model)
	require.Equal(t, "acme", selected.Model.Provider)
	require.Equal(t, config.SelectedModelTypeLarge, selected.ModelType)
}

func TestModelShortlistBrowseAll(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID: "acme",
		Models: []catwalk.Model{
			pricedModel("a", 200_000, 15, 75),
			pricedModel("b", 128_000, 3, 15),
			pricedModel("c", 64_000, 1, 4),
			pricedModel("d", 32_000, 0.5, 2),
			pricedModel("e", 100_000, 2, 8),
		},
	}
	m := newTestShortlist(t, provider)

	for range 10 {
		if item, ok := m.list.SelectedItem().(*ShortlistItem); ok && item.browse {
			break
		}
		m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	browse, ok := action.(ActionBrowseModels)
	require.True(t, ok)
	require.Equal(t, "acme", string(browse.Provider.ID))
}

func TestModelShortlistNeverEmpty(t *testing.T) {
	t.Parallel()

	_, err := NewModelShortlist(testCommon(testConfig()), catwalk.Provider{ID: "empty", Name: "Empty"}, config.SelectedModelTypeLarge)
	require.Error(t, err)
}
