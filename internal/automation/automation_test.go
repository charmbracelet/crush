package automation

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"2 hours", 2 * time.Hour},
		{"1d", 24 * time.Hour},
		{"every 15 minutes", 15 * time.Minute},
	}
	for _, tt := range tests {
		got, err := ParseInterval(tt.in)
		require.NoError(t, err, tt.in)
		require.Equal(t, tt.want, got, tt.in)
	}

	for _, in := range []string{"", "0m", "-5m", "5", "5w"} {
		_, err := ParseInterval(in)
		require.Error(t, err, in)
	}
}

func TestNormalizeInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   time.Duration
		want time.Duration
	}{
		{30 * time.Second, time.Minute},
		{90 * time.Second, 2 * time.Minute},
		{8 * time.Minute, 10 * time.Minute},
		{7 * time.Minute, 6 * time.Minute},
		{31 * time.Minute, 30 * time.Minute},
		{45 * time.Minute, time.Hour},
		{5 * time.Hour, 6 * time.Hour},
		{24 * time.Hour, 24 * time.Hour},
		{36 * time.Hour, 48 * time.Hour},
		{10 * 24 * time.Hour, 10 * 24 * time.Hour},
		{32 * 24 * time.Hour, 24 * time.Hour},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, NormalizeInterval(tt.in), tt.in.String())
	}
}

func TestNextFixedAlignsAndJitters(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 3, 0, 0, time.UTC)
	task := Task{ID: "abcd1234", Interval: 10 * time.Minute}
	next := NextFixed(now, task)
	want := time.Date(2026, 1, 1, 12, 10, 0, 0, time.UTC).Add(Jitter(task.ID, task.Interval))
	require.Equal(t, want, next)
	require.True(t, next.After(now))

	daily := Task{ID: "abcd1234", Interval: 24 * time.Hour}
	next = NextFixed(now, daily)
	want = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Add(Jitter(daily.ID, daily.Interval))
	require.Equal(t, want, next)
}

func TestJitterIsDeterministicAndBounded(t *testing.T) {
	t.Parallel()

	a := Jitter("task-1", 10*time.Minute)
	b := Jitter("task-1", 10*time.Minute)
	require.Equal(t, a, b)
	require.GreaterOrEqual(t, a, time.Duration(0))
	require.Less(t, a, time.Minute)

	capped := Jitter("task-1", 24*time.Hour)
	require.Less(t, capped, 15*time.Minute)
}

func TestParseLoopRequest(t *testing.T) {
	t.Parallel()

	const def = "default task"

	req, err := ParseLoopRequest("", def)
	require.NoError(t, err)
	require.Equal(t, def, req.Task)
	require.False(t, req.Fixed)
	require.True(t, req.Default)

	req, err = ParseLoopRequest("5m check the deploy", def)
	require.NoError(t, err)
	require.Equal(t, "check the deploy", req.Task)
	require.Equal(t, 5*time.Minute, req.Interval)
	require.True(t, req.Fixed)

	req, err = ParseLoopRequest("check the deploy every 2 hours", def)
	require.NoError(t, err)
	require.Equal(t, "check the deploy", req.Task)
	require.Equal(t, 2*time.Hour, req.Interval)
	require.True(t, req.Fixed)

	req, err = ParseLoopRequest("15m", def)
	require.NoError(t, err)
	require.Equal(t, def, req.Task)
	require.Equal(t, 15*time.Minute, req.Interval)
	require.True(t, req.Fixed)

	req, err = ParseLoopRequest("check the deploy", def)
	require.NoError(t, err)
	require.Equal(t, "check the deploy", req.Task)
	require.False(t, req.Fixed)
}

func TestParseLoopMarkers(t *testing.T) {
	t.Parallel()

	delay, reason, ok := ParseLoopWakeup(`<loop_wakeup delay="5m">still rolling out</loop_wakeup>`)
	require.True(t, ok)
	require.Equal(t, 5*time.Minute, delay)
	require.Equal(t, "still rolling out", reason)

	delay, _, ok = ParseLoopWakeup(`<loop_wakeup delay="4h" reason="quiet"/>`)
	require.True(t, ok)
	require.Equal(t, MaxSelfPacedDelay, delay)

	_, _, ok = ParseLoopWakeup("no marker here")
	require.False(t, ok)

	report, ok := ParseLoopDone(`<loop_done>deploy is healthy</loop_done>`)
	require.True(t, ok)
	require.Equal(t, "deploy is healthy", report)

	report, ok = ParseLoopDone(`<loop_done>no closing tag`)
	require.True(t, ok)
	require.Equal(t, "no closing tag", report)
}

func TestParseGoalComplete(t *testing.T) {
	t.Parallel()

	evidence, ok := ParseGoalComplete("done\n<goal_complete>tests pass; 3 files changed</goal_complete>")
	require.True(t, ok)
	require.Equal(t, "tests pass; 3 files changed", evidence)

	evidence, ok = ParseGoalComplete("<goal_complete>no closing tag")
	require.True(t, ok)
	require.Equal(t, "no closing tag", evidence)

	_, ok = ParseGoalComplete("<goal_complete></goal_complete>")
	require.False(t, ok)

	_, ok = ParseGoalComplete("still working")
	require.False(t, ok)
}

func TestParseVerdict(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("```json\n{\"verified\": true, \"reason\": \"tests pass\"}\n```")
	require.True(t, v.Verified)
	require.Equal(t, "tests pass", v.Reason)

	v = ParseVerdict("{\"verified\": false, \"reason\": \"no test output\"}")
	require.False(t, v.Verified)
	require.Equal(t, "no test output", v.Reason)

	v = ParseVerdict("not json")
	require.False(t, v.Verified)
	require.Contains(t, v.Reason, "not json")
}

func TestGoalBudget(t *testing.T) {
	t.Parallel()

	now := time.Now()
	g := NewGoal("ship it", 2, now)
	require.True(t, g.Active())
	require.True(t, g.AddTurns(1))
	require.False(t, g.AddTurns(1))
	require.Equal(t, 2, g.Turn)

	g.Pause("out of turns", now)
	require.Equal(t, GoalPaused, g.Status)
	require.Contains(t, g.StatusLine(now), "out of turns")
}

func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir)
	now := time.Now().UTC().Truncate(time.Second)
	state := State{
		Goal:  NewGoal("green tests", DefaultGoalBudget, now),
		Loops: []Task{NewTask("abcd1234", "check deploy", LoopFixed, 5*time.Minute, now)},
	}
	require.NoError(t, store.Save("session-1", state))

	got, err := store.Load("session-1")
	require.NoError(t, err)
	require.Equal(t, state.Goal.Objective, got.Goal.Objective)
	require.Len(t, got.Loops, 1)
	require.Equal(t, "check deploy", got.Loops[0].Task)

	require.NoError(t, store.Delete("session-1"))
	got, err = store.Load("session-1")
	require.NoError(t, err)
	require.Nil(t, got.Goal)
	require.Empty(t, got.Loops)
}

func TestDefaultTaskPrefersProjectFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".crush"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".crush", "loop.md"), []byte("project task"), 0o644))
	global := filepath.Join(dir, "global.md")
	require.NoError(t, os.WriteFile(global, []byte("global task"), 0o644))

	require.Equal(t, "project task", DefaultTask(dir, global))

	require.NoError(t, os.Remove(filepath.Join(dir, ".crush", "loop.md")))
	require.Equal(t, "global task", DefaultTask(dir, global))

	require.NoError(t, os.WriteFile(global, []byte("  \n"), 0o644))
	require.Equal(t, defaultLoopTask, DefaultTask(dir, global))
}
