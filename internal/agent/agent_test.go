package agent

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/x/vcr"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/joho/godotenv/autoload"
)

func TestMain(m *testing.M) {
	slog.SetLogLoggerLevel(slog.LevelError)
	m.Run()
}

var modelPairs = []modelPair{
	{"glm-5.1", hyperBuilder("glm-5.1"), hyperBuilder("gpt-oss-120b")},
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
	agent, err := coderAgent(r, env, large, small)
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
							if tc.Name == tools.ViewToolName {
								readTCID = tc.ID
							}
							if tc.Name == tools.EditToolName || tc.Name == tools.WriteToolName {
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
							if tc.Name == tools.BashToolName {
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
							if tc.Name == tools.DownloadToolName {
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
							if tc.Name == tools.FetchToolName {
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
							if tc.Name == tools.GlobToolName {
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
							if tc.Name == tools.GrepToolName {
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
							if tc.Name == tools.LSToolName {
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
							if tc.Name == tools.MultiEditToolName {
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
							if tc.Name == tools.SourcegraphToolName {
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
							if tc.Name == tools.WriteToolName {
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
					if tc.Name == tools.GlobToolName {
						foundGlob = true
						globTCID = tc.ID
					}
					if tc.Name == tools.LSToolName {
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

func TestPreparePrompt_FiltersImageAttachments(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// User message with text, a text attachment, and an image attachment.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello world"},
			message.BinaryContent{Path: "notes.txt", MIMEType: "text/plain", Data: []byte("important notes")},
			message.BinaryContent{Path: "image.png", MIMEType: "image/png", Data: []byte("fake-image-data")},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	// New-turn image attachment (not yet stored in the DB).
	imageAtt := message.Attachment{
		FileName: "screenshot.png",
		MimeType: "image/png",
		Content:  []byte("fake-screenshot"),
	}

	// When supportsImages is false, image attachments should be stripped
	// from history AND from the files list.
	history, files := agent.preparePrompt(msgs, false, nil, imageAtt)
	// First message is the system reminder, second is the user message.
	require.Len(t, history, 2)
	require.Len(t, history[1].Content, 1)
	text, ok := fantasy.AsMessagePart[fantasy.TextPart](history[1].Content[0])
	require.True(t, ok)
	require.Contains(t, text.Text, "hello world")
	require.Contains(t, text.Text, "important notes")
	require.Empty(t, files, "image files should be excluded when model does not support images")

	// When supportsImages is true, image attachments should remain in
	// history and be included in the files list.
	history, files = agent.preparePrompt(msgs, true, nil, imageAtt)
	require.Len(t, history, 2)
	require.Len(t, history[1].Content, 2)
	text, ok = fantasy.AsMessagePart[fantasy.TextPart](history[1].Content[0])
	require.True(t, ok)
	require.Contains(t, text.Text, "hello world")
	file, ok := fantasy.AsMessagePart[fantasy.FilePart](history[1].Content[1])
	require.True(t, ok)
	require.Equal(t, "image.png", file.Filename)
	require.Len(t, files, 1, "new-turn image attachment should be included when model supports images")
	require.Equal(t, "screenshot.png", files[0].Filename)
}

func TestCreateUserMessage_RetainsAllAttachments(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// Mix of text and image attachments - all should be stored.
	call := SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "look at this image",
		Attachments: []message.Attachment{
			{FileName: "notes.txt", FilePath: "notes.txt", MimeType: "text/plain", Content: []byte("notes")},
			{FileName: "photo.png", FilePath: "photo.png", MimeType: "image/png", Content: []byte("fake-png")},
		},
	}

	msg, err := agent.createUserMessage(ctx, call)
	require.NoError(t, err)

	// All attachments should be present as BinaryContent parts.
	binaryParts := msg.BinaryContent()
	require.Len(t, binaryParts, 2, "both text and image attachments should be stored in the user message")
	require.Equal(t, "notes.txt", binaryParts[0].Path)
	require.Equal(t, "text/plain", binaryParts[0].MIMEType)
	require.Equal(t, "photo.png", binaryParts[1].Path)
	require.Equal(t, "image/png", binaryParts[1].MIMEType)

	// Reload from DB to verify persistence.
	reloaded, err := env.messages.Get(ctx, msg.ID)
	require.NoError(t, err)
	binaryParts = reloaded.BinaryContent()
	require.Len(t, binaryParts, 2, "attachments should survive DB round-trip")
	require.Equal(t, "photo.png", binaryParts[1].Path)
}

func TestPreparePrompt_OrphanedToolUse(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// Create a user message.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	})
	require.NoError(t, err)

	// Create an assistant message with a tool call but no tool result -
	// this simulates a cancelled/interrupted agent tool call.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "let me check"},
			message.ToolCall{
				ID:       "call_orphaned_1",
				Name:     "agent",
				Input:    `{"prompt":"do something"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	// Create the next user message (the one that interrupted the tool call).
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Fix #2"},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, true, nil)

	// The history must contain a synthetic tool result for the orphaned call.
	found := false
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				if tr.ToolCallID == "call_orphaned_1" {
					found = true
					_, isError := tr.Output.(fantasy.ToolResultOutputContentError)
					require.True(t, isError, "orphaned tool result should be an error")
				}
			}
		}
	}
	require.True(t, found, "expected synthetic tool result for orphaned tool call")
}

func TestPreparePrompt_OrphanedToolUseMixed(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	})
	require.NoError(t, err)

	// Assistant with 2 tool calls: one has a result, one is orphaned.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call_ok",
				Name:     "view",
				Input:    `{"path":"/foo"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call_orphaned",
				Name:     "agent",
				Input:    `{"prompt":"search"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	// Only one tool result - for call_ok.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call_ok",
				Name:       "view",
				Content:    "file contents",
			},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, true, nil)

	// Should have a synthetic result only for the orphaned call.
	var syntheticCount int
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				if tr.ToolCallID == "call_orphaned" {
					syntheticCount++
				}
			}
		}
	}
	require.Equal(t, 1, syntheticCount, "expected exactly one synthetic result for the orphaned call")
}

func TestWorkaroundProviderMediaLimitations_TextOnlyModel(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Non-Anthropic provider, no image support - should replace media with
	// a text placeholder and not create a synthetic user message.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: "openai"},
		CatwalkCfg: catwalk.Model{
			SupportsImages: false,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)

	// Should produce exactly one message: the tool message with a text
	// placeholder. No synthetic user message with FilePart.
	require.Len(t, result, 1)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)

	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	_, ok = fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Output)
	require.True(t, ok)
}

func TestWorkaroundProviderMediaLimitations_VisionModel(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Non-Anthropic provider, image support - should create a synthetic
	// user message with FilePart.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: "openai"},
		CatwalkCfg: catwalk.Model{
			SupportsImages: true,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)

	// Should produce two messages: tool message with placeholder text,
	// and synthetic user message with FilePart.
	require.Len(t, result, 2)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)
	require.Equal(t, fantasy.MessageRoleUser, result[1].Role)

	// The tool message should have text placeholder.
	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	textOutput, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Output)
	require.True(t, ok)
	require.Contains(t, textOutput.Text, "see attached file")

	// The synthetic user message should contain a TextPart and a FilePart.
	require.Len(t, result[1].Content, 2)
	file, ok := fantasy.AsMessagePart[fantasy.FilePart](result[1].Content[1])
	require.True(t, ok)
	require.Equal(t, "image/png", file.MediaType)
}

func TestWorkaroundProviderMediaLimitations_AnthropicProvider(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Anthropic provider - should return messages unchanged regardless of
	// SupportsImages, since Anthropic handles media in tool results natively.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: string(catwalk.InferenceProviderAnthropic)},
		CatwalkCfg: catwalk.Model{
			SupportsImages: true,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)
	require.Len(t, result, 1)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)

	// The media should still be in the tool result, untouched.
	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](tr.Output)
	require.True(t, ok)
	require.Equal(t, "image/png", media.MediaType)
}

func TestProviderRetryLogFields(t *testing.T) {
	t.Run("nil provider error", func(t *testing.T) {
		fields := providerRetryLogFields(nil, 2*time.Second, 1, 10)
		require.Equal(t, []any{
			"retry_delay", "2s",
			"attempt", 1,
			"max_retries", 10,
		}, fields)
	})

	t.Run("provider error with title and message", func(t *testing.T) {
		fields := providerRetryLogFields(&fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "too many requests",
		}, 1500*time.Millisecond, 3, 10)
		require.Equal(t, []any{
			"retry_delay", "1.5s",
			"attempt", 3,
			"max_retries", 10,
			"status_code", 429,
			"title", "rate limit",
			"message", "too many requests",
		}, fields)
	})

	t.Run("provider error without optional strings", func(t *testing.T) {
		fields := providerRetryLogFields(&fantasy.ProviderError{
			StatusCode: 503,
		}, time.Second, 2, 10)
		require.Equal(t, []any{
			"retry_delay", "1s",
			"attempt", 2,
			"max_retries", 10,
			"status_code", 503,
		}, fields)
	})
}

func TestFormatProviderError(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "provider error", formatProviderError(nil))
	})

	t.Run("quota error with HTTP status 429", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "You exceeded your current quota, please check your plan and billing details.",
		}
		require.Equal(t, "HTTP 429 - Quota Exceeded / Out of Credits - You exceeded your current quota, please check your plan and billing details.", formatProviderError(err))
	})

	t.Run("rate limit error with generic message", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "too many requests",
		}
		require.Equal(t, "HTTP 429 - Rate Limit Reached", formatProviderError(err))
	})

	t.Run("extract message from response body JSON", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode:   429,
			Title:        "rate limit",
			Message:      "too many requests",
			ResponseBody: []byte(`{"error": {"message": "Quota exceeded for metric GenerateContent"}}`),
		}
		require.Equal(t, "HTTP 429 - Quota Exceeded / Out of Credits - Quota exceeded for metric GenerateContent", formatProviderError(err))
	})

	t.Run("server overloaded 503", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode: 503,
			Title:      "overloaded",
			Message:    "Service Unavailable",
		}
		require.Equal(t, "HTTP 503 - Provider Server Down / Overloaded - Service Unavailable", formatProviderError(err))
	})

	t.Run("cause carrying underlying quota error", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "too many requests",
			Cause:      errors.New("googleapi: Error 429: Quota exceeded for quota metric 'Generate Content API requests'"),
		}
		require.Equal(t, "HTTP 429 - Quota Exceeded / Out of Credits - googleapi: Error 429: Quota exceeded for quota metric 'Generate Content API requests'", formatProviderError(err))
	})

	t.Run("embedded JSON string in message with FreeUsageLimitError and retryAfter", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			StatusCode: 429,
			Title:      "too many requests",
			Message:    `{"type":"Account.FreeUsageLimitError","message":"Rate limit exceeded. Please try again later.","retryAfter":27109}`,
		}
		require.Equal(t, "HTTP 429 - Quota Exceeded / Out of Credits (resets in 7h31m) - Rate limit exceeded. Please try again later.", formatProviderError(err))
	})
}

func TestFormatRetryStatus(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		"Retrying in 5s (9/10 retries left) - rate limit",
		notify.FormatRetryStatus(notify.Notification{
			Type:       notify.TypeRetry,
			Message:    "rate limit",
			RetryDelay: 5 * time.Second,
			Attempt:    2,
			MaxRetries: 10,
		}, 5*time.Second),
	)

	require.Equal(
		t,
		"Retrying in 5s (9/10 retries left) - [gemini] HTTP 429 - Quota Exceeded / Out of Credits - quota exceeded",
		notify.FormatRetryStatus(notify.Notification{
			Type:       notify.TypeRetry,
			ProviderID: "gemini",
			Message:    "HTTP 429 - Quota Exceeded / Out of Credits - quota exceeded",
			RetryDelay: 5 * time.Second,
			Attempt:    2,
			MaxRetries: 10,
		}, 5*time.Second),
	)

	require.Equal(
		t,
		"Retrying in 1s (10/10 retries left)",
		notify.FormatRetryStatus(notify.Notification{
			Type:       notify.TypeRetry,
			RetryDelay: time.Second,
			Attempt:    1,
			MaxRetries: 10,
		}, time.Second),
	)

	// Sub-second remainders still show as 1s so the bar never flashes 0s.
	require.Equal(
		t,
		"Retrying in 1s (10/10 retries left) - overloaded",
		notify.FormatRetryStatus(notify.Notification{
			Type:       notify.TypeRetry,
			Message:    "overloaded",
			RetryDelay: 500 * time.Millisecond,
			Attempt:    1,
			MaxRetries: 10,
		}, 200*time.Millisecond),
	)
}

func TestTodoSystemReminder(t *testing.T) {
	t.Parallel()

	t.Run("sub agent never reminded", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, todoSystemReminder(true, nil))
		require.Empty(t, todoSystemReminder(true, []session.Todo{{Status: session.TodoStatusCompleted}}))
	})

	t.Run("empty list gets create hint", func(t *testing.T) {
		t.Parallel()
		got := todoSystemReminder(false, nil)
		require.Contains(t, got, "currently empty")
	})

	t.Run("all completed nudges clear", func(t *testing.T) {
		t.Parallel()
		got := todoSystemReminder(false, []session.Todo{
			{Content: "a", Status: session.TodoStatusCompleted},
		})
		require.Contains(t, got, "fully completed")
		require.Contains(t, got, "empty todos array")
	})

	t.Run("incomplete without in_progress nudges", func(t *testing.T) {
		t.Parallel()
		got := todoSystemReminder(false, []session.Todo{
			{Content: "a", Status: session.TodoStatusPending},
		})
		require.Contains(t, got, "none are marked in_progress")
	})

	t.Run("healthy in_progress is silent", func(t *testing.T) {
		t.Parallel()
		got := todoSystemReminder(false, []session.Todo{
			{Content: "a", Status: session.TodoStatusInProgress},
		})
		require.Empty(t, got)
	})
}

// TestProviderRetryBudgetIsBounded runs fantasy's real retry middleware
// (scaled down 1000x) to measure the worst-case wall time a user can be
// made to wait on providerMaxRetries. Fantasy's backoff is uncapped
// exponential and offers no maximum-delay knob, so the retry count is
// the only lever Crush has; a budget large enough to produce a
// 40-minute countdown is indistinguishable from a hang.
func TestProviderRetryBudgetIsBounded(t *testing.T) {
	t.Parallel()

	defaults := fantasy.DefaultRetryOptions()
	const scale = 1000
	opts := fantasy.RetryOptions{
		MaxRetries:     providerMaxRetries,
		InitialDelayIn: defaults.InitialDelayIn / scale,
		BackoffFactor:  defaults.BackoffFactor,
	}
	var attempts int
	var scaledTotal, scaledLongest time.Duration
	opts.OnRetry = func(_ *fantasy.ProviderError, delay time.Duration) {
		attempts++
		scaledTotal += delay
		scaledLongest = delay
	}
	retry := fantasy.RetryWithExponentialBackoffRespectingRetryHeaders[int](opts)
	_, err := retry(t.Context(), func() (int, error) {
		return 0, &fantasy.ProviderError{Title: "rate limit", StatusCode: 429}
	})
	require.Error(t, err)
	require.Equal(t, providerMaxRetries, attempts)

	total := scaledTotal * scale
	longest := scaledLongest * scale
	t.Logf("providerMaxRetries=%d longest single wait=%v total wall time=%v",
		providerMaxRetries, longest, total)

	require.LessOrEqual(t, longest, 3*time.Minute,
		"longest single backoff (%v) leaves the user watching a countdown with no way to tell Crush from a hang", longest)
	require.LessOrEqual(t, total, 6*time.Minute,
		"worst-case total retry wall time is %v", total)
}

// recordingPublisher captures published notifications.
type recordingPublisher struct {
	mu   sync.Mutex
	sent []notify.Notification
}

func (p *recordingPublisher) Publish(_ pubsub.EventType, n notify.Notification) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, n)
}

func (p *recordingPublisher) PublishMustDeliver(_ context.Context, e pubsub.EventType, n notify.Notification) {
	p.Publish(e, n)
}

func (p *recordingPublisher) attempts() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]int, 0, len(p.sent))
	for _, n := range p.sent {
		out = append(out, n.Attempt)
	}
	return out
}

// TestRetryAttemptReporterResetsPerCall guards the countdown's attempt
// number. Fantasy resets the retry budget for every AgentStreamCall it
// runs, so GenerateTitle's small->large model fallback performs two
// independent retry passes. A single counter shared by both (the shape
// before newRetryAttemptReporter existed) carried the first pass's count
// into the second, and the user-facing countdown read "attempt 7/6".
//
// Note on scope: this pins the helper's contract -- each reporter starts
// at 1 and never continues a previous one. The GenerateTitle call site
// installs a fresh reporter per loop iteration; that wiring is
// structural (no counter exists in the loop's scope to share any more)
// rather than covered here, because the loop needs a live provider.
func TestRetryAttemptReporterResetsPerCall(t *testing.T) {
	t.Parallel()

	pub := &recordingPublisher{}
	a := &sessionAgent{notify: pub}
	perr := &fantasy.ProviderError{Title: "rate limit"}

	first := newRetryAttemptReporter(a, "s1", "prov", providerMaxRetries)
	first(perr, time.Second)
	first(perr, 2*time.Second)

	second := newRetryAttemptReporter(a, "s1", "prov", providerMaxRetries)
	second(perr, time.Second)

	require.Equal(t, []int{1, 2, 1}, pub.attempts(),
		"the second model attempt must restart the attempt count at 1")
	for _, n := range pub.sent {
		require.LessOrEqual(t, n.Attempt, n.MaxRetries,
			"a published attempt number must never exceed the budget")
	}
}

// TestRetryAttemptCounterResetsPerStep pins the multi-step run contract:
// fantasy allocates a fresh MaxRetries budget per step, and Crush's
// user-visible attempt counter must reset at PrepareStep so a recovery
// on step N does not make step N+1 start at "attempt 5/6".
func TestRetryAttemptCounterResetsPerStep(t *testing.T) {
	t.Parallel()

	var c retryAttemptCounter

	// Step 1: fail twice, then succeed (counter would sit at 2).
	require.Equal(t, 1, c.Next())
	require.Equal(t, 2, c.Next())

	// Step 2: PrepareStep clears the counter before any OnRetry.
	c.Reset()
	require.Equal(t, 1, c.Next(),
		"a new step must publish attempt 1, not continue from the prior step")
}
