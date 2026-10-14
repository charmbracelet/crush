package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type recentWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *recentWorkspace) Config() *config.Config { return w.cfg }

// The JSON config merge concatenates arrays, so a project-scoped
// recent_models list can repeat a model; the dialog must show it once.
func TestModelsDialogRecentListHasNoDuplicates(t *testing.T) {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{
		ID:     "p",
		Models: []catwalk.Model{{ID: "a"}, {ID: "b"}, {ID: "x"}},
	})
	a := config.SelectedModel{Provider: "p", Model: "a"}
	b := config.SelectedModel{Provider: "p", Model: "b"}
	x := config.SelectedModel{Provider: "p", Model: "x"}
	cfg := &config.Config{
		Options:   &config.Options{DisableDefaultProviders: true, DisableProviderAutoUpdate: true},
		Providers: providers,
		Models:    map[config.SelectedModelType]config.SelectedModel{config.SelectedModelTypeLarge: a},
		RecentModels: map[config.SelectedModelType][]config.SelectedModel{
			config.SelectedModelTypeLarge: {a, b, x, a, b},
		},
	}

	m, err := NewModels(common.DefaultCommon(&recentWorkspace{cfg: cfg}), false)
	require.NoError(t, err)

	var recent *ModelGroup
	for i := range m.list.groups {
		if m.list.groups[i].Title == "Recently used" {
			recent = &m.list.groups[i]
		}
	}
	require.NotNil(t, recent)
	var ids []string
	for _, it := range recent.Items {
		ids = append(ids, it.ID())
	}
	require.Equal(t, []string{"p:a", "p:b", "p:x"}, ids)
}
