package model

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestTruncateRows(t *testing.T) {
	t.Parallel()

	rows := []string{"a", "b", "c", "d"}
	hint := lipgloss.NewStyle()

	require.Nil(t, truncateRows(rows, 0, 20, hint))
	require.Equal(t, rows, truncateRows(rows, 4, 20, hint))
	require.Equal(t, rows, truncateRows(rows, 9, 20, hint))
	require.Equal(t, []string{"a", "b", "…and 2 more"}, truncateRows(rows, 3, 20, hint))
	require.Equal(t, []string{"…and 4 more"}, truncateRows(rows, 1, 20, hint))
	require.Equal(t, []string{"a", "b", "+2"}, truncateRows(rows, 3, 6, hint))
	require.Equal(t, []string{"a", "b", "+"}, truncateRows(rows, 3, 1, hint))
	require.Equal(t, []string{"a", "b", "c", "d"}, rows, "input must not be mutated")
}
