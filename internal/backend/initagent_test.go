package backend

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fakeLLM is an OpenAI-compatible endpoint. An agent turn (a request that
// offers tools) streams one chunk and then holds its stream open until the
// client cancels the request; any other request, such as title generation,
// finishes at once.
type fakeLLM struct {
	url       string
	started   chan struct{}
	cancelled chan struct{}
	active    atomic.Int32
}

func newFakeLLM(t *testing.T) *fakeLLM {
	t.Helper()
	const chunk = `{"id":"c","object":"chat.completion.chunk","created":0,"model":"mock-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`

	llm := &fakeLLM{
		started:   make(chan struct{}, 16),
		cancelled: make(chan struct{}, 16),
	}
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		if !bytes.Contains(body, []byte(`"tools"`)) {
			fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"title"}`, "null")
			fmt.Fprintf(w, "data: "+chunk+"\n\n", `{}`, `"stop"`)
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"working"}`, "null")
		w.(http.Flusher).Flush()
		llm.active.Add(1)
		defer llm.active.Add(-1)
		llm.started <- struct{}{}
		select {
		case <-r.Context().Done():
			llm.cancelled <- struct{}{}
		case <-stop:
		}
	}))
	// Cleanups run last-in first-out: release any stream still held open,
	// then close the server.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(stop) })
	llm.url = srv.URL
	return llm
}

func (l *fakeLLM) awaitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-l.started:
	case <-time.After(10 * time.Second):
		t.Fatal("agent turn never reached the model")
	}
}

func (l *fakeLLM) awaitCancelled(t *testing.T) {
	t.Helper()
	select {
	case <-l.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatalf("model request was never cancelled (%d still streaming)", l.active.Load())
	}
}

// newLLMWorkspace creates a real workspace through the backend whose
// project config points the coder agent at a fake model.
func newLLMWorkspace(t *testing.T) (*Backend, *Workspace, *fakeLLM) {
	t.Helper()
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	llm := newFakeLLM(t)
	cwd := t.TempDir()
	crushJSON := fmt.Sprintf(`{
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": %q, "api_key": "test",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 128000, "default_max_tokens": 1000}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, llm.url)
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "crush.json"), []byte(crushJSON), 0o644))

	b := New(t.Context(), nil, func() {})
	t.Cleanup(func() { drainBackend(t, b) })

	ws, _, err := b.CreateWorkspace(protoWS(cwd, filepath.Join(cwd, ".crush"), uuid.New().String()))
	require.NoError(t, err)
	return b, ws, llm
}

// joinWorkspace registers another client on ws the way a second
// terminal does: it creates the workspace at the same path and gets the
// existing one back.
func joinWorkspace(t *testing.T, b *Backend, ws *Workspace) {
	t.Helper()
	joined, _, err := b.CreateWorkspace(protoWS(ws.Path, filepath.Join(ws.Path, ".crush"), uuid.New().String()))
	require.NoError(t, err)
	require.Equal(t, ws.ID, joined.ID)
}

// TestInitAgent_JoiningClientKeepsLiveRun covers a second client joining a
// workspace whose first client has a run streaming. Every client initializes
// the agent when it attaches; `crush run` asks for a non-interactive one.
// The join must leave the shared coordinator alone, so the first client's
// run stays visible, a follow-up prompt queues behind it instead of
// starting a second run in the same session, and cancel stops it.
func TestInitAgent_JoiningClientKeepsLiveRun(t *testing.T) {
	b, ws, llm := newLLMWorkspace(t)
	ctx := t.Context()

	require.NoError(t, b.InitAgent(ctx, ws.ID, true))
	first := ws.AgentCoordinator
	require.NotNil(t, first)

	sess, err := b.CreateSession(ctx, ws.ID, "session")
	require.NoError(t, err)
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: sess.ID, Prompt: "refactor"}))
	llm.awaitStarted(t)

	joinWorkspace(t, b, ws)
	require.NoError(t, b.InitAgent(ctx, ws.ID, false))

	agentSession, err := b.GetAgentSession(ctx, ws.ID, sess.ID)
	require.NoError(t, err)
	require.True(t, agentSession.IsBusy, "the running session must still report busy after another client joined")
	require.True(t, ws.AgentCoordinator == first, "joining must not replace the workspace's coordinator")

	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: sess.ID, Prompt: "follow up"}))
	require.Eventually(t, func() bool {
		queued, err := b.QueuedPrompts(ws.ID, sess.ID)
		return err == nil && queued == 1
	}, 5*time.Second, 10*time.Millisecond, "a follow-up prompt must queue behind the running turn")
	require.EqualValues(t, 1, llm.active.Load(), "only one turn may stream in the session")

	require.NoError(t, b.CancelSession(ws.ID, sess.ID))
	llm.awaitCancelled(t)
}

// TestInitAgent_ConcurrentJoinsAreRaceFree has several clients initialize
// the agent at once while others read it, for a workspace that already has
// a coordinator and for one that has to create it. Run with -race. Exactly
// one coordinator must serve every client.
func TestInitAgent_ConcurrentJoinsAreRaceFree(t *testing.T) {
	for _, existing := range []bool{true, false} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			b, ws, _ := newLLMWorkspace(t)
			ctx := t.Context()
			if !existing {
				// Stand in for a workspace created before its provider was
				// set up, which has no coordinator until a client asks.
				ws.AgentCoordinator = nil
			}

			stop := make(chan struct{})
			var readers sync.WaitGroup
			for range 2 {
				readers.Go(func() {
					for {
						select {
						case <-stop:
							return
						default:
						}
						_, _ = b.GetAgentInfo(ws.ID)
						_, _ = b.QueuedPrompts(ws.ID, "session")
						_ = b.CancelSession(ws.ID, "session")
					}
				})
			}

			const joins = 4
			errs := make(chan error, joins)
			seen := make(chan agent.Coordinator, joins)
			var joiners sync.WaitGroup
			for i := range joins {
				joiners.Go(func() {
					errs <- b.InitAgent(ctx, ws.ID, i%2 == 0)
					seen <- ws.AgentCoordinator
				})
			}
			joiners.Wait()
			close(stop)
			readers.Wait()
			close(errs)
			close(seen)

			for err := range errs {
				require.NoError(t, err)
			}
			want := ws.AgentCoordinator
			require.NotNil(t, want)
			for got := range seen {
				require.True(t, got == want, "every client must share one coordinator")
			}
		})
	}
}
