package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolA2UISurfaceWidth(t *testing.T) {
	t.Parallel()

	// Mirror of the real chain: listWidth = chatWidth-1 (scrollbar), two
	// cappedMessageWidth passes (-2 each, 120/118 caps), tool body indent
	// (-2), a2tea card chrome (-4).
	require.Equal(t, 89, ToolA2UISurfaceWidth(100))
	// Wide viewports saturate at maxTextWidth: min(...,120)-2-2-4 = 112.
	require.Equal(t, 112, ToolA2UISurfaceWidth(140))
	require.Equal(t, 112, ToolA2UISurfaceWidth(500))
	// Degenerate viewports must yield 0 (no hint), never negative.
	require.Equal(t, 0, ToolA2UISurfaceWidth(0))
	require.Equal(t, 0, ToolA2UISurfaceWidth(8))
}
