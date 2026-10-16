package config

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestToolWebSearchDefaultsToExa(t *testing.T) {
	t.Parallel()

	require.Equal(t, SearchEngineExa, ToolWebSearch{}.Engine())
	require.Equal(t, SearchEngineExa, ToolWebSearch{SearchEngine: "invalid"}.Engine())
	require.Equal(t, SearchEngineDuckDuckGo, ToolWebSearch{SearchEngine: SearchEngineDuckDuckGo}.Engine())
}

func TestToolWebSearchResolvedExaAPIKey(t *testing.T) {
	t.Parallel()

	cfg := ToolWebSearch{ExaAPIKey: "$EXA_API_KEY"}
	resolver := NewShellVariableResolver(env.NewFromMap(map[string]string{"EXA_API_KEY": "resolved-key"}))

	require.Equal(t, "resolved-key", cfg.ResolvedExaAPIKey(resolver))
	require.Equal(t, "$EXA_API_KEY", cfg.ResolvedExaAPIKey(nil))
	require.Empty(t, cfg.ResolvedExaAPIKey(stubResolver{err: errors.New("resolver failure")}))
	require.Empty(t, ToolWebSearch{}.ResolvedExaAPIKey(resolver))
}
