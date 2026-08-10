package herdr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingSender captures state transitions without connecting to a
// real Unix socket.
type recordingSender struct {
	states []string
}

func (r *recordingSender) send(req reportRequest) error {
	r.states = append(r.states, req.Params.State)
	return nil
}

func (r *recordingSender) close() {}

// newTestClient creates a Client that records state transitions
// without connecting to a real Unix socket.
func newTestClient() *Client {
	rec := &recordingSender{states: make([]string, 0, 16)}
	return &Client{
		state: stateIdle,
		snd:   rec,
	}
}

// reportedStates returns the states recorded by the test sender.
func reportedStates(c *Client) []string {
	return c.snd.(*recordingSender).states
}

func TestBasicLifecycle(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Assistant message starts working.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// Run complete returns to idle.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestPermissionBlockAndUnblock(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Start working.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})

	// Permission request blocks.
	c.HandleEvent(PermissionRequested{})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	// Permission granted returns to working (run still active).
	c.HandleEvent(PermissionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateWorking}, reportedStates(c))

	// Run complete returns to idle.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateWorking, stateIdle}, reportedStates(c))
}

func TestPermissionBeforeAssistantMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Permission request arrives before any assistant message.
	// This can happen when tool calls fire before text output.
	c.HandleEvent(PermissionRequested{})
	assert.Equal(t, []string{stateBlocked}, reportedStates(c))

	// Permission resolved should return to working, not idle,
	// because the permission request implied a run was active.
	c.HandleEvent(PermissionResolved{})
	assert.Equal(t, []string{stateBlocked, stateWorking}, reportedStates(c))
}

func TestSessionIDPropagation(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// SetSessionID before events.
	c.SetSessionID("early-session")
	assert.Equal(t, "early-session", c.sessionID)

	// Events for the current session drive the state.
	c.HandleEvent(AssistantMessage{SessionID: "early-session"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// A lifecycle event for a different session is ignored: it must
	// neither overwrite the authoritative session id nor change the
	// reported state.
	c.HandleEvent(RunComplete{SessionID: "other-session"})
	assert.Equal(t, "early-session", c.sessionID)
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// An event for the current session is accepted.
	c.HandleEvent(RunComplete{SessionID: "early-session"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestSessionIDLearnedFromFirstEvent(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// With no SetSessionID call, the first top-level lifecycle event
	// establishes the session.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	assert.Equal(t, "s1", c.sessionID)

	// Events for other sessions are then ignored.
	c.HandleEvent(AssistantMessage{SessionID: "s2"})
	c.HandleEvent(RunComplete{SessionID: "s2"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// SetSessionID re-points the client at the new session (session
	// switch); events for the old one are now the stale ones.
	c.SetSessionID("s2")
	c.HandleEvent(RunComplete{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
	c.HandleEvent(RunComplete{SessionID: "s2"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestSubSessionEventsIgnored(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Main session starts a run.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// A sub-agent (agent tool) run publishes its own lifecycle events
	// on the shared broker with ids of the form
	// "messageID$$toolCallID". Its messages must not corrupt the
	// reported session id.
	c.HandleEvent(AssistantMessage{SessionID: "msg-1$$tc-1"})
	assert.Equal(t, "sess-1", c.sessionID)

	// Its RunComplete must not drop the pane to idle while the main
	// run is still going.
	c.HandleEvent(RunComplete{SessionID: "msg-1$$tc-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// The main run's completion still lands on idle.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestSubSessionNeverEstablishesSessionID(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A sub-session event arriving before any SetSessionID must be
	// ignored rather than learned as the current session.
	c.HandleEvent(RunComplete{SessionID: "msg-1$$tc-1"})
	assert.Empty(t, c.sessionID)
	assert.Empty(t, reportedStates(c))
}

func TestSubSessionSummarizeEventsIgnored(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Auto-compaction inside a sub-agent session must not start a
	// working report the main session never clears.
	c.HandleEvent(SummarizeStarted{SessionID: "msg-1$$tc-1"})
	assert.Empty(t, reportedStates(c))

	// A summarize event for the main session is accepted, and its
	// finish returns the pane to idle.
	c.HandleEvent(SummarizeStarted{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
	c.HandleEvent(SummarizeFinished{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))

	// A sub-session finish arriving while idle must not drive any
	// state change either.
	c.HandleEvent(SummarizeFinished{SessionID: "msg-1$$tc-1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestDedupSkipsRedundantState(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Two assistant messages in a row should only report working once.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
}

func TestSummarizeStartFinishReturnsToIdle(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Regression: compaction never publishes a RunComplete, so the
	// summarize lifecycle must return the pane to idle by itself.
	c.HandleEvent(SummarizeStarted{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// Repeated starts are deduplicated.
	c.HandleEvent(SummarizeStarted{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// Finishing the compaction returns the pane to idle.
	c.HandleEvent(SummarizeFinished{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestSummarizeDuringRunStaysWorking(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Auto-compaction fires mid-turn (agent.go:1194): a run is
	// already active when the summarize starts.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(SummarizeStarted{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// Finishing the compaction must not drop the pane to idle while
	// the turn that triggered it is still running.
	c.HandleEvent(SummarizeFinished{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// The turn's completion finally lands on idle.
	c.HandleEvent(RunComplete{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestSummarizeFinishWithoutStartIsNoop(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A finish without a matching start (e.g. a stale summary
	// message deleted during session cleanup) must not report
	// anything: idle is already the current state.
	c.HandleEvent(SummarizeFinished{SessionID: "s1"})
	assert.Empty(t, reportedStates(c))
}

func TestBlockOutranksRunComplete(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A run starts and a permission prompt opens mid-turn.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(PermissionRequested{})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	// A run completion arriving while the block is still pending
	// (e.g. the turn was cancelled) must not drop the pane to idle:
	// the block wins until it is resolved.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	// With the run already finished, resolving the block lands on
	// idle rather than working.
	c.HandleEvent(PermissionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateIdle}, reportedStates(c))
}

func TestNilClientSafe(t *testing.T) {
	t.Parallel()
	var c *Client
	// These should not panic on a nil receiver.
	c.SetSessionID("s1")
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(RunComplete{SessionID: "s1"})
	c.HandleEvent(PermissionRequested{})
	c.HandleEvent(PermissionResolved{})
	c.HandleEvent(SummarizeStarted{SessionID: "s1"})
	c.HandleEvent(SummarizeFinished{SessionID: "s1"})
}

func TestRegisterInitial(t *testing.T) {
	t.Parallel()
	rec := &recordingSender{states: make([]string, 0, 16)}
	c := &Client{
		state: stateIdle,
		seq:   100,
		snd:   rec,
	}
	c.registerInitial()
	assert.Equal(t, []string{stateIdle}, rec.states)
	// seq must strictly increase so herdr accepts the report.
	assert.Equal(t, uint64(101), c.seq)
}

// TestInitDisabledUnderTest guards the critical safety property that
// herdr never attaches to a real pane from a test binary. Test
// processes inherit the developer's HERDR_* environment, so a missing
// guard would release the live pane's agent on teardown. Because this
// test itself runs under `go test`, Init must return nil even with a
// complete, valid-looking environment.
func TestInitDisabledUnderTest(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/does-not-matter.sock")
	t.Setenv("HERDR_PANE_ID", "test:pane")
	assert.Nil(t, newFromEnv())
}
