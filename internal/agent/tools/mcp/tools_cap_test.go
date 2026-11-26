package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapResultLeavesSmallResultsAlone(t *testing.T) {
	t.Parallel()

	text := "issue 1234 is assigned to nobody"
	require.Equal(t, text, capResult(text, t.TempDir()))
	require.Equal(t, "", capResult("", t.TempDir()))
}

// TestCapResultTruncatesAndPointsAtTheFullText is the whole reason the cap
// exists: an oversized server result has to stop being re-sent, without the
// data being lost to the model that asks for it later.
func TestCapResultTruncatesAndPointsAtTheFullText(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a very long line of server output\n", 5_000)
	got := capResult(text, t.TempDir())

	require.NotEqual(t, text, got)
	require.Less(t, len(got), len(text))
	require.Contains(t, got, "full output:")

	marker := "full output: "
	rest := got[strings.LastIndex(got, marker)+len(marker):]
	path := strings.Fields(strings.SplitN(rest, "\n", 2)[0])[0]
	require.True(t, filepath.IsAbs(path), "the pointer has to resolve from anywhere")

	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, text, string(saved), "the pointer must lead to everything, not more fragments")
}
