package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestIsVaguePrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prompt string
		want   bool
	}{
		{"fix the bug", true},
		{"it crashes on startup", true},
		{"update the config", true},
		{"the test fails", true},
		{"fix the bug in internal/agent/agent.go", false},
		{"fix internal/agent/agent.go", false},
		{"", false},
		{"ls", false},
		{"add a README section explaining the project layout and how to run the tests", false},
		{"rename foo to bar everywhere in the codebase and update all the callers", false},
	}
	for _, tc := range tests {
		t.Run(tc.prompt, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isVaguePrompt(tc.prompt))
		})
	}
}

// newTurnCtxAgent builds the minimal sessionAgent the tail builders
// touch: config store, sessions, message service, filetracker, tools.
func newTurnCtxAgent(t *testing.T, cfg *config.Config) (*sessionAgent, fakeEnv, string) {
	t.Helper()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	a := &sessionAgent{
		configStore: config.NewTestStoreWithDir(cfg, env.workingDir),
		sessions:    env.sessions,
		messages:    env.messages,
		filetracker: *env.filetracker,
		tools:       csync.NewSlice[fantasy.AgentTool](),
	}
	return a, env, sess.ID
}

func userMsg(text string) message.Message {
	return message.Message{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
}

func TestAmbiguityDirective(t *testing.T) {
	t.Parallel()

	t.Run("flag off never fires", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		require.Empty(t, a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix the bug",
		}, nil))
	})

	t.Run("headless variant degrades to state-assumptions", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		d := a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix the bug",
		}, nil)
		require.Contains(t, d, "cannot ask")
		require.NotContains(t, d, "question tool")
	})

	t.Run("interactive variant routes to the question tool", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		a.tools = csync.NewSliceFrom([]fantasy.AgentTool{&fakeTool{name: tools.QuestionToolName}})
		d := a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix the bug",
		}, nil)
		require.Contains(t, d, "question tool")
	})

	t.Run("resolvable prompt does not fire", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		require.Empty(t, a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix the bug in internal/agent/agent.go",
		}, nil))
	})

	t.Run("working set suppresses the gate", func(t *testing.T) {
		t.Parallel()
		a, env, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		(*env.filetracker).RecordRead(t.Context(), sessionID, "main.go")
		require.Empty(t, a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix the bug",
		}, nil))
	})

	t.Run("earlier user text suppresses the gate", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		require.Empty(t, a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID, Prompt: "fix it",
		}, []message.Message{userMsg("auth.go panics on nil tokens")}))
	})

	t.Run("long prompt does not fire", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.ambiguityClarification = true
		require.Empty(t, a.ambiguityDirective(t.Context(), SessionAgentCall{
			SessionID: sessionID,
			Prompt:    "the bug in the auth middleware returns a 500 when the token is expired; add a refresh path and a regression test",
		}, nil))
	})
}

func TestTurnContextBlob(t *testing.T) {
	t.Parallel()

	t.Run("off by default", func(t *testing.T) {
		t.Parallel()
		a, env, sessionID := newTurnCtxAgent(t, &config.Config{})
		(*env.filetracker).RecordRead(t.Context(), sessionID, "main.go")
		require.Empty(t, a.turnContextBlob(t.Context(), SessionAgentCall{SessionID: sessionID}))
	})

	t.Run("session tier renders the working set", func(t *testing.T) {
		t.Parallel()
		a, env, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.turnContext = "session"
		(*env.filetracker).RecordRead(t.Context(), sessionID, "main.go")
		blob := a.turnContextBlob(t.Context(), SessionAgentCall{SessionID: sessionID})
		require.Contains(t, blob, "<turn_context>")
		require.Contains(t, blob, "<working_set>")
		require.Contains(t, blob, "main.go")
	})

	t.Run("session tier renders open todos", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.turnContext = "session"
		sess, err := a.sessions.Get(t.Context(), sessionID)
		require.NoError(t, err)
		sess.Todos = []session.Todo{
			{Content: "ship it", Status: session.TodoStatusPending},
			{Content: "done item", Status: session.TodoStatusCompleted},
		}
		_, err = a.sessions.Save(t.Context(), sess)
		require.NoError(t, err)
		blob := a.turnContextBlob(t.Context(), SessionAgentCall{SessionID: sessionID})
		require.Contains(t, blob, "<open_todos>")
		require.Contains(t, blob, "ship it")
		require.NotContains(t, blob, "done item")
	})

	t.Run("empty session produces no blob", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.turnContext = "session"
		require.Empty(t, a.turnContextBlob(t.Context(), SessionAgentCall{SessionID: sessionID}))
	})

	t.Run("sub-agent never emits a blob", func(t *testing.T) {
		t.Parallel()
		a, env, sessionID := newTurnCtxAgent(t, &config.Config{})
		a.turnContext = "session"
		a.isSubAgent = true
		(*env.filetracker).RecordRead(t.Context(), sessionID, "main.go")
		require.Empty(t, a.turnContextBlob(t.Context(), SessionAgentCall{SessionID: sessionID}))
	})
}

func TestTurnTailMessages(t *testing.T) {
	t.Parallel()
	a, _, sessionID := newTurnCtxAgent(t, &config.Config{})
	a.ambiguityClarification = true
	tail := a.turnTailMessages(t.Context(), SessionAgentCall{
		SessionID: sessionID, Prompt: "fix the bug",
	}, nil)
	require.Len(t, tail, 1)
	require.Equal(t, fantasy.MessageRoleSystem, tail[0].Role)
}
