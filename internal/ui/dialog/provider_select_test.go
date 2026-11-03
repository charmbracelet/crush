package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// configWorkspace is the minimal workspace a dialog needs: the config it
// reads providers and models from.
type configWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *configWorkspace) Config() *config.Config { return w.cfg }

func testConfig() *config.Config {
	return &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
}

func testCommon(cfg *config.Config) *common.Common {
	sty := styles.CharmtonePantera()
	if cfg == nil {
		cfg = testConfig()
	}
	return &common.Common{Workspace: &configWorkspace{cfg: cfg}, Styles: &sty}
}

func testProviderItems() []list.FilterableItem {
	sty := styles.CharmtonePantera()
	return []list.FilterableItem{
		NewProviderItem(&sty, catwalk.Provider{ID: "hyper", Name: "Hyper"}),
		NewProviderItem(&sty, catwalk.Provider{ID: "openai", Name: "OpenAI"}),
		NewProviderItem(&sty, catwalk.Provider{ID: "anthropic", Name: "Anthropic"}),
	}
}

func TestProviderSelectPromotesHyper(t *testing.T) {
	t.Parallel()

	m := newProviderSelect(testCommon(nil), testProviderItems())
	require.Equal(t, ProviderSelectID, m.ID())

	first, ok := m.list.SelectedItem().(*ProviderItem)
	require.True(t, ok)
	require.Equal(t, "hyper", first.ID())
	require.Equal(t, providerPromotedBadge, first.badge)
	require.Contains(t, first.Render(64), providerPromotedBadge)
}

func TestProviderSelectEmitsSelectedProvider(t *testing.T) {
	t.Parallel()

	m := newProviderSelect(testCommon(nil), testProviderItems())

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectProvider)
	require.True(t, ok)
	require.Equal(t, "hyper", string(selected.Provider.ID))
}

func TestProviderSelectNavigatesWithWrapping(t *testing.T) {
	t.Parallel()

	m := newProviderSelect(testCommon(nil), testProviderItems())

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, "anthropic", string(action.(ActionSelectProvider).Provider.ID))

	m = newProviderSelect(testCommon(nil), testProviderItems())
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, "openai", string(action.(ActionSelectProvider).Provider.ID))
}

func TestProviderSelectFilters(t *testing.T) {
	t.Parallel()

	m := newProviderSelect(testCommon(nil), testProviderItems())

	for _, r := range "anth" {
		m.HandleMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectProvider)
	require.True(t, ok)
	require.Equal(t, "anthropic", string(selected.Provider.ID))
}

func TestProviderSelectIgnoresEscape(t *testing.T) {
	t.Parallel()

	m := newProviderSelect(testCommon(nil), testProviderItems())
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
}

func TestProviderSelectBlurbs(t *testing.T) {
	t.Parallel()

	hyper := NewProviderItem(new(styles.Styles), catwalk.Provider{ID: "hyper", Name: "Hyper"})
	require.Contains(t, hyper.blurb, "Charm subscription")

	unknown := NewProviderItem(new(styles.Styles), catwalk.Provider{ID: "acme", Name: "Acme"})
	require.Equal(t, providerFallbackBlurb, unknown.blurb)
}
