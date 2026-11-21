package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// rawSSEEvent is one SSE event as written on the wire.
type rawSSEEvent struct {
	ID      string
	Payload pubsub.Payload
}

// subscribeRawSSE opens an event stream and returns its events with their
// ids. lastEventID, if set, is sent in the header, or in the query when
// viaQuery is set.
func (h *e2eHarness) subscribeRawSSE(t *testing.T, ctx context.Context, clientID, lastEventID string, viaQuery bool) (<-chan rawSSEEvent, context.CancelFunc) {
	t.Helper()
	streamCtx, cancel := context.WithCancel(ctx)
	q := url.Values{"client_id": []string{clientID}}
	if lastEventID != "" && viaQuery {
		q.Set("last_event_id", lastEventID)
	}
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+h.workspace.ID+"/events?"+q.Encode(), nil)
	require.NoError(t, err)
	if lastEventID != "" && !viaQuery {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	out := make(chan rawSSEEvent, 256)
	h.sseWG.Go(func() {
		defer resp.Body.Close()
		defer close(out)
		reader := bufio.NewReader(resp.Body)
		var id string
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				id = ""
				continue
			}
			if v, ok := bytes.CutPrefix(line, []byte("id:")); ok {
				id = string(bytes.TrimSpace(v))
				continue
			}
			data, ok := bytes.CutPrefix(line, []byte("data:"))
			if !ok {
				continue
			}
			var p pubsub.Payload
			if err := json.Unmarshal(bytes.TrimSpace(data), &p); err != nil {
				continue
			}
			select {
			case out <- rawSSEEvent{ID: id, Payload: p}:
			case <-streamCtx.Done():
				return
			}
		}
	})
	return out, cancel
}

// nextOfType returns the next event of the given payload type.
func nextOfType(t *testing.T, evc <-chan rawSSEEvent, typ pubsub.PayloadType) rawSSEEvent {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-evc:
			require.True(t, ok, "stream closed waiting for %s", typ)
			if ev.Payload.Type == typ {
				return ev
			}
		case <-timeout:
			require.FailNow(t, "timed out waiting for event", "type %s", typ)
		}
	}
}

// configChanged publishes a numbered workspace event.
func (h *e2eHarness) configChanged(n string) {
	h.app.SendEvent(pubsub.Event[proto.ConfigChanged]{Payload: proto.ConfigChanged{WorkspaceID: n}})
}

func configChangedTag(t *testing.T, ev rawSSEEvent) string {
	t.Helper()
	var e pubsub.Event[proto.ConfigChanged]
	require.NoError(t, json.Unmarshal(ev.Payload.Payload, &e))
	return e.Payload.WorkspaceID
}

func seqOf(t *testing.T, id string) (string, int) {
	t.Helper()
	epoch, n, ok := strings.Cut(id, "-")
	require.True(t, ok, "id %q is not <epoch>-<n>", id)
	var seq int
	require.NoError(t, json.Unmarshal([]byte(n), &seq))
	return epoch, seq
}

func TestE2E_EventIDsMonotonic(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	evc, cancel := h.subscribeRawSSE(t, t.Context(), uuid.New().String(), "", false)
	t.Cleanup(cancel)
	h.waitForAttached(t, 1)

	for _, tag := range []string{"a", "b", "c"} {
		h.configChanged(tag)
	}
	var epoch string
	prev := 0
	for _, tag := range []string{"a", "b", "c"} {
		ev := nextOfType(t, evc, pubsub.PayloadTypeConfigChanged)
		require.Equal(t, tag, configChangedTag(t, ev))
		e, seq := seqOf(t, ev.ID)
		if epoch == "" {
			epoch = e
		}
		require.Equal(t, epoch, e)
		require.Greater(t, seq, prev)
		prev = seq
	}
}

// TestE2E_ResumeReplaysMissedFiltered drops a stream, publishes while it
// is gone, and resumes: exactly the missed events come back, still scoped
// to the client's session, then live events follow.
func TestE2E_ResumeReplaysMissedFiltered(t *testing.T) {
	t.Parallel()
	for _, viaQuery := range []bool{false, true} {
		t.Run(map[bool]string{false: "header", true: "query"}[viaQuery], func(t *testing.T) {
			t.Parallel()
			h := newE2EHarness(t)
			ctx := t.Context()
			cid := uuid.New().String()
			first, cancelFirst := h.subscribeRawSSE(t, ctx, cid, "", false)
			h.waitForAttached(t, 1)
			h.setCurrentSession(t, ctx, h.workspace.ID, cid, "S1")
			// Wait for the numbered S1 update, so it is sequenced before
			// "seen" and the ids below are deterministic.
			s1Update := nextOfType(t, first, pubsub.PayloadTypeClientSession)
			require.NotEmpty(t, s1Update.ID)

			h.configChanged("seen")
			last := nextOfType(t, first, pubsub.PayloadTypeConfigChanged)
			require.NotEmpty(t, last.ID)
			cancelFirst()

			// Published while the client is away: one prompt for its
			// session, one for another session (filtered), two others.
			h.configChanged("missed-1")
			h.app.SendEvent(pubsub.Event[permission.PermissionRequest]{
				Type: pubsub.CreatedEvent, Payload: permission.PermissionRequest{ID: "p-other", SessionID: "S2"},
			})
			h.app.SendEvent(pubsub.Event[permission.PermissionRequest]{
				Type: pubsub.CreatedEvent, Payload: permission.PermissionRequest{ID: "p-mine", SessionID: "S1"},
			})
			h.configChanged("missed-2")

			resumed, cancelResumed := h.subscribeRawSSE(t, ctx, cid, last.ID, viaQuery)
			t.Cleanup(cancelResumed)
			h.configChanged("live")

			var got []string
			var ids []string
			timeout := time.After(5 * time.Second)
			for len(got) < 4 {
				select {
				case ev, ok := <-resumed:
					require.True(t, ok)
					switch ev.Payload.Type {
					case pubsub.PayloadTypeConfigChanged:
						got = append(got, configChangedTag(t, ev))
						ids = append(ids, ev.ID)
					case pubsub.PayloadTypePermissionRequest:
						var e pubsub.Event[proto.PermissionRequest]
						require.NoError(t, json.Unmarshal(ev.Payload.Payload, &e))
						got = append(got, e.Payload.ID)
						ids = append(ids, ev.ID)
					case pubsub.PayloadTypeResync:
						require.FailNow(t, "unexpected resync")
					case pubsub.PayloadTypeClientSession:
						require.Empty(t, ev.ID, "only the attach snapshot is a client session event here")
					}
				case <-timeout:
					require.FailNow(t, "timed out", "got %v", got)
				}
			}
			require.Equal(t, []string{"missed-1", "p-mine", "missed-2", "live"}, got)
			_, lastSeq := seqOf(t, last.ID)
			_, s0 := seqOf(t, ids[0])
			require.Equal(t, lastSeq+1, s0)
			// p-other took a seq the client never sees.
			_, s1 := seqOf(t, ids[1])
			require.Equal(t, lastSeq+3, s1)
		})
	}
}

func TestE2E_ResumeResync(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		id     func(epoch string) string
		reason proto.ResyncReason
	}{
		"wrong epoch": {func(string) string { return "000000000000-1" }, proto.ResyncReasonEpochMismatch},
		"unknown":     {func(epoch string) string { return epoch + "-9999" }, proto.ResyncReasonUnknownID},
		"garbage":     {func(string) string { return "not-an-id" }, proto.ResyncReasonInvalidID},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newE2EHarness(t)
			ctx := t.Context()
			h.configChanged("one")

			// Learn the current id from a stream.
			probe, cancelProbe := h.subscribeRawSSE(t, ctx, uuid.New().String(), "", false)
			h.waitForAttached(t, 1)
			h.configChanged("two")
			current := nextOfType(t, probe, pubsub.PayloadTypeConfigChanged)
			cancelProbe()
			epoch, seq := seqOf(t, current.ID)

			evc, cancel := h.subscribeRawSSE(t, ctx, uuid.New().String(), tc.id(epoch), false)
			defer cancel()
			ev := <-evc
			require.Equal(t, pubsub.PayloadTypeResync, ev.Payload.Type, "resync must come first")
			var r proto.Resync
			require.NoError(t, json.Unmarshal(ev.Payload.Payload, &r))
			require.Equal(t, tc.reason, r.Reason)
			require.Equal(t, current.ID, r.LastEventID)
			require.Equal(t, r.LastEventID, ev.ID)

			h.configChanged("after")
			live := nextOfType(t, evc, pubsub.PayloadTypeConfigChanged)
			_, liveSeq := seqOf(t, live.ID)
			require.Equal(t, seq+1, liveSeq, "live continues right after the resync point")
		})
	}
}

// TestE2E_ResumeAfterBufferOverflow resumes from an id that has fallen
// out of the ring and gets a resync.
func TestE2E_ResumeAfterBufferOverflow(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()
	cid := uuid.New().String()
	evc, cancel := h.subscribeRawSSE(t, ctx, cid, "", false)
	h.waitForAttached(t, 1)
	h.configChanged("first")
	first := nextOfType(t, evc, pubsub.PayloadTypeConfigChanged)
	cancel()

	// A watcher that sees the last filler proves the sequencer has
	// recorded them all, pushing first out of the buffer.
	watch, cancelWatch := h.subscribeRawSSE(t, ctx, uuid.New().String(), "", false)
	t.Cleanup(cancelWatch)
	h.waitForAttached(t, 1)
	for range 1100 {
		h.configChanged("filler")
	}
	h.configChanged("done")
	for configChangedTag(t, nextOfType(t, watch, pubsub.PayloadTypeConfigChanged)) != "done" {
	}
	resumed, cancelResumed := h.subscribeRawSSE(t, ctx, cid, first.ID, false)
	t.Cleanup(cancelResumed)
	ev := <-resumed
	require.Equal(t, pubsub.PayloadTypeResync, ev.Payload.Type)
	var r proto.Resync
	require.NoError(t, json.Unmarshal(ev.Payload.Payload, &r))
	require.Equal(t, proto.ResyncReasonExpired, r.Reason)
}

// TestE2E_ClientSessionChangesSequenced checks that live client session
// changes carry ids and are replayed on resume, while the attach-time
// snapshot of client sessions carries none.
func TestE2E_ClientSessionChangesSequenced(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()
	tui, watcher := uuid.New().String(), uuid.New().String()
	tuiEvents, cancelTUI := h.subscribeRawSSE(t, ctx, tui, "", false)
	t.Cleanup(cancelTUI)
	watch, cancelWatch := h.subscribeRawSSE(t, ctx, watcher, "", false)
	h.waitForAttached(t, 2)

	h.postCurrentSession(t, ctx, h.workspace.ID, tui, "S1")
	live := nextOfType(t, watch, pubsub.PayloadTypeClientSession)
	require.NotEmpty(t, live.ID, "a live client session change carries an id")
	cancelWatch()

	h.postCurrentSession(t, ctx, h.workspace.ID, tui, "S2")
	// The still-connected client sees the numbered S2 update, so it is
	// sequenced, and buffered, before the watcher resumes.
	for {
		ev := nextOfType(t, tuiEvents, pubsub.PayloadTypeClientSession)
		var cs pubsub.Event[proto.ClientSession]
		require.NoError(t, json.Unmarshal(ev.Payload.Payload, &cs))
		if cs.Payload.SessionID == "S2" {
			require.NotEmpty(t, ev.ID)
			break
		}
	}
	resumed, cancelResumed := h.subscribeRawSSE(t, ctx, watcher, live.ID, false)
	t.Cleanup(cancelResumed)

	// The replay comes first and carries the missed change with an id;
	// the snapshot follows without one.
	replayed := nextOfType(t, resumed, pubsub.PayloadTypeClientSession)
	require.NotEmpty(t, replayed.ID)
	_, liveSeq := seqOf(t, live.ID)
	_, replayedSeq := seqOf(t, replayed.ID)
	require.Equal(t, liveSeq+1, replayedSeq)
	var cs pubsub.Event[proto.ClientSession]
	require.NoError(t, json.Unmarshal(replayed.Payload.Payload, &cs))
	require.Equal(t, proto.ClientSession{ClientID: tui, SessionID: "S2"}, cs.Payload)

	snapshot := nextOfType(t, resumed, pubsub.PayloadTypeClientSession)
	require.Empty(t, snapshot.ID, "the attach snapshot carries no id")
}
