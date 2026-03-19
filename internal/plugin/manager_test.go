package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// testPlugin is a simple test plugin that records hook calls.
type testPlugin struct {
	name       string
	initErr    error
	hooks      Hooks
	initCalled bool
}

func (p *testPlugin) Name() string { return p.name }

func (p *testPlugin) Init(ctx context.Context, input PluginInput) (Hooks, error) {
	p.initCalled = true
	return p.hooks, p.initErr
}

func TestPluginRegister(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{name: "test-plugin"}
	Register(p)

	names := ListPlugins()
	require.Equal(t, []string{"test-plugin"}, names)
}

func TestPluginInit(t *testing.T) {
	Reset()
	defer Reset()

	p1 := &testPlugin{
		name: "plugin-1",
		hooks: Hooks{
			ShellEnv: func(ctx context.Context, input ShellEnvInput) map[string]string {
				return map[string]string{"TEST_VAR": "test-value"}
			},
		},
	}
	p2 := &testPlugin{
		name: "plugin-2",
		hooks: Hooks{
			PermissionAsk: func(input PermissionAskInput) PermissionAskOutput {
				return PermissionAskOutput{Action: PermissionAllow}
			},
		},
	}

	Register(p1)
	Register(p2)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	require.True(t, p1.initCalled)
	require.True(t, p2.initCalled)
}

func TestTriggerShellEnv(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "env-plugin",
		hooks: Hooks{
			ShellEnv: func(ctx context.Context, input ShellEnvInput) map[string]string {
				return map[string]string{
					"SESSION_ID": input.SessionID,
					"CUSTOM_VAR": "custom-value",
				}
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	env := TriggerShellEnv(context.Background(), ShellEnvInput{
		CWD:       "/test/dir",
		SessionID: "session-123",
		CallID:    "call-456",
	})

	require.Equal(t, "session-123", env["SESSION_ID"])
	require.Equal(t, "custom-value", env["CUSTOM_VAR"])
}

func TestTriggerShellEnvNoHook(t *testing.T) {
	Reset()
	defer Reset()

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	env := TriggerShellEnv(context.Background(), ShellEnvInput{})
	require.Empty(t, env)
}

func TestTriggerPermissionAsk(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "perm-plugin",
		hooks: Hooks{
			PermissionAsk: func(input PermissionAskInput) PermissionAskOutput {
				return PermissionAskOutput{Action: PermissionDeny}
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	result := TriggerPermissionAsk(PermissionAskInput{
		Permission: PermissionRequest{
			ToolName: "bash",
			Action:   "execute",
		},
	})

	require.Equal(t, PermissionDeny, result.Action)
}

func TestTriggerPermissionAskNoHook(t *testing.T) {
	Reset()
	defer Reset()

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	result := TriggerPermissionAsk(PermissionAskInput{})

	// Without a hook, should return "ask" (default behavior)
	require.Equal(t, PermissionAsk, result.Action)
}

func TestToolBeforeExecuteHook(t *testing.T) {
	Reset()
	defer Reset()

	var executedArgs map[string]any

	p := &testPlugin{
		name: "tool-plugin",
		hooks: Hooks{
			ToolBeforeExecute: func(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error) {
				executedArgs = input.Args
				// Modify args
				modified := make(map[string]any)
				for k, v := range input.Args {
					modified[k] = v
				}
				modified["modified"] = true
				return &ToolBeforeExecuteOutput{Args: modified}, nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	output, err := TriggerToolBeforeExecute(context.Background(), ToolBeforeExecuteInput{
		Tool:      "bash",
		SessionID: "session-1",
		CallID:    "call-1",
		Args: map[string]any{
			"command": "ls -la",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	require.True(t, output.Args["modified"].(bool))
	require.Equal(t, "ls -la", executedArgs["command"])
}

func TestToolBeforeExecuteSkip(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "skip-plugin",
		hooks: Hooks{
			ToolBeforeExecute: func(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error) {
				return &ToolBeforeExecuteOutput{
					Skip:      true,
					PreResult: "blocked by plugin",
				}, nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	output, err := TriggerToolBeforeExecute(context.Background(), ToolBeforeExecuteInput{
		Tool: "bash",
		Args: map[string]any{"command": "rm -rf /"},
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	require.True(t, output.Skip)
	require.Equal(t, "blocked by plugin", output.PreResult)
}

func TestToolAfterExecuteHook(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "after-plugin",
		hooks: Hooks{
			ToolAfterExecute: func(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error) {
				return &ToolAfterExecuteOutput{
					Result:        input.Result + "\n[audited]",
					ResultChanged: true,
				}, nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	output, err := TriggerToolAfterExecute(context.Background(), ToolAfterExecuteInput{
		Tool:   "bash",
		Result: "file1.txt\nfile2.txt",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, "file1.txt\nfile2.txt\n[audited]", output.Result)
	require.True(t, output.ResultChanged)
}

func TestToolAfterExecuteHookCanClearResult(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "clear-after-plugin",
		hooks: Hooks{
			ToolAfterExecute: func(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error) {
				return &ToolAfterExecuteOutput{Result: "", ResultChanged: true}, nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	output, err := TriggerToolAfterExecute(context.Background(), ToolAfterExecuteInput{
		Tool:   "bash",
		Result: "non-empty",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, "", output.Result)
	require.True(t, output.ResultChanged)
}

func TestGetCustomTools(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "tools-plugin",
		hooks: Hooks{
			Tools: map[string]ToolDefinition{
				"custom_tool": {
					Name:        "custom_tool",
					Description: "A custom tool for testing",
				},
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	tools := GetCustomTools()
	require.Len(t, tools, 1)
	require.Contains(t, tools, "custom_tool")
	require.Equal(t, "A custom tool for testing", tools["custom_tool"].Description)
}

func TestPluginInitError(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name:    "error-plugin",
		initErr: context.Canceled,
	}
	Register(p)

	// Init should not fail, but plugin should be skipped
	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	// Plugin's init was attempted
	require.True(t, p.initCalled)
	// No hooks from this plugin
	hooks := GetHooks()
	require.Nil(t, hooks.ShellEnv)
}

func TestMultiplePluginsMerge(t *testing.T) {
	Reset()
	defer Reset()

	p1 := &testPlugin{
		name: "plugin-1",
		hooks: Hooks{
			ShellEnv: func(ctx context.Context, input ShellEnvInput) map[string]string {
				return map[string]string{"VAR1": "value1"}
			},
		},
	}
	p2 := &testPlugin{
		name: "plugin-2",
		hooks: Hooks{
			ShellEnv: func(ctx context.Context, input ShellEnvInput) map[string]string {
				return map[string]string{"VAR2": "value2"}
			},
			Tools: map[string]ToolDefinition{
				"tool2": {Name: "tool2"},
			},
		},
	}

	Register(p1)
	Register(p2)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	env := TriggerShellEnv(context.Background(), ShellEnvInput{})
	require.Equal(t, "value1", env["VAR1"])
	require.Equal(t, "value2", env["VAR2"])

	tools := GetCustomTools()
	require.Len(t, tools, 1)
	require.Contains(t, tools, "tool2")
}

func TestTriggerChatMessagesTransform(t *testing.T) {
	Reset()
	defer Reset()

	Register(&testPlugin{
		name: "transform-1",
		hooks: Hooks{
			ChatMessagesTransform: func(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error {
				output.Messages = append(output.Messages, message.Message{Role: message.User})
				return nil
			},
		},
	})
	Register(&testPlugin{
		name: "transform-2",
		hooks: Hooks{
			ChatMessagesTransform: func(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error {
				output.Messages[0].Role = message.Assistant
				return nil
			},
		},
	})

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	transformed, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "session-1",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatMessagesTransformOutput{Messages: []message.Message{{Role: message.User}}})
	require.NoError(t, err)
	require.Len(t, transformed.Messages, 2)
	require.Equal(t, message.Assistant, transformed.Messages[0].Role)
	require.Equal(t, message.User, transformed.Messages[1].Role)
}

func TestTriggerChatSystemTransform(t *testing.T) {
	Reset()
	defer Reset()

	Register(&testPlugin{
		name: "system-1",
		hooks: Hooks{
			ChatSystemTransform: func(ctx context.Context, input ChatSystemTransformInput, output *ChatSystemTransformOutput) error {
				output.System = append(output.System, "plugin section")
				return nil
			},
		},
	})
	Register(&testPlugin{
		name: "system-2",
		hooks: Hooks{
			ChatSystemTransform: func(ctx context.Context, input ChatSystemTransformInput, output *ChatSystemTransformOutput) error {
				output.Prefix = "prefix"
				return nil
			},
		},
	})

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	transformed, err := TriggerChatSystemTransform(context.Background(), ChatSystemTransformInput{
		SessionID: "session-1",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatSystemTransformOutput{System: []string{"base"}})
	require.NoError(t, err)
	require.Equal(t, []string{"base", "plugin section"}, transformed.System)
	require.Equal(t, "prefix", transformed.Prefix)
}

func TestTriggerSessionCompacting(t *testing.T) {
	Reset()
	defer Reset()

	Register(&testPlugin{
		name: "compacting-1",
		hooks: Hooks{
			SessionCompacting: func(ctx context.Context, input SessionCompactingInput, output *SessionCompactingOutput) error {
				output.Context = append(output.Context, "extra context")
				return nil
			},
		},
	})
	Register(&testPlugin{
		name: "compacting-2",
		hooks: Hooks{
			SessionCompacting: func(ctx context.Context, input SessionCompactingInput, output *SessionCompactingOutput) error {
				output.Prompt = "custom prompt"
				return nil
			},
		},
	})

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	transformed, err := TriggerSessionCompacting(context.Background(), SessionCompactingInput{
		SessionID: "session-1",
		Purpose:   ChatTransformPurposeRecover,
	}, SessionCompactingOutput{})
	require.NoError(t, err)
	require.Equal(t, []string{"extra context"}, transformed.Context)
	require.Equal(t, "custom prompt", transformed.Prompt)
}

func TestTriggerChatBeforeRequest(t *testing.T) {
	Reset()
	defer Reset()

	called := false
	captured := ChatBeforeRequestInput{}
	p := &testPlugin{
		name: "chat-before-plugin",
		hooks: Hooks{
			ChatBeforeRequest: func(ctx context.Context, input ChatBeforeRequestInput) error {
				called = true
				captured = input
				return nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	err = TriggerChatBeforeRequest(context.Background(), ChatBeforeRequestInput{
		SessionID: "session-1",
		Agent:     "session",
		Model: ModelInfo{
			ProviderID: "anthropic",
			ModelID:    "claude-sonnet-4",
		},
	})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "session-1", captured.SessionID)
	require.Equal(t, "session", captured.Agent)
	require.Equal(t, "anthropic", captured.Model.ProviderID)
	require.Equal(t, "claude-sonnet-4", captured.Model.ModelID)
}

func TestTriggerChatAfterResponse(t *testing.T) {
	Reset()
	defer Reset()

	called := false
	captured := ChatAfterResponseInput{}
	p := &testPlugin{
		name: "chat-after-plugin",
		hooks: Hooks{
			ChatAfterResponse: func(ctx context.Context, input ChatAfterResponseInput) error {
				called = true
				captured = input
				return nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	result := &fantasy.AgentResult{}
	err = TriggerChatAfterResponse(context.Background(), ChatAfterResponseInput{
		SessionID: "session-2",
		Agent:     "session",
		Model: ModelInfo{
			ProviderID: "openai",
			ModelID:    "gpt-5",
		},
		Result: result,
	})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "session-2", captured.SessionID)
	require.Equal(t, "session", captured.Agent)
	require.Equal(t, "openai", captured.Model.ProviderID)
	require.Equal(t, "gpt-5", captured.Model.ModelID)
	require.Same(t, result, captured.Result)
}

func TestWrappedCustomToolWithHooksIntegration(t *testing.T) {
	Reset()
	defer Reset()

	p := &testPlugin{
		name: "integration-plugin",
		hooks: Hooks{
			ToolBeforeExecute: func(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error) {
				modified := make(map[string]any, len(input.Args)+1)
				for k, v := range input.Args {
					modified[k] = v
				}
				modified["name"] = "plugin"
				return &ToolBeforeExecuteOutput{Args: modified}, nil
			},
			ToolAfterExecute: func(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error) {
				return &ToolAfterExecuteOutput{Result: input.Result + "-done", ResultChanged: true}, nil
			},
			Tools: map[string]ToolDefinition{
				"custom_echo": {
					Name:        "custom_echo",
					Description: "echo name",
					Execute: func(ctx context.Context, args map[string]any, execCtx ToolContext) (string, error) {
						name, _ := args["name"].(string)
						return name, nil
					},
				},
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{WorkingDir: t.TempDir()})
	require.NoError(t, err)

	tools := GetCustomTools()
	customTool, ok := tools["custom_echo"]
	require.True(t, ok)

	agentTool := NewCustomToolAgentTool(customTool, t.TempDir())
	wrapped := WrapAgentTool(agentTool)
	resp, err := wrapped.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Input: `{"name":"user"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "plugin-done", resp.Content)
}

func TestInitLoadsLocalToolsFromGlobalAndProject(t *testing.T) {
	Reset()
	defer Reset()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	globalConfigRoot := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalConfigRoot)

	globalToolsDir := filepath.Join(globalConfigRoot, "tools")
	projectToolsDir := filepath.Join(projectDir, ".crush", "tools")
	require.NoError(t, os.MkdirAll(globalToolsDir, 0o755))
	require.NoError(t, os.MkdirAll(projectToolsDir, 0o755))

	writeTool := func(path string, def localToolDefinition) {
		content, marshalErr := json.Marshal(def)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(path, content, 0o644))
	}

	writeTool(filepath.Join(globalToolsDir, "shared_tool.json"), localToolDefinition{
		Name:        "shared_tool",
		Description: "global version",
		Parameters: map[string]any{
			"type": "object",
		},
		Execute: "printf global",
	})
	writeTool(filepath.Join(projectToolsDir, "shared_tool.json"), localToolDefinition{
		Name:        "shared_tool",
		Description: "project version",
		Parameters: map[string]any{
			"type": "object",
		},
		Execute: "printf project",
	})
	writeTool(filepath.Join(projectToolsDir, "echo_arg.json"), localToolDefinition{
		Name:        "echo_arg",
		Description: "echo arg from env",
		Parameters: map[string]any{
			"type": "object",
		},
		Execute: "printf \"%s\" \"$CRUSH_TOOL_ARG_VALUE\"",
	})

	err := Init(context.Background(), PluginInput{WorkingDir: projectDir})
	require.NoError(t, err)

	tools := GetCustomTools()
	require.Len(t, tools, 2)

	shared, ok := tools["shared_tool"]
	require.True(t, ok)
	sharedOut, err := shared.Execute(context.Background(), map[string]any{}, ToolContext{})
	require.NoError(t, err)
	require.Equal(t, "project", sharedOut)

	echoArg, ok := tools["echo_arg"]
	require.True(t, ok)
	echoOut, err := echoArg.Execute(context.Background(), map[string]any{"value": "hello"}, ToolContext{})
	require.NoError(t, err)
	require.Equal(t, "hello", echoOut)
	require.NotContains(t, logBuf.String(), "Custom tool registration collision")
}

func TestReset(t *testing.T) {
	Reset()
	defer Reset()

	Register(&testPlugin{name: "plugin-1"})
	Register(&testPlugin{name: "plugin-2"})

	require.Len(t, plugins, 2)

	Reset()

	require.Empty(t, plugins)
	require.Empty(t, initializedHooks)
	require.Empty(t, customTools)
}
