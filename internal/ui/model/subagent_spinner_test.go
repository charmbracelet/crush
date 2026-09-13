package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// The header spins while sub-agents work, and they work out of sight: that
// is the reason the header mentions them at all. The shared clock otherwise
// only arms for an item currently on screen, so with an empty or scrolled
// transcript nothing would drive the spinner and it would sit still until an
// unrelated message forced a repaint.
func TestChatAnimatesForSomethingOutsideTheList(t *testing.T) {
	t.Parallel()

	newChat := func() *Chat {
		return NewChat(common.DefaultCommon(&prismWorkspace{}), config.ScrollbarDefault)
	}

	t.Run("nothing visible and nothing external stays stopped", func(t *testing.T) {
		t.Parallel()
		c := newChat()
		require.Nil(t, c.EnsureAnimating(), "no reason to run the clock")
		require.False(t, c.animRunning)
	})

	t.Run("nothing visible but something external starts the clock", func(t *testing.T) {
		t.Parallel()
		c := newChat()
		c.SetExternalAnimation(true)
		require.NotNil(t, c.EnsureAnimating(),
			"the header spinner has to keep the clock running on its own")
		require.True(t, c.animRunning)
	})

	t.Run("the external reason going away stops the clock", func(t *testing.T) {
		t.Parallel()
		c := newChat()
		c.SetExternalAnimation(true)
		require.NotNil(t, c.EnsureAnimating())

		// A clock already running is left alone until it lapses, so the stop
		// is observed through the gate rather than the outstanding tick.
		c.SetExternalAnimation(false)
		c.animRunning = false
		require.Nil(t, c.EnsureAnimating(), "nothing is animating any more")
		require.False(t, c.animRunning)
	})
}
