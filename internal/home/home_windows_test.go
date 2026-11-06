//go:build windows

package home

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Windows accepts both `\` and `/` as path separators, and users habitually
// type `~/foo`. The boundary check must accept either one, otherwise `~/foo`
// comes back unexpanded.
func TestLong_AcceptsBothSeparators(t *testing.T) {
	home := Dir()
	if home == "" {
		t.Skip("no home directory")
	}
	require.Equal(t, home+`\foo`, Long(`~\foo`))
	require.Equal(t, home+"/foo", Long("~/foo"))
}

// Same for the other direction: a path that starts with the home directory
// but continues with a forward slash is still inside the home directory.
func TestShort_AcceptsBothSeparators(t *testing.T) {
	home := Dir()
	if home == "" {
		t.Skip("no home directory")
	}
	require.Equal(t, `~\foo`, Short(home+`\foo`))
	require.Equal(t, `~\foo`, Short(home+"/foo"))
}
