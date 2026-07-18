package model

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
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
		for line := range strings.SplitSeq(plain, "\n") {
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
		for line := range strings.SplitSeq(plain, "\n") {
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
		for line := range strings.SplitSeq(plain, "\n") {
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
		for line := range strings.SplitSeq(plain, "\n") {
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

func TestFetchParentMeta_ReturnsTitleAndColor(t *testing.T) {
	t.Parallel()

	ws := &getSessionWorkspace{
		sessions: map[string]session.Session{
			"parent-id": {ID: "parent-id", Title: "My Parent Session"},
		},
		running: map[string][]workspace.RunningSubagentInfo{
			"parent-id": {{ChildSessionID: "child-id", Color: "purple"}},
		},
	}
	m := &UI{com: &common.Common{Workspace: ws}}

	cmd := m.fetchParentMeta("parent-id", "child-id")
	msg := cmd()

	ptm, ok := msg.(parentTitleMsg)
	require.True(t, ok, "expected parentTitleMsg")
	require.Equal(t, "My Parent Session", ptm.title)
	require.Equal(t, "purple", ptm.color)
}

func TestFetchParentMeta_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()

	ws := &getSessionWorkspace{sessions: map[string]session.Session{}}
	m := &UI{com: &common.Common{Workspace: ws}}

	cmd := m.fetchParentMeta("missing", "")
	msg := cmd()

	require.Nil(t, msg)
}

// getSessionWorkspace stubs GetSession and RunningSubagents for the
// fetchParentMeta tests.
type getSessionWorkspace struct {
	workspace.Workspace
	sessions     map[string]session.Session
	running      map[string][]workspace.RunningSubagentInfo
	sessionFiles []history.File
}

func (w *getSessionWorkspace) RunningSubagents(parentSessionID string) []workspace.RunningSubagentInfo {
	return w.running[parentSessionID]
}

func (w *getSessionWorkspace) GetSession(_ context.Context, sessionID string) (session.Session, error) {
	if sess, ok := w.sessions[sessionID]; ok {
		return sess, nil
	}
	return session.Session{}, errors.New("not found")
}

// ListSessionHistory returns the stubbed sessionFiles regardless of the
// sessionID requested, so handleFileEvent's follow-up load never panics on
// a nil workspace.
func (w *getSessionWorkspace) ListSessionHistory(_ context.Context, _ string) ([]history.File, error) {
	return w.sessionFiles, nil
}

func TestHandleFileEvent_NilSession_Ignored(t *testing.T) {
	t.Parallel()

	m := &UI{session: nil}

	cmd := m.handleFileEvent(history.File{SessionID: "anything"})

	require.Nil(t, cmd)
}

func TestHandleFileEvent_MatchingSessionID_Allowed(t *testing.T) {
	t.Parallel()

	ws := &getSessionWorkspace{sessionFiles: []history.File{}}
	m := &UI{
		session: &session.Session{ID: "parent-1"},
		com:     &common.Common{Workspace: ws},
	}

	cmd := m.handleFileEvent(history.File{SessionID: "parent-1"})

	require.NotNil(t, cmd)
	_, ok := cmd().(sessionFilesUpdatesMsg)
	require.True(t, ok, "expected sessionFilesUpdatesMsg")
}

func TestHandleFileEvent_KnownChildSessionID_Allowed(t *testing.T) {
	t.Parallel()

	ws := &getSessionWorkspace{sessionFiles: []history.File{}}
	m := &UI{
		session:              &session.Session{ID: "parent-1"},
		knownChildSessionIDs: map[string]bool{"child-1": true},
		com:                  &common.Common{Workspace: ws},
	}

	cmd := m.handleFileEvent(history.File{SessionID: "child-1"})

	require.NotNil(t, cmd)
	_, ok := cmd().(sessionFilesUpdatesMsg)
	require.True(t, ok, "expected sessionFilesUpdatesMsg")
}

func TestHandleFileEvent_UnknownSessionID_Ignored(t *testing.T) {
	t.Parallel()

	m := &UI{
		session:              &session.Session{ID: "parent-1"},
		knownChildSessionIDs: map[string]bool{"child-1": true},
	}

	cmd := m.handleFileEvent(history.File{SessionID: "some-unrelated-session"})

	require.Nil(t, cmd)
}

func TestHandleFileEvent_NilKnownChildSessionIDs_DoesNotPanic(t *testing.T) {
	t.Parallel()

	m := &UI{session: &session.Session{ID: "parent-1"}}

	cmd := m.handleFileEvent(history.File{SessionID: "some-other-session"})

	require.Nil(t, cmd)
}
