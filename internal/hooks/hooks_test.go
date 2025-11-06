package hooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHookExecutor_Execute(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	tests := []struct {
		name    string
		config  config.HookConfig
		hookCtx HookContext
		wantErr bool
	}{
		{
			name: "simple command hook",
			config: config.HookConfig{
				config.PreToolUse: []config.HookMatcher{
					{
						Matcher: "bash",
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: "echo 'hook executed'",
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
		},
		{
			name: "hook with jq processing",
			config: config.HookConfig{
				config.PreToolUse: []config.HookMatcher{
					{
						Matcher: "bash",
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: `jq -r '.tool_name'`,
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
		},
		{
			name: "hook that writes to file",
			config: config.HookConfig{
				config.PostToolUse: []config.HookMatcher{
					{
						Matcher: "*",
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: `jq -r '"\(.tool_name): \(.tool_result)"' >> ` + filepath.Join(tempDir, "hook-log.txt"),
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType:  config.PostToolUse,
				ToolName:   "edit",
				ToolResult: "file edited successfully",
			},
		},
		{
			name: "hook with timeout",
			config: config.HookConfig{
				config.Stop: []config.HookMatcher{
					{
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: "sleep 0.1 && echo 'done'",
								Timeout: ptrInt(1),
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType: config.Stop,
			},
		},
		{
			name: "failed hook command",
			config: config.HookConfig{
				config.PreToolUse: []config.HookMatcher{
					{
						Matcher: "bash",
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: "exit 1",
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
			wantErr: true,
		},
		{
			name: "hook with single quote in JSON",
			config: config.HookConfig{
				config.PostToolUse: []config.HookMatcher{
					{
						Matcher: "edit",
						Hooks: []config.Hook{
							{
								Type:    "command",
								Command: `jq -r '.tool_result'`,
							},
						},
					},
				},
			},
			hookCtx: HookContext{
				EventType:  config.PostToolUse,
				ToolName:   "edit",
				ToolResult: "it's a test with 'quotes'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			executor := NewExecutor(tt.config, tempDir)
			require.NotNil(t, executor)

			ctx := context.Background()
			err := executor.Execute(ctx, tt.hookCtx)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHookExecutor_MatcherApplies(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	executor := NewExecutor(config.HookConfig{}, tempDir)

	tests := []struct {
		name    string
		matcher config.HookMatcher
		ctx     HookContext
		want    bool
	}{
		{
			name: "empty matcher matches all",
			matcher: config.HookMatcher{
				Matcher: "",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
			want: true,
		},
		{
			name: "wildcard matcher matches all",
			matcher: config.HookMatcher{
				Matcher: "*",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "edit",
			},
			want: true,
		},
		{
			name: "specific tool matcher matches",
			matcher: config.HookMatcher{
				Matcher: "bash",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
			want: true,
		},
		{
			name: "specific tool matcher doesn't match different tool",
			matcher: config.HookMatcher{
				Matcher: "bash",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "edit",
			},
			want: false,
		},
		{
			name: "pipe-separated matcher matches first tool",
			matcher: config.HookMatcher{
				Matcher: "edit|write|multiedit",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "edit",
			},
			want: true,
		},
		{
			name: "pipe-separated matcher matches middle tool",
			matcher: config.HookMatcher{
				Matcher: "edit|write|multiedit",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "write",
			},
			want: true,
		},
		{
			name: "pipe-separated matcher matches last tool",
			matcher: config.HookMatcher{
				Matcher: "edit|write|multiedit",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "multiedit",
			},
			want: true,
		},
		{
			name: "pipe-separated matcher doesn't match different tool",
			matcher: config.HookMatcher{
				Matcher: "edit|write|multiedit",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "bash",
			},
			want: false,
		},
		{
			name: "pipe-separated matcher with spaces",
			matcher: config.HookMatcher{
				Matcher: "edit | write | multiedit",
			},
			ctx: HookContext{
				EventType: config.PreToolUse,
				ToolName:  "write",
			},
			want: true,
		},
		{
			name: "non-tool event matches empty matcher",
			matcher: config.HookMatcher{
				Matcher: "",
			},
			ctx: HookContext{
				EventType: config.Stop,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := executor.matcherApplies(tt.matcher, tt.ctx)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHookExecutor_Timeout(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	shortTimeout := 1

	hookConfig := config.HookConfig{
		config.Stop: []config.HookMatcher{
			{
				Hooks: []config.Hook{
					{
						Type:    "command",
						Command: "sleep 10",
						Timeout: &shortTimeout,
					},
				},
			},
		},
	}

	executor := NewExecutor(hookConfig, tempDir)
	ctx := context.Background()

	start := time.Now()
	err := executor.Execute(ctx, HookContext{
		EventType: config.Stop,
	})
	duration := time.Since(start)

	require.Error(t, err)
	require.Less(t, duration, 2*time.Second)
}

func TestHookExecutor_MultipleHooks(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "multi-hook-log.txt")

	hookConfig := config.HookConfig{
		config.PreToolUse: []config.HookMatcher{
			{
				Matcher: "bash",
				Hooks: []config.Hook{
					{
						Type:    "command",
						Command: "echo 'hook1' >> " + logFile,
					},
					{
						Type:    "command",
						Command: "echo 'hook2' >> " + logFile,
					},
					{
						Type:    "command",
						Command: "echo 'hook3' >> " + logFile,
					},
				},
			},
		},
	}

	executor := NewExecutor(hookConfig, tempDir)
	ctx := context.Background()

	err := executor.Execute(ctx, HookContext{
		EventType: config.PreToolUse,
		ToolName:  "bash",
	})

	require.NoError(t, err)

	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 3)
	require.Equal(t, "hook1", strings.TrimSpace(lines[0]))
	require.Equal(t, "hook2", strings.TrimSpace(lines[1]))
	require.Equal(t, "hook3", strings.TrimSpace(lines[2]))
}

func TestHookExecutor_PipeSeparatedMatcher(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "pipe-matcher-log.txt")

	hookConfig := config.HookConfig{
		config.PostToolUse: []config.HookMatcher{
			{
				Matcher: "edit|write|multiedit",
				Hooks: []config.Hook{
					{
						Type:    "command",
						Command: `jq -r '.tool_name' >> ` + logFile,
					},
				},
			},
		},
	}

	executor := NewExecutor(hookConfig, tempDir)
	ctx := context.Background()

	// Test that edit triggers the hook
	err := executor.Execute(ctx, HookContext{
		EventType: config.PostToolUse,
		ToolName:  "edit",
	})
	require.NoError(t, err)

	// Test that write triggers the hook
	err = executor.Execute(ctx, HookContext{
		EventType: config.PostToolUse,
		ToolName:  "write",
	})
	require.NoError(t, err)

	// Test that multiedit triggers the hook
	err = executor.Execute(ctx, HookContext{
		EventType: config.PostToolUse,
		ToolName:  "multiedit",
	})
	require.NoError(t, err)

	// Test that bash does NOT trigger the hook
	err = executor.Execute(ctx, HookContext{
		EventType: config.PostToolUse,
		ToolName:  "bash",
	})
	require.NoError(t, err)

	// Verify only the matching tools were logged
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 3)
	require.Equal(t, "edit", strings.TrimSpace(lines[0]))
	require.Equal(t, "write", strings.TrimSpace(lines[1]))
	require.Equal(t, "multiedit", strings.TrimSpace(lines[2]))
}

func TestHookExecutor_ContextCancellation(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "cancel-log.txt")

	hookConfig := config.HookConfig{
		config.PreToolUse: []config.HookMatcher{
			{
				Matcher: "bash",
				Hooks: []config.Hook{
					{
						Type:    "command",
						Command: "echo 'hook1' >> " + logFile,
					},
					{
						Type:    "command",
						Command: "sleep 10 && echo 'hook2' >> " + logFile,
					},
				},
			},
		},
	}

	executor := NewExecutor(hookConfig, tempDir)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := executor.Execute(ctx, HookContext{
		EventType: config.PreToolUse,
		ToolName:  "bash",
	})

	require.Error(t, err)
	// On context cancellation, the shell may return either context.Canceled or an exit status
	// depending on timing. Check for both.
	if !errors.Is(err, context.Canceled) {
		// If not context.Canceled, we should still have an error from the cancelled context
		require.NotNil(t, ctx.Err())
	}
}

func ptrInt(i int) *int {
	return &i
}
