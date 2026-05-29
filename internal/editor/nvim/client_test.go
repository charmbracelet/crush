package nvim

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/taigrr/crush/internal/editor"
)

func TestNew_NoEnv_ReturnsNotOK(t *testing.T) {
	t.Setenv("NVIM", "")
	t.Setenv("NVIM_LISTEN_ADDRESS", "")
	b, ok := New()
	require.False(t, ok)
	require.Nil(t, b)
}

func TestNew_BadAddress_ReturnsNotOK(t *testing.T) {
	// Point at a path that cannot exist; Dial must fail and New returns (nil, false).
	t.Setenv("NVIM", "/tmp/neocrush-bridge-test-does-not-exist-")
	t.Setenv("NVIM_LISTEN_ADDRESS", "")
	b, ok := New()
	require.False(t, ok)
	require.Nil(t, b)
}

func TestDetectAddress_Precedence(t *testing.T) {
	t.Setenv("NVIM", "/tmp/preferred")
	t.Setenv("NVIM_LISTEN_ADDRESS", "/tmp/legacy")
	require.Equal(t, "/tmp/preferred", detectAddress())

	t.Setenv("NVIM", "")
	require.Equal(t, "/tmp/legacy", detectAddress())
}

// Confirm Noop satisfies the interface and matches our error contract.
func TestNoopBridge(t *testing.T) {
	var b editor.Bridge = editor.Noop{}
	require.False(t, b.Available())

	_, err := b.Context(t.Context())
	require.ErrorIs(t, err, editor.ErrUnavailable)

	require.ErrorIs(t, b.ShowLocations(t.Context(), "t", []editor.Location{{Filename: "x", Line: 1}}), editor.ErrUnavailable)
	// Best-effort no-ops must not error.
	require.NoError(t, b.FlashEdit(t.Context(), "x", 0, 1))
	require.NoError(t, b.NotifyFileChanged(t.Context(), "x"))
	require.NoError(t, b.Close())
}
