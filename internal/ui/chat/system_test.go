package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSystemMessageItemRender(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewSystemMessageItem(&sty, SystemMessageContextWarning, "Model Context Warning", "This model has a tiny context window.")

	require.Equal(t, "system:context-warning", item.ID())
	require.Equal(t, SystemMessageContextWarning, item.Kind())
	require.True(t, item.Finished())

	out := ansi.Strip(item.Render(80))
	require.Contains(t, out, "!")
	require.Contains(t, out, "Model Context Warning")
	require.Contains(t, out, "tiny context window")
	require.Contains(t, out, systemMessageFooter)
}

func TestSystemMessageItemStableIDPerKind(t *testing.T) {
	t.Parallel()

	require.Equal(t, "system:context-warning", SystemMessageID(SystemMessageContextWarning))
	require.Equal(t, "system:super-yolo", SystemMessageID(SystemMessageSuperYolo))
}
