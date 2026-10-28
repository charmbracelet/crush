package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// runIsolated keeps tests off the developer's own global config and data.
func runIsolated(m *testing.M) int {
	home, err := os.MkdirTemp("", "crush-agent-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)
	os.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(home, "config"))
	os.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(home, "data"))
	return m.Run()
}
