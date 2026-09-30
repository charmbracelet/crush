// internal/ui/dialog/router_model_pool_test.go
package dialog

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// multiProviderPoolCatalog is a two-provider catalog used to prove the pool
// dialog lists every provider, not just OpenRouter.
func multiProviderPoolCatalog() ([]catwalk.Provider, map[string]config.ProviderConfig) {
	openrouter := []catwalk.Model{
		{ID: "anthropic/claude-opus-4.5", Name: "Opus", CostPer1MIn: 5, CostPer1MOut: 25},
	}
	anthropic := []catwalk.Model{
		{ID: "claude-opus-4-5", Name: "Claude Opus", CostPer1MIn: 3, CostPer1MOut: 15},
	}
	providers := []catwalk.Provider{
		{ID: "openrouter", Name: "OpenRouter", Models: openrouter},
		{ID: "anthropic", Name: "Anthropic", Models: anthropic},
	}
	configured := map[string]config.ProviderConfig{
		"openrouter": {ID: "openrouter", Models: openrouter},
		"anthropic":  {ID: "anthropic", Models: anthropic},
	}
	return providers, configured
}

// TestRouterModelPool_ListsEveryProvider proves the pool offers chat models
// from every provider in the catalog, matching the switch-model dialog,
// rather than hardcoding OpenRouter's catalog.
func TestRouterModelPool_ListsEveryProvider(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	providers, configured := multiProviderPoolCatalog()
	cfg := &config.Config{
		Options:   &config.Options{Router: &config.RouterOptions{ModelPool: []string{"claude-opus-4-5"}}},
		Providers: csync.NewMapFrom(configured),
	}
	ws := &routerTestWorkspace{cfg: cfg}
	d := newRouterModelPool(&common.Common{Workspace: ws, Styles: &s}, providers)

	byID := make(map[string]*ModelItem, len(d.items))
	for _, it := range d.items {
		byID[it.ModelID()] = it
	}
	require.Contains(t, byID, "anthropic/claude-opus-4.5", "OpenRouter models must still be listed")
	require.Contains(t, byID, "claude-opus-4-5", "non-OpenRouter provider models must be listed")
	require.True(t, byID["claude-opus-4-5"].Checked(), "the configured pool entry must be pre-checked")
	require.False(t, byID["anthropic/claude-opus-4.5"].Checked())
}

// poolScreenRow flattens a screen buffer row into a plain string so a test
// can assert which rendered line the cursor landed on.
func poolScreenRow(scr uv.ScreenBuffer, y, width int) string {
	var b strings.Builder
	for x := 0; x < width; x++ {
		c := scr.CellAt(x, y)
		if c == nil {
			continue
		}
		b.WriteString(c.Content)
	}
	return b.String()
}

// TestRouterModelPool_CursorSitsOnInputRow proves the cursor lands on the
// filter input, not one row above it: the purpose line rendered between the
// title and the input must be counted in the cursor's Y offset.
func TestRouterModelPool_CursorSitsOnInputRow(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)
	scr := uv.NewScreenBuffer(80, 30)
	cur := d.Draw(scr, image.Rect(0, 0, 80, 30))
	require.NotNil(t, cur)

	require.Contains(t, poolScreenRow(scr, cur.Y, 80), "Filter chat models",
		"cursor must sit on the input row")
	require.Contains(t, poolScreenRow(scr, cur.Y-2, 80), "Not the decision model",
		"the purpose line must sit above the input, shifting the cursor down one row")
}

func newTestRouterModelPoolDialog(t *testing.T, pool []string) (*RouterModelPool, *routerTestWorkspace) {
	t.Helper()
	s := styles.CharmtonePantera()
	models := []catwalk.Model{
		{ID: "anthropic/claude-haiku-4.5", Name: "Haiku", CostPer1MIn: 1.1, CostPer1MOut: 5.5},
		{ID: "anthropic/claude-opus-4.5", Name: "Opus", CostPer1MIn: 5, CostPer1MOut: 25},
	}
	cfg := &config.Config{
		Options: &config.Options{Router: &config.RouterOptions{ModelPool: pool}},
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openrouter": {
				ID:     "openrouter",
				Models: models,
			},
		}),
	}
	ws := &routerTestWorkspace{cfg: cfg}
	providers := []catwalk.Provider{
		{ID: "openrouter", Name: "OpenRouter", Models: models},
	}
	return newRouterModelPool(&common.Common{Workspace: ws, Styles: &s}, providers), ws
}

// TestRouterModelPool_SeedsCheckedStateFromConfig proves models already in
// the configured pool render pre-checked, so opening the picker shows what
// is actually active rather than starting from a blank slate.
func TestRouterModelPool_SeedsCheckedStateFromConfig(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, []string{"anthropic/claude-opus-4.5"})

	require.Len(t, d.items, 2)
	for _, it := range d.items {
		require.Equal(t, it.ModelID() == "anthropic/claude-opus-4.5", it.Checked())
	}
}

// TestRouterModelPool_ToggleTogglesAndPersistsSortedPool proves toggling a
// model persists the full pool immediately (no separate save step) and
// that the written list is sorted, so the config stays diffable
// regardless of catalog or toggle order.
func TestRouterModelPool_ToggleTogglesAndPersistsSortedPool(t *testing.T) {
	t.Parallel()

	d, ws := newTestRouterModelPoolDialog(t, []string{"anthropic/claude-opus-4.5"})

	// Items are seeded sorted by ID: claude-haiku-4.5 comes first.
	require.False(t, d.selected().checked)
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.True(t, d.selected().checked, "toggling must flip the row's own checked state")
	require.Len(t, ws.setCalls, 1)
	require.Equal(t, "options.router.model_pool", ws.setCalls[0].key)
	require.Equal(t, []string{"anthropic/claude-haiku-4.5", "anthropic/claude-opus-4.5"}, ws.setCalls[0].value)
}

// TestRouterModelPool_FilterNarrowsBySubstring proves typing into the
// filter narrows the visible list by a fuzzy match against the model id,
// shown under a single "OpenRouter" section header so the list is one
// header row plus the matching model rows.
func TestRouterModelPool_FilterNarrowsBySubstring(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)
	for _, r := range "haiku" {
		d.HandleMsg(tea.KeyPressMsg{Text: string(r), Code: r})
	}

	require.Equal(t, 3, d.list.List.Len(), "header + matching model row + trailing spacer")
	require.Equal(t, "anthropic/claude-haiku-4.5", d.selected().ModelID())
}

// TestRouterModelPool_FilterMatchesProviderTitle proves typing the
// provider's own name (as shown in the switch-model dialog, e.g.
// "openrouter") surfaces the whole catalog instead of nothing, since the
// group title is part of what gets matched.
func TestRouterModelPool_FilterMatchesProviderTitle(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)
	for _, r := range "openrouter" {
		d.HandleMsg(tea.KeyPressMsg{Text: string(r), Code: r})
	}

	require.Equal(t, 4, d.list.List.Len(), "header + both catalog models + trailing spacer")
}

// TestRouterModelPool_ListShowsGroupHeader proves the model list is
// grouped under a section header naming the provider, matching the
// grouped, titled look of the regular model switcher.
func TestRouterModelPool_ListShowsGroupHeader(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)

	first := d.list.ItemAt(0)
	header, isHeader := first.(*ModelGroup)
	require.True(t, isHeader, "first row must be the provider section header")
	require.Equal(t, "OpenRouter", header.Title)
}

// TestRouterModelPool_ShowsPerMillionPricing proves each row's right-hand
// column shows the catalog's per-1M input/output pricing, matching what
// the switch-model dialog shows for a model, so the pool is not a
// price-blind list.
func TestRouterModelPool_ShowsPerMillionPricing(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)

	rendered := make(map[string]string)
	for _, it := range d.items {
		rendered[it.ModelID()] = ansi.Strip(it.Render(80))
	}

	require.Contains(t, rendered["anthropic/claude-haiku-4.5"], "$1.10/$5.50 per 1M")
	require.Contains(t, rendered["anthropic/claude-opus-4.5"], "$5.00/$25.00 per 1M")
}

// TestRouterModelPool_ReadsAsChatModelPickerNotDecisionModel proves the
// model-pool picker names itself and explains its purpose in terms that
// cannot be mistaken for the decision-model picker.
func TestRouterModelPool_ReadsAsChatModelPickerNotDecisionModel(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)
	require.Equal(t, "Model Pool", d.dialogTitle())
	require.Contains(t, d.purposeLine(), "Not the decision model")
}

// TestRouterSettings_ModelRowIsLabelledDecisionModel proves the row that
// opens the decision-model picker says so, rather than the bare "Model"
// that read like the chat-model pool row.
func TestRouterSettings_ModelRowIsLabelledDecisionModel(t *testing.T) {
	t.Parallel()

	labels := make(map[string]string, len(routerFieldSpecs))
	for _, spec := range routerFieldSpecs {
		labels[spec.key] = spec.label
	}
	require.Equal(t, "Decision Model", labels["model"])
	require.Equal(t, "Model Pool", labels["model_pool"])
}

// TestRouterModelPool_CloseReturnsActionClose proves Esc closes the
// picker without requiring an explicit save action, matching every other
// dialog's close convention.
func TestRouterModelPool_CloseReturnsActionClose(t *testing.T) {
	t.Parallel()

	d, _ := newTestRouterModelPoolDialog(t, nil)
	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.Equal(t, ActionClose{}, action)
}

// TestRouter_RefreshModelPoolDisplayPicksUpPersistedPool proves the
// parent Router Settings dialog's Model Pool row re-reads config on
// every Draw, so a toggle made in the RouterModelPool picker (which
// persists directly to config, not through this dialog) shows up as soon
// as Router Settings is topmost again.
func TestRouter_RefreshModelPoolDisplayPicksUpPersistedPool(t *testing.T) {
	t.Parallel()

	router, ws := newTestRouterDialog(t, &config.Config{Options: &config.Options{
		Router: &config.RouterOptions{},
	}})
	require.Equal(t, "", router.selectedFieldValue(t, "model_pool"))

	// Simulate the picker persisting directly to the shared config, the
	// way RouterModelPool.persistPool does.
	ws.cfg.Options.Router.ModelPool = []string{"anthropic/claude-haiku-4.5"}

	router.refreshFromConfig()
	require.Equal(t, "anthropic/claude-haiku-4.5", router.selectedFieldValue(t, "model_pool"))
}
