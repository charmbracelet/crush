package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type stubModelsWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *stubModelsWorkspace) Config() *config.Config {
	return w.cfg
}

func newTestModelsDialog(t *testing.T) *Models {
	t.Helper()
	s := styles.CharmtonePantera()
	cfg := &config.Config{
		// Keep config.Providers offline and deterministic in tests.
		Options: &config.Options{DisableDefaultProviders: true},
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "custom", Model: "beta-model"},
		},
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"custom": {
				ID:      "custom",
				Name:    "Custom",
				BaseURL: "http://localhost:0/v1",
				Models: []catwalk.Model{
					{ID: "alpha-model", Name: "Alpha"},
					{ID: "beta-model", Name: "Beta"},
				},
			},
		}),
	}
	com := &common.Common{
		Workspace: &stubModelsWorkspace{cfg: cfg},
		Styles:    &s,
	}
	d, err := NewModels(com, false)
	require.NoError(t, err)
	return d
}

func TestModelsDialog_ReloadItemsFilteredSelectsFirstMatch(t *testing.T) {
	d := newTestModelsDialog(t)

	// The current model (beta-model) is selected when the dialog opens;
	// the user then types a filter that hides it.
	d.input.SetValue("alpha")
	d.list.SetFilter("alpha")

	require.NoError(t, d.ReloadItems())

	item, ok := d.list.SelectedItem().(*ModelItem)
	require.True(t, ok, "a model item should be selected after a filtered reload")
	require.Equal(t, "alpha-model", item.SelectedModel().Model,
		"filtered reload must select the first visible match, not a hidden item")
}

func TestModelsDialog_ReloadItemsNoFilterKeepsCurrentModel(t *testing.T) {
	d := newTestModelsDialog(t)

	require.NoError(t, d.ReloadItems())

	item, ok := d.list.SelectedItem().(*ModelItem)
	require.True(t, ok)
	require.Equal(t, "beta-model", item.SelectedModel().Model,
		"without a filter the current model stays selected")
}
