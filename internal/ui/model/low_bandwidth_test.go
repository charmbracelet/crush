package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLowBandwidthEnabled_NilSafe verifies the UI helper guards against
// the zero-value UI used in unit tests \u2014 startTitleAnimation must not
// panic when `m.com` is nil.
func TestLowBandwidthEnabled_NilSafe(t *testing.T) {
	t.Parallel()

	var m *UI
	require.False(t, m.lowBandwidthEnabled())

	m = &UI{}
	require.False(t, m.lowBandwidthEnabled())
}
