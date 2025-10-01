package agent

import (
	"net/http"
	"os"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/fantasy/ai"
	"github.com/charmbracelet/fantasy/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	_ "github.com/joho/godotenv/autoload"
)

type env struct {
	workingDir  string
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
}

type builderFunc func(r *recorder.Recorder) (ai.LanguageModel, error)

func TestSessionAgent(t *testing.T) {
	t.Run("simple test", func(t *testing.T) {
		r := newRecorder(t)
		sonnet, err := anthropicBuilder("claude-sonnet-4-5-20250929")(r)
		require.NoError(t, err)
		haiku, err := anthropicBuilder("claude-3-5-haiku-20241022")(r)
		require.NoError(t, err)

		env := testEnv(t)
		agent := testSessionAgent(env, sonnet, haiku, "You are a helpful assistant")
		session, err := env.sessions.Create(t.Context(), "New Session")
		require.NoError(t, err)

		res, err := agent.Run(t.Context(), SessionAgentCall{
			Prompt:          "Hello",
			SessionID:       session.ID,
			MaxOutputTokens: 10000,
		})

		require.NoError(t, err)
		assert.NotNil(t, res)

		t.Run("should create session messages", func(t *testing.T) {
			msgs, err := env.messages.List(t.Context(), session.ID)
			require.NoError(t, err)
			// Should have the agent and user message
			assert.Equal(t, len(msgs), 2)
		})
	})
}

func TestCoderAgent(t *testing.T) {
	t.Run("simple test", func(t *testing.T) {
		r := newRecorder(t)
		sonnet, err := anthropicBuilder("claude-sonnet-4-5-20250929")(r)
		require.NoError(t, err)
		haiku, err := anthropicBuilder("claude-3-5-haiku-20241022")(r)
		require.NoError(t, err)

		env := testEnv(t)
		agent, err := coderAgent(env, sonnet, haiku)
		require.NoError(t, err)
		session, err := env.sessions.Create(t.Context(), "New Session")
		require.NoError(t, err)

		res, err := agent.Run(t.Context(), SessionAgentCall{
			Prompt:          "Hello",
			SessionID:       session.ID,
			MaxOutputTokens: 10000,
		})

		require.NoError(t, err)
		assert.NotNil(t, res)

		msgs, err := env.messages.List(t.Context(), session.ID)
		require.NoError(t, err)
		// Should have the agent and user message
		assert.Equal(t, len(msgs), 2)
	})
}

func anthropicBuilder(model string) builderFunc {
	return func(r *recorder.Recorder) (ai.LanguageModel, error) {
		provider := anthropic.New(
			anthropic.WithAPIKey(os.Getenv("CRUSH_ANTHROPIC_API_KEY")),
			anthropic.WithHTTPClient(&http.Client{Transport: r}),
		)
		return provider.LanguageModel(model)
	}
}

func testEnv(t *testing.T) env {
	workingDir := t.TempDir()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	permissions := permission.NewPermissionService(workingDir, true, []string{})
	history := history.NewService(q, conn)
	lspClients := csync.NewMap[string, *lsp.Client]()
	return env{
		workingDir,
		sessions,
		messages,
		permissions,
		history,
		lspClients,
	}
}

func testSessionAgent(env env, large, small ai.LanguageModel, systemPrompt string, tools ...ai.AgentTool) SessionAgent {
	largeModel := Model{
		model:  large,
		config: catwalk.Model{
			// todo: add values
		},
	}
	smallModel := Model{
		model:  small,
		config: catwalk.Model{
			// todo: add values
		},
	}
	agent := NewSessionAgent(largeModel, smallModel, systemPrompt, env.sessions, env.messages, tools...)
	return agent
}

func coderAgent(env env, large, small ai.LanguageModel) (SessionAgent, error) {
	prompt, err := coderPrompt()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Init(env.workingDir, "", false)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := prompt.Build(large.Provider(), large.Model(), *cfg)
	if err != nil {
		return nil, err
	}
	allTools := []ai.AgentTool{
		tools.NewBashTool(env.permissions, env.workingDir, cfg.Options.Attribution),
		tools.NewDownloadTool(env.permissions, env.workingDir),
		tools.NewEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewMultiEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewFetchTool(env.permissions, env.workingDir),
		tools.NewGlobTool(env.workingDir),
		tools.NewGrepTool(env.workingDir),
		tools.NewLsTool(env.permissions, env.workingDir),
		tools.NewSourcegraphTool(),
		tools.NewViewTool(env.lspClients, env.permissions, env.workingDir),
		tools.NewWriteTool(env.lspClients, env.permissions, env.history, env.workingDir),
	}

	return testSessionAgent(env, large, small, systemPrompt, allTools...), nil
}
