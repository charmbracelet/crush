package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

// fakeQuestionService returns a canned answer selection per Ask call.
type fakeQuestionService struct {
	selected []string
	err      error
	asks     int
}

func (f *fakeQuestionService) Subscribe(context.Context) <-chan pubsub.Event[question.Request] {
	return nil
}

func (f *fakeQuestionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[question.Notification] {
	return nil
}

func (f *fakeQuestionService) Ask(_ context.Context, _ question.Request) ([]question.Answer, error) {
	f.asks++
	if f.err != nil {
		return nil, f.err
	}
	return []question.Answer{{SelectedIDs: f.selected}}, nil
}

func (f *fakeQuestionService) Answer([]question.Answer) bool { return false }
func (f *fakeQuestionService) Cancel() bool                  { return false }

// gateCtx stamps the session and run identity the gate keys on.
func gateCtx(sessionID string, stamp uint64) context.Context {
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, sessionID)
	return context.WithValue(ctx, tools.RunStampContextKey, stamp)
}

func exploreN(t *testing.T, ctx context.Context, tool fantasy.AgentTool, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "e", Name: "view"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
	}
}

func TestScopeGate(t *testing.T) {
	t.Parallel()

	newGate := func(t *testing.T, selected []string) (*fakeQuestionService, *fakeTool, fantasy.AgentTool, fantasy.AgentTool) {
		svc := &fakeQuestionService{selected: selected}
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		read := &fakeTool{name: "view", resp: fantasy.NewTextResponse("file contents")}
		wrapped := wrapToolsWithScopeGate([]fantasy.AgentTool{read, write}, svc)
		return svc, write, wrapped[0], wrapped[1]
	}

	t.Run("write before the threshold passes un-gated", func(t *testing.T) {
		t.Parallel()
		svc, write, readTool, writeTool := newGate(t, []string{"proceed"})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, readTool, scopeGateMinExploration-1)
		resp, err := writeTool.Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
		require.Equal(t, 0, svc.asks)
	})

	t.Run("first write after deep exploration asks once", func(t *testing.T) {
		t.Parallel()
		svc, write, readTool, writeTool := newGate(t, []string{"proceed"})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, readTool, scopeGateMinExploration)
		resp, err := writeTool.Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
		require.Equal(t, 1, svc.asks)

		// Resolved for the rest of the run — no second question.
		resp, err = writeTool.Run(ctx, fantasy.ToolCall{ID: "w2", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.Equal(t, 1, svc.asks)
	})

	t.Run("new run re-arms the gate", func(t *testing.T) {
		t.Parallel()
		svc, write, readTool, writeTool := newGate(t, []string{"narrow"})
		// A prior resolved run does not carry its exploration count
		// forward — the boundary is per run.
		exploreN(t, gateCtx("s1", 1), readTool, scopeGateMinExploration)
		ctx := gateCtx("s1", 2)
		exploreN(t, ctx, readTool, scopeGateMinExploration)
		resp, err := writeTool.Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.True(t, resp.IsError, "narrow answer must not execute the write")
		require.False(t, write.called)
		require.Equal(t, 1, svc.asks)
	})

	t.Run("declared todos satisfy the gate", func(t *testing.T) {
		t.Parallel()
		svc := &fakeQuestionService{}
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		shared := wrapToolsWithScopeGate([]fantasy.AgentTool{
			&fakeTool{name: tools.TodosToolName, resp: fantasy.NewTextResponse("ok")},
			&fakeTool{name: "view", resp: fantasy.NewTextResponse("x")},
			write,
		}, svc)
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, shared[1], scopeGateMinExploration)
		_, err := shared[0].Run(ctx, fantasy.ToolCall{ID: "t", Name: tools.TodosToolName})
		require.NoError(t, err)
		resp, err := shared[2].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
		require.Equal(t, 0, svc.asks)
	})

	t.Run("user stop ends the turn", func(t *testing.T) {
		t.Parallel()
		svc, write, readTool, writeTool := newGate(t, []string{"stop"})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, readTool, scopeGateMinExploration)
		resp, err := writeTool.Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.True(t, resp.StopTurn)
		require.False(t, write.called)
		require.Equal(t, 1, svc.asks)
	})

	t.Run("degraded ask lets the write through", func(t *testing.T) {
		t.Parallel()
		svc := &fakeQuestionService{err: context.DeadlineExceeded}
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		read := &fakeTool{name: "view", resp: fantasy.NewTextResponse("x")}
		wrapped := wrapToolsWithScopeGate([]fantasy.AgentTool{read, write}, svc)
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, wrapped[0], scopeGateMinExploration)
		resp, err := wrapped[1].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
	})

	t.Run("nil service returns the tools unchanged", func(t *testing.T) {
		t.Parallel()
		tl := []fantasy.AgentTool{&fakeTool{name: "edit"}}
		require.Equal(t, tl, wrapToolsWithScopeGate(tl, nil))
	})
}
