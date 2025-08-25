package home

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	require.NotEmpty(t, Dir(), "home directory should not be empty")
}

func TestShort(t *testing.T) {
	d := filepath.Join(Dir(), "documents", "file.txt")
	require.Equal(t, "~/documents/file.txt", Short(d), "should replace home directory with ~")
	ad := "/absolute/path/file.txt"
	require.Equal(t, ad, Short(ad))
}

func TestLong(t *testing.T) {
	d := "~/documents/file.txt"
	require.Equal(t, filepath.Join(Dir(), "documents", "file.txt"), Long(d), "should replace ~ with home directory")
	ad := "/absolute/path/file.txt"
	require.Equal(t, ad, Long(ad))
}
