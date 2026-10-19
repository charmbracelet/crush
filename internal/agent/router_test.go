package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func testRoutes() []config.RouterRoute {
	return []config.RouterRoute{
		{Name: "simple", Description: "quick questions", Model: config.SelectedModel{Provider: "p", Model: "small"}},
		{Name: "complex", Description: "hard work", Model: config.SelectedModel{Provider: "p", Model: "big"}},
	}
}

func TestMatchRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer string
		want   string
		ok     bool
	}{
		"exact":            {answer: "complex", want: "complex", ok: true},
		"case insensitive": {answer: "SIMPLE", want: "simple", ok: true},
		"punctuation":      {answer: " `complex`.\n", want: "complex", ok: true},
		"contained":        {answer: "I would pick complex here", want: "complex", ok: true},
		"think tags":       {answer: "<think>maybe simple</think>complex", want: "complex", ok: true},
		"garbage":          {answer: "banana", ok: false},
		"empty":            {answer: "", ok: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := matchRoute(testRoutes(), tc.answer)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, tc.want, got.Name)
			}
		})
	}
}

func TestGuardContext(t *testing.T) {
	t.Parallel()

	routes := testRoutes()
	window := func(r config.RouterRoute) int64 {
		if r.Name == "simple" {
			return 10_000
		}
		return 200_000
	}
	require.Equal(t, "simple", guardContext(routes, routes[0], 1_000, window).Name)
	require.Equal(t, "complex", guardContext(routes, routes[0], 9_000, window).Name,
		"a nearly full window must switch to the largest one")
	require.Equal(t, "complex", guardContext(routes, routes[1], 9_000, window).Name)
}

func TestBuildRouterPrompt(t *testing.T) {
	t.Parallel()

	p := buildRouterPrompt(config.RouterConfig{Routes: testRoutes(), Instructions: "Prefer simple."})
	require.Contains(t, p, "- simple: quick questions")
	require.Contains(t, p, "- complex: hard work")
	require.Contains(t, p, "Prefer simple.")
	require.NotContains(t, p, "{{")

	in := buildRouterInput(RouteInput{Prompt: "fix it", PrevAssistant: strings.Repeat("x", 1000), Attachments: 2})
	require.Contains(t, in, "<request>\nfix it\n</request>")
	require.Contains(t, in, "2 attachment(s)")
	require.Less(t, len(in), 700)
}

type fakeRouter struct {
	route func(ctx context.Context, in RouteInput) (RouteResult, error)
	calls int
}

func (f *fakeRouter) Route(ctx context.Context, in RouteInput) (RouteResult, error) {
	f.calls++
	return f.route(ctx, in)
}

func lastAssistant(t *testing.T, env fakeEnv, sessionID string) message.Message {
	t.Helper()
	msgs, err := env.messages.List(t.Context(), sessionID)
	require.NoError(t, err)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.Assistant {
			return msgs[i]
		}
	}
	t.Fatal("no assistant message")
	return message.Message{}
}

func TestRunUsesRoutedModel(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	routed := &finishStreamModel{text: "routed answer"}
	router := &fakeRouter{route: func(_ context.Context, in RouteInput) (RouteResult, error) {
		require.Equal(t, "hello", in.Prompt)
		return RouteResult{
			Router: "auto",
			Route:  "complex",
			Model: Model{
				Model:    routed,
				ModelCfg: config.SelectedModel{Provider: "routed", Model: "routed-model"},
			},
			ClassifierCost: 0.5,
		}, nil
	}}

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello", Router: router})
	require.NoError(t, err)
	require.Equal(t, 1, router.calls)

	msg := lastAssistant(t, env, sess.ID)
	require.Equal(t, "routed", msg.Provider)
	require.Equal(t, "routed-model", msg.Model)
	require.Equal(t, "routed answer", msg.Content().String())
	require.Equal(t, "auto", msg.RouterName)
	require.Equal(t, "complex", msg.RouteName)

	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, 0.5, updated.Cost, 1e-9, "classifier cost is charged to the session")
}

func TestRunFallsBackWhenRouterFails(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	router := &fakeRouter{route: func(context.Context, RouteInput) (RouteResult, error) {
		return RouteResult{}, errors.New("boom")
	}}
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello", Router: router})
	require.NoError(t, err)
	last := lastAssistant(t, env, sess.ID)
	require.Equal(t, "done", last.Content().String())
}

func TestRunCanceledDuringRouting(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	router := &fakeRouter{route: func(ctx context.Context, _ RouteInput) (RouteResult, error) {
		go sa.Cancel(sess.ID)
		<-ctx.Done()
		return RouteResult{}, ctx.Err()
	}}
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello", Router: router})
	require.ErrorIs(t, err, context.Canceled)
	last := lastAssistant(t, env, sess.ID)
	require.Equal(t, message.FinishReasonCanceled, last.FinishReason())
}

func TestSubAgentSkipsRouter(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)
	sa.isSubAgent = true
	sess, err := env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)

	router := &fakeRouter{route: func(context.Context, RouteInput) (RouteResult, error) {
		return RouteResult{Model: Model{Model: &finishStreamModel{text: "nope"}}}, nil
	}}
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello", Router: router})
	require.NoError(t, err)
	require.Zero(t, router.calls)
}

var _ fantasy.LanguageModel = (*finishStreamModel)(nil)
