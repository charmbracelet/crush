package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSaveSearchEngine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	ws := &testWorkspace{cfg: cfg, configFields: map[string]any{}}
	ui := newTestUIWithConfig(t, cfg)
	ui.com.Workspace = ws

	err := ui.saveSearchEngine(config.SearchEngineKagi)
	require.NoError(t, err)
	require.Equal(t, "kagi", ws.configFields["tools.web_search.search_engine"])
}

func TestSaveSearchEngineRejectsInvalidEngine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	ws := &testWorkspace{cfg: cfg, configFields: map[string]any{}}
	ui := newTestUIWithConfig(t, cfg)
	ui.com.Workspace = ws

	err := ui.saveSearchEngine(config.SearchEngine("bad"))
	require.Error(t, err)
	require.Empty(t, ws.configFields)
}

func TestSearchEngineDisplayName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "DuckDuckGo", searchEngineDisplayName(config.SearchEngineDuckDuckGo))
	require.Equal(t, "Kagi", searchEngineDisplayName(config.SearchEngineKagi))
}
