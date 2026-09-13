package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiagnosticsSnapshot_NewErrorsSince(t *testing.T) {
	t.Parallel()

	baseline := DiagnosticsSnapshot{
		"a.go|undefined: x": 1,
		"b.go|old error":    2,
	}
	after := DiagnosticsSnapshot{
		"a.go|undefined: x":    1, // unchanged
		"b.go|old error":       2, // unchanged
		"c.go|new error":       1, // brand new
		"b.go|old error twin":  0, // absent keys contribute nothing
		"a.go|same msg, moved": 2, // count beyond baseline
	}

	newErrs := after.NewErrorsSince(baseline)
	require.ElementsMatch(t, []string{
		"c.go|new error",
		"a.go|same msg, moved",
		"a.go|same msg, moved",
	}, newErrs, "delta is a multiset: extra counts surface once each")
}

// Keys are position-insensitive: an edit that shifts a pre-existing
// error's line must not read as a new error.
func TestDiagnosticsSnapshot_PositionShiftIsNotNew(t *testing.T) {
	t.Parallel()

	baseline := DiagnosticsSnapshot{"a.go|undefined: x": 1}
	after := DiagnosticsSnapshot{"a.go|undefined: x": 1}

	require.Empty(t, after.NewErrorsSince(baseline))
}

func TestDiagnosticsSnapshot_NewErrorsSince_RemovalOnly(t *testing.T) {
	t.Parallel()

	baseline := DiagnosticsSnapshot{"a.go|err": 2}
	after := DiagnosticsSnapshot{"a.go|err": 1}

	require.Empty(t, after.NewErrorsSince(baseline),
		"fewer diagnostics than baseline must not report new errors")
}

func TestSnapshotDiagnostics_NilManager(t *testing.T) {
	t.Parallel()

	require.Empty(t, SnapshotDiagnostics(nil))
}

func TestAnyClientHandles(t *testing.T) {
	t.Parallel()

	require.False(t, AnyClientHandles(nil, "/tmp/x.go"))
	require.False(t, AnyClientHandles(nil, ""))
}
