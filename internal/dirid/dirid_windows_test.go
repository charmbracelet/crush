//go:build windows

package dirid

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOf_Junction checks that a directory junction, which EvalSymlinks
// does not resolve, identifies the same directory as its target.
func TestOf_Junction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "repo")
	other := filepath.Join(root, "other")
	link := filepath.Join(root, "link")
	require.NoError(t, os.Mkdir(target, 0o755))
	require.NoError(t, os.Mkdir(other, 0o755))
	cmd := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", link, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Cannot create junction: %v: %s", err, out)
	}

	idt, ok := Of(target)
	require.True(t, ok)
	idl, ok := Of(link)
	require.True(t, ok)
	ido, ok := Of(other)
	require.True(t, ok)

	require.True(t, idt.Same(idl))
	require.False(t, idt.Same(ido))
}
