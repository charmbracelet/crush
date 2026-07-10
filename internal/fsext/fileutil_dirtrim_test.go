package fsext

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirTrimUnicode(t *testing.T) {
	sep := string(filepath.Separator)

	// An abbreviated component keeps its first character.
	require.Equal(t,
		filepath.Join("~", "...", "p", "file"),
		DirTrim(sep+"home"+sep+"user"+sep+"project"+sep+"file", 2),
	)

	// A multi-byte (e.g. CJK) component must keep its first whole character,
	// not its first byte, which would render a wrong character.
	require.Equal(t,
		filepath.Join("~", "...", "项", "file"),
		DirTrim(sep+"home"+sep+"项目"+sep+"file", 2),
	)

	// A grapheme cluster spanning multiple runes (a base letter plus a
	// combining accent) must be kept whole, not split after the base rune.
	require.Equal(t,
		filepath.Join("~", "...", "e\u0301", "file"),
		DirTrim(sep+"home"+sep+"e\u0301cole"+sep+"file", 2),
	)
}
