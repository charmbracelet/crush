package fantasyadapter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRegistryBuildsCoreProviders(t *testing.T) {
	t.Parallel()
	for _, providerType := range []string{"openai", "anthropic"} {
		provider, err := Build(providerType, Request{Headers: make(map[string]string)})
		require.NoError(t, err)
		require.NotNil(t, provider)
	}
}

func TestRegistryRejectsDuplicateAndUnknownTypes(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	factory := defaultRegistry.factories["openai"]
	require.NoError(t, r.Register("openai", factory))
	require.Error(t, r.Register("openai", factory))
	_, err := r.Build("missing", Request{})
	require.ErrorContains(t, err, "not registered")
}
