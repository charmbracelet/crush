package anim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTakeDrawnReportsRenders(t *testing.T) {
	t.Parallel()

	a := New(Settings{Size: 5, Label: "Working"})
	require.False(t, a.TakeDrawn(), "nothing rendered yet")

	a.Advance()
	require.False(t, a.TakeDrawn(), "advancing is not drawing")

	a.Render()
	require.True(t, a.TakeDrawn())
	require.False(t, a.TakeDrawn(), "taking the record resets it")
}
