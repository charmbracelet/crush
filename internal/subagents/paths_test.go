package subagents

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInGlobalDir verifies global-dir membership: files inside a global
// subagents directory pass, anything else (project/custom paths, empty)
// fails. Not parallel: pins the global dir via CRUSH_SUBAGENTS_DIR.
func TestInGlobalDir(t *testing.T) {
	globalDir := t.TempDir()
	otherDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", globalDir)

	require.True(t, InGlobalDir(filepath.Join(globalDir, "agent.md")))
	require.True(t, InGlobalDir(filepath.Join(globalDir, "nested", "agent.md")))
	require.False(t, InGlobalDir(filepath.Join(otherDir, "agent.md")))
	require.False(t, InGlobalDir(globalDir+"-sibling/agent.md"), "sibling dir sharing a name prefix must not match")
	require.False(t, InGlobalDir(""))
}
