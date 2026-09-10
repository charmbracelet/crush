package prompt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T, workingDir string) *config.ConfigStore {
	t.Helper()
	store, err := config.Init(workingDir, filepath.Join(t.TempDir(), "data"), false)
	require.NoError(t, err)
	return store
}

func TestLoadContextFilesDeterministicOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "c.md"), []byte("c"), 0o644))

	store := newTestStore(t, dir)

	// Configured order is preserved for explicit paths; directory
	// contents come back in lexical order.
	files := loadContextFiles([]string{
		filepath.Join(dir, "sub"),
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "a.md"),
	}, store)

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	require.Equal(t, []string{
		filepath.Join(dir, "sub", "c.md"),
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "a.md"),
	}, paths)
}

func TestLoadContextFilesDeduplicatesPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(path, []byte("policy"), 0o644))

	store := newTestStore(t, dir)
	// Dedup is keyed on the expanded configured path.
	files := loadContextFiles([]string{path, path, strings.ToUpper(path)}, store)
	require.Len(t, files, 1)
	require.Equal(t, "policy", files[0].Content)
}

func TestBuildSectionsPopulated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CTX.md"), []byte("project rules"), 0o644))

	store := newTestStore(t, dir)
	store.Config().Options.ContextPaths = []string{filepath.Join(dir, "CTX.md")}
	store.Config().Options.GlobalContextPaths = nil

	tmpl := `You are a test agent.
<env>
Working directory: {{.WorkingDir}}
</env>
{{if .ContextFiles}}
<project_context>
{{range .ContextFiles}}<file path="{{.Path}}">
{{.Content}}
</file>
{{end}}</project_context>
{{end}}`

	p, err := NewPrompt("test", tmpl, WithWorkingDir(dir))
	require.NoError(t, err)

	built, err := p.Build(context.Background(), "prov", "model", store)
	require.NoError(t, err)
	require.NotEmpty(t, built.Text)

	byName := make(map[string]PromptSection, len(built.Sections))
	for _, s := range built.Sections {
		byName[s.Name] = s
		require.Positive(t, s.Bytes)
		require.Positive(t, s.EstTokens)
	}

	require.Contains(t, byName, "core_policy")
	require.Contains(t, byName, "env")
	require.Contains(t, byName, "project_context")
	require.Equal(t, CacheClassVolatile, byName["env"].CacheClass)
	require.Equal(t, CacheClassStable, byName["project_context"].CacheClass)
	require.Contains(t, byName["project_context"].Content, "project rules")
}

func TestBuildDeterministicAcrossBuilds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0o644))

	store := newTestStore(t, dir)
	store.Config().Options.ContextPaths = []string{
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "a.md"),
	}
	store.Config().Options.GlobalContextPaths = nil

	tmpl := `{{range .ContextFiles}}<file path="{{.Path}}">{{.Content}}</file>{{end}}`
	p, err := NewPrompt("test", tmpl, WithWorkingDir(dir))
	require.NoError(t, err)

	first, err := p.Build(context.Background(), "prov", "model", store)
	require.NoError(t, err)
	second, err := p.Build(context.Background(), "prov", "model", store)
	require.NoError(t, err)
	require.Equal(t, first.Text, second.Text)

	// Configured order is preserved: b.md listed before a.md.
	idxA := strings.Index(first.Text, filepath.Join(dir, "a.md"))
	idxB := strings.Index(first.Text, filepath.Join(dir, "b.md"))
	require.GreaterOrEqual(t, idxA, 0)
	require.GreaterOrEqual(t, idxB, 0)
	require.Less(t, idxB, idxA)
}
