package dialog

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestRouterModelDialog(t *testing.T, provider string) (*RouterModel, *routerTestWorkspace) {
	t.Helper()
	s := styles.CharmtonePantera()
	ws := &routerTestWorkspace{cfg: &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{Provider: provider},
	}}}
	return NewRouterModel(&common.Common{Workspace: ws, Styles: &s}), ws
}

func typeInto(d *RouterModel, text string) {
	for _, r := range text {
		d.HandleMsg(tea.KeyPressMsg{Text: string(r), Code: r})
	}
}

// TestRouterModel_LocalHasNoPresetsAndUsesTypedModel proves "local" ships
// with no built-in server presets (any self-hosted System One server is
// user-supplied via base_url) and typing an arbitrary model id just sets
// the model.
func TestRouterModel_LocalHasNoPresetsAndUsesTypedModel(t *testing.T) {
	t.Parallel()

	d, ws := newTestRouterModelDialog(t, "local")
	require.Nil(t, d.Init(), "local presets are static, nothing to fetch")
	require.Empty(t, d.items, "local ships with no built-in presets")
	typeInto(d, "my-local-model")

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ActionClose{}, action)
	require.Equal(t, []routerSetCall{
		{key: "options.router.model", value: "my-local-model"},
	}, ws.setCalls)
}

// TestRouterModel_TypedIDIsUsedVerbatim proves the list never locks the
// user in: any typed id that isn't a known preset can be selected as is.
func TestRouterModel_TypedIDIsUsedVerbatim(t *testing.T) {
	t.Parallel()

	d, ws := newTestRouterModelDialog(t, "opencode-zen")
	typeInto(d, "jev-2.0")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, []routerSetCall{{key: "options.router.model", value: "jev-2.0"}}, ws.setCalls)
}

// TestRouterModel_OpenRouterListIsLoadedLive proves the OpenRouter list
// comes from the fetched catalog, and a failed fetch still allows typing.
func TestRouterModel_OpenRouterListIsLoadedLive(t *testing.T) {
	t.Parallel()

	d, ws := newTestRouterModelDialog(t, "openrouter")
	require.NotNil(t, d.Init())
	require.Zero(t, d.list.Len())

	d.HandleMsg(routerModelsLoadedMsg{presets: []router.Preset{
		{Label: "TypeSafe: Jev 1.13", Model: "typesafe/jev-1.13"},
	}})
	require.Equal(t, 1, d.list.Len())
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, []routerSetCall{{key: "options.router.model", value: "typesafe/jev-1.13"}}, ws.setCalls)

	failed, _ := newTestRouterModelDialog(t, "openrouter")
	failed.HandleMsg(routerModelsLoadedMsg{err: errors.New("offline")})
	require.Contains(t, failed.status, "type a model id")
}

// TestRouterModel_ReadsAsDecisionModelPickerNotModelPool proves the
// decision-model picker names itself and explains its purpose in terms
// that cannot be mistaken for the router's model-pool picker.
func TestRouterModel_ReadsAsDecisionModelPickerNotModelPool(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelDialog(t, "typesafe")
	require.Equal(t, "Decision Model (typesafe)", d.dialogTitle())
	require.Contains(t, d.purposeLine(), "Not a chat model")

	pool, _ := newTestRouterModelPoolDialog(t, nil)
	require.NotEqual(t, d.dialogTitle(), pool.dialogTitle(),
		"the two pickers must not share a title")
	require.NotEqual(t, d.purposeLine(), pool.purposeLine())
}
