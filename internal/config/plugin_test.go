package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// writePlugin drops a plugin script into the working directory's plugin
// folder, which is where a project ships providers.
func writePlugin(t *testing.T, workDir, name, script string) {
	t.Helper()
	dir := filepath.Join(workDir, ".crush", "plugins")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
}

// TestPluginAddsOAuthProvider proves the point of the plugin system: a
// provider that Crush does not ship becomes sign-in-able from a Bash
// script alone, with no code and no Crush release.
func TestPluginAddsOAuthProvider(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	writePlugin(t, workDir, "example.sh", `#!/usr/bin/env bash
provider add example \
  --type openai-compat \
  --base-url "https://api.example.com/v1" \
  --oauth-issuer "https://auth.example.com" \
  --oauth-client-id "crush-example" \
  --oauth-scope openid \
  --oauth-scope offline_access \
  --oauth-flow device \
  --discover-models false
model add example/code-1 --name "Code 1" --context-window 128000`)

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)

	pc, ok := store.Config().Providers.Get("example")
	require.True(t, ok, "the plugin's provider should be configured")
	require.True(t, pc.UsesOAuth(), "an --oauth-* declaration means the provider signs in")
	require.NotNil(t, pc.Auth)
	require.Equal(t, "https://auth.example.com", pc.Auth.Issuer)
	require.Equal(t, "crush-example", pc.Auth.ClientID)
	require.Equal(t, []string{"openid", "offline_access"}, pc.Auth.Scopes)
	require.Equal(t, "device", pc.Auth.Flow)
	require.Equal(t, "https://api.example.com/v1", pc.BaseURL)
	require.Len(t, pc.Models, 1)
	require.Equal(t, "code-1", pc.Models[0].ID)
}

// TestPluginOverridesGlobalConfigAndYieldsToCrushrc pins the precedence: a
// plugin outranks the user's global config but a project's own crushrc
// outranks the plugin, so plugins never trap a project.
func TestPluginOverridesGlobalConfigAndYieldsToCrushrc(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	globalDir := filepath.Join(isolated, ".config", "crush")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "crushrc"),
		[]byte(`provider add shared --type openai-compat --base-url "https://global.example/v1" --api-key gk --discover-models false
model add shared/m1 --name M1`),
		0o644,
	))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	writePlugin(t, workDir, "shared.sh", `provider add shared --base-url "https://plugin.example/v1"`)

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	pc, ok := store.Config().Providers.Get("shared")
	require.True(t, ok)
	require.Equal(t, "https://plugin.example/v1", pc.BaseURL, "the plugin should outrank the global crushrc")

	// The same project, now with a crushrc that disagrees.
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crushrc"),
		[]byte(`provider add shared --base-url "https://project.example/v1"`),
		0o644,
	))

	store, err = config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	pc, ok = store.Config().Providers.Get("shared")
	require.True(t, ok)
	require.Equal(t, "https://project.example/v1", pc.BaseURL, "the project crushrc should outrank the plugin")
}

// TestPluginFailureFailsLoad keeps the trust model: a plugin is Bash, and a
// broken one is a load error rather than a silently missing provider.
func TestPluginFailureFailsLoad(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	writePlugin(t, workDir, "broken.sh", `provider add broken --oauth-flwo device`)

	_, err := config.Load(workDir, dataDir, false)
	require.Error(t, err, "a plugin with a bad flag must fail the load")
	require.Contains(t, err.Error(), "unknown flag")
}

// TestIncompleteOAuthProviderIsSkipped makes sure a provider whose OAuth
// block names no authorization server at all is dropped rather than left
// half-configured.
func TestIncompleteOAuthProviderIsSkipped(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crushrc"),
		[]byte(`provider add unusable --type openai-compat --base-url "https://api.example.com/v1" --oauth-client-id abc --discover-models false
model add unusable/m1 --name M1`),
		0o644,
	))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	_, ok := store.Config().Providers.Get("unusable")
	require.False(t, ok, "an OAuth provider with no issuer or endpoints cannot be used and must be dropped")
}

// TestUsageReportWithoutEndpointIsDropped keeps a bad usage declaration from
// breaking an otherwise usable provider: the endpoint is dropped and warned
// about, the provider stays.
func TestUsageReportWithoutEndpointIsDropped(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("QUOTA_TOKEN", "quota-key")
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crushrc"),
		[]byte(`provider add quotaless --type openai-compat --base-url "https://api.example.com/v1" --api-key k --usage-meters buckets --discover-models false
provider add reported --type openai-compat --base-url "https://api.example.com/v1" --api-key k \
  --usage-url "https://api.example.com/quota/$QUOTA_TOKEN" --usage-method POST --discover-models false
model add reported/m1 --name M1
model add quotaless/m1 --name M1`),
		0o644,
	))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)

	// Declaring usage fields without a URL is a mistake in the plugin, not a
	// reason to lose the provider.
	quotaless, ok := store.Config().Providers.Get("quotaless")
	require.True(t, ok, "the provider must survive a bad usage block")
	require.Nil(t, quotaless.Usage)

	// The endpoint is expanded like any other credential, so a plugin can
	// carry a token through the environment.
	reported, ok := store.Config().Providers.Get("reported")
	require.True(t, ok)
	require.NotNil(t, reported.Usage)
	require.Equal(t, "https://api.example.com/quota/quota-key", reported.Usage.URL)
	require.Equal(t, "POST", reported.Usage.Method)
}

// A gateway adapter is jq the plugin author wrote, so a typo in it has to
// fail the load naming the provider, not surface as a failed request later.
func TestGatewayAdapterWithBadProgramFailsLoad(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crushrc"),
		[]byte(`provider add broken --type openai-compat --base-url "https://api.example.com/v1" --api-key k \
  --gateway-request 'if' --gateway-response '.' --discover-models false
model add broken/m1 --name M1`),
		0o644,
	))

	_, err := config.Load(workDir, t.TempDir(), false)
	require.ErrorContains(t, err, "invalid gateway adapter for provider broken")
}

// isolateConfig points every config and data location at a temporary home, so
// a test cannot read a real user's plugins.
func isolateConfig(t *testing.T) string {
	t.Helper()
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))
	require.NoError(t, os.MkdirAll(filepath.Join(isolated, ".config", "crush"), 0o755))
	return isolated
}

// writePluginDir drops scripts into a subdirectory of a plugins directory,
// which is where an installed plugin repository lives.
func writePluginDir(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, script := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	}
}

// TestPluginSubdirectoriesLoad pins the directory shape an installed
// repository brings: the scripts one level deep run, the hidden lock file
// beside them does not, and anything nested deeper stays ignored.
func TestPluginSubdirectoriesLoad(t *testing.T) {
	isolated := isolateConfig(t)
	workDir := t.TempDir()

	writePlugin(t, workDir, "top.sh", `provider add topsrc --type openai-compat --base-url "https://top.example/v1" --api-key k --discover-models false
model add topsrc/m1 --name M1`)
	writePluginDir(t, filepath.Join(workDir, ".crush", "plugins", "example__repo"), map[string]string{
		"repo.sh": `provider add repoone --type openai-compat --base-url "https://repo.example/v1" --api-key k --discover-models false
model add repoone/m1 --name M1`,
		".hidden.sh": `provider add hiddenone --type openai-compat --base-url "https://hidden.example/v1" --api-key k --discover-models false
model add hiddenone/m1 --name M1`,
		".plugin.json": `{"source":"example/repo","commit":"deadbeef"}`,
	})
	writePluginDir(t, filepath.Join(workDir, ".crush", "plugins", "example__repo", "nested"), map[string]string{
		"deep.sh": `provider add repotwo --type openai-compat --base-url "https://deep.example/v1" --api-key k --discover-models false
model add repotwo/m1 --name M1`,
	})
	writePluginDir(t, filepath.Join(isolated, ".config", "crush", "plugins", "global__repo"), map[string]string{
		"global.sh": `provider add globalone --type openai-compat --base-url "https://globalrepo.example/v1" --api-key k --discover-models false
model add globalone/m1 --name M1`,
	})

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)

	cfg := store.Config()
	for _, id := range []string{"topsrc", "repoone", "globalone"} {
		_, ok := cfg.Providers.Get(id)
		require.True(t, ok, "%s should load from a plugin subdirectory", id)
	}
	for _, id := range []string{"repotwo", "hiddenone"} {
		_, ok := cfg.Providers.Get(id)
		require.False(t, ok, "%s is nested deeper or hidden and must not run", id)
	}
}

// TestPluginSubdirectoryOrder keeps load order predictable: scripts sort by
// their full name across a plugins directory, so a repository that comes later
// alphabetically wins a declaration both of them make.
func TestPluginSubdirectoryOrder(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()

	declare := func(url string) string {
		return `provider add shared --type openai-compat --base-url "` + url + `" --api-key k --discover-models false
model add shared/m1 --name M1`
	}
	plugins := filepath.Join(workDir, ".crush", "plugins")
	writePlugin(t, workDir, "shared.sh", declare("https://loose.example/v1"))
	writePluginDir(t, filepath.Join(plugins, "a__repo"), map[string]string{
		"shared.sh": declare("https://first.example/v1"),
	})
	writePluginDir(t, filepath.Join(plugins, "z__repo"), map[string]string{
		"shared.sh": declare("https://last.example/v1"),
	})

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	pc, ok := store.Config().Providers.Get("shared")
	require.True(t, ok)
	require.Equal(t, "https://last.example/v1", pc.BaseURL, "the last script in name order should win")
}
