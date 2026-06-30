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

	// RunComplete also updates session ID.
	c.HandleEvent(RunComplete{SessionID: "final-session"})
	assert.Equal(t, "final-session", c.sessionID)
}

func TestDedupSkipsRedundantState(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Two assistant messages in a row should only report working once.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
}

func TestSummarizingTriggersWorking(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Summarizing event should trigger working.
	c.HandleEvent(Summarizing{})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// Second summarizing should not trigger another state change.
	c.HandleEvent(Summarizing{})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
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
	c.HandleEvent(Summarizing{})
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
