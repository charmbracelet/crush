package a2a

import (
	"context"
	"errors"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	a2aspec "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent"
)

// fakeRunner is a test double for the SessionAgent slice the Executor drives.
type fakeRunner struct {
	result *fantasy.AgentResult
	err    error

	gotCall     agent.SessionAgentCall
	ran         bool
	canceledFor string
}

func (f *fakeRunner) Run(_ context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	f.ran = true
	f.gotCall = call
	return f.result, f.err
}

func (f *fakeRunner) Cancel(sessionID string) { f.canceledFor = sessionID }

func textResult(s string) *fantasy.AgentResult {
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{fantasy.TextContent{Text: s}},
		},
	}
}

func newExecCtx(msg *a2aspec.Message) *a2asrv.ExecutorContext {
	return &a2asrv.ExecutorContext{
		Message:   msg,
		TaskID:    "task-1",
		ContextID: "ctx-1",
	}
}

func collect(t *testing.T, seq iter.Seq2[a2aspec.Event, error]) []a2aspec.Event {
	t.Helper()
	var evs []a2aspec.Event
	for ev, err := range seq {
		// The executor reports failures as events, never as the second
		// value of the sequence.
		require.NoError(t, err, "unexpected error from executor sequence")
		evs = append(evs, ev)
	}
	return evs
}

// artifactState is the sentinel states uses for artifact events so ordering
// can be asserted in one shot alongside status updates.
const artifactState a2aspec.TaskState = "<artifact>"

// states summarizes an event stream as a slice of task states.
func states(t *testing.T, evs []a2aspec.Event) []a2aspec.TaskState {
	t.Helper()
	out := make([]a2aspec.TaskState, 0, len(evs))
	for _, ev := range evs {
		switch e := ev.(type) {
		case *a2aspec.Task:
			out = append(out, e.Status.State)
		case *a2aspec.TaskStatusUpdateEvent:
			out = append(out, e.Status.State)
		case *a2aspec.TaskArtifactUpdateEvent:
			out = append(out, artifactState)
		default:
			t.Fatalf("unexpected event type %T", ev)
		}
	}
	return out
}

func statusUpdate(t *testing.T, ev a2aspec.Event) *a2aspec.TaskStatusUpdateEvent {
	t.Helper()
	sue, ok := ev.(*a2aspec.TaskStatusUpdateEvent)
	require.True(t, ok, "event is %T, want *TaskStatusUpdateEvent", ev)
	return sue
}

func statusMessageText(t *testing.T, ev a2aspec.Event) string {
	t.Helper()
	sue := statusUpdate(t, ev)
	if sue.Status.Message == nil {
		return ""
	}
	return partsText(sue.Status.Message.Parts)
}

func partsText(parts a2aspec.ContentParts) string {
	var s string
	for _, p := range parts {
		s += p.Text()
	}
	return s
}

func TestExecuteHappyPathWithDiff(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("all done")}
	exec := NewExecutor(runner, "sess-1", WithDiff(func(context.Context) (string, error) {
		return "the diff", nil
	}))

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("do the thing"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
		artifactState,
		a2aspec.TaskStateCompleted,
	}
	require.Equal(t, want, states(t, evs))

	require.True(t, runner.ran, "runner.Run was not called")
	require.Equal(t, "sess-1", runner.gotCall.SessionID)
	require.Equal(t, "do the thing", runner.gotCall.Prompt)

	// The artifact carries the diff and the task's identifiers.
	art, ok := evs[2].(*a2aspec.TaskArtifactUpdateEvent)
	require.True(t, ok, "event is %T, want *TaskArtifactUpdateEvent", evs[2])
	require.Equal(t, "the diff", partsText(art.Artifact.Parts))
	require.Equal(t, a2aspec.TaskID("task-1"), art.TaskID)
	require.Equal(t, "ctx-1", art.ContextID)

	// The terminal status carries the agent's text output, stamped with
	// the task's identifiers.
	terminal := statusUpdate(t, evs[3])
	require.Equal(t, a2aspec.TaskID("task-1"), terminal.TaskID)
	require.Equal(t, "ctx-1", terminal.ContextID)
	require.Equal(t, "all done", statusMessageText(t, evs[3]))
	require.NotNil(t, terminal.Status.Message)
	require.Equal(t, a2aspec.TaskID("task-1"), terminal.Status.Message.TaskID)
}

func TestExecuteNoDiffFunc(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("done")}
	exec := NewExecutor(runner, "sess-1")

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
		a2aspec.TaskStateCompleted,
	}
	require.Equal(t, want, states(t, evs), "no artifact expected")
}

func TestExecuteEmptyDiffEmitsNoArtifact(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("done")}
	exec := NewExecutor(runner, "sess-1", WithDiff(func(context.Context) (string, error) {
		return "", nil // clean worktree
	}))

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	for _, ev := range evs {
		_, isArtifact := ev.(*a2aspec.TaskArtifactUpdateEvent)
		require.False(t, isArtifact, "emitted an artifact event for an empty diff")
	}
}

func TestExecuteDiffErrorStillCompletes(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("done")}
	exec := NewExecutor(runner, "sess-1", WithDiff(func(context.Context) (string, error) {
		return "", errors.New("not a git repo")
	}))

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
		a2aspec.TaskStateCompleted,
	}
	require.Equal(t, want, states(t, evs), "diff error must not fail the run")
}

func TestExecuteRunFailure(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{err: errors.New("model exploded")}
	exec := NewExecutor(runner, "sess-1", WithDiff(func(context.Context) (string, error) {
		return "should not be called", nil
	}))

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
		a2aspec.TaskStateFailed,
	}
	require.Equal(t, want, states(t, evs))

	// The error is surfaced in the failed status message.
	require.Equal(t, "model exploded", statusMessageText(t, evs[2]))
}

func TestExecuteNilResultFails(t *testing.T) {
	t.Parallel()

	// SessionAgent.Run returns (nil, nil) without running anything when
	// the session is busy (prompt silently queued) or a cancel landed
	// during dispatch. The task must not report Completed.
	runner := &fakeRunner{result: nil, err: nil}
	exec := NewExecutor(runner, "sess-1", WithDiff(func(context.Context) (string, error) {
		return "should not become an artifact", nil
	}))

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
		a2aspec.TaskStateFailed,
	}
	require.Equal(t, want, states(t, evs), "a turn that never ran must not complete")
	require.Contains(t, statusMessageText(t, evs[2]), "did not start a turn")
}

func TestExecuteCanceledRunEmitsNoTerminalStatus(t *testing.T) {
	t.Parallel()

	// A canceled run is reported by Cancel's own Canceled status; Execute
	// must not race it with a Failed status.
	runner := &fakeRunner{err: context.Canceled}
	exec := NewExecutor(runner, "sess-1")

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateWorking,
	}
	require.Equal(t, want, states(t, evs), "no terminal status for a canceled run")
}

func TestExecuteEmptyPromptRejects(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("never")}
	exec := NewExecutor(runner, "sess-1")

	// A message with no text parts carries nothing to run.
	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewDataPart(map[string]any{"k": "v"}))
	evs := collect(t, exec.Execute(context.Background(), newExecCtx(msg)))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateSubmitted,
		a2aspec.TaskStateRejected,
	}
	require.Equal(t, want, states(t, evs))
	require.False(t, runner.ran, "runner.Run must not be called for an empty prompt")
}

func TestExecuteConsumerStopsBeforeRun(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("never")}
	exec := NewExecutor(runner, "sess-1")

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	// Stop consuming after the first (Submitted) event, as a disconnecting
	// consumer would.
	for range exec.Execute(context.Background(), newExecCtx(msg)) {
		break
	}

	require.False(t, runner.ran, "runner.Run must not be called after the consumer stops")
}

func TestExecuteExistingTaskSkipsSubmitted(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: textResult("done")}
	exec := NewExecutor(runner, "sess-1")

	msg := a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("go"))
	execCtx := newExecCtx(msg)
	execCtx.StoredTask = &a2aspec.Task{ID: "task-1", ContextID: "ctx-1"}

	evs := collect(t, exec.Execute(context.Background(), execCtx))

	want := []a2aspec.TaskState{
		a2aspec.TaskStateWorking,
		a2aspec.TaskStateCompleted,
	}
	require.Equal(t, want, states(t, evs), "no submitted for an existing task")
}

func TestCancel(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	exec := NewExecutor(runner, "sess-1")

	evs := collect(t, exec.Cancel(context.Background(), newExecCtx(nil)))

	require.Equal(t, "sess-1", runner.canceledFor)
	require.Equal(t, []a2aspec.TaskState{a2aspec.TaskStateCanceled}, states(t, evs))
}

func TestMessageText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  *a2aspec.Message
		want string
	}{
		{"nil message", nil, ""},
		{
			"single part",
			a2aspec.NewMessage(a2aspec.MessageRoleUser, a2aspec.NewTextPart("hello")),
			"hello",
		},
		{
			"multiple parts joined by newline",
			a2aspec.NewMessage(a2aspec.MessageRoleUser,
				a2aspec.NewTextPart("line one"),
				a2aspec.NewTextPart("line two"),
			),
			"line one\nline two",
		},
		{
			// A nil parts entry is constructible from a remote peer's
			// JSON ([null, ...]) and must be skipped, not dereferenced.
			"nil part entry skipped",
			&a2aspec.Message{Parts: a2aspec.ContentParts{nil, a2aspec.NewTextPart("x")}},
			"x",
		},
		{
			// Non-text parts are ignored without leaving stray
			// separators between the surviving text parts.
			"non-text parts ignored",
			a2aspec.NewMessage(a2aspec.MessageRoleUser,
				a2aspec.NewTextPart("a"),
				a2aspec.NewDataPart(map[string]any{"k": "v"}),
				a2aspec.NewRawPart([]byte("zz")),
				a2aspec.NewTextPart("b"),
			),
			"a\nb",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, messageText(tc.msg))
		})
	}
}

// initTestRepo creates a git repo in a temp dir with one committed file and
// returns the dir. Skips the test when git is unavailable.
func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", append([]string{
			"-C", dir,
			"-c", "user.name=test",
			"-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false",
		}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original\n"), 0o644))
	run("init", "-q")
	run("add", "tracked.txt")
	run("commit", "-q", "-m", "initial")
	return dir
}

func TestGitDiff(t *testing.T) {
	t.Parallel()

	dir := initTestRepo(t)

	// Unstaged change to a tracked file, plus a brand-new untracked file:
	// both are the dispatched agent's work product and must appear.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "created.txt"), []byte("new file\n"), 0o644))

	diff, err := GitDiff(dir)(t.Context())
	require.NoError(t, err)
	require.Contains(t, diff, "tracked.txt")
	require.Contains(t, diff, "modified")
	require.Contains(t, diff, "created.txt")
	require.Contains(t, diff, "new file")

	// The real index must be untouched: the new file stays untracked and
	// nothing is staged.
	out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "status", "--porcelain").Output()
	require.NoError(t, err)
	require.Contains(t, string(out), "?? created.txt")
	staged, err := exec.CommandContext(t.Context(), "git", "-C", dir, "diff", "--cached", "--name-only").Output()
	require.NoError(t, err)
	require.Empty(t, string(staged), "GitDiff must not stage anything in the real index")
}

func TestGitDiffCleanWorktree(t *testing.T) {
	t.Parallel()

	dir := initTestRepo(t)
	diff, err := GitDiff(dir)(t.Context())
	require.NoError(t, err)
	require.Empty(t, diff)
}

func TestGitDiffNotARepo(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	diff, err := GitDiff(t.TempDir())(t.Context())
	require.Error(t, err)
	require.Empty(t, diff)
}
