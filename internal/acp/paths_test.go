package acp

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// absPathTestBridge builds a bridge with no config store. workingDir is then
// empty, so relative paths resolve against the process directory, which is
// enough to prove the "must be absolute" contract (overview.md:160).
func absPathTestBridge() *eventBridge {
	return &eventBridge{
		app: &app.App{Sessions: &stubSessionService{}},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestAbsPathLeavesAbsolutePathsAlone(t *testing.T) {
	b := absPathTestBridge()
	// An absolute path is returned untouched, so a client can match it against
	// what it sent rather than getting a rewritten form back.
	abs := filepath.Join(t.TempDir(), "f.go")
	require.True(t, filepath.IsAbs(abs))
	require.Equal(t, abs, b.absPath(abs))
}

func TestAbsPathResolvesRelativePaths(t *testing.T) {
	b := absPathTestBridge()
	got := b.absPath("src/main.go")
	require.True(t, filepath.IsAbs(got), "relative path %q must become absolute, got %q", "src/main.go", got)
	require.Equal(t, "src/main.go", filepath.ToSlash(got)[len(filepath.ToSlash(got))-len("src/main.go"):])
}

func TestAbsPathEmptyStaysEmpty(t *testing.T) {
	b := absPathTestBridge()
	require.Empty(t, b.absPath(""))
}

func TestToolLocationsAreAbsolute(t *testing.T) {
	b := absPathTestBridge()
	locs := b.toolLocations(`{"file_path":"rel/x.go","path":"abs/y.go"}`)
	require.Len(t, locs, 2)
	for _, loc := range locs {
		require.True(t, filepath.IsAbs(loc.Path), "tool call location %q must be absolute", loc.Path)
	}
}

func TestDiffContentPathIsAbsolute(t *testing.T) {
	b := absPathTestBridge()
	tr := message.ToolResult{
		Name:     "edit",
		Content:  "ok",
		Metadata: `{"old_content":"a","new_content":"b"}`,
	}
	diff := diffContentFor(b, "edit", `{"file_path":"rel/z.go"}`, tr)
	require.NotNil(t, diff)
	require.NotNil(t, diff.Diff)
	require.True(t, filepath.IsAbs(diff.Diff.Path), "diff path %q must be absolute", diff.Diff.Path)
}
