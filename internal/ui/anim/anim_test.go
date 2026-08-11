package anim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStartSupersedesPreviousChain verifies that calling Start() twice
// does not result in two concurrent tick chains advancing the same Anim.
// The second Start() bumps the generation, so ticks from the first chain
// (carrying the old generation) are dropped by Animate().
func TestStartSupersedesPreviousChain(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})

	// First chain.
	cmd1 := a.Start()
	gen1 := a.gen.Load()
	require.Equal(t, int64(1), gen1)

	// Second chain supersedes the first.
	cmd2 := a.Start()
	gen2 := a.gen.Load()
	require.Equal(t, int64(2), gen2)

	// Execute both commands to get their StepMsgs.
	msg1 := cmd1().(StepMsg)
	msg2 := cmd2().(StepMsg)

	require.Equal(t, gen1, msg1.Gen, "first chain carries old generation")
	require.Equal(t, gen2, msg2.Gen, "second chain carries new generation")

	// The old-generation tick must be dropped.
	framesBefore := a.framesSinceStart.Load()
	next := a.Animate(msg1)
	require.Nil(t, next, "old-generation tick must not schedule another step")
	require.Equal(t, framesBefore, a.framesSinceStart.Load(),
		"old-generation tick must not advance the frame")

	// The new-generation tick must advance.
	next = a.Animate(msg2)
	require.NotNil(t, next, "current-generation tick must schedule another step")
	require.Equal(t, framesBefore+1, a.framesSinceStart.Load(),
		"current-generation tick must advance the frame")
}

// TestStopKillsChain verifies that Stop() bumps the generation so any
// in-flight tick chain is terminated.
func TestStopKillsChain(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	cmd := a.Start()
	msg := cmd().(StepMsg)

	a.Stop()
	require.NotEqual(t, msg.Gen, a.gen.Load(), "Stop must bump the generation")

	// The in-flight tick must be dropped.
	framesBefore := a.framesSinceStart.Load()
	next := a.Animate(msg)
	require.Nil(t, next, "tick after Stop must not schedule another step")
	require.Equal(t, framesBefore, a.framesSinceStart.Load(),
		"tick after Stop must not advance the frame")
}

// TestForeignIDStillDropped verifies that the existing ID gate still works
// alongside the generation gate.
func TestForeignIDStillDropped(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	cmd := a.Start()
	msg := cmd().(StepMsg)

	framesBefore := a.framesSinceStart.Load()
	next := a.Animate(StepMsg{ID: "other", Gen: msg.Gen})
	require.Nil(t, next)
	require.Equal(t, framesBefore, a.framesSinceStart.Load(),
		"foreign ID must not advance the frame")
}

// TestStepMsgCarriesGeneration verifies that Step() stamps the current
// generation into the emitted StepMsg.
func TestStepMsgCarriesGeneration(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	require.Equal(t, int64(0), a.gen.Load(), "fresh Anim starts at generation 0")

	cmd := a.Step()
	msg := cmd().(StepMsg)
	require.Equal(t, int64(0), msg.Gen, "Step before Start carries gen 0")

	a.Start()
	cmd = a.Step()
	msg = cmd().(StepMsg)
	require.Equal(t, int64(1), msg.Gen, "Step after Start carries gen 1")
}

// TestAnimateSchedulesNextStep verifies the normal happy path: a matching
// tick advances the frame and schedules the next step.
func TestAnimateSchedulesNextStep(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	cmd := a.Start()
	msg := cmd().(StepMsg)

	next := a.Animate(msg)
	require.NotNil(t, next, "matching tick must schedule the next step")

	// The next command should also produce a StepMsg with the same gen.
	nextMsg := next().(StepMsg)
	require.Equal(t, msg.Gen, nextMsg.Gen, "chained tick must carry the same generation")
	require.Equal(t, msg.ID, nextMsg.ID)
}

// TestSingleChainAdvances verifies that a single Start() produces a
// working chain that advances frames on each tick.
func TestSingleChainAdvances(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	cmd := a.Start()
	require.NotNil(t, cmd)

	msg := cmd().(StepMsg)
	require.Equal(t, "test", msg.ID)
	require.Equal(t, int64(1), msg.Gen)

	// Advance a few frames.
	for range 5 {
		next := a.Animate(msg)
		require.NotNil(t, next)
		msg = next().(StepMsg)
	}
	require.Equal(t, int64(5), a.framesSinceStart.Load())
}

// TestConcurrentStartDoesNotDoubleAdvance simulates the bug scenario:
// two Start() calls produce two chains, and both chains' ticks arrive.
// Only the second chain's ticks should advance the frame.
func TestConcurrentStartDoesNotDoubleAdvance(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})

	cmd1 := a.Start()
	cmd2 := a.Start()

	msg1 := cmd1().(StepMsg)
	msg2 := cmd2().(StepMsg)

	// Interleave ticks from both chains.
	frames := int64(0)
	for range 10 {
		if next := a.Animate(msg1); next != nil {
			frames++
			msg1 = next().(StepMsg)
		}
		if next := a.Animate(msg2); next != nil {
			frames++
			msg2 = next().(StepMsg)
		}
	}

	// Only chain 2 should have advanced. Chain 1's ticks are all dropped
	// after the first Start() supersedes it.
	require.Equal(t, int64(10), a.framesSinceStart.Load(),
		"only the latest chain should advance the frame")
	require.Equal(t, frames, a.framesSinceStart.Load())
}

// TestMultipleAnimsIndependent verifies that two Anim instances with
// different IDs don't interfere with each other.
func TestMultipleAnimsIndependent(t *testing.T) {
	t.Parallel()

	a1 := New(Settings{ID: "a1", Size: 5})
	a2 := New(Settings{ID: "a2", Size: 5})

	cmd1 := a1.Start()
	cmd2 := a2.Start()

	msg1 := cmd1().(StepMsg)
	msg2 := cmd2().(StepMsg)

	// Advance a1 only.
	a1.Animate(msg1)
	require.Equal(t, int64(1), a1.framesSinceStart.Load())
	require.Equal(t, int64(0), a2.framesSinceStart.Load())

	// Advance a2 only.
	a2.Animate(msg2)
	require.Equal(t, int64(1), a1.framesSinceStart.Load())
	require.Equal(t, int64(1), a2.framesSinceStart.Load())

	// Cross-talk: a1's tick must not advance a2.
	a2.Animate(msg1)
	require.Equal(t, int64(1), a2.framesSinceStart.Load())
}

// TestStopThenStart verifies that Stop() followed by Start() produces a
// working chain with a fresh generation.
func TestStopThenStart(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	cmd := a.Start()
	msg := cmd().(StepMsg)

	a.Stop()
	genAfterStop := a.gen.Load()

	cmd = a.Start()
	msg2 := cmd().(StepMsg)
	require.Equal(t, genAfterStop+1, msg2.Gen)

	// Old tick must be dropped.
	require.Nil(t, a.Animate(msg))

	// New tick must work.
	require.NotNil(t, a.Animate(msg2))
}

// TestAnimateWithoutStart verifies that Animate works on a fresh Anim
// whose generation is still zero, matching a zero-gen StepMsg.
func TestAnimateWithoutStart(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})
	msg := StepMsg{ID: "test", Gen: 0}
	next := a.Animate(msg)
	require.NotNil(t, next, "matching gen-0 tick must advance a fresh Anim")
}
