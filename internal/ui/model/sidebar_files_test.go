package model

import (
	"testing"
	"unsafe"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func sidebarTestFiles(paths ...string) []SessionFile {
	files := make([]SessionFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, SessionFile{
			FirstVersion:  history.File{Path: "/work/" + p},
			LatestVersion: history.File{Path: "/work/" + p},
			Additions:     3,
			Deletions:     1,
		})
	}
	return files
}

// sameString reports whether a and b share their backing bytes, which shows
// the second one was served from the cache instead of rendered again.
func sameString(a, b string) bool {
	return len(a) == len(b) && unsafe.StringData(a) == unsafe.StringData(b)
}

func TestSidebarFilesSection_Memoized(t *testing.T) {
	t.Parallel()

	u := newHostThemeTestUI(t, nil)
	u.sessionFiles = sidebarTestFiles("a.go", "b.go")

	first := u.sidebarFilesSection("/work", 40)
	require.Contains(t, stripANSI(first), "a.go")
	require.Equal(t, u.filesInfo("/work", 40, 2, true), first)

	again := u.sidebarFilesSection("/work", 40)
	require.True(t, sameString(first, again), "unchanged inputs must hit the cache")

	// A new files slice is rendered again.
	u.sessionFiles = sidebarTestFiles("a.go", "c.go")
	changed := u.sidebarFilesSection("/work", 40)
	require.Contains(t, stripANSI(changed), "c.go")
	require.NotContains(t, stripANSI(changed), "b.go")

	// So is a different width.
	narrow := u.sidebarFilesSection("/work", 30)
	require.Equal(t, u.filesInfo("/work", 30, 2, true), narrow)
	require.NotEqual(t, changed, narrow)

	// And a different working directory.
	other := u.sidebarFilesSection("/elsewhere", 30)
	require.Equal(t, u.filesInfo("/elsewhere", 30, 2, true), other)

	// Clearing the session files shows the empty state.
	u.sessionFiles = nil
	require.Contains(t, stripANSI(u.sidebarFilesSection("/elsewhere", 30)), "None")
}

// A theme change replaces the styles in place, keeping the pointer, so the
// cached section must be dropped when the styles are refreshed.
func TestSidebarFilesSection_ThemeChangeInvalidates(t *testing.T) {
	t.Parallel()

	u := newHostThemeTestUI(t, nil)
	u.sessionFiles = sidebarTestFiles("a.go")

	before := u.sidebarFilesSection("/work", 40)
	gruvbox, err := styles.LoadTheme("gruvbox-dark")
	require.NoError(t, err)
	u.applyTheme(gruvbox)

	after := u.sidebarFilesSection("/work", 40)
	require.Equal(t, u.filesInfo("/work", 40, 1, true), after)
	require.NotEqual(t, before, after, "a theme change must re-render the files section")
}
