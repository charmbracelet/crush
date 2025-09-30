package agent

import (
	"database/sql"
	"net/http"
	"os"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/fantasy/ai"
	"github.com/charmbracelet/fantasy/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	_ "github.com/joho/godotenv/autoload"
)

type builderFunc func(r *recorder.Recorder) (ai.LanguageModel, error)

func TestSessionSimpleAgent(t *testing.T) {
	r := newRecorder(t)
	sonnet, err := anthropicBuilder("claude-sonnet-4-5-20250929")(r)
	require.NoError(t, err)
	haiku, err := anthropicBuilder("claude-3-5-haiku-20241022")(r)
	require.NoError(t, err)
	agent, sessions, messages := testSessionAgent(t, sonnet, haiku, "You are a helpful assistant")
	session, err := sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	res, err := agent.Run(t.Context(), SessionAgentCall{
		Prompt:          "Hello",
		SessionID:       session.ID,
		MaxOutputTokens: 10000,
	})

	require.NoError(t, err)
	assert.NotNil(t, res)

	t.Run("should create session messages", func(t *testing.T) {
		msgs, err := messages.List(t.Context(), session.ID)
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

func testDBConn(t *testing.T) (*sql.DB, error) {
	return db.Connect(t.Context(), t.TempDir())
}

func testSessionAgent(t *testing.T, large, small ai.LanguageModel, systemPrompt string, tools ...ai.AgentTool) (SessionAgent, session.Service, message.Service) {
	conn, err := testDBConn(t)
	require.Nil(t, err)
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)

	largeModel := Model{
		model:  large,
		config: catwalk.Model{
			// todo: add values
		},
	}
	smallModel := Model{
		model:  large,
		config: catwalk.Model{
			// todo: add values
		},
	}
	agent := NewSessionAgent(largeModel, smallModel, systemPrompt, sessions, messages, tools...)
	return agent, sessions, messages
}
