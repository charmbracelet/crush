package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestInitAgent_JoiningClientsShareCoordinator guards against a joining
// client's InitAgent replacing the workspace's shared coordinator, which
// orphaned the first client's live run. Run with -race.
func TestInitAgent_JoiningClientsShareCoordinator(t *testing.T) {
	for _, existing := range []bool{true, false} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			xdgIsolated(t)
			t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
			cwd := t.TempDir()
			cfg := `{"providers": {"mock": {"type": "openai-compat",
  "base_url": "http://127.0.0.1:1", "api_key": "k", "models": [{"id": "m"}]}},
  "models": {"large": {"provider": "mock", "model": "m"}, "small": {"provider": "mock", "model": "m"}}}`
			require.NoError(t, os.WriteFile(filepath.Join(cwd, "crush.json"), []byte(cfg), 0o644))
			b := New(t.Context(), nil, func() {})
			t.Cleanup(func() { drainBackend(t, b) })
			ws, _, err := b.CreateWorkspace(protoWS(cwd, filepath.Join(cwd, ".crush"), uuid.NewString()))
			require.NoError(t, err)

			first := ws.Coordinator()
			if existing {
				require.NotNil(t, first)
			} else {
				// A workspace created before its provider was set up.
				ws.AgentCoordinator = nil
			}

			stop := make(chan struct{})
			var readers sync.WaitGroup
			readers.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
						_, _ = b.GetAgentInfo(ws.ID)
						_, _ = b.QueuedPrompts(ws.ID, "s")
						_ = b.CancelSession(ws.ID, "s")
					}
				}
			})
			const joins = 4
			seen := make(chan agent.Coordinator, joins)
			var joiners sync.WaitGroup
			for i := range joins {
				joiners.Go(func() {
					if err := b.InitAgent(t.Context(), ws.ID, i%2 == 0); err != nil {
						t.Error(err)
					}
					seen <- ws.Coordinator()
				})
			}
			joiners.Wait()
			close(stop)
			readers.Wait()
			close(seen)

			want := ws.Coordinator()
			require.NotNil(t, want)
			if existing {
				require.True(t, want == first, "a joining client replaced the coordinator")
			}
			for got := range seen {
				require.True(t, got == want, "every client must share one coordinator")
			}
		})
	}
}
