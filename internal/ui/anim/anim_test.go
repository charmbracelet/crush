package anim

import (
	"image/color"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFrameInterval covers the shared animation clock: every Anim in the
// UI is driven by a single tea.Tick chain at this interval.
func TestFrameInterval(t *testing.T) {
	t.Parallel()
	require.Equal(t, time.Second/time.Duration(fps), FrameInterval())
}

// TestAdvanceMovesStep verifies that each Advance call advances the frame
// step counter and wraps it so the prerendered frames loop.
func TestAdvanceMovesStep(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5})

	require.Equal(t, int64(0), a.framesSinceStart.Load())
	for range prerenderedFrames * 3 {
		require.True(t, a.Advance(), "an advance must report that output changed")
	}
	require.Equal(t, int64(prerenderedFrames*3), a.framesSinceStart.Load(),
		"every frame must be counted")
	require.Less(t, int(a.step.Load()), len(a.cyclingFrames),
		"step must wrap within the prerendered frame range")
}

// TestAdvanceInitializesBirth verifies that the birth animation completes
// after maxBirthSteps frames and that the ellipsis only animates once all
// characters have been initialized.
func TestAdvanceInitializesBirth(t *testing.T) {
	t.Parallel()

	a := New(Settings{ID: "test", Size: 5, Label: "Generating"})
	require.False(t, a.initialized.Load())

	ellipsisBefore := int(a.ellipsisStep.Load())
	for range maxBirthSteps - 1 {
		a.Advance()
	}
	require.False(t, a.initialized.Load(), "birth must not complete before maxBirthSteps frames")

	a.Advance()
	require.True(t, a.initialized.Load())

	for i := 1; i <= ellipsisAnimSpeed*2; i++ {
		a.Advance()
	}
	require.NotEqual(t, ellipsisBefore, int(a.ellipsisStep.Load()),
		"the ellipsis must animate once initialized")
}

// TestAdvanceIndependentInstances verifies that two Anim instances advance
// their own counters; the shared clock simply calls Advance on each.
func TestAdvanceIndependentInstances(t *testing.T) {
	t.Parallel()

	a1 := New(Settings{ID: "a1", Size: 5})
	a2 := New(Settings{ID: "a2", Size: 5})

	a1.Advance()
	require.Equal(t, int64(1), a1.framesSinceStart.Load())
	require.Equal(t, int64(0), a2.framesSinceStart.Load())

	a2.Advance()
	require.Equal(t, int64(1), a2.framesSinceStart.Load())
}

// TestSetColorsRebuildsFrames verifies that a theme swap on a live spinner
// rebuilds the pre-rendered frames with the new colors while preserving
// animation progress.
func TestSetColorsRebuildsFrames(t *testing.T) {
	t.Parallel()

	red := color.RGBA{R: 0xff, A: 0xff}
	blue := color.RGBA{B: 0xff, A: 0xff}
	green := color.RGBA{G: 0xff, A: 0xff}
	yellow := color.RGBA{R: 0xff, G: 0xff, A: 0xff}

	a := New(Settings{
		ID:         "theme-swap",
		Size:       5,
		Label:      "Working",
		LabelColor: red,
		GradColorA: red,
		GradColorB: blue,
	})
	for range 3 {
		a.Advance()
	}

	step := a.step.Load()
	widthBefore := a.width
	birthBefore := slices.Clone(a.birthSteps)

	a.SetColors(green, green, yellow, yellow)

	require.Equal(t, step, a.step.Load(), "color swaps must not reset the animation")
	require.Equal(t, widthBefore, a.width, "color swaps must not change the layout")
	require.Equal(t, birthBefore, a.birthSteps, "the birth schedule must survive a color swap")
	_, rebuilt := animCacheMap.Get(settingsHash(a.settings))
	require.True(t, rebuilt, "the pre-rendered frames must be rebuilt for the new colors")
	require.Equal(t, color.Color(green), a.labelColor)
	require.Equal(t, color.Color(yellow), a.suffixColor)

	// A nil suffix color falls back to the label color, matching New.
	b := New(Settings{ID: "suffix-fallback", Size: 5, LabelColor: red, GradColorA: red, GradColorB: blue})
	b.SetColors(green, green, yellow, nil)
	require.Equal(t, color.Color(green), b.suffixColor)
}
