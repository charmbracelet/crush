package anim

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestLowBandwidthRender exercises the reduced-motion render path.
// Table-driven: each step in the cycle should produce a deterministic
// "Label N-dots" output with no gradient/cycling chars.
func TestLowBandwidthRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		label     string
		ticks     int
		wantPlain string
	}{
		{
			name:      "step 0 shows one dot",
			label:     "Generating",
			ticks:     0,
			wantPlain: "Generating .",
		},
		{
			name:      "step 1 shows two dots",
			label:     "Generating",
			ticks:     1,
			wantPlain: "Generating ..",
		},
		{
			name:      "step 2 shows three dots",
			label:     "Generating",
			ticks:     2,
			wantPlain: "Generating ...",
		},
		{
			name:      "wraps back to one dot after three",
			label:     "Generating",
			ticks:     3,
			wantPlain: "Generating .",
		},
		{
			name:      "no label still renders dots",
			label:     "",
			ticks:     0,
			wantPlain: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := New(Settings{Label: tt.label, LowBandwidth: true})
			for range tt.ticks {
				a.Animate(StepMsg{ID: a.id})
			}
			require.Equal(t, tt.wantPlain, ansi.Strip(a.Render()))
		})
	}
}

// TestLowBandwidthSkipsHeavyState verifies that the constructor does not
// allocate the gradient/cycling-frames machinery in low-bandwidth mode.
// This guards against regressions where someone re-introduces the
// expensive prerender path.
func TestLowBandwidthSkipsHeavyState(t *testing.T) {
	t.Parallel()
	a := New(Settings{Label: "Generating", LowBandwidth: true})
	require.True(t, a.lowBandwidth)
	require.Nil(t, a.cyclingFrames, "cyclingFrames should not be populated")
	require.Nil(t, a.initialFrames, "initialFrames should not be populated")
}

// TestSetDefaultLowBandwidth proves the package-level fallback wires
// through New() when Settings doesn't set LowBandwidth explicitly.
func TestSetDefaultLowBandwidth(t *testing.T) {
	// Cannot use t.Parallel: mutates package state.
	t.Cleanup(func() { SetDefaultLowBandwidth(false) })

	SetDefaultLowBandwidth(true)
	a := New(Settings{Label: "Generating"})
	require.True(t, a.lowBandwidth, "expected default flag to be inherited")

	SetDefaultLowBandwidth(false)
	b := New(Settings{Label: "Generating"})
	require.False(t, b.lowBandwidth, "expected default flag to be cleared")
}

// TestLowBandwidthLabelStaysVisible guards against the spinner visually
// disappearing between cycles \u2014 we explicitly chose "., .., ..." rather
// than the standard "., .., ..., (empty)" so the user always has a sign
// the agent is alive.
func TestLowBandwidthLabelStaysVisible(t *testing.T) {
	t.Parallel()
	a := New(Settings{Label: "Generating", LowBandwidth: true})
	for i := range 12 {
		plain := ansi.Strip(a.Render())
		require.True(t, strings.Contains(plain, "."), "tick %d had no dot: %q", i, plain)
		a.Animate(StepMsg{ID: a.id})
	}
}
