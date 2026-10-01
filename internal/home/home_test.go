package home

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	require.NotEmpty(t, Dir())
}

func TestShort(t *testing.T) {
	d := filepath.Join(Dir(), "documents", "file.txt")
	require.Equal(t, filepath.FromSlash("~/documents/file.txt"), Short(d))
	ad := filepath.FromSlash("/absolute/path/file.txt")
	require.Equal(t, ad, Short(ad))
}

func TestLong(t *testing.T) {
	d := filepath.FromSlash("~/documents/file.txt")
	require.Equal(t, filepath.Join(Dir(), "documents", "file.txt"), Long(d))
	ad := filepath.FromSlash("/absolute/path/file.txt")
	require.Equal(t, ad, Long(ad))
}

func TestShort_DoesNotMatchSiblingPrefix(t *testing.T) {
	realHome := Dir()
	if realHome == "" {
		t.Skip("no home directory")
	}
	require.Equal(t, "~", Short(realHome))
	child := filepath.Join(realHome, "src", "main.go")
	require.Equal(t, filepath.FromSlash("~/src/main.go"), Short(child))
	sibling := realHome + "2" + string(filepath.Separator) + "foo"
	require.Equal(t, sibling, Short(sibling),
		"Short must leave paths that only share a string prefix with home alone")
	extra := realHome + "extra"
	require.Equal(t, extra, Short(extra))
}

func TestLong_OnlyExpandsBareTildeOrTildeSlash(t *testing.T) {
	require.Equal(t, Dir(), Long("~"))
	require.Equal(t, filepath.Join(Dir(), "documents"), Long(filepath.FromSlash("~/documents")))
	otherUser := "~alice/src"
	require.Equal(t, otherUser, Long(otherUser),
		"Long must not rewrite ~user paths into the current user's home")
	require.Equal(t, "~~/x", Long("~~/x"))
}
