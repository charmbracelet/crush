package config

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestPrepareGateway(t *testing.T) {
	// Not parallel: captures the default logger.
	valid := &oauth.GatewaySpec{Request: ".", Response: ".body"}
	tests := []struct {
		name    string
		id      string
		typ     catwalk.Type
		spec    *oauth.GatewaySpec
		wantErr string
		kept    bool
		warns   bool
	}{
		{name: "none", id: "local"},
		{name: "incomplete programs are dropped", id: "local", spec: &oauth.GatewaySpec{Request: "."}, warns: true},
		{name: "bad jq fails the load", id: "local", spec: &oauth.GatewaySpec{Request: "if", Response: "."}, wantErr: "invalid gateway adapter for provider local"},
		{name: "an anthropic-type provider takes it silently", id: "local", typ: catwalk.TypeMessages, spec: valid, kept: true},
		{name: "gemini takes it silently", id: string(catwalk.InferenceProviderGemini), spec: valid, kept: true},
		{name: "a completions provider warns", id: "openrouter", typ: catwalk.TypeCompletions, spec: valid, kept: true, warns: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			pc := ProviderConfig{Type: tt.typ, Gateway: tt.spec}
			err := prepareGateway(tt.id, &pc)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.kept, pc.Gateway != nil)
			require.Equal(t, tt.warns, strings.Contains(strings.ToLower(logs.String()), "gateway adapter"), logs.String())
		})
	}
}

// A known provider's gateway is validated at load like a custom one's.
func TestConfigureProviders_ValidatesKnownProviderGateway(t *testing.T) {
	t.Parallel()
	cfg := &Config{Providers: csync.NewMap[string, ProviderConfig]()}
	cfg.Providers.Set("openai", ProviderConfig{Gateway: &oauth.GatewaySpec{Request: "if", Response: "."}})
	cfg.setDefaults(t.TempDir(), "")
	testEnv := env.NewFromMap(map[string]string{"OPENAI_API_KEY": "k"})
	known := []catwalk.Provider{{ID: "openai", APIKey: "$OPENAI_API_KEY", APIEndpoint: "https://api.openai.com/v1", Models: []catwalk.Model{{ID: "m"}}}}

	err := cfg.configureProviders(context.Background(), testStore(cfg), testEnv, NewShellVariableResolver(testEnv), known)
	require.ErrorContains(t, err, "invalid gateway adapter for provider openai")
}
