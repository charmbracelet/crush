package anim

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticEllipsisCycling(t *testing.T) {
	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff},
		GradColorB:  color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff},
		LabelColor:  color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff},
		CycleColors: true,
	})

	// Capture renders for each step
	renders := make([]string, len(staticEllipsisFrames))
	for i := range staticEllipsisFrames {
		a.step.Store(int64(i))
		renders[i] = a.Render()
	}

	// Each render should contain "Working" and the appropriate dots
	for i, r := range renders {
		if !strings.Contains(r, "Working") {
			t.Errorf("expected render to contain 'Working', got %q", r)
		}
		expectedDots := staticEllipsisFrames[i]
		if expectedDots != "" && !strings.Contains(r, expectedDots) {
			t.Errorf("step %d: expected render to contain %q, got %q", i, expectedDots, r)
		}
	}

	// Verify cycle wraps correctly
	a.step.Store(int64(len(staticEllipsisFrames)))
	for range staticFrameDivisor {
		a.Advance()
	}
	if int(a.step.Load()) != 0 {
		t.Errorf("expected step to wrap to 0, got %d", a.step.Load())
	}
}

func TestStaticStartsWithWorking(t *testing.T) {
	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}

	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff},
		GradColorB:  color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff},
		LabelColor:  label,
		CycleColors: true,
	})

	// At step 0, should show "Working" (no dots yet).
	r := a.Render()
	if !strings.Contains(r, "Working") {
		t.Fatalf("expected render to contain 'Working', got %q", r)
	}
	if a.staticRendered == "" {
		t.Fatal("expected staticRendered to be set")
	}
}

func TestStaticEllipsisColor(t *testing.T) {
	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	ellipsis := color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}

	a := New(Settings{
		Static:        true,
		Size:          15,
		LabelColor:    label,
		EllipsisColor: ellipsis,
		CycleColors:   true,
	})

	if a.ellipsisColor != ellipsis {
		t.Errorf("expected ellipsisColor to be set, got %v", a.ellipsisColor)
	}

	// When EllipsisColor is unset, it should default to LabelColor
	b := New(Settings{
		Static:      true,
		Size:        15,
		LabelColor:  label,
		CycleColors: true,
	})
	if b.ellipsisColor != label {
		t.Errorf("expected ellipsisColor to default to LabelColor, got %v", b.ellipsisColor)
	}
}

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

// TestStaticAdvanceTicksEllipsis verifies that a static (reduced) Anim
// still animates its ellipsis under the shared clock: Advance reports no
// change between steps and advances the ellipsis once per
// staticFrameDivisor frames.
func TestStaticAdvanceTicksEllipsis(t *testing.T) {
	t.Parallel()

	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	a := New(Settings{
		ID:          "static",
		Static:      true,
		Size:        5,
		LabelColor:  label,
		CycleColors: true,
	})

	// Frames before the divisor boundary must not change the output.
	for range staticFrameDivisor - 1 {
		require.False(t, a.Advance(), "static advance must be a no-op between ellipsis steps")
	}
	require.Equal(t, int64(0), a.step.Load())

	// The divisor-th frame advances the ellipsis.
	require.True(t, a.Advance())
	require.Equal(t, int64(1), a.step.Load())

	// After one step the rendered output shows the first dot.
	rendered := a.Render()
	require.Contains(t, rendered, "Working")
	require.Contains(t, rendered, ".")
}
