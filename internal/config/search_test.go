package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type testResolver map[string]string

func (r testResolver) ResolveValue(value string) (string, error) {
	return r[value], nil
}

type errorResolver struct{}

func (errorResolver) ResolveValue(value string) (string, error) {
	return "", fmt.Errorf("missing value")
}

func TestToolWebSearchEngine(t *testing.T) {
	t.Parallel()

	require.Equal(t, SearchEngineDuckDuckGo, ToolWebSearch{}.Engine())
	require.Equal(t, SearchEngineDuckDuckGo, ToolWebSearch{SearchEngine: SearchEngine("bad")}.Engine())
	require.Equal(t, SearchEngineKagi, ToolWebSearch{SearchEngine: SearchEngineKagi}.Engine())
}

func TestToolWebSearchResolvedKagiAPIKey(t *testing.T) {
	t.Parallel()

	cfg := ToolWebSearch{KagiAPIKey: "$KAGI_API_KEY"}
	resolver := testResolver{"$KAGI_API_KEY": "resolved-key"}

	require.Equal(t, "resolved-key", cfg.ResolvedKagiAPIKey(resolver))
	require.Equal(t, "$KAGI_API_KEY", cfg.ResolvedKagiAPIKey(nil))
	require.Equal(t, "$KAGI_API_KEY", cfg.ResolvedKagiAPIKey(errorResolver{}))
	require.Empty(t, ToolWebSearch{}.ResolvedKagiAPIKey(resolver))
}
