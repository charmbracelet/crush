package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/stretchr/testify/require"
)

const otherChangesHeader = "\n\nOther files changed while it ran (bash, you, or the main agent):\n- "

// sessionFilesHistory serves fixed file rows per session, so tests control
// the UpdatedAt values.
type sessionFilesHistory struct {
	history.Service
	files map[string][]history.File
}

func (h sessionFilesHistory) ListLatestSessionFiles(_ context.Context, sessionID string) ([]history.File, error) {
	return h.files[sessionID], nil
}

// historyRow is a history entry for a path, relative to the working
// directory unless absolute.
type historyRow struct {
	path      string
	updatedAt int64
}

func numberedNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("f%03d.txt", i)
	}
	return names
}

func TestOtherChangesNote(t *testing.T) {
	t.Parallel()

	since := time.Unix(1000, 0)
	many := numberedNames(105)

	tests := []struct {
		name     string
		changed  []string
		child    []historyRow
		parent   []historyRow
		nilHist  bool
		wantFunc func() string
	}{
		{
			name:    "lists changed files relative to the working directory, sorted",
			changed: []string{"z.txt", "a.txt"},
			wantFunc: func() string {
				return otherChangesHeader + "a.txt\n- z.txt"
			},
		},
		{
			name:     "path outside the working directory stays absolute",
			changed:  []string{"/elsewhere/outside.txt", "in.txt"},
			wantFunc: func() string { return otherChangesHeader + "in.txt\n- /elsewhere/outside.txt" },
		},
		{
			name:     "child session files are subtracted",
			changed:  []string{"edited.go", "bash.txt"},
			child:    []historyRow{{path: "edited.go", updatedAt: 1}},
			wantFunc: func() string { return otherChangesHeader + "bash.txt" },
		},
		{
			name:    "parent files written since the start are subtracted, older ones kept",
			changed: []string{"at-start.txt", "later.txt", "older.txt", "bash.txt"},
			parent: []historyRow{
				{path: "at-start.txt", updatedAt: since.Unix()},
				{path: "later.txt", updatedAt: since.Unix() + 500},
				{path: "older.txt", updatedAt: since.Unix() - 1},
			},
			wantFunc: func() string { return otherChangesHeader + "bash.txt\n- older.txt" },
		},
		{
			name:     "nothing left after subtraction",
			changed:  []string{"edited.go", "parent.go"},
			child:    []historyRow{{path: "edited.go", updatedAt: 1}},
			parent:   []historyRow{{path: "parent.go", updatedAt: since.Unix() + 1}},
			wantFunc: func() string { return "" },
		},
		{
			name:     "no changed files",
			wantFunc: func() string { return "" },
		},
		{
			name:    "nil history still lists",
			changed: []string{"b.txt", "a.txt"},
			nilHist: true,
			wantFunc: func() string {
				return otherChangesHeader + "a.txt\n- b.txt"
			},
		},
		{
			name:    "lists at most 100 paths and counts the rest",
			changed: many,
			wantFunc: func() string {
				return otherChangesHeader + strings.Join(many[:100], "\n- ") + "\n- ... and 5 more"
			},
		},
		{
			name:    "exactly 100 paths have no overflow line",
			changed: many[:100],
			wantFunc: func() string {
				return otherChangesHeader + strings.Join(many[:100], "\n- ")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			coord := newGateTestCoordinator(t, false)
			// The agent build reads coord.history, so let it finish before
			// the test replaces the field.
			require.NoError(t, coord.readyWg.Wait())
			wd := coord.cfg.WorkingDir()
			resolve := func(p string) string {
				if filepath.IsAbs(p) {
					return p
				}
				return filepath.Join(wd, p)
			}
			rows := func(in []historyRow) []history.File {
				var out []history.File
				for _, r := range in {
					out = append(out, history.File{Path: resolve(r.path), UpdatedAt: r.updatedAt})
				}
				return out
			}
			var changed []string
			for _, p := range tt.changed {
				changed = append(changed, resolve(p))
			}

			if tt.nilHist {
				coord.history = nil
			} else {
				coord.history = sessionFilesHistory{
					Service: coord.history,
					files:   map[string][]history.File{"child": rows(tt.child), "parent": rows(tt.parent)},
				}
			}

			got := coord.otherChangesNote(t.Context(), changed, "child", "parent", since)
			require.Equal(t, tt.wantFunc(), got)
		})
	}
}

// newGitBGHarness is a background harness whose working directory is a git
// repository.
func newGitBGHarness(t *testing.T) *bgHarness {
	t.Helper()
	h := newBGHarness(t, true)
	initGitRepo(t, h.coord.cfg.WorkingDir())
	return h
}

// bashLikeChild writes files straight to disk, like a bash command, and
// finishes with "done". extra runs first with the child's session ID.
func bashLikeChild(wd string, extra func(ctx context.Context, sessionID string) error, files ...string) *mockSessionAgent {
	return newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		if extra != nil {
			if err := extra(ctx, call.SessionID); err != nil {
				return nil, err
			}
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(wd, f), []byte("written by bash"), 0o644); err != nil {
				return nil, err
			}
		}
		return agentResultWithText("done"), nil
	})
}

func TestDeliverReport_ListsFilesChangedOutsideTheEditTools(t *testing.T) {
	t.Parallel()
	h := newGitBGHarness(t)
	wd := h.coord.cfg.WorkingDir()

	child := bashLikeChild(wd, func(ctx context.Context, sessionID string) error {
		return recordFiles(ctx, h.coord.history, sessionID, filepath.Join(wd, "edited.go"))
	}, "edited.go", "bash.txt")
	h.start(t, h.params(h.parentID, "task", "call-1", child))

	call := receive(t, h.mainRuns)
	require.NotNil(t, call.Report)
	require.Equal(t, "done"+filesChangedHeader+"edited.go"+otherChangesHeader+"bash.txt", call.Report.Output)
}

func TestDeliverReport_OtherChangesOmitMainAgentEdits(t *testing.T) {
	t.Parallel()
	h := newGitBGHarness(t)
	wd := h.coord.cfg.WorkingDir()

	child := bashLikeChild(wd, func(ctx context.Context, _ string) error {
		return recordFiles(ctx, h.coord.history, h.parentID, filepath.Join(wd, "parent.txt"))
	}, "parent.txt", "bash.txt")
	h.start(t, h.params(h.parentID, "task", "call-1", child))

	call := receive(t, h.mainRuns)
	require.NotNil(t, call.Report)
	require.Equal(t, "done"+otherChangesHeader+"bash.txt", call.Report.Output)
}

func TestDeliverReport_NoOtherChangesOutsideAGitRepository(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	wd := h.coord.cfg.WorkingDir()

	h.start(t, h.params(h.parentID, "task", "call-1", bashLikeChild(wd, nil, "bash.txt")))

	call := receive(t, h.mainRuns)
	require.NotNil(t, call.Report)
	require.Equal(t, "done", call.Report.Output)
}

func TestCancelAll_StoppedReportOmitsOtherChanges(t *testing.T) {
	t.Parallel()
	h := newGitBGHarness(t)
	wd := h.coord.cfg.WorkingDir()

	started := make(chan struct{}, 1)
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		if err := os.WriteFile(filepath.Join(wd, "bash.txt"), []byte("written by bash"), 0o644); err != nil {
			return nil, err
		}
		started <- struct{}{}
		return blockUntil(ctx, nil, "never released")
	})
	h.start(t, h.params(h.parentID, "task", "call-1", child))
	receive(t, started)

	h.coord.CancelAll()

	reports := listReports(t.Context(), h.coord.messages, h.parentID)
	require.Len(t, reports, 1)
	require.Equal(t, "Stopped: Crush exited", reports[0].Output)
}
