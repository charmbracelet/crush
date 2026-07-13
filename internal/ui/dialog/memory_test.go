package dialog

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func newTestMemory(t *testing.T) *Memory {
	t.Helper()
	sty := styles.CharmtonePantera()
	return NewMemory(&common.Common{Styles: &sty}, workspace.MemorySnapshot{
		Available: true,
		Records: []memory.Record{{
			ID:          "memory-1",
			Scope:       memory.ScopeGlobal,
			Kind:        memory.KindUser,
			Name:        "preference",
			Description: "Preference",
			Content:     "Use the structured MCP configuration state.",
			Status:      memory.StatusActive,
			Confidence:  0.9,
			UpdatedAt:   time.Now(),
		}},
	})
}

func TestMemoryForgetRequiresVisibleConfirmation(t *testing.T) {
	t.Parallel()

	dialog := newTestMemory(t)
	require.Nil(t, dialog.HandleMsg(keyMsg('x')))
	require.True(t, dialog.confirmForget)

	help := dialog.ShortHelp()
	require.Len(t, help, 2)
	require.Equal(t, "confirm", help[0].Help().Desc)
	require.Equal(t, "cancel", help[1].Help().Desc)

	action, ok := dialog.HandleMsg(keyMsg('y')).(ActionMemorySetStatus)
	require.True(t, ok)
	require.Equal(t, "memory-1", action.ID)
	require.Equal(t, memory.StatusDeleted, action.Status)
	require.False(t, dialog.confirmForget)
}

func TestMemoryForgetCanBeCancelled(t *testing.T) {
	t.Parallel()

	dialog := newTestMemory(t)
	dialog.HandleMsg(keyMsg('x'))
	require.Nil(t, dialog.HandleMsg(keyMsg('n')))
	require.False(t, dialog.confirmForget)
}
