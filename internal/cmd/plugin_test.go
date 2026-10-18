package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/plugins"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const (
	pluginOne     = `provider add one --type openai-compat --base-url "https://one.example/v1" --api-key k`
	pluginTwo     = `provider add two --type openai-compat --base-url "https://two.example/v1" --api-key k`
	pluginSHAOne  = "1111111111111111111111111111111111111111"
	pluginSHATwo  = "2222222222222222222222222222222222222222"
	pluginTestRef = "HEAD"
)

// fakePluginAPI is a stand-in for api.github.com. A repository resolves its ref
// to one commit whose archive holds a set of files, and push replaces the
// commit the way an upstream repository would.
type fakePluginAPI struct {
	sha    string
	files  map[string]string
	server *httptest.Server
}

func newFakePluginAPI(t *testing.T) *fakePluginAPI {
	t.Helper()
	f := &fakePluginAPI{sha: pluginSHAOne, files: map[string]string{"one.sh": pluginOne}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)

	prev := newGitHubClient
	newGitHubClient = func() plugins.GitHub {
		return plugins.GitHub{BaseURL: f.server.URL, Client: f.server.Client()}
	}
	t.Cleanup(func() { newGitHubClient = prev })
	return f
}

func (f *fakePluginAPI) push(sha string, files map[string]string) {
	f.sha = sha
	f.files = files
}

func (f *fakePluginAPI) serve(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 4)
	if len(parts) != 4 || parts[0] != "repos" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// A ref can contain slashes, so only the first segment identifies the kind.
	kind, _, _ := strings.Cut(parts[3], "/")
	switch kind {
	case "commits":
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sha":      f.sha,
			"html_url": "https://github.com/example/crush-plugins/commit/" + f.sha,
		})
	case "tarball":
		if f.files == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := pluginArchive("crush-plugins-"+f.sha, f.files)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// pluginArchive builds a repository archive: every file under one top-level
// directory, as GitHub's archives are.
func pluginArchive(top string, files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		hdr := &tar.Header{Name: top + "/" + name, Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0o644}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pluginEnv isolates both plugins directories and returns the working
// directory's project root.
func pluginEnv(t *testing.T) (globalRoot, projectRoot string) {
	t.Helper()
	home := mustSymlinkFree(t, t.TempDir())
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(home, "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(home, "data"))
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	cwd := mustSymlinkFree(t, t.TempDir())
	t.Chdir(cwd)
	return plugins.GlobalRoot(filepath.Join(home, "crush", "crush.json")), plugins.ProjectRoot(cwd)
}

func mustSymlinkFree(t *testing.T, dir string) string {
	t.Helper()
	got, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return got
}

// runPlugin calls a subcommand the way cobra would, capturing its output.
// Flags are set directly because the scope vars are shared package-level state.
func runPlugin(t *testing.T, c *cobra.Command, args ...string) (string, error) {
	t.Helper()
	t.Cleanup(resetPluginFlags)
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetContext(t.Context())
	err := c.RunE(c, args)
	return buf.String(), err
}

func resetPluginFlags() {
	pluginInstallProject = false
	pluginGlobalScope = false
	pluginProjectScope = false
	pluginForce = false
	pluginListJSON = false
}

func TestPluginInstallWritesToTheGlobalDirectory(t *testing.T) {
	globalRoot, projectRoot := pluginEnv(t)
	api := newFakePluginAPI(t)
	api.push(pluginSHAOne, map[string]string{"one.sh": pluginOne, "two.sh": pluginTwo, "README.md": "not a plugin"})

	out, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins")
	require.NoError(t, err)
	require.Contains(t, out, "Installed example/crush-plugins at 11111111")
	require.Contains(t, out, "+ one.sh")
	require.Contains(t, out, "+ two.sh")
	require.Contains(t, out, "Plugins are trusted code")
	require.Contains(t, out, "https://github.com/example/crush-plugins/commit/"+pluginSHAOne)

	dir := filepath.Join(globalRoot, "example__crush-plugins")
	require.FileExists(t, filepath.Join(dir, "one.sh"))
	require.NoFileExists(t, filepath.Join(dir, "README.md"))
	require.NoDirExists(t, projectRoot)

	m, err := plugins.LoadManifest(dir)
	require.NoError(t, err)
	require.Equal(t, pluginSHAOne, m.Commit)
	require.Equal(t, pluginTestRef, m.Ref)
}

func TestPluginInstallProjectScope(t *testing.T) {
	globalRoot, projectRoot := pluginEnv(t)
	newFakePluginAPI(t)

	pluginInstallProject = true
	out, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins@v1.2.3")
	require.NoError(t, err)
	require.Contains(t, out, "Installed example/crush-plugins")

	require.FileExists(t, filepath.Join(projectRoot, "example__crush-plugins", "one.sh"))
	require.NoDirExists(t, globalRoot)
}

func TestPluginInstallRejectsABareName(t *testing.T) {
	pluginEnv(t)
	_, err := runPlugin(t, pluginInstallCmd, "crush-plugins")
	require.ErrorContains(t, err, "expected <author>/<repo>[@ref]")
}

func TestPluginInstallReportsAPrivateRepository(t *testing.T) {
	globalRoot, _ := pluginEnv(t)
	api := newFakePluginAPI(t)
	api.files = nil // the archive for the recorded commit no longer resolves

	out, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins")
	require.ErrorContains(t, err, "not found")
	require.NotContains(t, out, "Installed")
	require.NoDirExists(t, filepath.Join(globalRoot, "example__crush-plugins"))
}

func TestPluginUpdateAllFollowsAnUpstreamPush(t *testing.T) {
	globalRoot, _ := pluginEnv(t)
	api := newFakePluginAPI(t)

	_, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins")
	require.NoError(t, err)

	api.push(pluginSHATwo, map[string]string{"two.sh": pluginTwo})
	out, err := runPlugin(t, pluginUpdateCmd)
	require.NoError(t, err)
	require.Contains(t, out, "example/crush-plugins (global): 11111111 → 22222222")
	require.Contains(t, out, "- one.sh")
	require.Contains(t, out, "+ two.sh")

	dir := filepath.Join(globalRoot, "example__crush-plugins")
	require.NoFileExists(t, filepath.Join(dir, "one.sh"))
	require.FileExists(t, filepath.Join(dir, "two.sh"))
}

func TestPluginUpdateLeavesLocalEditsAlone(t *testing.T) {
	globalRoot, _ := pluginEnv(t)
	api := newFakePluginAPI(t)
	_, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins")
	require.NoError(t, err)

	api.push(pluginSHATwo, map[string]string{"one.sh": pluginTwo})
	dir := filepath.Join(globalRoot, "example__crush-plugins")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.sh"), []byte("# mine"), 0o755))

	_, err = runPlugin(t, pluginUpdateCmd)
	require.ErrorContains(t, err, "could not be updated")

	out, err := runPlugin(t, pluginUpdateCmd, "example/crush-plugins")
	require.ErrorContains(t, err, "could not be updated")
	require.Contains(t, out, "has local changes")
	require.Contains(t, out, "crush plugin trust example/crush-plugins")
	require.FileExists(t, filepath.Join(dir, "one.sh"))

	body, err := os.ReadFile(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	require.Equal(t, "# mine", string(body), "a refused update must not rewrite the file")

	pluginForce = true
	out, err = runPlugin(t, pluginUpdateCmd, "example/crush-plugins")
	require.NoError(t, err)
	require.Contains(t, out, "11111111 → 22222222")
	body, err = os.ReadFile(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	require.Equal(t, pluginTwo, string(body))
}

func TestPluginUpdateReportsNothingInstalled(t *testing.T) {
	pluginEnv(t)
	out, err := runPlugin(t, pluginUpdateCmd)
	require.NoError(t, err)
	require.Contains(t, out, "No plugin repositories are installed.")
}

func TestPluginUpdateRejectsAnUninstalledRepo(t *testing.T) {
	pluginEnv(t)
	newFakePluginAPI(t)
	_, err := runPlugin(t, pluginUpdateCmd, "example/crush-plugins")
	require.ErrorContains(t, err, "is not installed")
}

func TestPluginListAndTrustAndUninstall(t *testing.T) {
	globalRoot, _ := pluginEnv(t)
	newFakePluginAPI(t)
	_, err := runPlugin(t, pluginInstallCmd, "example/crush-plugins")
	require.NoError(t, err)

	dir := filepath.Join(globalRoot, "example__crush-plugins")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.sh"), []byte("# mine"), 0o755))

	out, err := runPlugin(t, pluginListCmd)
	require.NoError(t, err)
	require.Contains(t, out, "SOURCE")
	require.Contains(t, out, "example/crush-plugins")
	require.Contains(t, out, "1 (modified)")
	require.Contains(t, out, "do not match the files recorded for them")

	pluginListJSON = true
	out, err = runPlugin(t, pluginListCmd)
	require.NoError(t, err)
	var rows []struct {
		Scope    string `json:"scope"`
		Modified bool   `json:"modified"`
		Manifest struct {
			Commit string `json:"commit"`
			Files  []struct {
				Path string `json:"path"`
			} `json:"files"`
		} `json:"manifest"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &rows))
	require.Len(t, rows, 1)
	require.Equal(t, "global", rows[0].Scope)
	require.True(t, rows[0].Modified)
	require.Equal(t, pluginSHAOne, rows[0].Manifest.Commit)

	_, err = runPlugin(t, pluginTrustCmd, "example/crush-plugins")
	require.NoError(t, err)
	out, err = runPlugin(t, pluginListCmd)
	require.NoError(t, err)
	require.NotContains(t, out, "(modified)")

	_, err = runPlugin(t, pluginUninstallCmd, "example/crush-plugins")
	require.NoError(t, err)
	require.NoDirExists(t, dir)

	_, err = runPlugin(t, pluginUninstallCmd, "example/crush-plugins")
	require.ErrorContains(t, err, "is not installed")
}

func TestPluginScopeFlagsConflict(t *testing.T) {
	pluginEnv(t)
	pluginGlobalScope = true
	pluginProjectScope = true
	_, err := runPlugin(t, pluginListCmd)
	require.ErrorContains(t, err, "choose only one of --global and --project")
}

func TestPluginCommandsAreRegistered(t *testing.T) {
	names := make([]string, 0, 5)
	for _, c := range pluginCmd.Commands() {
		names = append(names, c.Name())
	}
	require.ElementsMatch(t, []string{"install", "update", "list", "uninstall", "trust"}, names)
	require.Contains(t, rootCmd.Commands(), pluginCmd, "crush plugin must reach the user")
}
