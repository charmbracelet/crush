//go:build !windows

package home

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// On Unix a backslash is an ordinary filename character, so it must not be
// treated as a path boundary. `~\foo` is a file literally named `~\foo`, and
// Dir()+`\foo` is a sibling of the home directory, not a child of it.
func TestBackslashIsNotAPathBoundaryOnUnix(t *testing.T) {
	home := Dir()
	if home == "" {
		t.Skip("no home directory")
	}
	require.Equal(t, `~\foo`, Long(`~\foo`))
	require.Equal(t, home+`\foo`, Short(home+`\foo`))
}
