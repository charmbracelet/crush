package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// Arm the fault after the first turn saves its usage, so session lookup
// fails during the promoted turn's preparation, not during streaming.
type preparationSessions struct {
	session.Service
	ready atomic.Bool
	err   error
}

func (s *preparationSessions) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	saved, err := s.Service.Save(ctx, sess)
	if err == nil {
		s.ready.Store(true)
	}
	return saved, err
}

func (s *preparationSessions) Get(ctx context.Context, id string) (session.Session, error) {
	if s.ready.Load() && s.err != nil {
		return session.Session{}, s.err
	}
	return s.Service.Get(ctx, id)
}

type preparationMessages struct {
	message.Service
	sessions   *preparationSessions
	historyErr error
	createErr  error
	createRole message.MessageRole
}

func (m *preparationMessages) ListFromSummary(ctx context.Context, sessionID, summaryID string) ([]message.Message, error) {
	if m.sessions.ready.Load() && m.historyErr != nil {
		return nil, m.historyErr
	}
	return m.Service.ListFromSummary(ctx, sessionID, summaryID)
}

func (m *preparationMessages) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	if m.sessions.ready.Load() && params.Role == m.createRole && m.createErr != nil {
		return message.Message{}, m.createErr
	}
	return m.Service.Create(ctx, sessionID, params)
}

type completedDispatch struct {
	runID  string
	marked bool
}

// Observe the real coordinator's completion marker without changing its
// callbacks or the backend's fallback decision.
type observedCoordinator struct {
	agent.Coordinator
	returned chan completedDispatch
}

func (c *observedCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	result, err := c.Coordinator.RunAccepted(ctx, accept, sessionID, prompt, attachments...)
	c.returned <- completedDispatch{agent.RunIDFromContext(ctx), agent.RunCompletePublished(ctx)}
	return result, err
}

func TestSendMessage_PreparationFailureCompletion(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	for _, promoted := range []bool{false, true} {
		for _, stage := range []string{"session", "history", "user", "assistant"} {
			t.Run(fmt.Sprintf("promoted=%t/%s", promoted, stage), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				entered := make(chan struct{}, 1)
				gate := make(chan struct{})
				var requests atomic.Int64
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					select {
					case entered <- struct{}{}:
					default:
					}
					select {
					case <-gate:
					case <-ctx.Done():
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					_, err := fmt.Fprint(w, "data: {\"id\":\"reply\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"first answer\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"reply\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					if err != nil {
						t.Error(err)
					}
				}))
				defer server.Close()
				defer cancel()

				conn, err := db.Connect(ctx, t.TempDir())
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, conn.Close()) })
				queries := db.New(conn)
				sessions := &preparationSessions{Service: session.NewService(queries, conn)}
				messages := &preparationMessages{Service: message.NewService(queries), sessions: sessions}
				sess, err := sessions.Create(ctx, "session")
				require.NoError(t, err)
				// Avoid a title-generation request racing the fault injection.
				_, err = messages.Create(ctx, sess.ID, message.CreateMessageParams{
					Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "existing conversation"}},
				})
				require.NoError(t, err)
				failure := errors.New("preparation storage failure")
				switch stage {
				case "session":
					sessions.err = failure
				case "history":
					messages.historyErr = failure
				case "user":
					messages.createRole = message.User
					messages.createErr = failure
				case "assistant":
					// This failure is past the guard's handoff to the normal
					// completion path; it must not emit twice.
					messages.createRole = message.Assistant
					messages.createErr = failure
				}
				sessions.ready.Store(!promoted)

				workingDir := t.TempDir()
				cfg, err := config.Init(workingDir, "", false)
				require.NoError(t, err)
				cfg.Config().Providers.Set("test", config.ProviderConfig{
					ID: "test", Name: "Test", Type: openaicompat.Name,
					BaseURL: server.URL + "/v1", APIKey: "test",
					Models: []catwalk.Model{{ID: "test-model", DefaultMaxTokens: 4096}},
				})
				selected := config.SelectedModel{Provider: "test", Model: "test-model"}
				cfg.OverridePreferredModel(config.SelectedModelTypeLarge, selected)
				cfg.OverridePreferredModel(config.SelectedModelTypeSmall, selected)
				cfg.SetupAgents()
				coder := cfg.Config().Agents[config.AgentCoder]
				coder.AllowedTools = nil
				cfg.Config().Agents[config.AgentCoder] = coder

				backend, _ := newTestBackend(t)
				ws := insertRunCompleteWorkspace(t, backend, ctx, nil)
				coord, err := agent.NewCoordinator(ctx, agent.CoordinatorOptions{
					Config: cfg, Sessions: sessions, Messages: messages,
					Permissions: permission.NewPermissionService(workingDir, true, nil),
					RunComplete: ws.RunCompletions(),
					Skills:      skills.NewManager(nil, nil, nil),
				})
				require.NoError(t, err)
				observed := &observedCoordinator{Coordinator: coord, returned: make(chan completedDispatch, 2)}
				ws.AgentCoordinator = observed
				completions := ws.RunCompletions().Subscribe(ctx)

				if promoted {
					require.NoError(t, backend.SendMessage(ws.ID, proto.AgentMessage{SessionID: sess.ID, RunID: "first", Prompt: "first"}))
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal("first turn never streamed")
					}
				}
				require.NoError(t, backend.SendMessage(ws.ID, proto.AgentMessage{SessionID: sess.ID, RunID: "failing", Prompt: "second"}))
				select {
				case returned := <-observed.returned:
					require.Equal(t, completedDispatch{runID: "failing", marked: !promoted && stage == "assistant"}, returned,
						"only the normal completion path transfers ownership from the backend")
				case <-ctx.Done():
					t.Fatal("second dispatch did not return")
				}
				if promoted {
					require.Equal(t, 1, coord.QueuedPrompts(sess.ID))
					close(gate)
					select {
					case returned := <-observed.returned:
						require.Equal(t, completedDispatch{runID: "first", marked: true}, returned)
					case <-ctx.Done():
						t.Fatal("queue promotion did not return")
					}
				}
				ws.runWG.Wait()

				// All publishers have returned. Read buffered events without
				// sleeps, counting duplicates rather than overwriting by RunID.
				var got []notify.RunComplete
				for len(completions) > 0 {
					got = append(got, (<-completions).Payload)
				}
				wantCount := 1
				if promoted {
					wantCount = 2
				}
				require.Len(t, got, wantCount)
				for _, complete := range got {
					require.Equal(t, sess.ID, complete.SessionID)
					require.False(t, complete.Cancelled)
					switch complete.RunID {
					case "first":
						require.Empty(t, complete.Error)
						require.Equal(t, "first answer", complete.Text)
					case "failing":
						require.Contains(t, complete.Error, failure.Error())
						require.Empty(t, complete.MessageID)
						require.Empty(t, complete.Text)
					default:
						t.Fatalf("unexpected completion: %+v", complete)
					}
				}
				if promoted {
					require.NotEqual(t, got[0].RunID, got[1].RunID)
					require.EqualValues(t, 1, requests.Load(), "the failed turn must never reach the provider")
				} else {
					require.Equal(t, "failing", got[0].RunID)
					require.Zero(t, requests.Load())
				}
			})
		}
	}
}
