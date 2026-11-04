package agent

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/stretchr/testify/require"
)

const filesChangedHeader = "\n\nFiles changed by this subagent so far:\n- "

// failingHistory is a history service whose file listing always fails.
type failingHistory struct {
	history.Service
}

func (failingHistory) ListLatestSessionFiles(context.Context, string) ([]history.File, error) {
	return nil, errors.New("boom")
}

// recordFiles stores one history row per path under sessionID, like the edit
// and write tools do.
func recordFiles(ctx context.Context, svc history.Service, sessionID string, paths ...string) error {
	for _, p := range paths {
		if _, err := svc.Create(ctx, sessionID, p, "content"); err != nil {
			return err
		}
	}
	return nil
}

func TestChangedFilesNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T, coord *coordinator, sessionID, wd string)
		want  func(wd string) string
	}{
		{
			name:  "no files",
			setup: func(*testing.T, *coordinator, string, string) {},
			want:  func(string) string { return "" },
		},
		{
			name: "one file inside the working directory",
			setup: func(t *testing.T, coord *coordinator, sessionID, wd string) {
				require.NoError(t, recordFiles(t.Context(), coord.history, sessionID, filepath.Join(wd, "a.go")))
			},
			want: func(string) string { return filesChangedHeader + "a.go" },
		},
		{
			name: "several files are sorted and de-duplicated",
			setup: func(t *testing.T, coord *coordinator, sessionID, wd string) {
				require.NoError(t, recordFiles(t.Context(), coord.history, sessionID,
					filepath.Join(wd, "z.go"), filepath.Join(wd, "a.go"), filepath.Join(wd, "m.go")))
				_, err := coord.history.CreateVersion(t.Context(), sessionID, filepath.Join(wd, "a.go"), "newer")
				require.NoError(t, err)
			},
			want: func(string) string { return filesChangedHeader + "a.go\n- m.go\n- z.go" },
		},
		{
			name: "path outside the working directory stays absolute",
			setup: func(t *testing.T, coord *coordinator, sessionID, _ string) {
				require.NoError(t, recordFiles(t.Context(), coord.history, sessionID, "/elsewhere/outside.txt"))
			},
			want: func(string) string { return filesChangedHeader + "/elsewhere/outside.txt" },
		},
		{
			name: "nil history",
			setup: func(_ *testing.T, coord *coordinator, _, _ string) {
				coord.history = nil
			},
			want: func(string) string { return "" },
		},
		{
			name: "failing history",
			setup: func(_ *testing.T, coord *coordinator, _, _ string) {
				coord.history = failingHistory{Service: coord.history}
			},
			want: func(string) string { return "" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			coord := newGateTestCoordinator(t, false)
			// The agent build reads coord.history, so let it finish before
			// the test replaces the field.
			require.NoError(t, coord.readyWg.Wait())
			sess, err := coord.sessions.Create(t.Context(), "Child")
			require.NoError(t, err)
			wd := coord.cfg.WorkingDir()

			tt.setup(t, coord, sess.ID, wd)

			require.Equal(t, tt.want(wd), coord.changedFilesNote(t.Context(), sess.ID))
		})
	}
}

func TestDeliverReport_CompletedReportListsChangedFiles(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	wd := h.coord.cfg.WorkingDir()

	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		err := recordFiles(ctx, h.coord.history, call.SessionID, filepath.Join(wd, "b.go"), filepath.Join(wd, "a.go"))
		if err != nil {
			return nil, err
		}
		return agentResultWithText("all good"), nil
	})
	h.start(t, h.params(h.parentID, "task", "call-1", child))

	call := receive(t, h.mainRuns)
	require.NotNil(t, call.Report)
	require.Equal(t, "all good"+filesChangedHeader+"a.go\n- b.go", call.Report.Output)
}

func TestDeliverReport_CompletedReportWithoutChangesIsUnchanged(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	h.start(t, h.params(h.parentID, "task", "call-1", instantChild("all good")))

	call := receive(t, h.mainRuns)
	require.NotNil(t, call.Report)
	require.Equal(t, "all good", call.Report.Output)
}

func TestDeliverReport_CancelledReportListsChangedFiles(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	wd := h.coord.cfg.WorkingDir()

	started := make(chan struct{}, 1)
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		if err := recordFiles(ctx, h.coord.history, call.SessionID, filepath.Join(wd, "a.go")); err != nil {
			return nil, err
		}
		started <- struct{}{}
		return blockUntil(ctx, nil, "never released")
	})
	h.start(t, h.params(h.parentID, "task", "call-1", child))
	receive(t, started)

	h.coord.stopBackground(h.parentID, "task-1")

	reports := h.awaitReports(t, h.parentID, 1)
	require.Equal(t, "Subagent cancelled by user"+filesChangedHeader+"a.go", reports[0].Output)
}

func TestCancelAll_StoppedReportOmitsChangedFiles(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	wd := h.coord.cfg.WorkingDir()

	started := make(chan struct{}, 1)
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		if err := recordFiles(ctx, h.coord.history, call.SessionID, filepath.Join(wd, "a.go")); err != nil {
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
