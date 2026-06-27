package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeadlockOnInvalidModel(t *testing.T) {
	tempDir := t.TempDir()

	// Write a mock config with a configured provider but a non-existent model.
	configJSON := `{
  "providers": {
    "openai": { "api_key": "sk-xxxx" }
  },
  "models": {
    "large": {
      "provider": "openai",
      "model": "this-model-does-not-exist"
    }
  }
}`
	configPath := filepath.Join(tempDir, "crush.json")
	err := os.WriteFile(configPath, []byte(configJSON), 0o600)
	require.NoError(t, err)

	// Set env vars so Load picks up this config
	t.Setenv("CRUSH_GLOBAL_CONFIG", tempDir)
	t.Setenv("CRUSH_GLOBAL_DATA", tempDir)

	// We run this in a channel to detect timeout/deadlock.
	done := make(chan struct{})

	go func() {
		defer close(done)
		store, loadErr := Load(tempDir, tempDir, false)
		if loadErr != nil {
			t.Logf("Load completed with error: %v", loadErr)
			return
		}
		t.Logf("Loaded config models: %+v", store.config.Models)
	}()

	select {
	case <-done:
		// Read files in tempDir
		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		t.Logf("crush.json content after Load: %s", string(data))
		
		dataPath := filepath.Join(tempDir, "crush", "crush.json")
		if data2, err := os.ReadFile(dataPath); err == nil {
			t.Logf("crush data path content: %s", string(data2))
		} else {
			t.Logf("No data file: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Load hung for more than 3 seconds!")
	}
}
