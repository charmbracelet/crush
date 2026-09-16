package agent

import (
	"context"
	"fmt"
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
	texts    []string
}

func (f *fakeQuestionService) Subscribe(context.Context) <-chan pubsub.Event[question.Request] {
	return nil
}

func (f *fakeQuestionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[question.Notification] {
	return nil
}

func (f *fakeQuestionService) Ask(_ context.Context, req question.Request) ([]question.Answer, error) {
	f.asks++
	for _, q := range req.Questions {
		f.texts = append(f.texts, q.Text)
	}
	if f.err != nil {
		return nil, f.err
	}
	return []question.Answer{{SelectedIDs: f.selected}}, nil
}

func (f *fakeQuestionService) Answer([]question.Answer) bool { return false }
func (f *fakeQuestionService) Cancel() bool                  { return false }

func TestIsMutatingCall(t *testing.T) {
	t.Parallel()
	bash := func(cmd string) fantasy.ToolCall {
		return fantasy.ToolCall{Name: "bash", Input: fmt.Sprintf(`{"command":%q}`, cmd)}
	}
	tests := []struct {
		name string
		call fantasy.ToolCall
		want bool
	}{
		{"write tool", fantasy.ToolCall{Name: "edit"}, true},
		{"read tool", fantasy.ToolCall{Name: "view"}, false},
		{"rm", bash("rm -rf dist"), true},
		{"sed -i", bash(`sed -i 's/a/b/' f.go`), true},
		{"sed -n -i", bash(`sed -n -i 's/a/b/' f.go`), true},
		{"sed --in-place", bash(`sed --in-place 's/a/b/' f.go`), true},
		{"sed -ie bundled", bash(`sed -ie 's/a/b/' f.go`), true},
		{"sed stream-only", bash(`sed -n 's/a/b/p' f.go`), false},
		{"git commit", bash("git commit -m x"), true},
		{"git config --get", bash("git config --get user.name"), false},
		{"git fetch", bash("git fetch origin"), false},
		{"git worktree list", bash("git worktree list"), false},
		{"git branch -D", bash("git branch -D old"), true},
		{"git branch list", bash("git branch"), false},
		{"git update-ref", bash("git update-ref HEAD abc123"), true},
		{"download tool", fantasy.ToolCall{Name: "download"}, true},
		{"kubectl delete", bash("kubectl delete pod x"), true},
		{"kubectl get", bash("kubectl get pods"), false},
		{"apt-get remove", bash("apt-get remove pkg"), true},
		{"rsync", bash("rsync -a src dst"), true},
		{"scp", bash("scp f host:/tmp"), true},
		{"redirect to file", bash("go build -o /dev/null && go test > out.log ./..."), true},
		{"redirect to dev null", bash("go test ./... > /dev/null"), false},
		{"stderr dup", bash("go test ./... 2>&1"), false},
		{"quoted redirect", bash(`echo "progress > bar"`), false},
		{"quoted redirect target", bash(`echo x > 'out'`), true},
		{"stderr-and-stdout to file", bash("go build >& out.log"), true},
		{"go test", bash("go test ./..."), false},
		{"make test", bash("make test"), false},
		{"empty input", fantasy.ToolCall{Name: "bash"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tools.IsMutatingCall(tc.call.Name, tc.call.Input))
		})
	}
}

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
		wrapped := newScopeGate(svc, true).wrap([]fantasy.AgentTool{read, write})
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

	t.Run("the question reports the real exploration count", func(t *testing.T) {
		t.Parallel()
		svc, _, readTool, writeTool := newGate(t, []string{"proceed"})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, readTool, scopeGateMinExploration+4)
		_, err := writeTool.Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.Len(t, svc.texts, 1)
		require.Contains(t, svc.texts[0], fmt.Sprintf("explored %d steps", scopeGateMinExploration+4))
	})

	t.Run("mutating bash gates like a write", func(t *testing.T) {
		t.Parallel()
		svc := &fakeQuestionService{selected: []string{"proceed"}}
		bash := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("done")}
		read := &fakeTool{name: "view", resp: fantasy.NewTextResponse("x")}
		wrapped := newScopeGate(svc, true).wrap([]fantasy.AgentTool{read, bash})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, wrapped[0], scopeGateMinExploration)

		resp, err := wrapped[1].Run(ctx, fantasy.ToolCall{
			ID: "b", Name: "bash", Input: `{"command":"rm -rf dist"}`,
		})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.Equal(t, 1, svc.asks)
		require.True(t, bash.called)
	})

	t.Run("read-only bash counts as exploration", func(t *testing.T) {
		t.Parallel()
		svc := &fakeQuestionService{selected: []string{"proceed"}}
		bash := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("ok")}
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		wrapped := newScopeGate(svc, true).wrap([]fantasy.AgentTool{bash, write})
		ctx := gateCtx("s1", 1)
		for range scopeGateMinExploration {
			resp, err := wrapped[0].Run(ctx, fantasy.ToolCall{
				ID: "b", Name: "bash", Input: `{"command":"go test ./internal/..."}`,
			})
			require.NoError(t, err)
			require.False(t, resp.IsError)
		}
		require.Equal(t, 0, svc.asks)
		// The accumulated bash exploration arms the gate for the write.
		_, err := wrapped[1].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.Equal(t, 1, svc.asks)
	})

	t.Run("declared todos satisfy the gate", func(t *testing.T) {
		t.Parallel()
		svc := &fakeQuestionService{}
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		shared := newScopeGate(svc, true).wrap([]fantasy.AgentTool{
			&fakeTool{name: tools.TodosToolName, resp: fantasy.NewTextResponse("ok")},
			&fakeTool{name: "view", resp: fantasy.NewTextResponse("x")},
			write,
		})
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
		wrapped := newScopeGate(svc, true).wrap([]fantasy.AgentTool{read, write})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, wrapped[0], scopeGateMinExploration)
		resp, err := wrapped[1].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
	})

	t.Run("headless degrade proceeds without asking", func(t *testing.T) {
		t.Parallel()
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		read := &fakeTool{name: "view", resp: fantasy.NewTextResponse("x")}
		wrapped := newScopeGate(nil, false).wrap([]fantasy.AgentTool{read, write})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, wrapped[0], scopeGateMinExploration)
		resp, err := wrapped[1].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.True(t, write.called)
	})

	t.Run("nil service builds no gate", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, newScopeGate(nil, true))
	})

	t.Run("a tool rebuild keeps gate state", func(t *testing.T) {
		t.Parallel()
		// SetTools rebuilds re-wrap the toolset — the same gate must
		// keep its resolved mark so a turn isn't re-asked.
		svc := &fakeQuestionService{selected: []string{"proceed"}}
		gate := newScopeGate(svc, true)
		write := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")}
		read := &fakeTool{name: "view", resp: fantasy.NewTextResponse("x")}
		wrapped := gate.wrap([]fantasy.AgentTool{read, write})
		ctx := gateCtx("s1", 1)
		exploreN(t, ctx, wrapped[0], scopeGateMinExploration)
		_, err := wrapped[1].Run(ctx, fantasy.ToolCall{ID: "w", Name: "edit"})
		require.NoError(t, err)
		require.Equal(t, 1, svc.asks)

		rewrapped := gate.wrap([]fantasy.AgentTool{read, write})
		resp, err := rewrapped[1].Run(ctx, fantasy.ToolCall{ID: "w2", Name: "edit"})
		require.NoError(t, err)
		require.False(t, resp.IsError)
		require.Equal(t, 1, svc.asks)
	})
}
