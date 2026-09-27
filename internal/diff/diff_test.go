package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The edit tool reads a CRLF file as LF and writes it back as CRLF, so the two
// sides of a diff routinely disagree about line endings while the text they
// hold is otherwise identical. Compared as they are, every line of the file
// looks changed. Line endings are a property of the file, not of the edit, and
// should not turn one changed line into a whole-file rewrite.
func TestGenerateDiffIgnoresLineEndingConvention(t *testing.T) {
	t.Parallel()

	unified, additions, removals := GenerateDiff(
		"alpha\nbeta\ngamma\ndelta\n",
		"alpha\r\nBETA\r\ngamma\r\ndelta\r\n",
		"test.txt",
	)

	require.Equal(t, 1, additions)
	require.Equal(t, 1, removals)
	require.NotContains(t, unified, "\r", "the diff should not carry the file's CRLF endings")
}

func TestGenerateDiffCountsMatchingLineEndings(t *testing.T) {
	t.Parallel()

	_, additions, removals := GenerateDiff(
		"alpha\r\nbeta\r\n",
		"alpha\r\nBETA\r\n",
		"test.txt",
	)

	require.Equal(t, 1, additions)
	require.Equal(t, 1, removals)
}
