package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestScopeB_InPlaceMutationRace probes whether the store's in-place field
// mutators (e.g. SetCompactMode) race with concurrent Config() readers that
// walk the same field. This is the "Scope B" race left open after the
// pointer-swap fix. Run with -race.
// TestScopeB_InPlaceMutationRace verifies that the store's typed field
// mutators (e.g. SetCompactMode) no longer race concurrent Config()
// readers walking the same field. Copy-on-write publishing means the
// mutator swaps in a fresh Config rather than writing through the live
// pointer, so a reader always sees an immutable snapshot. Run with -race.
func TestScopeB_InPlaceMutationRace(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	cfg := `{
		"models": {"large": {"provider": "openai", "model": "gpt-4"}},
		"providers": {"openai": {"api_key": "k", "models": [{"id": "gpt-4", "name": "GPT-4"}]}},
		"options": {"tui": {"compact_mode": false}}
	}`
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Load(dir, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	store.globalDataPath = configPath

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader: walks Options.TUI.CompactMode, the field SetCompactMode writes.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c := store.Config()
				if c != nil && c.Options != nil {
					_ = c.Options.TUI.CompactMode
				}
			}
		}
	}()

	// Writer: flips compact mode, mutating Options in place.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = store.SetCompactMode(ScopeGlobal, i%2 == 0)
		}
		close(stop)
	}()

	wg.Wait()
}
