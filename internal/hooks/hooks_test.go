package hooks

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAggregation(t *testing.T) {
	t.Parallel()

	t.Run("empty results", func(t *testing.T) {
		t.Parallel()
		agg := aggregate(nil, "{}")
		require.Equal(t, DecisionNone, agg.Decision)
		require.Empty(t, agg.Reason)
		require.Empty(t, agg.Context)
		require.False(t, agg.Halt)
	})

	t.Run("single allow", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
	})

	t.Run("deny wins over allow", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, Context: "ctx1"},
			{Decision: DecisionDeny, Reason: "blocked"},
		}, "{}")
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "blocked", agg.Reason)
		require.Equal(t, "ctx1", agg.Context)
	})

	t.Run("multiple deny reasons concatenated", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionDeny, Reason: "reason1"},
			{Decision: DecisionDeny, Reason: "reason2"},
		}, "{}")
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "reason1\nreason2", agg.Reason)
	})

	t.Run("context concatenated from all hooks", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, Context: "ctx-a"},
			{Decision: DecisionNone, Context: "ctx-b"},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
		require.Equal(t, "ctx-a\nctx-b", agg.Context)
	})

	t.Run("allow wins over none", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionNone},
			{Decision: DecisionAllow},
		}, "{}")
		require.Equal(t, DecisionAllow, agg.Decision)
	})

	t.Run("halt is sticky across results", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
			{Halt: true, Reason: "stop now"},
		}, "{}")
		require.True(t, agg.Halt)
		require.Contains(t, agg.Reason, "stop now")
	})

	t.Run("halt with deny only records reason once", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionDeny, Halt: true, Reason: "stop"},
		}, "{}")
		require.True(t, agg.Halt)
		require.Equal(t, DecisionDeny, agg.Decision)
		require.Equal(t, "stop", agg.Reason)
	})
}

func TestParseStdout(t *testing.T) {
	t.Parallel()

	t.Run("empty stdout", func(t *testing.T) {
		t.Parallel()
		r := parseStdout("")
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("valid allow", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"some context"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "some context", r.Context)
	})

	t.Run("valid deny", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"deny","reason":"not allowed"}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "not allowed", r.Reason)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{bad json}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("unknown decision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"maybe"}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("version 1 accepted", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"version":1,"decision":"allow"}`)
		require.Equal(t, DecisionAllow, r.Decision)
	})

	t.Run("unknown higher version still parses", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"version":99,"decision":"deny","reason":"future"}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "future", r.Reason)
	})

	t.Run("halt true without decision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"halt":true,"reason":"turn over"}`)
		require.True(t, r.Halt)
		require.Equal(t, "turn over", r.Reason)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("context string form", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"one note"}`)
		require.Equal(t, "one note", r.Context)
	})

	t.Run("context array form", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":["first","second"]}`)
		require.Equal(t, "first\nsecond", r.Context)
	})

	t.Run("context array drops empty entries", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":["","keep",""]}`)
		require.Equal(t, "keep", r.Context)
	})

	t.Run("context null becomes empty", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":null}`)
		require.Empty(t, r.Context)
	})
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	env := BuildEnv(EventPreToolUse, "bash", "sess-1", "/work", "/project", `{"command":"ls","file_path":"/tmp/f.txt"}`)

	envMap := make(map[string]string)
	for _, e := range env {
		parts := splitFirst(e, "=")
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	require.Equal(t, EventPreToolUse, envMap["CRUSH_EVENT"])
	require.Equal(t, "bash", envMap["CRUSH_TOOL_NAME"])
	require.Equal(t, "sess-1", envMap["CRUSH_SESSION_ID"])
	require.Equal(t, "/work", envMap["CRUSH_CWD"])
	require.Equal(t, "/project", envMap["CRUSH_PROJECT_DIR"])
	require.Equal(t, "ls", envMap["CRUSH_TOOL_INPUT_COMMAND"])
	require.Equal(t, "/tmp/f.txt", envMap["CRUSH_TOOL_INPUT_FILE_PATH"])
}

func splitFirst(s, sep string) []string {
	before, after, found := strings.Cut(s, sep)
	if !found {
		return []string{s}
	}
	return []string{before, after}
}

func TestBuildPayload(t *testing.T) {
	t.Parallel()
	payload := BuildPayload(EventPreToolUse, "sess-1", "/work", "bash", `{"command":"ls"}`)
	s := string(payload)
	require.Contains(t, s, `"event":"`+EventPreToolUse+`"`)
	require.Contains(t, s, `"tool_name":"bash"`)
	// tool_input should be an object, not a string.
	require.Contains(t, s, `"tool_input":{"command":"ls"}`)
}

func TestRunnerExitCode0Allow(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow","context":"ok"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.Equal(t, "ok", result.Context)
}

func TestRunnerExitCode2Deny(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo "forbidden" >&2; exit 2`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, result.Decision)
	require.False(t, result.Halt)
	require.Equal(t, "forbidden", result.Reason)
}

func TestRunnerExitCode49Halt(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo "stop the turn" >&2; exit 49`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.True(t, result.Halt)
	require.Equal(t, DecisionDeny, result.Decision)
	require.Equal(t, "stop the turn", result.Reason)
}

func TestRunnerHaltViaJSON(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"halt":true,"reason":"via json"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.True(t, result.Halt)
	require.Equal(t, "via json", result.Reason)
}

func TestRunnerExitCodeOtherNonBlocking(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `exit 1`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `sleep 10`,
		Timeout: 1,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	start := time.Now()
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
	require.Less(t, elapsed, 5*time.Second)
}

func TestRunnerDeduplication(t *testing.T) {
	t.Parallel()
	// Two hooks with the same command should only run once.
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg, hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
}

func TestRunnerNoMatchingHooks(t *testing.T) {
	t.Parallel()
	// Hooks are empty.
	r := NewRunner(nil, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionNone, result.Decision)
}

// validatedHooks builds hook configs and runs ValidateHooks to compile
// matcher regexes, mirroring the real config-load path.
func validatedHooks(t *testing.T, hooks []config.HookConfig) []config.HookConfig {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: hooks,
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return cfg.Hooks[EventPreToolUse]
}

func TestRunnerMatcherFiltering(t *testing.T) {
	t.Parallel()

	t.Run("compiled regex matches", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^bash$"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, result.Decision)
	})

	t.Run("compiled regex does not match", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^edit$"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
	})

	t.Run("no matcher matches everything", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"allow"}'`},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())
		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionAllow, result.Decision)
	})

	t.Run("partial regex match", func(t *testing.T) {
		t.Parallel()
		hooks := validatedHooks(t, []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"mcp blocked"}'`, Matcher: "^mcp_"},
		})
		r := NewRunner(hooks, t.TempDir(), t.TempDir())

		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "mcp_github_get_me", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, result.Decision)

		result, err = r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
	})

	// Runner must compile matchers itself; it cannot rely on
	// ValidateHooks having run first. This is the guarantee that prevents
	// the reload-drops-matcher class of bug.
	t.Run("runner compiles matcher without ValidateHooks", func(t *testing.T) {
		t.Parallel()
		raw := []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"blocked"}'`, Matcher: "^bash$"},
		}
		r := NewRunner(raw, t.TempDir(), t.TempDir())

		deny, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionDeny, deny.Decision)

		noop, err := r.Run(context.Background(), EventPreToolUse, "sess", "view", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, noop.Decision)
	})

	// A matcher that fails to compile at Runner construction must not
	// degrade to match-everything; the hook is dropped instead.
	t.Run("runner skips hooks with invalid matcher", func(t *testing.T) {
		t.Parallel()
		raw := []config.HookConfig{
			{Command: `echo '{"decision":"deny","reason":"should not fire"}'`, Matcher: "[invalid"},
		}
		r := NewRunner(raw, t.TempDir(), t.TempDir())

		result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
		require.NoError(t, err)
		require.Equal(t, DecisionNone, result.Decision)
		require.Empty(t, r.Hooks())
	})
}

func TestValidateHooksInvalidRegex(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: {
				{Command: "true", Matcher: "[invalid"},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid matcher regex")
}

func TestValidateHooksEmptyCommand(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			EventPreToolUse: {
				{Command: ""},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "command is required")
}

func TestValidateHooksNormalizesEventNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"canonical", "PreToolUse"},
		{"lowercase", "pretooluse"},
		{"snake_case", "pre_tool_use"},
		{"upper_snake", "PRE_TOOL_USE"},
		{"mixed_case", "preToolUse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{
				Hooks: map[string][]config.HookConfig{
					tt.input: {
						{Command: "true"},
					},
				},
			}
			require.NoError(t, cfg.ValidateHooks())
			require.Len(t, cfg.Hooks[EventPreToolUse], 1)
		})
	}
}

func TestRunnerParallelExecution(t *testing.T) {
	t.Parallel()
	// Two hooks: one allows, one denies. Deny should win.
	hooks := []config.HookConfig{
		{Command: `echo '{"decision":"allow","context":"hook1"}'`},
		{Command: `echo '{"decision":"deny","reason":"nope"}' ; exit 0`},
	}
	r := NewRunner(hooks, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, result.Decision)
	require.Equal(t, "nope", result.Reason)
}

func TestRunnerEnvVarsPropagated(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `printf '{"decision":"allow","context":"%s"}' "$CRUSH_TOOL_NAME"`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.Equal(t, "bash", result.Context)
}

func TestParseStdoutUpdatedInput(t *testing.T) {
	t.Parallel()

	t.Run("nested object", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","updated_input":{"command":"rtk cat foo.go"}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("stringified backward compat", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","updated_input":"{\"command\":\"rtk cat foo.go\"}"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("no updated_input", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow"}`)
		require.Empty(t, r.UpdatedInput)
	})
}

func TestAggregationUpdatedInput(t *testing.T) {
	t.Parallel()

	t.Run("patches merge in config order with later overriding", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"first","keep":"me"}`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"second"}`},
		}, `{"command":"orig","timeout":60}`)
		require.Equal(t, DecisionAllow, agg.Decision)
		// command overridden by second patch; keep preserved from first
		// patch; timeout preserved from original input.
		require.JSONEq(t,
			`{"command":"second","keep":"me","timeout":60}`,
			agg.UpdatedInput,
		)
	})

	t.Run("shallow: nested objects are replaced wholesale", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"env":{"FOO":"bar"}}`},
		}, `{"env":{"BAZ":"qux"},"command":"ls"}`)
		// "env" is replaced entirely; "command" preserved.
		require.JSONEq(t,
			`{"env":{"FOO":"bar"},"command":"ls"}`,
			agg.UpdatedInput,
		)
	})

	t.Run("deny still reports merged input (caller ignores it)", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"rewritten"}`},
			{Decision: DecisionDeny, Reason: "blocked"},
		}, `{"command":"orig"}`)
		require.Equal(t, DecisionDeny, agg.Decision)
	})

	t.Run("no patches leaves updated_input empty", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow},
			{Decision: DecisionNone},
		}, `{"command":"orig"}`)
		require.Empty(t, agg.UpdatedInput)
	})

	t.Run("invalid patch is ignored", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `"not-an-object"`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"good"}`},
		}, `{"command":"orig"}`)
		require.JSONEq(t, `{"command":"good"}`, agg.UpdatedInput)
	})

	t.Run("malformed patch JSON is ignored and merge continues", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{broken json`},
			{Decision: DecisionAllow, UpdatedInput: `{"command":"good"}`},
		}, `{"command":"orig"}`)
		require.JSONEq(t, `{"command":"good"}`, agg.UpdatedInput)
	})

	t.Run("non-object tool_input rejects all patches", func(t *testing.T) {
		t.Parallel()
		agg := aggregate([]HookResult{
			{Decision: DecisionAllow, UpdatedInput: `{"command":"rewrite"}`},
		}, `"just-a-string"`)
		require.Empty(t, agg.UpdatedInput)
	})

	t.Run("null updated_input is a no-op", func(t *testing.T) {
		t.Parallel()
		// parseStdout converts null updated_input to "", so aggregate
		// never sees a patch — the merged input is empty and the
		// original tool_input is used unchanged.
		r := parseStdout(`{"decision":"allow","updated_input":null}`)
		require.Empty(t, r.UpdatedInput)
		agg := aggregate([]HookResult{r}, `{"command":"orig"}`)
		require.Empty(t, agg.UpdatedInput)
	})
}

func TestRunnerUpdatedInput(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow","updated_input":{"command":"echo rewritten"}}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	result, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{"command":"echo original","timeout":60}`)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, result.Decision)
	require.JSONEq(t,
		`{"command":"echo rewritten","timeout":60}`,
		result.UpdatedInput,
	)
}

func TestParseStdoutClaudeCodeFormat(t *testing.T) {
	t.Parallel()

	t.Run("allow with reason", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"allow","permissionDecisionReason":"RTK auto-rewrite"}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "RTK auto-rewrite", r.Reason)
	})

	t.Run("allow with updatedInput", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"allow","updatedInput":{"command":"rtk cat foo.go"}}}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, `{"command":"rtk cat foo.go"}`, r.UpdatedInput)
	})

	t.Run("deny", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"not allowed"}}`)
		require.Equal(t, DecisionDeny, r.Decision)
		require.Equal(t, "not allowed", r.Reason)
	})

	t.Run("no permissionDecision", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"hookSpecificOutput":{}}`)
		require.Equal(t, DecisionNone, r.Decision)
	})

	t.Run("crush format still works", func(t *testing.T) {
		t.Parallel()
		r := parseStdout(`{"decision":"allow","context":"hello"}`)
		require.Equal(t, DecisionAllow, r.Decision)
		require.Equal(t, "hello", r.Context)
	})
}
