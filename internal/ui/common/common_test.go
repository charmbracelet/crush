package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImagePath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"image.png",
		"image.jpg",
		"image.jpeg",
		"dir/IMAGE.PNG",
		"/abs/path/to/photo.Jpeg",
	} {
		require.True(t, IsImagePath(path), "expected %q to be an image path", path)
	}

	for _, path := range []string{
		"file.txt",
		"image.gif",
		"image.png.txt",
		"",
	} {
		require.False(t, IsImagePath(path), "expected %q to not be an image path", path)
	}
}

func TestPlanReadyMarkerPresent(t *testing.T) {
	t.Parallel()

	require.True(t, PlanReadyMarkerPresent("plan\n"+PlanReadyMarker))
	require.True(t, PlanReadyMarkerPresent("plan\n  "+PlanReadyMarker+"  \ntrailing note"))
	// Inline-code backticks around the marker still count.
	require.True(t, PlanReadyMarkerPresent("plan\n`"+PlanReadyMarker+"`"))
	require.False(t, PlanReadyMarkerPresent("plan without marker"))
	require.False(t, PlanReadyMarkerPresent("I will end with "+PlanReadyMarker+" when done."))
}

func TestStripPlanReadyMarker(t *testing.T) {
	t.Parallel()

	require.Equal(t, "plan", StripPlanReadyMarker("plan\n"+PlanReadyMarker))
	require.Equal(t, "plan\nnote", StripPlanReadyMarker("plan\n  "+PlanReadyMarker+"  \nnote"))
	// Mentions inside prose are left untouched.
	prose := "I will end with " + PlanReadyMarker + " when done."
	require.Equal(t, prose, StripPlanReadyMarker(prose))
	require.Equal(t, "no marker here", StripPlanReadyMarker("no marker here"))
	// Inline-code backticks around the marker are stripped with it.
	require.Equal(t, "plan", StripPlanReadyMarker("plan\n`"+PlanReadyMarker+"`"))
	// A fence wrapping only the marker is removed whole, so no empty code
	// block is left behind.
	require.Equal(t, "plan", StripPlanReadyMarker("plan\n```\n"+PlanReadyMarker+"\n```"))
	// A real code block above the marker keeps its fences.
	require.Equal(t,
		"plan\n```go\nfmt.Println(1)\n```",
		StripPlanReadyMarker("plan\n```go\nfmt.Println(1)\n```\n"+PlanReadyMarker))
}

func TestPlanStartMarkerPresent(t *testing.T) {
	t.Parallel()

	require.True(t, PlanStartMarkerPresent(PlanStartMarker+"\nplan"))
	require.True(t, PlanStartMarkerPresent("note\n  "+PlanStartMarker+"  \nplan"))
	// Inline-code backticks around the marker still count.
	require.True(t, PlanStartMarkerPresent("`"+PlanStartMarker+"`\nplan"))
	require.False(t, PlanStartMarkerPresent("plan without marker"))
	require.False(t, PlanStartMarkerPresent("It begins with "+PlanStartMarker+" on its own line."))
	// The ready marker must not satisfy the start check.
	require.False(t, PlanStartMarkerPresent("plan\n"+PlanReadyMarker))
}

func TestStripPlanMarkers(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"plan",
		StripPlanMarkers(PlanStartMarker+"\nplan\n"+PlanReadyMarker))
	// A start marker whose ready companion never arrived (interrupted
	// plan) is stripped on its own.
	require.Equal(t,
		"half a plan",
		StripPlanMarkers(PlanStartMarker+"\nhalf a plan"))
	// Only the start marker is removed; prose mentions stay untouched.
	prose := "The plan begins with " + PlanStartMarker + "."
	require.Equal(t, prose, StripPlanMarkers(prose))
}
