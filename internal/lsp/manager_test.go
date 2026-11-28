package lsp

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestUnavailableBackoff(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	now := base

	manager := &Manager{
		unavailable: csync.NewMap[string, time.Time](),
		now:         func() time.Time { return now },
	}

	require.False(t, manager.recentlyUnavailable("gopls"))

	manager.markUnavailable("gopls")
	require.True(t, manager.recentlyUnavailable("gopls"))

	now = now.Add(unavailableRetryDelay + time.Second)
	require.False(t, manager.recentlyUnavailable("gopls"))
	_, exists := manager.unavailable.Get("gopls")
	require.False(t, exists)

	manager.markUnavailable("gopls")
	manager.clearUnavailable("gopls")
	require.False(t, manager.recentlyUnavailable("gopls"))
}

func TestCanAutoStartFiltersBeforeLookingUpCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		server  *powernapconfig.ServerConfig
		want    bool
		lookups int
	}{
		{
			name: "unhandled file type",
			server: &powernapconfig.ServerConfig{
				Command:   "typescript-language-server",
				FileTypes: []string{"typescript"},
			},
		},
		{
			name: "generic command",
			server: &powernapconfig.ServerConfig{
				Command:   "node",
				FileTypes: []string{"go"},
			},
		},
		{
			name: "handled file type",
			server: &powernapconfig.ServerConfig{
				Command:   "gopls",
				FileTypes: []string{"go"},
			},
			want:    true,
			lookups: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookups := 0
			manager := &Manager{
				unavailable: csync.NewMap[string, time.Time](),
				now:         time.Now,
				lookPath: func(string) (string, error) {
					lookups++
					return "/usr/local/bin/gopls", nil
				},
			}

			got := manager.canAutoStart("test", "main.go", t.TempDir(), tt.server)

			require.Equal(t, tt.want, got)
			require.Equal(t, tt.lookups, lookups)
		})
	}
}

func TestCanAutoStartCachesMissingCommand(t *testing.T) {
	t.Parallel()

	lookups := 0
	manager := &Manager{
		unavailable: csync.NewMap[string, time.Time](),
		now:         time.Now,
		lookPath: func(string) (string, error) {
			lookups++
			return "", errors.New("not found")
		},
	}
	server := &powernapconfig.ServerConfig{
		Command:   "gopls",
		FileTypes: []string{"go"},
	}

	require.False(t, manager.canAutoStart("gopls", "main.go", t.TempDir(), server))
	require.False(t, manager.canAutoStart("gopls", "main.go", t.TempDir(), server))
	require.Equal(t, 1, lookups)
}

func newTestManager(t *testing.T, lsps config.LSPs) *Manager {
	t.Helper()
	store := config.NewTestStore(&config.Config{
		LSP:     lsps,
		Options: &config.Options{},
	})
	return NewManager(store)
}

func TestStartServerBacksOffAfterStartFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
	}{
		{name: "missing command", command: "crush-test-missing-lsp-command"},
		{name: "initialize failure", command: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.command == "false" {
				if _, err := exec.LookPath("false"); err != nil {
					t.Skip("false command not available")
				}
			}

			lspCfg := config.LSPConfig{
				Command:   tt.command,
				FileTypes: []string{"go"},
				Timeout:   5,
			}
			manager := newTestManager(t, config.LSPs{"broken": lspCfg})
			base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
			now := base
			manager.now = func() time.Time { return now }

			server, ok := manager.manager.GetServer("broken")
			require.True(t, ok)

			manager.startServer("broken", "main.go", server)

			_, exists := manager.clients.Get("broken")
			require.False(t, exists)
			require.True(t, manager.recentlyUnavailable("broken"))

			retryAt, _ := manager.unavailable.Get("broken")
			require.Equal(t, base.Add(startFailureRetryDelay), retryAt)

			now = base.Add(startFailureRetryDelay + time.Second)
			require.False(t, manager.recentlyUnavailable("broken"))
		})
	}
}

func TestStartServerShutsDownErroredClient(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false command not available")
	}

	lspCfg := config.LSPConfig{
		Command:   "false",
		FileTypes: []string{"go"},
		Timeout:   5,
	}
	manager := newTestManager(t, config.LSPs{"broken": lspCfg})

	errored, err := New("broken", lspCfg, manager.cfg.Resolver(), "", false)
	require.NoError(t, err)
	errored.SetServerState(StateError)
	manager.clients.Set("broken", errored)

	server, ok := manager.manager.GetServer("broken")
	require.True(t, ok)
	manager.startServer("broken", "main.go", server)

	require.ErrorIs(t, errored.ctx.Err(), context.Canceled)
}

func TestHandlesFiletype(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileTypes []string
		path      string
		want      bool
	}{
		{name: "no file types", path: "/x/anything.txt", want: true},
		{name: "extension", fileTypes: []string{"go"}, path: "/x/main.go", want: true},
		{name: "dotted extension", fileTypes: []string{".go"}, path: "/x/main.go", want: true},
		{name: "upper case file type", fileTypes: []string{"GO"}, path: "/x/main.go", want: true},
		{name: "upper case file name", fileTypes: []string{"go"}, path: "/x/MAIN.GO", want: true},
		{name: "multi part extension", fileTypes: []string{"d.ts"}, path: "/x/a.d.ts", want: true},
		{name: "dot file", fileTypes: []string{"go"}, path: "/x/.go", want: true},
		{name: "bare name", fileTypes: []string{"go"}, path: "/x/go", want: false},
		{name: "partial extension", fileTypes: []string{"o"}, path: "/x/main.go", want: false},
		{name: "other extension", fileTypes: []string{"rs"}, path: "/x/main.go", want: false},
		{name: "language", fileTypes: []string{"dockerfile"}, path: "/x/Dockerfile", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, handlesFiletype("test", tt.fileTypes, tt.path))
		})
	}
}
