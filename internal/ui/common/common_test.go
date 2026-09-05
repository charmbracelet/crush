package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanReadyMarkerPresent(t *testing.T) {
	t.Parallel()

	require.True(t, PlanReadyMarkerPresent("plan\n"+PlanReadyMarker))
	require.True(t, PlanReadyMarkerPresent("plan\n  "+PlanReadyMarker+"  \ntrailing note"))
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
}
