package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateDiffCountsHeaderLikeChanges(t *testing.T) {
	t.Parallel()

	_, additions, removals := GenerateDiff("--old\nkeep\n", "++new\nkeep\n", "example")
	require.Equal(t, 1, additions)
	require.Equal(t, 1, removals)
}
