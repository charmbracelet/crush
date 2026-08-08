package workspace

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSubagentScope_GlobalDirWinsOverAncestorWorkingDir is the regression test
// for subagentScope misclassifying a genuinely user-scope subagent (one under
// a global subagents dir) as "project" whenever workingDir happens to be an
// ancestor of that global dir. The global-dir check must take priority over
// the workingDir/projectDirs check.
func TestSubagentScope_GlobalDirWinsOverAncestorWorkingDir(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", globalDir)

	filePath := filepath.Join(globalDir, "foo.md")
	// workingDir is an ancestor of globalDir, so fsext.HasPrefix(filePath,
	// workingDir) is true and the buggy code misclassifies this as "project".
	workingDir := filepath.Dir(globalDir)

	got := subagentScope(filePath, workingDir, nil)
	require.Equal(t, "user", got,
		"a file under a global subagents dir must be scoped \"user\" even when workingDir is an ancestor of that dir")
}

// TestSubagentScope_ProjectDirStillClassifiedAsProject guards against a
// blunt fix that always returns "user": a file genuinely under a project
// directory (not under any global dir) with a matching workingDir must still
// be scoped "project".
func TestSubagentScope_ProjectDirStillClassifiedAsProject(t *testing.T) {
	t.Setenv("CRUSH_SUBAGENTS_DIR", t.TempDir())

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, ".crush", "subagents", "bar.md")

	got := subagentScope(filePath, workingDir, nil)
	require.Equal(t, "project", got)
}
