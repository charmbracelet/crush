package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// The header spins while subagents work, and they work out of sight: that
// is the reason the header mentions them at all. With an empty or scrolled
// transcript nothing else would drive the spinner.
func TestAnimClockRunsForSubagentsOutsideTheList(t *testing.T) {
	t.Parallel()

	newUI := func() *UI {
		return &UI{chat: NewChat(common.DefaultCommon(&prismWorkspace{}), config.ScrollbarDefault)}
	}

	t.Run("nothing visible and no subagents stays stopped", func(t *testing.T) {
		t.Parallel()
		u := newUI()
		require.Nil(t, u.ensureAnimating(), "no reason to run the clock")
		require.False(t, u.animClock.running)
	})

	t.Run("a running subagent starts the clock", func(t *testing.T) {
		t.Parallel()
		u := newUI()
		u.subAgentsRunning = 1
		require.NotNil(t, u.ensureAnimating(),
			"the header spinner has to keep the clock running on its own")
		require.True(t, u.animClock.running)
	})

	t.Run("the last subagent finishing stops the clock", func(t *testing.T) {
		t.Parallel()
		u := newUI()
		u.subAgentsRunning = 1
		require.NotNil(t, u.ensureAnimating())

		u.subAgentsRunning = 0
		require.Nil(t, u.ensureAnimating(), "nothing is animating any more")
		require.False(t, u.animClock.running)
	})
}
