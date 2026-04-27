package dialog

import (
	"testing"

	"charm.land/bubbles/v2/key"
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

func TestSearchEnginesEditSelectedKagi(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Tools: config.Tools{
			WebSearch: config.ToolWebSearch{SearchEngine: config.SearchEngineKagi},
		},
	}})

	dlg, err := NewSearchEngines(com)
	require.NoError(t, err)

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	require.Equal(t, ActionSelectSearchEngine{Engine: config.SearchEngineKagi, Edit: true}, action)
}

func TestSearchEnginesIgnoresEditForDuckDuckGo(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Tools: config.Tools{
			WebSearch: config.ToolWebSearch{SearchEngine: config.SearchEngineDuckDuckGo},
		},
	}})

	dlg, err := NewSearchEngines(com)
	require.NoError(t, err)

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	require.Nil(t, action)
}

func TestSearchEnginesHelpIncludesEditForKagi(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Tools: config.Tools{
			WebSearch: config.ToolWebSearch{SearchEngine: config.SearchEngineKagi},
		},
	}})

	dlg, err := NewSearchEngines(com)
	require.NoError(t, err)

	require.Contains(t, helpBindingKeys(dlg.ShortHelp()), "ctrl+e")
}

func TestSearchEnginesHelpOmitsEditForDuckDuckGo(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{
		Tools: config.Tools{
			WebSearch: config.ToolWebSearch{SearchEngine: config.SearchEngineDuckDuckGo},
		},
	}})

	dlg, err := NewSearchEngines(com)
	require.NoError(t, err)

	require.NotContains(t, helpBindingKeys(dlg.ShortHelp()), "ctrl+e")
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

func TestKagiAPIKeyInputSavesVerifiedKey(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{}})
	dlg, cmd := NewKagiAPIKeyInput(com)
	require.Nil(t, cmd)

	dlg.input.SetValue("modified-key")
	dlg.HandleMsg(ActionChangeAPIKeyState{State: APIKeyInputStateVerified, APIKey: "verified-key"})

	action := dlg.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ActionSaveKagiAPIKey{APIKey: "verified-key"}, action)
}

func TestKagiAPIKeyInputIgnoresPasteWhileVerifying(t *testing.T) {
	t.Parallel()

	com := common.DefaultCommon(&testWorkspace{cfg: &config.Config{}})
	dlg, cmd := NewKagiAPIKeyInput(com)
	require.Nil(t, cmd)

	dlg.input.SetValue("original-key")
	action := dlg.HandleMsg(tea.PasteMsg{Content: "pasted-key"})
	require.Nil(t, action)
	require.Equal(t, "original-keypasted-key", dlg.input.Value())

	dlg.state = APIKeyInputStateVerifying
	action = dlg.HandleMsg(tea.PasteMsg{Content: "ignored-key"})
	require.Nil(t, action)
	require.Equal(t, "original-keypasted-key", dlg.input.Value())
}

func helpBindingKeys(bindings []key.Binding) []string {
	keys := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys = append(keys, binding.Keys()...)
	}
	return keys
}
