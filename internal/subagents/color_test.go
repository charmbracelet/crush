package subagents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsValidColor verifies that IsValidColor returns true for all eight
// defined color names and false for invalid values.
func TestIsValidColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{name: "red_valid", color: "red", want: true},
		{name: "orange_valid", color: "orange", want: true},
		{name: "yellow_valid", color: "yellow", want: true},
		{name: "green_valid", color: "green", want: true},
		{name: "cyan_valid", color: "cyan", want: true},
		{name: "blue_valid", color: "blue", want: true},
		{name: "purple_valid", color: "purple", want: true},
		{name: "pink_valid", color: "pink", want: true},
		{name: "empty_invalid", color: "", want: false},
		{name: "ultra_invalid", color: "ultra", want: false},
		{name: "RED_wrong_case", color: "RED", want: false},
		{name: "lime_invalid", color: "lime", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, IsValidColor(tt.color))
		})
	}
}

// TestAutoColor_Deterministic verifies that calling AutoColor with the same
// name twice always returns the same color.
func TestAutoColor_Deterministic(t *testing.T) {
	t.Parallel()

	names := []string{
		"my-agent",
		"code-reviewer",
		"data-analyst",
		"test-runner",
		"",
	}

	for _, name := range names {
		t.Run("name_"+name, func(t *testing.T) {
			t.Parallel()

			first := AutoColor(name)
			second := AutoColor(name)
			require.Equal(t, first, second, "AutoColor(%q) must return the same value on repeated calls", name)
		})
	}
}

// TestAutoColor_ReturnsValidColor verifies that AutoColor always returns one
// of the eight defined color names across a broad set of inputs.
func TestAutoColor_ReturnsValidColor(t *testing.T) {
	t.Parallel()

	names := []string{
		"alpha",
		"beta",
		"gamma",
		"delta",
		"epsilon",
		"zeta",
		"eta",
		"theta",
		"iota",
		"kappa",
		"lambda",
		"mu",
		"nu",
		"xi",
		"omicron",
		"pi",
		"rho",
		"sigma",
		"tau",
		"upsilon",
	}

	for _, name := range names {
		t.Run("name_"+name, func(t *testing.T) {
			t.Parallel()

			color := AutoColor(name)
			require.True(t, IsValidColor(color), "AutoColor(%q) returned %q which is not a valid color", name, color)
		})
	}
}

// TestAutoColor_DistributionNotConstant verifies that AutoColor does not map
// all inputs to the same color (i.e. the hash is not degenerate).
func TestAutoColor_DistributionNotConstant(t *testing.T) {
	t.Parallel()

	names := []string{
		"alpha", "beta", "gamma", "delta", "epsilon",
		"zeta", "eta", "theta", "iota", "kappa",
		"lambda", "mu", "nu", "xi", "omicron",
		"pi", "rho", "sigma", "tau", "upsilon",
	}

	seen := make(map[string]bool, len(names))
	for _, name := range names {
		seen[AutoColor(name)] = true
	}

	require.Greater(t, len(seen), 1, "AutoColor must map distinct names to at least 2 distinct colors")
}
