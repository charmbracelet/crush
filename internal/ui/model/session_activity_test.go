package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/workspace"
)

func TestSessionActivityTransitions(t *testing.T) {
	t.Parallel()

	const id, other = "s1", "s2"
	probe := func(busy, pending bool) map[string]workspace.SessionStatus {
		return map[string]workspace.SessionStatus{id: {Known: true, Busy: busy, PendingPrompt: pending}}
	}
	// Each case first drives the tracker to `from`, then applies `act`.
	setups := map[sessionActivity]func(*sessionActivityTracker){
		activityIdle:         func(*sessionActivityTracker) {},
		activityWorking:      func(tr *sessionActivityTracker) { tr.Working(id) },
		activityWaiting:      func(tr *sessionActivityTracker) { tr.Waiting(id) },
		activityDoneUnseen:   func(tr *sessionActivityTracker) { tr.Finished(id, false, other) },
		activityFailedUnseen: func(tr *sessionActivityTracker) { tr.Finished(id, true, other) },
	}

	tests := []struct {
		name string
		from sessionActivity
		act  func(*sessionActivityTracker)
		want sessionActivity
	}{
		{"idle working", activityIdle, func(tr *sessionActivityTracker) { tr.Working(id) }, activityWorking},
		{"working finished elsewhere", activityWorking, func(tr *sessionActivityTracker) { tr.Finished(id, false, other) }, activityDoneUnseen},
		{"working finished while viewing", activityWorking, func(tr *sessionActivityTracker) { tr.Finished(id, false, id) }, activityIdle},
		{"working failed elsewhere", activityWorking, func(tr *sessionActivityTracker) { tr.Finished(id, true, other) }, activityFailedUnseen},
		{"working failed while viewing", activityWorking, func(tr *sessionActivityTracker) { tr.Finished(id, true, id) }, activityIdle},
		{"doneUnseen viewed", activityDoneUnseen, func(tr *sessionActivityTracker) { tr.Viewed(id) }, activityIdle},
		{"failedUnseen viewed", activityFailedUnseen, func(tr *sessionActivityTracker) { tr.Viewed(id) }, activityIdle},
		{"working viewed stays working", activityWorking, func(tr *sessionActivityTracker) { tr.Viewed(id) }, activityWorking},
		{"working waiting", activityWorking, func(tr *sessionActivityTracker) { tr.Waiting(id) }, activityWaiting},
		{"waiting working", activityWaiting, func(tr *sessionActivityTracker) { tr.Working(id) }, activityWorking},
		{"waiting reconcile idle", activityWaiting, func(tr *sessionActivityTracker) { tr.Reconcile(probe(false, false)) }, activityIdle},
		{"working reconcile idle", activityWorking, func(tr *sessionActivityTracker) { tr.Reconcile(probe(false, false)) }, activityIdle},
		{"idle reconcile busy", activityIdle, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, false)) }, activityWorking},
		{"working reconcile pending", activityWorking, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, true)) }, activityWaiting},
		{"idle reconcile pending", activityIdle, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, true)) }, activityWaiting},
		{"doneUnseen reconcile pending", activityDoneUnseen, func(tr *sessionActivityTracker) { tr.Reconcile(probe(false, true)) }, activityWaiting},
		{"failedUnseen reconcile pending", activityFailedUnseen, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, true)) }, activityWaiting},
		{"waiting reconcile busy no prompt", activityWaiting, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, false)) }, activityWorking},
		{"doneUnseen reconcile idle", activityDoneUnseen, func(tr *sessionActivityTracker) { tr.Reconcile(probe(false, false)) }, activityDoneUnseen},
		{"failedUnseen reconcile idle", activityFailedUnseen, func(tr *sessionActivityTracker) { tr.Reconcile(probe(false, false)) }, activityFailedUnseen},
		{"doneUnseen reconcile busy", activityDoneUnseen, func(tr *sessionActivityTracker) { tr.Reconcile(probe(true, false)) }, activityWorking},
		{"forget working", activityWorking, func(tr *sessionActivityTracker) { tr.Forget(id) }, activityIdle},
		{"forget doneUnseen", activityDoneUnseen, func(tr *sessionActivityTracker) { tr.Forget(id) }, activityIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var tr sessionActivityTracker // Zero value must be usable.
			setups[tt.from](&tr)
			require.Equal(t, tt.from, tr.Get(id), "setup")
			tt.act(&tr)
			require.Equal(t, tt.want, tr.Get(id))
		})
	}
}

func TestSessionActivityReconcileOnlyTouchesProbed(t *testing.T) {
	t.Parallel()

	var tr sessionActivityTracker
	tr.Working("a")
	tr.Waiting("b")
	tr.Reconcile(map[string]workspace.SessionStatus{"a": {Known: true, Busy: false}})
	require.Equal(t, activityIdle, tr.Get("a"))
	require.Equal(t, activityWaiting, tr.Get("b"))
	require.Equal(t, activityIdle, tr.Get("never-seen"))
}

// TestSessionActivityReconcileIgnoresUnknownProbe: a failed probe (timeout,
// server error) is the zero status. It must neither clear ● or ▲ nor
// promote anything; a successful idle probe still clears.
func TestSessionActivityReconcileIgnoresUnknownProbe(t *testing.T) {
	t.Parallel()

	var tr sessionActivityTracker
	tr.Working("a")
	tr.Waiting("b")
	v := tr.version
	tr.Reconcile(map[string]workspace.SessionStatus{
		"a": {},
		"b": {},
		"c": {Busy: true, PendingPrompt: true}, // Unknown: must not promote.
	})
	require.Equal(t, activityWorking, tr.Get("a"))
	require.Equal(t, activityWaiting, tr.Get("b"))
	require.Equal(t, activityIdle, tr.Get("c"))
	require.Equal(t, v, tr.version)

	tr.Reconcile(map[string]workspace.SessionStatus{"a": {Known: true, Busy: false}})
	require.Equal(t, activityIdle, tr.Get("a"))
}

func TestSessionActivityRetainAndVersion(t *testing.T) {
	t.Parallel()

	var tr sessionActivityTracker
	tr.Working("a")
	tr.Finished("b", true, "")
	v := tr.version
	tr.Working("a") // No change.
	tr.Retain(map[string]struct{}{"a": {}, "b": {}})
	require.Equal(t, v, tr.version, "no-op calls must not look like a change")

	tr.Retain(map[string]struct{}{"a": {}})
	require.Equal(t, activityWorking, tr.Get("a"))
	require.Equal(t, activityIdle, tr.Get("b"), "a session missing from ids is forgotten")
	require.Greater(t, tr.version, v)
}
