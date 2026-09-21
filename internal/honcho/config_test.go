package honcho

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// honchoEnvVars are every environment variable Resolve consults. Tests
// blank them so a developer's own shell cannot leak into a result.
var honchoEnvVars = []string{
	"HONCHO_API_KEY",
	"HONCHO_URL",
	"HONCHO_BASE_URL",
	"HONCHO_WORKSPACE",
	"HONCHO_WORKSPACE_ID",
	"HONCHO_PEER_NAME",
	"HONCHO_AI_PEER",
}

// isolateConfig points the shared config at a fresh directory and
// clears every Honcho environment variable. It returns the directory.
//
// These tests cannot run in parallel: t.Setenv forbids it, and the
// shared config path is process-global state.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HONCHO_CONFIG_DIR", dir)
	for _, k := range honchoEnvVars {
		t.Setenv(k, "")
	}
	return dir
}

// writeShared writes raw JSON to the shared config file.
func writeShared(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o600))
}

// readShared reads the shared config file back as a generic object.
func readShared(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

const fullSharedConfig = `{
  "apiKey": "shared-key",
  "baseUrl": "https://shared.example.test",
  "peerName": "shared-user",
  "hosts": {
    "crush": {
      "apiKey": "crush-key",
      "workspace": "shared-ws",
      "aiPeer": "shared-agent",
      "recallMode": "tools",
      "observationMode": "directional",
      "sessionStrategy": "per-repo",
      "agentObserveMe": true,
      "captureTools": false
    }
  }
}`

func TestSharedConfigPathHonoursEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HONCHO_CONFIG_DIR", dir)

	path, err := SharedConfigPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "config.json"), path)
}

func TestSharedConfigPathDefaultsToHome(t *testing.T) {
	t.Setenv("HONCHO_CONFIG_DIR", "   ")

	path, err := SharedConfigPath()
	require.NoError(t, err)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".honcho", "config.json"), path)
}

func TestResolveDefaults(t *testing.T) {
	isolateConfig(t)

	cfg := Resolve(Config{})
	require.Equal(t, DefaultWorkspace, cfg.Workspace)
	require.Equal(t, DefaultPeerName, cfg.PeerName)
	require.Equal(t, DefaultAgentPeer, cfg.AgentPeer)
	require.Equal(t, RecallHybrid, cfg.RecallMode)
	require.Equal(t, ObserveUnified, cfg.ObservationMode)
	require.Equal(t, StrategyPerDirectory, cfg.SessionStrategy)
	require.True(t, cfg.CaptureTools)
	require.False(t, cfg.AgentObserveMe)
	require.Equal(t, 8, cfg.MaxConclusions)
	require.Equal(t, 2000, cfg.ContextTokens)
	require.Empty(t, cfg.APIKey)
	require.Empty(t, cfg.BaseURL)
	require.False(t, cfg.Enabled)
}

func TestResolveMissingSharedFile(t *testing.T) {
	dir := isolateConfig(t)
	require.NoFileExists(t, filepath.Join(dir, "config.json"))

	cfg := Resolve(Config{})
	require.Equal(t, DefaultWorkspace, cfg.Workspace)
	require.Equal(t, DefaultPeerName, cfg.PeerName)
	require.False(t, cfg.Enabled)
}

func TestResolveMalformedSharedFile(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "truncated object", content: `{ "apiKey": `},
		{name: "not json at all", content: "this is not json"},
		{name: "wrong root type", content: `["a","b"]`},
		{name: "wrong field type", content: `{"apiKey": 42}`},
		{name: "empty file", content: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateConfig(t)
			writeShared(t, dir, tc.content)

			require.NotPanics(t, func() {
				cfg := Resolve(Config{})
				require.Equal(t, DefaultWorkspace, cfg.Workspace)
				require.Equal(t, DefaultPeerName, cfg.PeerName)
				require.Equal(t, DefaultAgentPeer, cfg.AgentPeer)
				require.False(t, cfg.Enabled)
			})
		})
	}
}

func TestResolveSharedFile(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, fullSharedConfig)

	cfg := Resolve(Config{})
	// The host section's apiKey wins over the top-level one.
	require.Equal(t, "crush-key", cfg.APIKey)
	require.Equal(t, "https://shared.example.test", cfg.BaseURL)
	require.Equal(t, "shared-user", cfg.PeerName)
	require.Equal(t, "shared-ws", cfg.Workspace)
	require.Equal(t, "shared-agent", cfg.AgentPeer)
	require.Equal(t, RecallTools, cfg.RecallMode)
	require.Equal(t, ObserveDirectional, cfg.ObservationMode)
	require.Equal(t, StrategyPerRepo, cfg.SessionStrategy)
	require.True(t, cfg.AgentObserveMe)
	require.False(t, cfg.CaptureTools)
	require.True(t, cfg.Enabled)
}

func TestResolveSharedFileTopLevelKeyOnly(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, `{"apiKey":"top-key","peerName":"top-user"}`)

	cfg := Resolve(Config{})
	require.Equal(t, "top-key", cfg.APIKey)
	require.Equal(t, "top-user", cfg.PeerName)
	require.Equal(t, DefaultWorkspace, cfg.Workspace)
	require.True(t, cfg.Enabled)
}

func TestResolveOverridesBeatSharedFile(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, fullSharedConfig)

	cfg := Resolve(Config{
		APIKey:          "override-key",
		BaseURL:         "https://override.example.test",
		Workspace:       "override-ws",
		PeerName:        "override-user",
		AgentPeer:       "override-agent",
		RecallMode:      RecallContext,
		ObservationMode: ObserveUnified,
		SessionStrategy: StrategyGlobal,
		MaxConclusions:  11,
		ContextTokens:   4242,
	})
	require.Equal(t, "override-key", cfg.APIKey)
	require.Equal(t, "https://override.example.test", cfg.BaseURL)
	require.Equal(t, "override-ws", cfg.Workspace)
	require.Equal(t, "override-user", cfg.PeerName)
	require.Equal(t, "override-agent", cfg.AgentPeer)
	require.Equal(t, RecallContext, cfg.RecallMode)
	require.Equal(t, ObserveUnified, cfg.ObservationMode)
	require.Equal(t, StrategyGlobal, cfg.SessionStrategy)
	require.Equal(t, 11, cfg.MaxConclusions)
	require.Equal(t, 4242, cfg.ContextTokens)
}

func TestResolveZeroOverridesDeferToSharedFile(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, fullSharedConfig)

	cfg := Resolve(Config{})
	require.Equal(t, "shared-ws", cfg.Workspace)
	require.Equal(t, "shared-agent", cfg.AgentPeer)
	require.Equal(t, 8, cfg.MaxConclusions)
	require.Equal(t, 2000, cfg.ContextTokens)
}

func TestResolveEnvBeatsEverything(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, fullSharedConfig)

	t.Setenv("HONCHO_API_KEY", "env-key")
	t.Setenv("HONCHO_URL", "https://env.example.test")
	t.Setenv("HONCHO_WORKSPACE", "env-ws")
	t.Setenv("HONCHO_PEER_NAME", "env-user")
	t.Setenv("HONCHO_AI_PEER", "env-agent")

	cfg := Resolve(Config{
		APIKey:    "override-key",
		BaseURL:   "https://override.example.test",
		Workspace: "override-ws",
		PeerName:  "override-user",
		AgentPeer: "override-agent",
	})
	require.Equal(t, "env-key", cfg.APIKey)
	require.Equal(t, "https://env.example.test", cfg.BaseURL)
	require.Equal(t, "env-ws", cfg.Workspace)
	require.Equal(t, "env-user", cfg.PeerName)
	require.Equal(t, "env-agent", cfg.AgentPeer)
}

func TestResolveEnvAliases(t *testing.T) {
	cases := []struct {
		name      string
		env       map[string]string
		wantURL   string
		wantWsRaw string
	}{
		{
			name:      "HONCHO_BASE_URL and HONCHO_WORKSPACE_ID",
			env:       map[string]string{"HONCHO_BASE_URL": "https://alias.test", "HONCHO_WORKSPACE_ID": "alias-ws"},
			wantURL:   "https://alias.test",
			wantWsRaw: "alias-ws",
		},
		{
			name: "HONCHO_URL wins over HONCHO_BASE_URL",
			env: map[string]string{
				"HONCHO_URL":      "https://primary.test",
				"HONCHO_BASE_URL": "https://alias.test",
			},
			wantURL:   "https://primary.test",
			wantWsRaw: DefaultWorkspace,
		},
		{
			name: "HONCHO_WORKSPACE wins over HONCHO_WORKSPACE_ID",
			env: map[string]string{
				"HONCHO_WORKSPACE":    "primary-ws",
				"HONCHO_WORKSPACE_ID": "alias-ws",
			},
			wantWsRaw: "primary-ws",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg := Resolve(Config{})
			require.Equal(t, tc.wantURL, cfg.BaseURL)
			require.Equal(t, tc.wantWsRaw, cfg.Workspace)
		})
	}
}

func TestResolveIgnoresOtherHostSections(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, `{
  "peerName": "shared-user",
  "hosts": {
    "opencode": {
      "apiKey": "opencode-key",
      "workspace": "opencode-ws",
      "aiPeer": "opencode-agent",
      "recallMode": "tools",
      "sessionStrategy": "global"
    }
  }
}`)

	cfg := Resolve(Config{})
	require.Equal(t, "shared-user", cfg.PeerName)
	require.Equal(t, DefaultWorkspace, cfg.Workspace)
	require.Equal(t, DefaultAgentPeer, cfg.AgentPeer)
	require.Equal(t, RecallHybrid, cfg.RecallMode)
	require.Equal(t, StrategyPerDirectory, cfg.SessionStrategy)
	require.Empty(t, cfg.APIKey)
	require.False(t, cfg.Enabled)
}

func TestResolveMalformedHostSectionIsSkipped(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, `{"peerName":"shared-user","hosts":{"crush":"not-an-object"}}`)

	cfg := Resolve(Config{})
	require.Equal(t, "shared-user", cfg.PeerName)
	require.Equal(t, DefaultWorkspace, cfg.Workspace)
	require.Equal(t, DefaultAgentPeer, cfg.AgentPeer)
}

func TestResolveNormalizesIdentifiers(t *testing.T) {
	isolateConfig(t)

	cfg := Resolve(Config{Workspace: "My Work/Space", AgentPeer: "Crush Agent"})
	require.Equal(t, "my-work-space", cfg.Workspace)
	require.Equal(t, "crush-agent", cfg.AgentPeer)
}

func TestResolveEnabled(t *testing.T) {
	cases := []struct {
		name      string
		overrides Config
		env       map[string]string
		want      bool
	}{
		{name: "nothing configured", want: false},
		{name: "api key present", overrides: Config{APIKey: "k"}, want: true},
		{name: "api key from env", env: map[string]string{"HONCHO_API_KEY": "k"}, want: true},
		{
			name:      "custom base URL",
			overrides: Config{BaseURL: "http://localhost:8000"},
			want:      true,
		},
		{
			name:      "default base URL alone is not enough",
			overrides: Config{BaseURL: DefaultBaseURL},
			want:      false,
		},
		{
			name:      "explicit enable with no credentials",
			overrides: Config{Enabled: true},
			want:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			require.Equal(t, tc.want, Resolve(tc.overrides).Enabled)
		})
	}
}

func TestResolveExplicitEnableCarriesBooleans(t *testing.T) {
	isolateConfig(t)

	cfg := Resolve(Config{Enabled: true, AgentObserveMe: true, CaptureTools: true})
	require.True(t, cfg.AgentObserveMe)
	require.True(t, cfg.CaptureTools)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	base := func(mutate func(*Config)) Config {
		c := Config{
			Enabled:   true,
			APIKey:    "key",
			Workspace: "ws",
			PeerName:  "user",
			AgentPeer: "crush",
		}
		mutate(&c)
		return c
	}

	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid cloud config",
			cfg:  base(func(*Config) {}),
		},
		{
			name:    "disabled",
			cfg:     base(func(c *Config) { c.Enabled = false }),
			wantErr: "not configured",
		},
		{
			name:    "missing workspace",
			cfg:     base(func(c *Config) { c.Workspace = "" }),
			wantErr: "workspace is required",
		},
		{
			name:    "missing peer name",
			cfg:     base(func(c *Config) { c.PeerName = "" }),
			wantErr: "peer name is required",
		},
		{
			name: "no credential at all on cloud",
			cfg: base(func(c *Config) {
				c.APIKey = ""
				c.BaseURL = DefaultBaseURL
			}),
			wantErr: "sign in",
		},
		{
			name: "no credential with empty base URL",
			cfg: base(func(c *Config) {
				c.APIKey = ""
			}),
			wantErr: "sign in",
		},
		{
			name: "missing key on localhost",
			cfg: base(func(c *Config) {
				c.APIKey = ""
				c.BaseURL = "http://localhost:8000"
			}),
		},
		{
			name: "missing key on 127.0.0.1",
			cfg: base(func(c *Config) {
				c.APIKey = ""
				c.BaseURL = "http://127.0.0.1:8000"
			}),
		},
		{
			name: "missing key on ipv6 loopback",
			cfg: base(func(c *Config) {
				c.APIKey = ""
				c.BaseURL = "http://[::1]:8000"
			}),
		},
		{
			name: "user peer equals agent peer",
			cfg: base(func(c *Config) {
				c.PeerName = "crush"
			}),
			wantErr: "both",
		},
		{
			name: "user peer normalizes onto agent peer",
			cfg: base(func(c *Config) {
				c.PeerName = "Crush"
			}),
			wantErr: "both",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestConfigValidateDisabledReturnsErrDisabled(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, Config{}.Validate(), ErrDisabled)
}

func TestConfigIsLocal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{name: "empty", baseURL: "", want: false},
		{name: "localhost", baseURL: "http://localhost:8000", want: true},
		{name: "localhost no port", baseURL: "http://localhost", want: true},
		{name: "ipv4 loopback", baseURL: "http://127.0.0.1:8000", want: true},
		{name: "ipv4 loopback alt", baseURL: "http://127.1.2.3:8000", want: true},
		{name: "ipv6 loopback", baseURL: "http://[::1]:8000", want: true},
		{name: "real hostname", baseURL: "https://api.honcho.dev", want: false},
		{name: "default base URL", baseURL: DefaultBaseURL, want: false},
		{name: "lan address", baseURL: "http://192.168.1.10:8000", want: false},
		{name: "malformed", baseURL: "://not a url", want: false},
		{name: "control characters", baseURL: "http://loc\x7falhost", want: false},
		{name: "no scheme", baseURL: "localhost:8000", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Config{BaseURL: tc.baseURL}.IsLocal())
		})
	}
}

func TestSaveSharedRoundTrip(t *testing.T) {
	isolateConfig(t)

	want := Config{
		APIKey:          "round-trip-key",
		BaseURL:         "http://localhost:9000",
		Workspace:       "round-ws",
		PeerName:        "round-user",
		AgentPeer:       "round-agent",
		RecallMode:      RecallTools,
		ObservationMode: ObserveDirectional,
		SessionStrategy: StrategyGitBranch,
		AgentObserveMe:  true,
		CaptureTools:    false,
	}
	require.NoError(t, SaveShared(want))

	got := Resolve(Config{})
	require.Equal(t, want.APIKey, got.APIKey)
	require.Equal(t, want.BaseURL, got.BaseURL)
	require.Equal(t, want.Workspace, got.Workspace)
	require.Equal(t, want.PeerName, got.PeerName)
	require.Equal(t, want.AgentPeer, got.AgentPeer)
	require.Equal(t, want.RecallMode, got.RecallMode)
	require.Equal(t, want.ObservationMode, got.ObservationMode)
	require.Equal(t, want.SessionStrategy, got.SessionStrategy)
	require.True(t, got.AgentObserveMe)
	require.False(t, got.CaptureTools)
	require.True(t, got.Enabled)
}

func TestSaveSharedFileMode(t *testing.T) {
	dir := isolateConfig(t)

	require.NoError(t, SaveShared(Config{Workspace: "ws", PeerName: "user", AgentPeer: "crush"}))

	info, err := os.Stat(filepath.Join(dir, "config.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSaveSharedPreservesForeignData(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, `{
  "apiKey": "existing-key",
  "peerName": "existing-user",
  "someUnknownTopLevelKey": {"nested": [1, 2, 3]},
  "anotherUnknown": "keep me",
  "hosts": {
    "opencode": {
      "apiKey": "opencode-key",
      "workspace": "opencode-ws",
      "sessionStrategy": "global"
    }
  }
}`)

	require.NoError(t, SaveShared(Config{
		Workspace:       "crush-ws",
		AgentPeer:       "crush",
		PeerName:        "crush-user",
		RecallMode:      RecallHybrid,
		ObservationMode: ObserveUnified,
		SessionStrategy: StrategyPerRepo,
		CaptureTools:    true,
	}))

	out := readShared(t, dir)

	// Unknown top-level keys survive untouched.
	require.Equal(t, "keep me", out["anotherUnknown"])
	unknown, ok := out["someUnknownTopLevelKey"].(map[string]any)
	require.True(t, ok, "unknown top-level object was dropped")
	require.Equal(t, []any{float64(1), float64(2), float64(3)}, unknown["nested"])

	// The sibling harness's host section survives untouched.
	hosts, ok := out["hosts"].(map[string]any)
	require.True(t, ok)
	opencode, ok := hosts["opencode"].(map[string]any)
	require.True(t, ok, "hosts.opencode was dropped")
	require.Equal(t, "opencode-key", opencode["apiKey"])
	require.Equal(t, "opencode-ws", opencode["workspace"])
	require.Equal(t, "global", opencode["sessionStrategy"])

	// Crush's own section is written.
	crush, ok := hosts[HostKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "crush-ws", crush["workspace"])
	require.Equal(t, "crush", crush["aiPeer"])
	require.Equal(t, "per-repo", crush["sessionStrategy"])

	// An empty API key does not wipe the stored credential.
	require.Equal(t, "existing-key", out["apiKey"])
	// A supplied peer name does replace the stored one.
	require.Equal(t, "crush-user", out["peerName"])
}

func TestSaveSharedRefusesToClobberUnparseableFile(t *testing.T) {
	dir := isolateConfig(t)
	writeShared(t, dir, `{ this is broken`)

	require.Error(t, SaveShared(Config{Workspace: "ws"}))

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	require.NoError(t, err)
	require.Equal(t, `{ this is broken`, string(data))
}

func TestSaveSharedCreatesMissingDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	t.Setenv("HONCHO_CONFIG_DIR", nested)
	for _, k := range honchoEnvVars {
		t.Setenv(k, "")
	}

	require.NoError(t, SaveShared(Config{Workspace: "ws", PeerName: "user", AgentPeer: "crush"}))
	require.FileExists(t, filepath.Join(nested, "config.json"))
}

func TestApplyHelpers(t *testing.T) {
	t.Parallel()

	s := "original"
	applyString(&s, "")
	require.Equal(t, "original", s)
	applyString(&s, "   ")
	require.Equal(t, "original", s)
	applyString(&s, "  updated  ")
	require.Equal(t, "updated", s)

	b := false
	applyBool(&b, nil)
	require.False(t, b)
	applyBool(&b, boolPtr(true))
	require.True(t, b)
	applyBool(&b, boolPtr(false))
	require.False(t, b)

	require.Empty(t, firstNonEmpty())
	require.Empty(t, firstNonEmpty("", "  "))
	require.Equal(t, "b", firstNonEmpty("", "b", "c"))
}

func TestValidateAcceptsABrowserSignIn(t *testing.T) {
	// The documented way to connect is `crush login honcho`, which
	// stores an OAuth token and never sets an API key. Requiring a
	// key here refused every user who followed the instructions, and
	// did it silently: the service returned nil and memory recorded
	// nothing.
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())

	cfg := Config{
		Enabled:   true,
		Workspace: DefaultWorkspace,
		PeerName:  DefaultPeerName,
		AgentPeer: DefaultAgentPeer,
		BaseURL:   DefaultBaseURL,
	}
	require.Error(t, cfg.Validate(), "no credential of any kind must still fail")

	require.NoError(t, SaveToken(&OAuthToken{AccessToken: "hch-at-test", TokenType: "Bearer"}))
	require.True(t, SignedIn())
	require.NoError(t, cfg.Validate(), "a stored sign-in is a credential")
}
