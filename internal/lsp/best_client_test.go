package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func newScoreTestClient(name, cwd string, fileTypes []string) *Client {
	c := newTestClient()
	c.name = name
	c.cwd = cwd
	c.fileTypes = fileTypes
	return c
}

func newScoreTestManager(clients ...*Client) *Manager {
	m := &Manager{clients: csync.NewMap[string, *Client]()}
	for _, c := range clients {
		m.clients.Set(c.name, c)
	}
	return m
}

func TestMatchScore(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pyFile := filepath.Join(tmp, "main.py")
	require.NoError(t, os.WriteFile(pyFile, []byte("x = 1\n"), 0o644))

	other := t.TempDir()
	outsidePy := filepath.Join(other, "out.py")
	require.NoError(t, os.WriteFile(outsidePy, []byte("x"), 0o644))

	cases := []struct {
		name      string
		client    *Client
		path      string
		wantScore int
	}{
		{"explicit ext match", newScoreTestClient("py", tmp, []string{"py"}), pyFile, 2},
		{"empty filetypes is catch-all", newScoreTestClient("any", tmp, nil), pyFile, 1},
		{"explicit but mismatched", newScoreTestClient("rs", tmp, []string{"rs"}), pyFile, 0},
		{"outside workspace", newScoreTestClient("py", tmp, []string{"py"}), outsidePy, 0},
		{"nil client", (*Client)(nil), pyFile, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.wantScore, tc.client.MatchScore(tc.path))
		})
	}
}

// Reproduces #1751-style routing: when multiple LSPs are configured with
// empty FileTypes (e.g. because the user wrote `extensions:` instead of
// `filetypes:` and the field was silently dropped), a catch-all client
// must not steal a file from a client that explicitly claims the
// extension. Without scoring, this passed or failed by map iteration
// order — repeating the assertion many times catches the race.
func TestBestClientForPrefersSpecificMatchOverCatchAll(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pyFile := filepath.Join(tmp, "main.py")
	require.NoError(t, os.WriteFile(pyFile, []byte("x = 1\n"), 0o644))

	catchAll := newScoreTestClient("css", tmp, nil)
	pyClient := newScoreTestClient("python", tmp, []string{"py"})
	mgr := newScoreTestManager(catchAll, pyClient)

	for range 100 {
		require.Same(t, pyClient, mgr.BestClientFor(pyFile),
			"explicit python match must beat catch-all CSS regardless of map iteration order")
	}
}

func TestBestClientForFallsBackToCatchAll(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	txtFile := filepath.Join(tmp, "notes.txt")
	require.NoError(t, os.WriteFile(txtFile, []byte("x"), 0o644))

	catchAll := newScoreTestClient("anything", tmp, nil)
	pyClient := newScoreTestClient("python", tmp, []string{"py"})
	mgr := newScoreTestManager(catchAll, pyClient)

	require.Same(t, catchAll, mgr.BestClientFor(txtFile),
		"catch-all should still serve files no explicit client claims")
}

func TestBestClientForReturnsNilWhenNothingHandlesPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	other := t.TempDir()
	pyFile := filepath.Join(other, "out.py")
	require.NoError(t, os.WriteFile(pyFile, []byte("x"), 0o644))

	pyClient := newScoreTestClient("python", tmp, []string{"py"})
	mgr := newScoreTestManager(pyClient)

	require.Nil(t, mgr.BestClientFor(pyFile))
}

func TestBestClientForDeterministicTiebreak(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pyFile := filepath.Join(tmp, "main.py")
	require.NoError(t, os.WriteFile(pyFile, []byte("x"), 0o644))

	// Two equally specific clients (both score 2). Tiebreak picks the
	// alphabetically smaller name so the choice is stable.
	zPython := newScoreTestClient("zpython", tmp, []string{"py"})
	aPython := newScoreTestClient("apython", tmp, []string{"py"})
	mgr := newScoreTestManager(zPython, aPython)

	for range 100 {
		require.Same(t, aPython, mgr.BestClientFor(pyFile))
	}
}
