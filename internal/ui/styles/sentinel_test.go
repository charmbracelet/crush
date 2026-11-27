package styles

import (
	"testing"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/require"
)

// The copy sentinels stand in for raw markdown syntax markers so
// selection copies can restore them (see internal/ui/list/rawcopy.go).
// They share invariants that keep the display unchanged and the copy
// unambiguous, guarded here:
//
//   - exactly one cell wide under every width authority in play
//     (glamour wraps with x/ansi, cells are stored with uniseg), for
//     the same flicker reason TestCodespanPaddingIsOneCellWide covers;
//   - tagged with the text-presentation variation selector U+FE0E, so
//     real message text cannot collide with them (and never with
//     U+FE0F, which measures as two cells);
//   - the emphasis and strong bases are not Unicode whitespace,
//     because ansi.Wordwrap rebuilds whitespace runs rune by rune and
//     would drop the selector from them;
//   - all sentinels are distinct from one another.

func TestCopySentinelsAreOneCellWide(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []string{CodespanPadding, SentinelEmph, SentinelStrong, SentinelStrike, SentinelFence, SentinelTask} {
		require.Equal(t, 1, ansi.StringWidth(sentinel),
			"sentinel %q must budget one cell, or word wrap disagrees with the terminal", sentinel)
		require.Equal(t, 1, uniseg.StringWidth(sentinel),
			"sentinel %q must occupy one screen cell", sentinel)

		state := -1
		cluster, _, w, _ := uniseg.FirstGraphemeClusterInString(sentinel, state)
		require.Equal(t, sentinel, cluster, "sentinel %q must be a single grapheme cluster", sentinel)
		require.Equal(t, 1, w)

		require.Contains(t, sentinel, "\ufe0e", "sentinel %q must carry the text-presentation selector", sentinel)
		require.NotContains(t, sentinel, "\ufe0f", "sentinel %q must not use the emoji selector (measures two cells)", sentinel)
	}
}

func TestInlineSentinelBasesAreNotWhitespace(t *testing.T) {
	t.Parallel()

	// The emphasis and strong sentinels appear mid-line inside wrapped
	// paragraphs. ansi.Wordwrap drops the variation selector from any
	// cluster whose base rune is whitespace (only U+00A0 is special-
	// cased and preserved), which would leave a bare, unguarded blank
	// in the render. Their bases must therefore not be whitespace.
	// CodespanPadding is exempt: its base is the special-cased NBSP.
	for _, sentinel := range []string{SentinelEmph, SentinelStrong} {
		base, _ := utf8DecodeFirstRune(sentinel)
		require.False(t, unicode.IsSpace(base),
			"sentinel %q base must not be whitespace or word wrap strips its selector", sentinel)
	}

	// The whitespace-based sentinels (strike, fence, task) sit where
	// wrap cannot separate them from what they mark, and rawcopy.go
	// matches them with or without the selector.
	for _, sentinel := range []string{SentinelStrike, SentinelFence, SentinelTask} {
		base, _ := utf8DecodeFirstRune(sentinel)
		require.True(t, unicode.IsSpace(base),
			"sentinel %q is documented as whitespace-based; update the matcher if that changes", sentinel)
	}
}

func TestCopySentinelsAreDistinct(t *testing.T) {
	t.Parallel()

	sentinels := []string{CodespanPadding, SentinelEmph, SentinelStrong, SentinelStrike, SentinelFence, SentinelTask}
	seen := make(map[string]bool, len(sentinels))
	for _, s := range sentinels {
		require.False(t, seen[s], "sentinel %q must be distinct from the others", s)
		seen[s] = true
	}
}

func utf8DecodeFirstRune(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}
