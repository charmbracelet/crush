package model

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestFileList(t *testing.T) {
	t.Parallel()

	t.Run("empty stats no truncation needed", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 0, Deletions: 0},
		}
		got := fileList(st, "/", files, 30, 10)
		require.Contains(t, stripANSI(got), "main.go")
	})

	t.Run("empty stats path truncates to width", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "/very/long/path/to/some/deeply/nested/file.go"}, Additions: 0, Deletions: 0},
		}
		got := fileList(st, "/", files, 10, 10)
		plain := stripANSI(got)
		for _, line := range strings.Split(plain, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), 10, "line exceeds sidebar width: %q", line)
		}
	})

	t.Run("with additions and deletions fits within width", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 5, Deletions: 3},
		}
		got := fileList(st, "/", files, 20, 10)
		plain := stripANSI(got)
		require.Contains(t, plain, "+5")
		require.Contains(t, plain, "-3")
		for _, line := range strings.Split(plain, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), 20, "line exceeds sidebar width: %q", line)
		}
	})

	t.Run("narrow width with stats clamps path to zero", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 100, Deletions: 200},
		}
		got := fileList(st, "/", files, 5, 10)
		plain := stripANSI(got)
		require.NotContains(t, plain, "main.go")
		require.Equal(t, "+100 -200", strings.TrimSpace(plain))
	})

	t.Run("single addition only", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 3, Deletions: 0},
		}
		got := fileList(st, "/", files, 20, 10)
		plain := stripANSI(got)
		require.Contains(t, plain, "+3")
		require.NotContains(t, plain, "-0")
		for _, line := range strings.Split(plain, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), 20, "line exceeds sidebar width: %q", line)
		}
	})

	t.Run("single deletion only", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 0, Deletions: 7},
		}
		got := fileList(st, "/", files, 20, 10)
		plain := stripANSI(got)
		require.NotContains(t, plain, "+0")
		require.Contains(t, plain, "-7")
		for _, line := range strings.Split(plain, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), 20, "line exceeds sidebar width: %q", line)
		}
	})

	t.Run("max items zero returns empty", func(t *testing.T) {
		t.Parallel()

		st := minimalFileStyles()
		files := []SessionFile{
			{FirstVersion: history.File{Path: "main.go"}, Additions: 1, Deletions: 1},
		}
		got := fileList(st, "/", files, 20, 0)
		require.Empty(t, got)
	})
}

func TestLoadSessionFilesNormalizesLineEndings(t *testing.T) {
	t.Parallel()

	makeContent := func(sep string) string {
		var b strings.Builder
		for i := 0; i < 1000; i++ {
			b.WriteString(fmt.Sprintf("line %d", i))
			b.WriteString(sep)
		}
		return b.String()
	}

	t.Run("CRLF vs LF reports the real delta, not the line ending swap", func(t *testing.T) {
		t.Parallel()

		first := history.File{Path: "main.go", Content: makeContent("\r\n"), Version: 0}
		last := history.File{Path: "main.go", Content: makeContent("\n") + "added\n", Version: 1}

		firstContent, _ := fsext.ToUnixLineEndings(first.Content)
		lastContent, _ := fsext.ToUnixLineEndings(last.Content)
		_, additions, removals := diff.GenerateDiff(firstContent, lastContent, first.Path)

		require.Equal(t, 1, additions, "expected one addition for the appended line, got %d", additions)
		require.Equal(t, 0, removals, "expected zero removals, got %d", removals)
	})

	t.Run("identical content with mismatched line endings reports no changes", func(t *testing.T) {
		t.Parallel()

		first := history.File{Path: "main.go", Content: makeContent("\r\n"), Version: 0}
		last := history.File{Path: "main.go", Content: makeContent("\n"), Version: 1}

		firstContent, _ := fsext.ToUnixLineEndings(first.Content)
		lastContent, _ := fsext.ToUnixLineEndings(last.Content)
		_, additions, removals := diff.GenerateDiff(firstContent, lastContent, first.Path)

		require.Equal(t, 0, additions)
		require.Equal(t, 0, removals)
	})
}

func minimalFileStyles() *styles.Styles {
	st := styles.CharmtonePantera()
	st.Files.Path = lipgloss.NewStyle()
	st.Files.Additions = lipgloss.NewStyle()
	st.Files.Deletions = lipgloss.NewStyle()
	st.Files.SectionTitle = lipgloss.NewStyle()
	st.Files.EmptyMessage = lipgloss.NewStyle()
	st.Files.TruncationHint = lipgloss.NewStyle()
	return &st
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			esc = true
			continue
		}
		if esc {
			if s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' {
				esc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
