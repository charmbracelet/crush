package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/x/vcr"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/joho/godotenv/autoload"
)

var modelPairs = []modelPair{
	{"anthropic-sonnet", anthropicBuilder("claude-sonnet-4-6"), anthropicBuilder("claude-haiku-4-5-20251001")},
	{"openai-gpt-5", openaiBuilder("gpt-5"), openaiBuilder("gpt-4o")},
	{"openrouter-kimi-k2", openRouterBuilder("moonshotai/kimi-k2-0905"), openRouterBuilder("qwen/qwen3-next-80b-a3b-instruct")},
	{"zai-glm4.6", zAIBuilder("glm-4.6"), zAIBuilder("glm-4.5-air")},
}

func getModels(t *testing.T, r *vcr.Recorder, pair modelPair) (fantasy.LanguageModel, fantasy.LanguageModel) {
	large, err := pair.largeModel(t, r)
	require.NoError(t, err)
	small, err := pair.smallModel(t, r)
	require.NoError(t, err)
	return large, small
}

func setupAgent(t *testing.T, pair modelPair) (SessionAgent, fakeEnv) {
	r := vcr.NewRecorder(t)
	large, small := getModels(t, r, pair)
	env := testEnv(t)

	createSimpleGoProject(t, env.workingDir)
	agent, err := coderAgentNoTitle(r, env, large, small)
	require.NoError(t, err)
	return agent, env
}

func TestCoderAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

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
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.ViewToolName {
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
			t.Run("update a file", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "update the main.go file by changing the print to say hello from crush",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundRead := false
				foundWrite := false
				var readTCID, writeTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.ViewToolName {
								readTCID = tc.ID
							}
							if tc.Name == agenttools.EditToolName || tc.Name == agenttools.WriteToolName {
								writeTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == readTCID {
								foundRead = true
							}
							if tr.ToolCallID == writeTCID {
								foundWrite = true
							}
						}
					}
				}

				require.True(t, foundRead, "Expected to find a read operation")
				require.True(t, foundWrite, "Expected to find a write operation")

				mainGoPath := filepath.Join(env.workingDir, "main.go")
				content, err := os.ReadFile(mainGoPath)
				require.NoError(t, err)
				require.Contains(t, strings.ToLower(string(content)), "hello from crush")
			})
			t.Run("bash tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use bash to create a file named test.txt with content 'hello bash'. do not print its timestamp",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundBash := false
				var bashTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.BashToolName {
								bashTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == bashTCID {
								foundBash = true
							}
						}
					}
				}

				require.True(t, foundBash, "Expected to find a bash operation")

				testFilePath := filepath.Join(env.workingDir, "test.txt")
				content, err := os.ReadFile(testFilePath)
				require.NoError(t, err)
				require.Contains(t, string(content), "hello bash")
			})
			t.Run("download tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "download the file from https://example-files.online-convert.com/document/txt/example.txt and save it as example.txt",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundDownload := false
				var downloadTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.DownloadToolName {
								downloadTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == downloadTCID {
								foundDownload = true
							}
						}
					}
				}

				require.True(t, foundDownload, "Expected to find a download operation")

				examplePath := filepath.Join(env.workingDir, "example.txt")
				_, err = os.Stat(examplePath)
				require.NoError(t, err, "Expected example.txt file to exist")
			})
			t.Run("fetch tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "fetch the content from https://example-files.online-convert.com/website/html/example.html and tell me if it contains the word 'John Doe'",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundFetch := false
				var fetchTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.FetchToolName {
								fetchTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == fetchTCID {
								foundFetch = true
							}
						}
					}
				}

				require.True(t, foundFetch, "Expected to find a fetch operation")
			})
			t.Run("glob tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use glob to find all .go files in the current directory",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundGlob := false
				var globTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.GlobToolName {
								globTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == globTCID {
								foundGlob = true
								require.Contains(t, tr.Content, "main.go", "Expected glob to find main.go")
							}
						}
					}
				}

				require.True(t, foundGlob, "Expected to find a glob operation")
			})
			t.Run("grep tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use grep to search for the word 'package' in go files",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundGrep := false
				var grepTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.GrepToolName {
								grepTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == grepTCID {
								foundGrep = true
								require.Contains(t, tr.Content, "main.go", "Expected grep to find main.go")
							}
						}
					}
				}

				require.True(t, foundGrep, "Expected to find a grep operation")
			})
			t.Run("ls tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use ls to list the files in the current directory",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundLS := false
				var lsTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.LSToolName {
								lsTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == lsTCID {
								foundLS = true
								require.Contains(t, tr.Content, "main.go", "Expected ls to list main.go")
								require.Contains(t, tr.Content, "go.mod", "Expected ls to list go.mod")
							}
						}
					}
				}

				require.True(t, foundLS, "Expected to find an ls operation")
			})
			t.Run("multiedit tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use multiedit to change 'Hello, World!' to 'Hello, Crush!' and add a comment '// Greeting' above the fmt.Println line in main.go",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundMultiEdit := false
				var multiEditTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.MultiEditToolName {
								multiEditTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == multiEditTCID {
								foundMultiEdit = true
							}
						}
					}
				}

				require.True(t, foundMultiEdit, "Expected to find a multiedit operation")

				mainGoPath := filepath.Join(env.workingDir, "main.go")
				content, err := os.ReadFile(mainGoPath)
				require.NoError(t, err)
				require.Contains(t, string(content), "Hello, Crush!", "Expected file to contain 'Hello, Crush!'")
			})
			t.Run("sourcegraph tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use sourcegraph to search for 'func main' in Go repositories",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundSourcegraph := false
				var sourcegraphTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.SourcegraphToolName {
								sourcegraphTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == sourcegraphTCID {
								foundSourcegraph = true
							}
						}
					}
				}

				require.True(t, foundSourcegraph, "Expected to find a sourcegraph operation")
			})
			t.Run("write tool", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use write to create a new file called config.json with content '{\"name\": \"test\", \"version\": \"1.0.0\"}'",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				foundWrite := false
				var writeTCID string

				for _, msg := range msgs {
					if msg.Role == message.Assistant {
						for _, tc := range msg.ToolCalls() {
							if tc.Name == agenttools.WriteToolName {
								writeTCID = tc.ID
							}
						}
					}
					if msg.Role == message.Tool {
						for _, tr := range msg.ToolResults() {
							if tr.ToolCallID == writeTCID {
								foundWrite = true
							}
						}
					}
				}

				require.True(t, foundWrite, "Expected to find a write operation")

				configPath := filepath.Join(env.workingDir, "config.json")
				content, err := os.ReadFile(configPath)
				require.NoError(t, err)
				require.Contains(t, string(content), "test", "Expected config.json to contain 'test'")
				require.Contains(t, string(content), "1.0.0", "Expected config.json to contain '1.0.0'")
			})
			t.Run("parallel tool calls", func(t *testing.T) {
				agent, env := setupAgent(t, pair)

				session, err := env.sessions.Create(t.Context(), "New Session")
				require.NoError(t, err)

				res, err := agent.Run(t.Context(), SessionAgentCall{
					Prompt:          "use glob to find all .go files and use ls to list the current directory, it is very important that you run both tool calls in parallel",
					SessionID:       session.ID,
					MaxOutputTokens: 10000,
				})
				require.NoError(t, err)
				assert.NotNil(t, res)

				msgs, err := env.messages.List(t.Context(), session.ID)
				require.NoError(t, err)

				var assistantMsg *message.Message
				var toolMsgs []message.Message

				for _, msg := range msgs {
					if msg.Role == message.Assistant && len(msg.ToolCalls()) > 0 {
						assistantMsg = &msg
					}
					if msg.Role == message.Tool {
						toolMsgs = append(toolMsgs, msg)
					}
				}

				require.NotNil(t, assistantMsg, "Expected to find an assistant message with tool calls")
				require.NotNil(t, toolMsgs, "Expected to find a tool message")

				toolCalls := assistantMsg.ToolCalls()
				require.GreaterOrEqual(t, len(toolCalls), 2, "Expected at least 2 tool calls in parallel")

				foundGlob := false
				foundLS := false
				var globTCID, lsTCID string

				for _, tc := range toolCalls {
					if tc.Name == agenttools.GlobToolName {
						foundGlob = true
						globTCID = tc.ID
					}
					if tc.Name == agenttools.LSToolName {
						foundLS = true
						lsTCID = tc.ID
					}
				}

				require.True(t, foundGlob, "Expected to find a glob tool call")
				require.True(t, foundLS, "Expected to find an ls tool call")

				require.GreaterOrEqual(t, len(toolMsgs), 2, "Expected at least 2 tool results in the same message")

				foundGlobResult := false
				foundLSResult := false

				for _, msg := range toolMsgs {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == globTCID {
							foundGlobResult = true
							require.Contains(t, tr.Content, "main.go", "Expected glob result to contain main.go")
							require.False(t, tr.IsError, "Expected glob result to not be an error")
						}
						if tr.ToolCallID == lsTCID {
							foundLSResult = true
							require.Contains(t, tr.Content, "main.go", "Expected ls result to contain main.go")
							require.False(t, tr.IsError, "Expected ls result to not be an error")
						}
					}
				}

				require.True(t, foundGlobResult, "Expected to find glob tool result")
				require.True(t, foundLSResult, "Expected to find ls tool result")
			})
		})
	}
}

func makeTestTodos(n int) []session.Todo {
	todos := make([]session.Todo, n)
	for i := range n {
		todos[i] = session.Todo{
			Status:  session.TodoStatusPending,
			Content: fmt.Sprintf("Task %d: Implement feature with some description that makes it realistic", i),
		}
	}
	return todos
}

func BenchmarkBuildSummaryPrompt(b *testing.B) {
	cases := []struct {
		name     string
		numTodos int
	}{
		{"0todos", 0},
		{"5todos", 5},
		{"10todos", 10},
		{"50todos", 50},
	}

	for _, tc := range cases {
		todos := makeTestTodos(tc.numTodos)

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = buildSummaryPrompt(todos)
			}
		})
	}
}

func TestBuildSummaryPromptIncludesTrackedTasksWithoutTodoInstructions(t *testing.T) {
	t.Parallel()

	prompt := buildSummaryPrompt([]session.Todo{{
		Status:  session.TodoStatusInProgress,
		Content: "Investigate delegation bias",
	}})

	require.Contains(t, prompt, "## Tracked Tasks")
	require.Contains(t, prompt, "[in_progress] Investigate delegation bias")
	require.NotContains(t, prompt, "use the `todos` tool")
}

func TestPromptTokensForUsage_OpenAIStyle(t *testing.T) {
	t.Parallel()

	usage := fantasy.Usage{
		InputTokens:  120,
		OutputTokens: 45,
	}

	require.Equal(t, int64(120), promptTokensForUsage(usage, "openai"))
	require.Equal(t, int64(165), totalTokensForUsage(usage, "openai"))
}

func TestPromptTokensForUsage_AnthropicCacheStyle(t *testing.T) {
	t.Parallel()

	usage := fantasy.Usage{
		InputTokens:         120,
		CacheCreationTokens: 300,
		CacheReadTokens:     900,
		OutputTokens:        45,
	}

	// Anthropic-style: InputTokens does NOT include cached tokens
	require.Equal(t, int64(1320), promptTokensForUsage(usage, "anthropic"))
	require.Equal(t, int64(1365), totalTokensForUsage(usage, "anthropic"))
	require.Equal(t, int64(1365), totalTokensForUsage(usage, "@ai-sdk/anthropic"))
	require.Equal(t, int64(1365), totalTokensForUsage(usage, "@ai-sdk/google-vertex/anthropic"))

	// OpenAI-style: InputTokens ALREADY includes cached tokens
	// So we don't add CacheReadTokens again (would be double-counting)
	require.Equal(t, int64(420), promptTokensForUsage(usage, "openai")) // 120 + 300 (CacheCreation) + 0 (Reasoning)
}

func TestShouldAutoSummarize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		contextUsed     int64
		contextWindow   int64
		maxOutputTokens int64
		want            bool
	}{
		{name: "large window uses hard reserve threshold", contextUsed: 168_000, contextWindow: 200_000, maxOutputTokens: 50_000, want: true},
		{name: "large window below hard reserve threshold", contextUsed: 167_999, contextWindow: 200_000, maxOutputTokens: 50_000, want: false},
		{name: "small window reserves output tool and safety budget", contextUsed: 18_800, contextWindow: 32_000, maxOutputTokens: 8_000, want: true},
		{name: "small window below mixed threshold", contextUsed: 18_799, contextWindow: 32_000, maxOutputTokens: 8_000, want: false},
		{name: "soft limit caps very large windows at ninety percent", contextUsed: 450_000, contextWindow: 500_000, maxOutputTokens: 1_000, want: true},
		{name: "soft limit still leaves headroom below ninety percent", contextUsed: 449_999, contextWindow: 500_000, maxOutputTokens: 1_000, want: false},
		{name: "invalid context window", contextUsed: 1, contextWindow: 0, maxOutputTokens: 8_000, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, shouldAutoSummarize(tt.contextUsed, tt.contextWindow, tt.maxOutputTokens))
		})
	}
}

func TestRefreshCallConfigIfNeeded_UsesContextRuntimeConfig(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{MaxOutputTokens: 1}
	runtimeConfig := sessionAgentRuntimeConfig{MaxOutputTokens: 2048}
	agent := &sessionAgent{
		refreshCallConfig: func(context.Context) (sessionAgentRuntimeConfig, error) {
			return sessionAgentRuntimeConfig{MaxOutputTokens: 4096}, nil
		},
	}

	ctx := context.WithValue(context.Background(), sessionAgentRuntimeConfigContextKey{}, runtimeConfig)
	runtimeConfigPtr, err := agent.refreshCallConfigIfNeeded(ctx, &call)
	require.NoError(t, err)
	require.NotNil(t, runtimeConfigPtr)
	require.Equal(t, int64(2048), call.MaxOutputTokens)
}

func TestRefreshCallConfigIfNeeded_IgnoresNilPointerContextAndRefreshes(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{MaxOutputTokens: 1}
	agent := &sessionAgent{
		refreshCallConfig: func(context.Context) (sessionAgentRuntimeConfig, error) {
			return sessionAgentRuntimeConfig{MaxOutputTokens: 4096}, nil
		},
	}

	ctx := context.WithValue(context.Background(), sessionAgentRuntimeConfigContextKey{}, (*sessionAgentRuntimeConfig)(nil))
	runtimeConfigPtr, err := agent.refreshCallConfigIfNeeded(ctx, &call)
	require.NoError(t, err)
	require.NotNil(t, runtimeConfigPtr)
	require.Equal(t, int64(4096), call.MaxOutputTokens)
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func ptrAddress[T any](p *T) uintptr {
	if p == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(p))
}

func TestApplyRuntimeConfig_OnlyOverridesExplicitPointerFields(t *testing.T) {
	t.Parallel()

	temperature := 0.7
	topP := 0.9
	topK := int64(50)
	frequencyPenalty := 0.2
	presencePenalty := 0.3

	call := SessionAgentCall{
		MaxOutputTokens:  256,
		Temperature:      float64Ptr(0.1),
		TopP:             float64Ptr(0.2),
		TopK:             int64Ptr(3),
		FrequencyPenalty: float64Ptr(-0.1),
		PresencePenalty:  float64Ptr(-0.2),
	}

	applyRuntimeConfig(&call, sessionAgentRuntimeConfig{
		MaxOutputTokens:  1024,
		Temperature:      &temperature,
		TopP:             &topP,
		TopK:             &topK,
		FrequencyPenalty: &frequencyPenalty,
		PresencePenalty:  &presencePenalty,
	})

	require.Equal(t, int64(1024), call.MaxOutputTokens)
	require.Equal(t, ptrAddress(&temperature), ptrAddress(call.Temperature))
	require.Equal(t, ptrAddress(&topP), ptrAddress(call.TopP))
	require.Equal(t, ptrAddress(&topK), ptrAddress(call.TopK))
	require.Equal(t, ptrAddress(&frequencyPenalty), ptrAddress(call.FrequencyPenalty))
	require.Equal(t, ptrAddress(&presencePenalty), ptrAddress(call.PresencePenalty))

	existingTemp := call.Temperature
	existingTopP := call.TopP
	existingTopK := call.TopK
	existingFreq := call.FrequencyPenalty
	existingPresence := call.PresencePenalty

	applyRuntimeConfig(&call, sessionAgentRuntimeConfig{})
	require.Equal(t, int64(1024), call.MaxOutputTokens)
	require.Equal(t, ptrAddress(existingTemp), ptrAddress(call.Temperature))
	require.Equal(t, ptrAddress(existingTopP), ptrAddress(call.TopP))
	require.Equal(t, ptrAddress(existingTopK), ptrAddress(call.TopK))
	require.Equal(t, ptrAddress(existingFreq), ptrAddress(call.FrequencyPenalty))
	require.Equal(t, ptrAddress(existingPresence), ptrAddress(call.PresencePenalty))
}

func TestUpdateSessionUsage_AccumulatesTotals(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{
		PromptTokens:     1000,
		CompletionTokens: 400,
		Cost:             1.25,
	}

	usage := fantasy.Usage{
		InputTokens:         120,
		CacheCreationTokens: 300,
		CacheReadTokens:     900,
		OutputTokens:        45,
	}

	agent.updateSessionUsage(model, &sess, usage, nil, 0)

	// For Anthropic: promptTokens = InputTokens + CacheCreationTokens + CacheReadTokens
	require.Equal(t, int64(2320), sess.PromptTokens) // 1000 + (120 + 300 + 900)
	require.Equal(t, int64(445), sess.CompletionTokens)
	require.GreaterOrEqual(t, sess.Cost, 1.25)
	// LastPromptTokens should reflect only this step's input tokens (SET, not +=).
	require.Equal(t, int64(1320), sess.LastPromptTokens) // 120 + 300 + 900
}

func TestUpdateSessionUsage_AccumulatesTotals_AnthropicSDKProviderName(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{
		CatwalkCfg: catwalk.Model{},
		ModelCfg:   config.SelectedModel{Provider: "openai"},
		Model:      anthropicProviderLanguageModel{},
	}
	sess := session.Session{}

	usage := fantasy.Usage{
		InputTokens:         120,
		CacheCreationTokens: 300,
		CacheReadTokens:     900,
		OutputTokens:        45,
	}

	agent.updateSessionUsage(model, &sess, usage, nil, 0)

	// Model.Provider() = "@ai-sdk/anthropic" should be treated as Anthropic-style usage.
	require.Equal(t, int64(1320), sess.PromptTokens)
	require.Equal(t, int64(1320), sess.LastPromptTokens)
	require.Equal(t, int64(45), sess.CompletionTokens)
}

func TestUpdateSessionUsage_LastPromptTokensIsSetNotAccumulated(t *testing.T) {
	t.Parallel()

	// Verify that LastPromptTokens always reflects the MOST RECENT step's
	// input tokens, not a cumulative sum. This is used for the context
	// window display and summarization threshold.
	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{}

	firstUsage := fantasy.Usage{
		InputTokens:  15000,
		OutputTokens: 200,
	}
	secondUsage := fantasy.Usage{
		InputTokens:  15300,
		OutputTokens: 180,
	}

	agent.updateSessionUsage(model, &sess, firstUsage, nil, 0)
	require.Equal(t, int64(15000), sess.LastPromptTokens)
	require.Equal(t, int64(15000), sess.PromptTokens)

	agent.updateSessionUsage(model, &sess, secondUsage, nil, 0)
	// PromptTokens accumulates across steps (used for billing).
	require.Equal(t, int64(30300), sess.PromptTokens)
	// LastPromptTokens reflects only the second step (used for display/StopWhen).
	require.Equal(t, int64(15300), sess.LastPromptTokens)
}

func TestUpdateSessionUsage_FallbackToEstimatedTokens(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{}

	// Simulate a proxy that doesn't report input tokens in streaming mode.
	usage := fantasy.Usage{
		InputTokens:         0,
		CacheCreationTokens: 0,
		CacheReadTokens:     0,
		OutputTokens:        95,
	}

	estimatedTokens := int64(4200)
	agent.updateSessionUsage(model, &sess, usage, nil, estimatedTokens)

	// When API reports 0 input tokens, the estimated value should be used.
	require.Equal(t, estimatedTokens, sess.LastPromptTokens)
	// PromptTokens should also use the estimate.
	require.Equal(t, estimatedTokens, sess.PromptTokens)
	require.Equal(t, int64(95), sess.CompletionTokens)
}

func TestUpdateSessionUsage_PreferAPIOverEstimate(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{}

	// Real Anthropic correctly reports input tokens.
	usage := fantasy.Usage{
		InputTokens:         68,
		CacheCreationTokens: 4185,
		CacheReadTokens:     0,
		OutputTokens:        95,
	}

	estimatedTokens := int64(3500) // estimate is less accurate
	agent.updateSessionUsage(model, &sess, usage, nil, estimatedTokens)

	// API value (68+4185=4253) should be preferred over the estimate (3500)
	// because the API value exceeds the estimate.
	require.Equal(t, int64(4253), sess.LastPromptTokens)
	require.Equal(t, int64(4253), sess.PromptTokens)
}

func TestUpdateSessionUsage_FallbackWhenAPIUnderReports(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{}

	// Simulate a proxy that under-reports input tokens (e.g., only user
	// message tokens, omitting system prompt and tool definitions).
	usage := fantasy.Usage{
		InputTokens:  95,
		OutputTokens: 200,
	}

	estimatedTokens := int64(18500) // includes system prompt + tools
	agent.updateSessionUsage(model, &sess, usage, nil, estimatedTokens)

	// API reports 95 which is < estimatedTokens (18500), so the
	// estimate should be used instead.
	require.Equal(t, estimatedTokens, sess.LastPromptTokens)
	require.Equal(t, estimatedTokens, sess.PromptTokens)
}

func TestUpdateSessionUsage_FallbackWhenAPIReportsStaleValue(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	model := Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{Provider: "anthropic"}}
	sess := session.Session{}

	// Simulate a proxy that returns a constant (stale) input token count
	// that does not grow across tool-call steps.
	usage := fantasy.Usage{
		InputTokens:  5000,
		OutputTokens: 200,
	}

	estimatedTokens := int64(8000) // estimate grew with messages
	agent.updateSessionUsage(model, &sess, usage, nil, estimatedTokens)

	// API reports 5000 which is less than the estimate (8000). The
	// estimate should be used to keep the context display growing.
	require.Equal(t, estimatedTokens, sess.LastPromptTokens)
	require.Equal(t, estimatedTokens, sess.PromptTokens)
}

func TestEstimatePromptTokens(t *testing.T) {
	t.Parallel()

	messages := []fantasy.Message{
		fantasy.NewSystemMessage(strings.Repeat("x", 3000)), // 3000 bytes
		fantasy.NewUserMessage("Hello world"),               // 11 bytes
	}

	// No agenttools. (3000 + 11) / 4 = 752.
	estimate := estimatePromptTokens(messages, nil)
	require.Equal(t, int64(752), estimate)

	// With a mock tool: name(9) + desc(31) + schema("null"=4) = 10 estimated
	// tokens, so total is 752 + 10 = 762.
	tool := &mockAgentTool{
		name:        "read_file",
		description: "Read a file from the filesystem",
	}
	estimateWithTools := estimatePromptTokens(messages, []fantasy.AgentTool{tool})
	require.Equal(t, int64(762), estimateWithTools)

	// ToolCallPart input is counted.
	msgs2 := []fantasy.Message{
		{
			Role:    fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{fantasy.ToolCallPart{Input: `{"path":"file.go"}`}}, // 19 bytes
		},
	}
	// 19 / 4 = 4.
	require.Equal(t, int64(4), estimatePromptTokens(msgs2, nil))

	// ToolResultPart text is counted.
	msgs3 := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{fantasy.ToolResultPart{
				Output: fantasy.ToolResultOutputContentText{Text: strings.Repeat("a", 400)}, // 400 bytes
			}},
		},
	}
	// 400 / 4 = 100.
	require.Equal(t, int64(100), estimatePromptTokens(msgs3, nil))

	// ToolResultPart media payloads use fixed image token estimate plus metadata text.
	msgsMedia := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{fantasy.ToolResultPart{
				Output: fantasy.ToolResultOutputContentMedia{
					Data:      strings.Repeat("a", 64),
					MediaType: "image/png",
					Text:      "preview",
				},
			}},
		},
	}
	// image(2000) + media type(9)/4 + text(7)/4 => 2004 tokens.
	require.Equal(t, int64(2004), estimatePromptTokens(msgsMedia, nil))

	// FilePart attachments use fixed image token estimate plus metadata text.
	msgsFile := []fantasy.Message{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.FilePart{
				Filename:  "img.png",
				MediaType: "image/png",
				Data:      []byte("12345678"),
			}},
		},
	}
	// image(2000) + filename(7)/4 + media type(9)/4 => 2004 tokens.
	require.Equal(t, int64(2004), estimatePromptTokens(msgsFile, nil))

	msgs4 := []fantasy.Message{
		fantasy.NewUserMessage("你好世界"),
	}
	require.Equal(t, int64(4), estimatePromptTokens(msgs4, nil))
}

func TestEstimatePromptTokens_AggregatesShortASCIIFragments(t *testing.T) {
	t.Parallel()

	shortFragments := make([]fantasy.MessagePart, 8)
	for i := range shortFragments {
		shortFragments[i] = fantasy.TextPart{Text: "abc"}
	}

	messages := []fantasy.Message{
		{
			Role:    fantasy.MessageRoleUser,
			Content: shortFragments,
		},
	}

	require.Equal(t, int64(6), estimatePromptTokens(messages, nil))
}

func TestEstimatePromptTokens_ImageTokenEstimation(t *testing.T) {
	t.Parallel()

	// Non-image media types should not get the fixed image token estimate.
	msgsPDF := []fantasy.Message{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.FilePart{
				Filename:  "doc.pdf",
				MediaType: "application/pdf",
				Data:      []byte(strings.Repeat("x", 100)),
			}},
		},
	}
	// 100 bytes / 4 = 25 tokens for data, plus filename(7)/4 + media type(15)/4 = 30.
	require.Equal(t, int64(30), estimatePromptTokens(msgsPDF, nil))

	// Empty image data should not count as an image.
	msgsEmpty := []fantasy.Message{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.FilePart{
				Filename:  "empty.png",
				MediaType: "image/png",
				Data:      nil,
			}},
		},
	}
	// Only filename(9)/4 + media type(9)/4 = 4 tokens (integer division).
	require.Equal(t, int64(4), estimatePromptTokens(msgsEmpty, nil))

	// Multiple images should each get the fixed estimate.
	msgsMulti := []fantasy.Message{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.FilePart{
					Filename:  "img1.png",
					MediaType: "image/png",
					Data:      []byte(strings.Repeat("a", 10000)),
				},
				fantasy.FilePart{
					Filename:  "img2.jpg",
					MediaType: "image/jpeg",
					Data:      []byte(strings.Repeat("b", 20000)),
				},
			},
		},
	}
	// 2 images * 2000 = 4000, plus filename(7)/4 + media type(9)/4 * 2 = 8 => 4008.
	require.Equal(t, int64(4008), estimatePromptTokens(msgsMulti, nil))
}

// mockAgentTool implements fantasy.AgentTool for testing.
type mockAgentTool struct {
	name        string
	description string
	parallel    bool
}

func (m *mockAgentTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        m.name,
		Description: m.description,
		Parallel:    m.parallel,
	}
}

func (m *mockAgentTool) Run(_ context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{}, nil
}

func (m *mockAgentTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (m *mockAgentTool) SetProviderOptions(_ fantasy.ProviderOptions) {}
func (m *mockAgentTool) SetParallel(parallel bool)                    { m.parallel = parallel }

type anthropicProviderLanguageModel struct {
	stubLanguageModel
}

func (anthropicProviderLanguageModel) Provider() string {
	return "@ai-sdk/anthropic"
}

func (anthropicProviderLanguageModel) Model() string {
	return "test-model"
}

func TestEnableNativeToolParallelism(t *testing.T) {
	t.Parallel()

	t.Run("enables parallel for read-only concurrency-safe tools", func(t *testing.T) {
		t.Parallel()
		tool := &mockAgentTool{name: "glob", description: "find files"}
		enableNativeToolParallelism(tool, agenttools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true})
		require.True(t, tool.Info().Parallel)
	})

	t.Run("keeps write tools sequential", func(t *testing.T) {
		t.Parallel()
		tool := &mockAgentTool{name: "edit", description: "modify files"}
		enableNativeToolParallelism(tool, agenttools.ToolMetadata{ReadOnly: false, ConcurrencySafe: true})
		require.False(t, tool.Info().Parallel)
	})
}

func TestTitleUserPromptFromCall(t *testing.T) {
	t.Parallel()

	t.Run("returns plain prompt", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "hello world", titleUserPromptFromCall("hello world"))
	})

	t.Run("extracts auto resume original prompt", func(t *testing.T) {
		t.Parallel()
		wrapped := autoResumePromptPrefix + "please fix bug`"
		require.Equal(t, "please fix bug", titleUserPromptFromCall(wrapped))
	})

	t.Run("extracts context window resume original prompt", func(t *testing.T) {
		t.Parallel()
		wrapped := contextWindowResumePromptPrefix + "analyze logs quickly`"
		require.Equal(t, "analyze logs quickly", titleUserPromptFromCall(wrapped))
	})

	t.Run("returns empty for blank prompt", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, titleUserPromptFromCall("   \n\t  "))
	})
}

func TestShouldGenerateSessionTitle(t *testing.T) {
	t.Parallel()

	require.True(t, shouldGenerateSessionTitle(""))
	require.True(t, shouldGenerateSessionTitle("New Session"))
	require.True(t, shouldGenerateSessionTitle("new session"))
	require.True(t, shouldGenerateSessionTitle(DefaultSessionName))
	require.False(t, shouldGenerateSessionTitle("Bugfix summary"))
}

func TestTitlePromptFromCallOrHistory(t *testing.T) {
	t.Parallel()

	t.Run("prefers current call prompt", func(t *testing.T) {
		t.Parallel()
		history := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "older prompt"}}}}
		require.Equal(t, "latest prompt", titlePromptFromCallOrHistory("latest prompt", history))
	})

	t.Run("falls back to latest user history when call prompt empty", func(t *testing.T) {
		t.Parallel()
		history := []message.Message{
			{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "assistant"}}},
			{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "first prompt"}}},
			{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "latest user prompt"}}},
		}
		require.Equal(t, "latest user prompt", titlePromptFromCallOrHistory("", history))
	})

	t.Run("returns empty when no usable prompt", func(t *testing.T) {
		t.Parallel()
		history := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "assistant"}}}}
		require.Empty(t, titlePromptFromCallOrHistory("   ", history))
	})
}

func TestGenerateTitleResetsStreamedTitleOnModelFallback(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	streamCalls := 0
	titleModel := stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			streamCalls++
			if streamCalls == 1 {
				return func(yield func(fantasy.StreamPart) bool) {
					if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "title"}) {
						return
					}
					if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "title", Delta: "partial-"}) {
						return
					}
					yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: fmt.Errorf("small model failed")})
				}, nil
			}
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "title"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "title", Delta: "clean-title"}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			}, nil
		},
	}

	model := Model{
		Model: titleModel,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 1000,
		},
		ModelCfg: config.SelectedModel{
			Model:    "claude-sonnet-4",
			Provider: "anthropic",
		},
	}

	a := NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	})

	a.(*sessionAgent).generateTitle(t.Context(), testSession.ID, "user prompt", nil)

	after, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.Equal(t, "clean-title", after.Title)
	require.Equal(t, 2, streamCalls)
}

func TestGenerateTitleDoesNotOverwriteSessionUsage(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	orig := testSession
	orig.PromptTokens = 321
	orig.CompletionTokens = 654
	orig.Cost = 12.34
	orig.LastPromptTokens = 11
	orig.LastCompletionTokens = 22
	_, err = env.sessions.Save(t.Context(), orig)
	require.NoError(t, err)

	titleModel := stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "title"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "title", Delta: "kept-usage-title"}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			}, nil
		},
	}

	model := Model{
		Model: titleModel,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 1000,
		},
		ModelCfg: config.SelectedModel{
			Model:    "claude-sonnet-4",
			Provider: "anthropic",
		},
	}

	a := NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	})

	a.(*sessionAgent).generateTitle(t.Context(), testSession.ID, "user prompt", nil)

	after, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.Equal(t, "kept-usage-title", after.Title)
	require.Equal(t, orig.PromptTokens, after.PromptTokens)
	require.Equal(t, orig.CompletionTokens, after.CompletionTokens)
	require.InDelta(t, orig.Cost, after.Cost, 1e-9)
	require.Equal(t, orig.LastPromptTokens, after.LastPromptTokens)
	require.Equal(t, orig.LastCompletionTokens, after.LastCompletionTokens)
}

func TestGenerateTitleRespectsSessionLockDuringUsageUpdate(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	titleModel := stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "title"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "title", Delta: "locked-title"}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			}, nil
		},
	}

	model := Model{
		Model: titleModel,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 1000,
		},
		ModelCfg: config.SelectedModel{
			Model:    "claude-sonnet-4",
			Provider: "anthropic",
		},
	}

	a := NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	})

	lock := &sync.Mutex{}
	lock.Lock()
	done := make(chan struct{})
	go func() {
		a.(*sessionAgent).generateTitle(t.Context(), testSession.ID, "user prompt", lock)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("generateTitle should wait for session lock")
	default:
	}

	lock.Unlock()
	<-done

	after, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.Equal(t, "locked-title", after.Title)
}

func TestRunWaitsForTitleGenerationBeforeDequeuing(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	titleStarted := make(chan struct{})
	releaseTitle := make(chan struct{})
	var titleStartedOnce sync.Once
	titleModel := stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				titleStartedOnce.Do(func() { close(titleStarted) })
				<-releaseTitle
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "title"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "title", Delta: "queued-safe-title"}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			}, nil
		},
	}

	model := Model{
		Model: titleModel,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 1000,
		},
		ModelCfg: config.SelectedModel{
			Model:    "claude-sonnet-4",
			Provider: "anthropic",
		},
	}

	var sessAgent *sessionAgent
	testAgent := &queuePrepareTestAgent{t: t}
	sessAgent = NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		AgentFactory: func(fantasy.LanguageModel, ...fantasy.AgentOption) fantasy.Agent {
			return testAgent
		},
	}).(*sessionAgent)

	hasUserPrompt := func(prompt string) bool {
		msgs, listErr := env.messages.List(t.Context(), testSession.ID)
		require.NoError(t, listErr)
		for _, msg := range msgs {
			if msg.Role == message.User && msg.Content().Text == prompt {
				return true
			}
		}
		return false
	}

	testAgent.afterFirstPrepare = func() {
		_, runErr := sessAgent.Run(context.Background(), SessionAgentCall{
			SessionID:       testSession.ID,
			Prompt:          "queued later",
			MaxOutputTokens: 1000,
		})
		require.NoError(t, runErr)
	}

	runDone := make(chan error, 1)
	go func() {
		_, runErr := sessAgent.Run(t.Context(), SessionAgentCall{
			SessionID:       testSession.ID,
			Prompt:          "run now",
			MaxOutputTokens: 1000,
		})
		runDone <- runErr
	}()

	select {
	case <-titleStarted:
	case <-time.After(time.Second):
		t.Fatal("title generation did not start")
	}

	require.Eventually(t, func() bool {
		return sessAgent.QueuedPrompts(testSession.ID) == 1
	}, time.Second, 10*time.Millisecond)

	select {
	case err := <-runDone:
		require.NoError(t, err)
		t.Fatal("run finished before title generation was released")
	default:
	}

	require.False(t, hasUserPrompt("queued later"))
	close(releaseTitle)

	require.NoError(t, <-runDone)
	require.Eventually(t, func() bool {
		return hasUserPrompt("queued later")
	}, time.Second, 10*time.Millisecond)
}
