package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestCapMCPInstructions(t *testing.T) {
	t.Parallel()

	t.Run("sorted by server name", func(t *testing.T) {
		t.Parallel()
		out := capMCPInstructions(map[string]string{
			"zeta":  "z-instructions",
			"alpha": "a-instructions",
			"mid":   "m-instructions",
		})
		require.Less(t, strings.Index(out, "a-instructions"), strings.Index(out, "m-instructions"))
		require.Less(t, strings.Index(out, "m-instructions"), strings.Index(out, "z-instructions"))
	})

	t.Run("per-server cap truncates verbose server", func(t *testing.T) {
		t.Parallel()
		head := "HEAD-MARKER "
		tail := " TAIL-MARKER"
		verbose := head + strings.Repeat("verbose rule text. ", 500) + tail
		out := capMCPInstructions(map[string]string{"verbose": verbose})
		require.LessOrEqual(t, approxTokenCount(out), int64(maxMCPInstructionsPerServer))
		require.Contains(t, out, "HEAD-MARKER")
		require.Contains(t, out, "TAIL-MARKER")
		require.Contains(t, out, "[...truncated...]")
		require.True(t, utf8.ValidString(out))
	})

	t.Run("total cap skips or truncates overflow", func(t *testing.T) {
		t.Parallel()
		// Five servers each near the per-server cap: ~5K total exceeds
		// the 4K total budget.
		raw := map[string]string{}
		for _, name := range []string{"s1", "s2", "s3", "s4", "s5"} {
			raw[name] = strings.Repeat(name+" rule. ", 300)
		}
		out := capMCPInstructions(raw)
		require.LessOrEqual(t, approxTokenCount(out), int64(maxMCPInstructionsTotal))
	})

	t.Run("normal instructions pass through", func(t *testing.T) {
		t.Parallel()
		out := capMCPInstructions(map[string]string{"ok": "be nice."})
		require.Contains(t, out, "be nice.")
	})

	t.Run("invalid UTF-8 normalized", func(t *testing.T) {
		t.Parallel()
		out := capMCPInstructions(map[string]string{"bad": string([]byte{'a', 0xff, 'b'})})
		require.True(t, utf8.ValidString(out))
	})
}
