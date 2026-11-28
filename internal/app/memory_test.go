package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/stretchr/testify/require"
)

// isolateHonchoConfig points the shared Honcho config directory at a
// temporary path so a test never reads or writes the developer's real
// ~/.honcho.
func isolateHonchoConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())
	// Resolve also consults these, so a developer's environment must
	// not leak into the result.
	t.Setenv("HONCHO_API_KEY", "")
	t.Setenv("HONCHO_URL", "")
	t.Setenv("HONCHO_BASE_URL", "")
	t.Setenv("HONCHO_WORKSPACE", "")
	t.Setenv("HONCHO_WORKSPACE_ID", "")
	t.Setenv("HONCHO_PEER_NAME", "")
	t.Setenv("HONCHO_AI_PEER", "")
}

func TestNewMemoryDisabledByDefault(t *testing.T) {
	isolateHonchoConfig(t)

	// No config block, no key, no token: the integration must stay
	// off so an ordinary user pays nothing for it.
	svc := newMemory(t.Context(), &config.Config{}, t.TempDir())
	require.Nil(t, svc)
	require.Nil(t, memoryOrNil(svc), "a nil service must not become a non-nil interface")
}

func TestNewMemoryEnabledWithoutCredentialsStaysOff(t *testing.T) {
	isolateHonchoConfig(t)

	// Enabling without any way to authenticate cannot work against
	// Honcho Cloud, so the service refuses to start rather than
	// failing on every turn.
	cfg := &config.Config{Honcho: &config.Honcho{Enabled: true}}
	require.Nil(t, newMemory(t.Context(), cfg, t.TempDir()))
}

func TestNewMemoryLocalDeploymentNeedsNoKey(t *testing.T) {
	isolateHonchoConfig(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	// httptest binds loopback, which IsLocal treats as needing no
	// authentication.
	cfg := &config.Config{Honcho: &config.Honcho{BaseURL: srv.URL}}
	svc := newMemory(t.Context(), cfg, t.TempDir())
	require.NotNil(t, svc, "a loopback deployment should start without an API key")
	t.Cleanup(func() { svc.Close(t.Context()) })

	require.NotNil(t, memoryOrNil(svc))
	require.Equal(t, honcho.DefaultWorkspace, svc.Identity().Workspace)
}

func TestNewMemoryAppliesCrushConfig(t *testing.T) {
	isolateHonchoConfig(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Honcho: &config.Honcho{
		BaseURL:         srv.URL,
		Workspace:       "My Project",
		PeerName:        "kieran",
		AgentPeer:       "crush-dev",
		SessionStrategy: string(honcho.StrategyGlobal),
		RecallMode:      string(honcho.RecallTools),
	}}
	svc := newMemory(t.Context(), cfg, t.TempDir())
	require.NotNil(t, svc)
	t.Cleanup(func() { svc.Close(t.Context()) })

	id := svc.Identity()
	require.Equal(t, "my-project", id.Workspace, "workspace should be normalized")
	require.Equal(t, "kieran", id.UserPeer)
	require.Equal(t, "crush-dev", id.AgentPeer)
	require.Contains(t, id.SessionKey, "global")
	require.Equal(t, honcho.RecallTools, svc.Config().RecallMode)
}

func TestCaptureToolsDefaultsOn(t *testing.T) {
	isolateHonchoConfig(t)

	t.Run("omitted means on", func(t *testing.T) {
		// A block that enables memory without mentioning capture
		// should get the default rather than a silent false.
		got := honcho.FromCrushConfig(&config.Honcho{Enabled: true})
		require.True(t, got.CaptureTools)
	})

	t.Run("explicit false is honored", func(t *testing.T) {
		off := false
		got := honcho.FromCrushConfig(&config.Honcho{Enabled: true, CaptureTools: &off})
		require.False(t, got.CaptureTools)
	})

	t.Run("explicit true is honored", func(t *testing.T) {
		on := true
		got := honcho.FromCrushConfig(&config.Honcho{Enabled: true, CaptureTools: &on})
		require.True(t, got.CaptureTools)
	})

	t.Run("nil block is the zero value", func(t *testing.T) {
		require.Equal(t, honcho.Config{}, honcho.FromCrushConfig(nil))
	})
}

func TestHonchoFeaturesGateSkills(t *testing.T) {
	isolateHonchoConfig(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// With memory off, the feature is absent and memory-specific
	// skills stay out of the system prompt entirely.
	require.Empty(t, honcho.Features(&config.Config{}))
	require.Empty(t, honcho.Features(nil))

	enabled := &config.Config{Honcho: &config.Honcho{BaseURL: srv.URL}}
	require.Equal(t, []string{honcho.Feature}, honcho.Features(enabled))
}

func TestMemoryProviderStartsAndStopsWithConfig(t *testing.T) {
	isolateHonchoConfig(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	off := func() *config.ConfigStore { return config.NewTestStore(&config.Config{}) }
	on := func() *config.ConfigStore {
		return config.NewTestStore(&config.Config{Honcho: &config.Honcho{BaseURL: srv.URL}})
	}

	app := &App{config: off()}
	provider := app.memoryProvider(t.Context())
	t.Cleanup(func() { app.currentMemory().Close(t.Context()) })

	// Off to begin with: nobody has connected anything.
	require.Nil(t, provider(), "memory must stay off until it is configured")

	// Connecting is exactly this: memory starts resolving to enabled,
	// and the next agent rebuild asks the provider again.
	app.config = on()
	mem := provider()
	require.NotNil(t, mem, "connecting must take effect without a restart")

	// Asking again must not churn the service, or every unrelated
	// tool rebuild would restart the write queue under a running turn.
	require.Same(t, mem, provider(), "the service should be built once and reused")

	// Disconnecting is the mirror image.
	app.config = off()
	require.Nil(t, provider(), "disconnecting must take effect without a restart")
	require.Nil(t, app.currentMemory())
}
