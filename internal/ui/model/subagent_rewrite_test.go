package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestBuildSubagentCaches(t *testing.T) {
	t.Parallel()

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, buildSubagentCaches(nil))
	})

	t.Run("populates_items", func(t *testing.T) {
		t.Parallel()
		got := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "code-reviewer", Description: "reviews code"},
			{Name: "tester", Description: "writes tests"},
		})

		require.Len(t, got, 2)
		require.Equal(t, "code-reviewer", got[0].Name)
		require.Equal(t, "reviews code", got[0].Description)
		require.Equal(t, "tester", got[1].Name)
	})

	t.Run("preserves_input_order", func(t *testing.T) {
		t.Parallel()
		got := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "zeta"},
			{Name: "alpha"},
			{Name: "mu"},
		})
		require.Equal(t, "zeta", got[0].Name)
		require.Equal(t, "alpha", got[1].Name)
		require.Equal(t, "mu", got[2].Name)
	})
}
