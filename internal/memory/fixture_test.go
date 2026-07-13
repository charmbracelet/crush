package memory

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateMemoryFixture = flag.Bool("update-memory-fixture", false, "regenerate the tracked synthetic memory database")

func TestTrackedMemoryFixture(t *testing.T) {
	fixturePath := filepath.Join("testdata", "memory.db")
	if *updateMemoryFixture {
		generateMemoryFixture(t, fixturePath)
	}
	require.FileExists(t, fixturePath, "run go test ./internal/memory -run TestTrackedMemoryFixture -update-memory-fixture")

	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	lower := strings.ToLower(string(data))
	for _, private := range []string{
		"re_lax",
		"157.173.127.84",
		"tailscale",
		"lm studio",
		"c:\\users",
		"/root/",
	} {
		require.NotContains(t, lower, private, "fixture contains private marker %q", private)
	}

	dir := t.TempDir()
	require.NoError(t, copyAtomic(fixturePath, filepath.Join(dir, "memory.db"), 0o600))
	store, err := Open(t.Context(), Options{Directory: dir})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	records, err := store.List(t.Context(), "fixture-project", StatusActive, 10)
	require.NoError(t, err)
	require.Len(t, records, 2)
	stats, err := store.Stats(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Pending)
}

func generateMemoryFixture(t *testing.T, destination string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, Options{Directory: dir})
	require.NoError(t, err)
	project := Project{ID: "fixture-project", Name: "sample", Root: "/synthetic/sample"}
	_, err = store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindUser,
		Name:        "Explanation depth",
		Description: "The sample user prefers short explanations followed by examples",
		Content:     "Give a concise explanation first, then one concrete example.",
		Confidence:  1,
		Explicit:    true,
		SourceKind:  "fixture",
	})
	require.NoError(t, err)
	_, err = store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeProject,
		Kind:        KindFeedback,
		Name:        "Verification preference",
		Description: "Run focused tests before the full suite in this sample project",
		Content:     "Run focused tests before the full suite when changing shared behavior.",
		Confidence:  1,
		Explicit:    true,
		SourceKind:  "fixture",
	})
	require.NoError(t, err)
	_, err = store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeProject,
		Kind:        KindProject,
		Name:        "Pending release note",
		Description: "The sample release awaits a verification decision",
		Content:     "The sample release awaits a verification decision.",
		Confidence:  0.70,
		SourceKind:  "fixture",
	})
	require.NoError(t, err)
	_, err = store.db.ExecContext(ctx, "UPDATE memory_records SET file_path = ''")
	require.NoError(t, err)
	backup, err := store.Backup(ctx)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	require.NoError(t, os.MkdirAll(filepath.Dir(destination), 0o755))
	require.NoError(t, os.RemoveAll(destination))
	require.NoError(t, copyAtomic(backup, destination, 0o644))
}
