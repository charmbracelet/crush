package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/fantasy/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/joho/godotenv/autoload"
)

var modelPairs = []modelPair{
	{"anthropic-sonnet", anthropicBuilder("claude-sonnet-4-5-20250929"), anthropicBuilder("claude-3-5-haiku-20241022")},
	{"openai-gpt-5", openaiBuilder("gpt-5"), openaiBuilder("gpt-4o")},
	{"openrouter-kimi-k2", openRouterBuilder("moonshotai/kimi-k2-0905"), openRouterBuilder("qwen/qwen3-next-80b-a3b-instruct")},
}

func getModels(t *testing.T, pair modelPair) (ai.LanguageModel, ai.LanguageModel) {
	r := newRecorder(t)
	large, err := pair.largeModel(t, r)
	require.NoError(t, err)
	small, err := pair.smallModel(t, r)
	require.NoError(t, err)
	return large, small
}

func setupAgent(t *testing.T, pair modelPair) (SessionAgent, env) {
	large, small := getModels(t, pair)
	env := testEnv(t)

	createSimpleGoProject(t, env.workingDir)
	agent, err := coderAgent(env, large, small)
	require.NoError(t, err)
	return agent, env
}

func TestCoderAgent(t *testing.T) {
	for _, pair := range modelPairs {
		t.Run(pair.name, func(t *testing.T) {
			t.Run("simple test", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

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
			t.Run("read a file", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)
				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "Read the go mod",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})

				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)
				foundFile := false
				var tcID string
			out:
				for _, msg := range msgs {
					data, _ := json.Marshal(msg)
					fmt.Println(string(data))
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == tools.ViewToolName {
								tcID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == tcID {
								if strings.Contains(tr.Content, "module example.com/testproject") {
									foundFile = true
									break out
								}
							}
						}
					}
				}
				require.True(t, foundFile)
			})
		})
	}
}
