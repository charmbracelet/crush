package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestStyleAnswerMultilineFreeText(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	sty := &s

	answer := "User provided: cool flag\n500\nForensics Steganography"
	got := ansi.Strip(styleAnswer(sty, answer))

	// A multiline free-text answer must read as one line, not a
	// comma-separated list of its lines.
	require.Equal(t, "cool flag 500 Forensics Steganography", got)
	require.NotContains(t, got, ",")
}

func TestStyleAnswerSelectionPlusFreeText(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	sty := &s

	answer := "User selected: [\"speed\",\"readability\"]\nUser provided: maintainability"
	got := ansi.Strip(styleAnswer(sty, answer))

	// Distinct markers remain separate segments joined by a comma.
	require.Equal(t, "speed, readability, maintainability", got)
}

func TestSplitAnswerSegments(t *testing.T) {
	t.Parallel()
	segments := splitAnswerSegments("User selected: [\"a\"]\nUser provided: line one\nline two")
	require.Len(t, segments, 2)
	require.True(t, strings.HasPrefix(segments[1], "User provided:"))
	require.Contains(t, segments[1], "line two")
}
