package herdr

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingSender captures state transitions without connecting to a
// real Unix socket.
type recordingSender struct {
	mu       sync.Mutex
	requests []reportRequest
}

func (r *recordingSender) send(req reportRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	return nil
}

func (r *recordingSender) close() {}

// newTestClient creates a Client that records state transitions
// without connecting to a real Unix socket.
func newTestClient() *Client {
	rec := &recordingSender{requests: make([]reportRequest, 0, 16)}
	return &Client{
		state: stateIdle,
		snd:   rec,
	}
}

// reportedStates returns the states recorded by the test sender.
// Metadata requests carry no state and are skipped.
func reportedStates(c *Client) []string {
	rec := c.snd.(*recordingSender)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	states := make([]string, 0, len(rec.requests))
	for _, req := range rec.requests {
		if p, ok := req.Params.(reportParams); ok {
			states = append(states, p.State)
		}
	}
	return states
}

// reportedMessages returns the messages recorded by the test sender.
// Metadata requests carry no message and are skipped.
func reportedMessages(c *Client) []string {
	rec := c.snd.(*recordingSender)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	messages := make([]string, 0, len(rec.requests))
	for _, req := range rec.requests {
		if p, ok := req.Params.(reportParams); ok {
			messages = append(messages, p.Message)
		}
	}
	return messages
}

// reportedMetadata returns the metadata params recorded by the test
// sender.
func reportedMetadata(c *Client) []metadataParams {
	rec := c.snd.(*recordingSender)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var out []metadataParams
	for _, req := range rec.requests {
		if p, ok := req.Params.(metadataParams); ok {
			out = append(out, p)
		}
	}
	return out
}

// lastRequest returns the most recent request recorded by the test
// sender.
func lastRequest(c *Client) reportRequest {
	rec := c.snd.(*recordingSender)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.requests[len(rec.requests)-1]
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

func TestRunStartedReportsWorkingImmediately(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// The prompt submission itself flips the pane to working; no
	// assistant output is required first.
	c.HandleEvent(RunStarted{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// The first assistant message changes nothing (deduped), and
	// the run's completion returns to idle.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
}

func TestRunStartedSessionGating(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A sub-agent's prompt (agent-tool sub-session) must not drive
	// the pane nor establish the session id.
	c.HandleEvent(RunStarted{SessionID: "msg-1$$tc-1"})
	assert.Empty(t, c.sessionID)
	assert.Empty(t, reportedStates(c))

	// The main session's prompt is accepted and learned.
	c.HandleEvent(RunStarted{SessionID: "sess-1"})
	assert.Equal(t, "sess-1", c.sessionID)
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// A prompt for a different top-level session is stale and
	// ignored.
	c.HandleEvent(RunStarted{SessionID: "other-session"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
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

	// SetSession before events.
	c.SetSession("early-session", "Early")
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

	// With no SetSession call, the first top-level lifecycle event
	// establishes the session.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	assert.Equal(t, "s1", c.sessionID)

	// Events for other sessions are then ignored.
	c.HandleEvent(AssistantMessage{SessionID: "s2"})
	c.HandleEvent(RunComplete{SessionID: "s2"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))

	// SetSession re-points the client at the new session (session
	// switch); events for the old one are now the stale ones.
	c.SetSession("s2", "S2")
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

	// A sub-session event arriving before any SetSession must be
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

func TestQuestionBlockAndUnblock(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Start working.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})

	// A question blocks, carrying its text as the blocked message.
	c.HandleEvent(QuestionAsked{Text: "Which file should I edit?"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))
	assert.Equal(t, "Which file should I edit?", c.message)

	// Answering the question returns to working (run still active).
	c.HandleEvent(QuestionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateWorking}, reportedStates(c))
	assert.Empty(t, c.message)

	// Run complete returns to idle.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateWorking, stateIdle}, reportedStates(c))
}

func TestQuestionBeforeAssistantMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A question can only be asked mid-turn, so it implies an active
	// run even when no assistant message has arrived yet.
	c.HandleEvent(QuestionAsked{Text: "Proceed?"})
	assert.Equal(t, []string{stateBlocked}, reportedStates(c))

	// Resolving the question returns to working, not idle.
	c.HandleEvent(QuestionResolved{})
	assert.Equal(t, []string{stateBlocked, stateWorking}, reportedStates(c))
}

func TestOverlappingPermissionAndQuestionBlocks(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A permission prompt opens, then a question arrives while the
	// permission is still pending.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(PermissionRequested{})
	c.HandleEvent(QuestionAsked{Text: "Pick an option"})
	// The state stays blocked but the message changed from the
	// permission's empty one to the question's text, and dedup is
	// on the (state, message) pair: a second blocked report goes
	// out.
	assert.Equal(t, []string{stateWorking, stateBlocked, stateBlocked}, reportedStates(c))
	// The newest block's message wins.
	assert.Equal(t, "Pick an option", c.message)

	// Resolving only the permission leaves the pane blocked, with
	// the question's message unchanged, so nothing new is reported.
	c.HandleEvent(PermissionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateBlocked}, reportedStates(c))
	assert.Equal(t, "Pick an option", c.message)

	// Resolving the question finally unblocks.
	c.HandleEvent(QuestionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateBlocked, stateWorking}, reportedStates(c))
}

func TestQuestionBlockOutranksRunComplete(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A run starts and a question opens mid-turn.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(QuestionAsked{Text: "Proceed?"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	// A run completion arriving while the question is still pending
	// (e.g. the turn errored out) must not drop the pane to idle:
	// the block wins until it is resolved.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	// With the run already finished, resolving the question lands
	// on idle rather than working.
	c.HandleEvent(QuestionResolved{})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateIdle}, reportedStates(c))
}

func TestAuthBlockAndUnblock(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A run starts and hits an auth error mid-turn (e.g. a revoked
	// OAuth refresh token): the pane blocks on re-authentication.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(AuthRequired{ProviderID: "hyper"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))
	assert.Equal(t, "Re-authentication required: hyper", lastRequest(c).Params.(reportParams).Message)

	// The run that needed auth completes — one way or another, the
	// auth wait is over. Re-auth success publishes no event, so the
	// completion is what clears the block.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateIdle}, reportedStates(c))
	assert.Empty(t, lastRequest(c).Params.(reportParams).Message)
}

func TestAuthBlockWithoutProvider(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Client/server mode carries no provider id on the wire, so the
	// message falls back to the generic text.
	c.HandleEvent(AuthRequired{})
	assert.Equal(t, []string{stateBlocked}, reportedStates(c))
	assert.Equal(t, "Re-authentication required", lastRequest(c).Params.(reportParams).Message)
}

func TestAuthBlockBeforeAssistantMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// The 401 can arrive before any assistant output. The block
	// still reports, and the completing run lands on idle.
	c.HandleEvent(AuthRequired{ProviderID: "hyper"})
	assert.Equal(t, []string{stateBlocked}, reportedStates(c))

	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateBlocked, stateIdle}, reportedStates(c))
}

func TestAuthBlockOverlapsPermissionBlock(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A permission prompt and an auth prompt can both be open; the
	// newest block's message wins.
	c.HandleEvent(PermissionRequested{})
	c.HandleEvent(AuthRequired{ProviderID: "hyper"})
	assert.Equal(t, "Re-authentication required: hyper", lastRequest(c).Params.(reportParams).Message)

	// Resolving the permission keeps the pane blocked on auth. The
	// (state, message) pair is unchanged, so nothing new is sent.
	c.HandleEvent(PermissionResolved{})
	assert.Equal(t, []string{stateBlocked, stateBlocked}, reportedStates(c))

	// Run completion clears the auth block and returns to idle.
	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateBlocked, stateBlocked, stateIdle}, reportedStates(c))
}

func TestSubSessionRunCompleteKeepsAuthBlock(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(AuthRequired{ProviderID: "hyper"})

	// A sub-agent's run completion is session-filtered, so it must
	// not clear the top-level auth block.
	c.HandleEvent(RunComplete{SessionID: "msg-1$$call-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))

	c.HandleEvent(RunComplete{SessionID: "sess-1"})
	assert.Equal(t, []string{stateWorking, stateBlocked, stateIdle}, reportedStates(c))
}

func TestDedupSkipsRedundantState(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Two assistant messages in a row should only report working once.
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	assert.Equal(t, []string{stateWorking}, reportedStates(c))
}

func TestDedupReportsChangedBlockMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A repeated block with a different message must still reach
	// herdr: dedup is on the (state, message) pair, not state alone.
	c.HandleEvent(QuestionAsked{Text: "First question"})
	c.HandleEvent(QuestionAsked{Text: "Second question"})
	assert.Equal(t, []string{stateBlocked, stateBlocked}, reportedStates(c))
	assert.Equal(t, "Second question", c.message)

	// An identical repeat is deduplicated.
	c.HandleEvent(QuestionAsked{Text: "Second question"})
	assert.Equal(t, []string{stateBlocked, stateBlocked}, reportedStates(c))
}

func TestReportCarriesBlockMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Every report carries the current block message on the wire so
	// herdr can show what the agent is waiting on.
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	c.HandleEvent(QuestionAsked{Text: "Which file should I edit?"})
	assert.Equal(t,
		[]string{"", "Which file should I edit?"},
		reportedMessages(c))
	req := lastRequest(c)
	assert.Equal(t, stateBlocked, req.Params.(reportParams).State)
	assert.Equal(t, "Which file should I edit?", req.Params.(reportParams).Message)

	// Resolving the block clears the message on the very next
	// report; herdr must not keep showing the stale question.
	c.HandleEvent(QuestionResolved{})
	req = lastRequest(c)
	assert.Equal(t, stateWorking, req.Params.(reportParams).State)
	assert.Empty(t, req.Params.(reportParams).Message)
}

func TestPermissionBlockSendsEmptyMessage(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A question leaves its text as the block message; a permission
	// prompt then takes over with an empty one. The empty message
	// must go out on the wire, not be dropped, so herdr clears the
	// stale question text while the state stays blocked.
	c.HandleEvent(QuestionAsked{Text: "Pick an option"})
	c.HandleEvent(PermissionRequested{})
	assert.Equal(t,
		[]string{"Pick an option", ""},
		reportedMessages(c))
	req := lastRequest(c)
	assert.Equal(t, stateBlocked, req.Params.(reportParams).State)
	assert.Empty(t, req.Params.(reportParams).Message)
}

func TestReportRequestAlwaysIncludesMessageKey(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// The message key must be present in the marshaled JSON even
	// when empty: each report is a complete (state, message)
	// snapshot, so herdr never has to guess whether a missing key
	// means "clear" or "keep the last value".
	c.HandleEvent(AssistantMessage{SessionID: "sess-1"})
	raw, err := json.Marshal(lastRequest(c))
	assert.NoError(t, err)
	assert.Contains(t, string(raw), `"message":""`)

	c.HandleEvent(QuestionAsked{Text: "Proceed?"})
	raw, err = json.Marshal(lastRequest(c))
	assert.NoError(t, err)
	assert.Contains(t, string(raw), `"message":"Proceed?"`)
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
	c.SetSession("s1", "S1")
	c.HandleEvent(AssistantMessage{SessionID: "s1"})
	c.HandleEvent(RunComplete{SessionID: "s1"})
	c.HandleEvent(PermissionRequested{})
	c.HandleEvent(PermissionResolved{})
	c.HandleEvent(QuestionAsked{Text: "Proceed?"})
	c.HandleEvent(QuestionResolved{})
	c.HandleEvent(AuthRequired{ProviderID: "hyper"})
	c.HandleEvent(SummarizeStarted{SessionID: "s1"})
	c.HandleEvent(SummarizeFinished{SessionID: "s1"})
	c.HandleEvent(SessionUpdated{SessionID: "s1", Title: "S1"})
	c.ReportModel("model-1")
}

func TestSetSessionReportsMetadata(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// A session switch publishes the complete presentation set:
	// title plus the session token. herdr's presentation fields are
	// replace-all per source, so every report carries both.
	c.SetSession("s1", "My Title")
	meta := reportedMetadata(c)
	assert.Len(t, meta, 1)
	assert.Equal(t, "My Title", meta[0].Title)
	if assert.NotNil(t, meta[0].Tokens["session"]) {
		assert.Equal(t, "s1", *meta[0].Tokens["session"])
	}
	assert.NotContains(t, meta[0].Tokens, "model")
	assert.Equal(t, "pane.report_metadata", lastRequest(c).Method)

	// Identical reports are deduplicated.
	c.SetSession("s1", "My Title")
	assert.Len(t, reportedMetadata(c), 1)

	// A switch to another session refreshes title and token.
	c.SetSession("s2", "Other Title")
	meta = reportedMetadata(c)
	assert.Len(t, meta, 2)
	assert.Equal(t, "Other Title", meta[1].Title)
	if assert.NotNil(t, meta[1].Tokens["session"]) {
		assert.Equal(t, "s2", *meta[1].Tokens["session"])
	}

	// The session id still rides along on state reports.
	assert.Equal(t, "s2", c.sessionID)
}

func TestSetSessionClearRemovesPresentation(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Landing on the session picker (empty id) clears the title
	// (omitted, replace-all) and the session token (explicit null,
	// tokens merge per key).
	c.SetSession("s1", "My Title")
	c.SetSession("", "")
	meta := reportedMetadata(c)
	assert.Len(t, meta, 2)
	assert.Empty(t, meta[1].Title)
	assert.Nil(t, meta[1].Tokens["session"])

	raw, err := json.Marshal(lastRequest(c))
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), `"title"`)
	assert.Contains(t, string(raw), `"session":null`)
}

func TestReportModel(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// The model token merges with the rest of the set; before any
	// session is known the session token goes out as null.
	c.ReportModel("model-1")
	meta := reportedMetadata(c)
	assert.Len(t, meta, 1)
	if assert.NotNil(t, meta[0].Tokens["model"]) {
		assert.Equal(t, "model-1", *meta[0].Tokens["model"])
	}
	assert.Nil(t, meta[0].Tokens["session"])

	// Dedup, then a model change re-reports the complete set.
	c.ReportModel("model-1")
	assert.Len(t, reportedMetadata(c), 1)
	c.SetSession("s1", "T")
	c.ReportModel("model-2")
	meta = reportedMetadata(c)
	assert.Len(t, meta, 3)
	assert.Equal(t, "T", meta[2].Title)
	if assert.NotNil(t, meta[2].Tokens["model"]) {
		assert.Equal(t, "model-2", *meta[2].Tokens["model"])
	}
	if assert.NotNil(t, meta[2].Tokens["session"]) {
		assert.Equal(t, "s1", *meta[2].Tokens["session"])
	}

	// An empty model never clears the token.
	c.ReportModel("")
	assert.Len(t, reportedMetadata(c), 3)
}

func TestAssistantMessageRefreshesModelToken(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// The model riding on assistant output keeps the token fresh
	// after a mid-session model switch, in both run modes.
	c.HandleEvent(AssistantMessage{SessionID: "s1", Model: "model-1"})
	meta := reportedMetadata(c)
	assert.Len(t, meta, 1)
	if assert.NotNil(t, meta[0].Tokens["model"]) {
		assert.Equal(t, "model-1", *meta[0].Tokens["model"])
	}

	// Same model: deduped. New model: re-reported. Empty model: no
	// change.
	c.HandleEvent(AssistantMessage{SessionID: "s1", Model: "model-1"})
	assert.Len(t, reportedMetadata(c), 1)
	c.HandleEvent(AssistantMessage{SessionID: "s1", Model: "model-2"})
	assert.Len(t, reportedMetadata(c), 2)
	c.HandleEvent(AssistantMessage{SessionID: "s1", Model: ""})
	assert.Len(t, reportedMetadata(c), 2)
}

func TestSessionUpdatedRefreshesTitle(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// Auto-titling and renames arrive as session events; only the
	// authoritative current session may touch the pane title.
	c.SetSession("s1", "Old Title")
	c.HandleEvent(SessionUpdated{SessionID: "s1", Title: "New Title"})
	meta := reportedMetadata(c)
	assert.Len(t, meta, 2)
	assert.Equal(t, "New Title", meta[1].Title)

	// Events for other sessions, sub-sessions, and empty ids are
	// ignored, as are events while no current session is known.
	c.HandleEvent(SessionUpdated{SessionID: "s2", Title: "Wrong"})
	c.HandleEvent(SessionUpdated{SessionID: "msg-1$$tc-1", Title: "Sub"})
	c.HandleEvent(SessionUpdated{SessionID: "", Title: "Empty"})
	assert.Len(t, reportedMetadata(c), 2)

	c2 := newTestClient()
	c2.HandleEvent(SessionUpdated{SessionID: "s1", Title: "Unanchored"})
	assert.Empty(t, reportedMetadata(c2))
}

func TestSessionUpdatedTruncatesLongTitle(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	// herdr caps text fields at 80 characters; the cut must stay
	// rune-safe.
	long := strings.Repeat("é", 100)
	c.SetSession("s1", long)
	meta := reportedMetadata(c)
	assert.Len(t, meta, 1)
	assert.Equal(t, strings.Repeat("é", 80), meta[0].Title)
}

func TestMetadataSeqStrictlyIncreases(t *testing.T) {
	t.Parallel()
	c := newTestClient()
	c.seq = 100

	// State and metadata reports share one seq space; herdr drops
	// any report whose seq is not strictly greater than the last.
	c.HandleEvent(AssistantMessage{SessionID: "s1", Model: "m"})
	c.SetSession("s1", "T")
	c.HandleEvent(RunComplete{SessionID: "s1"})

	rec := c.snd.(*recordingSender)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var prev uint64
	for i, req := range rec.requests {
		var seq uint64
		switch p := req.Params.(type) {
		case reportParams:
			seq = p.Seq
		case metadataParams:
			seq = p.Seq
		default:
			t.Fatalf("unexpected params type %T", req.Params)
		}
		if i > 0 {
			assert.Greater(t, seq, prev)
		}
		prev = seq
	}
}

func TestRegisterInitial(t *testing.T) {
	t.Parallel()
	rec := &recordingSender{requests: make([]reportRequest, 0, 16)}
	c := &Client{
		state: stateIdle,
		seq:   100,
		snd:   rec,
	}
	c.registerInitial()
	assert.Equal(t, []string{stateIdle}, reportedStates(c))
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

// TestDisable verifies newFromEnv returns nil once Disable was
// called, even with a complete, valid-looking environment. Mutates
// process-global state, so it cannot run in parallel; the flag is
// restored on exit.
func TestDisable(t *testing.T) {
	old := disabled.Swap(false)
	defer disabled.Store(old)

	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/does-not-matter.sock")
	t.Setenv("HERDR_PANE_ID", "test:pane")

	Disable()
	assert.Nil(t, newFromEnv())
}
