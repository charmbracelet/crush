package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type testWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *testWorkspace) Config() *config.Config {
	return w.cfg
}

func TestSearchEnginesSelectsCurrentEngine(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Tools: config.Tools{
			WebSearch: config.ToolWebSearch{SearchEngine: config.SearchEngineKagi},
		},
	}})

	dlg, err := NewSearchEngines(com)
	require.NoError(t, err)

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ActionSelectSearchEngine{Engine: config.SearchEngineKagi}, action)
}

func TestKagiAPIKeyInputTrimsKey(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{}})
	dlg, cmd := NewKagiAPIKeyInput(com)
	require.Nil(t, cmd)

	dlg.input.SetValue("  $KAGI_API_KEY  ")
	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ActionSaveKagiAPIKey{APIKey: "$KAGI_API_KEY"}, action)
}

func TestKagiAPIKeyInputIgnoresEmptyKey(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{}})
	dlg, cmd := NewKagiAPIKeyInput(com)
	require.Nil(t, cmd)

	dlg.input.SetValue("   ")
	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, action)
}
