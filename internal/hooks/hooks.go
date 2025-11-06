package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
)

const DefaultHookTimeout = 30 * time.Second

// HookContext contains context information passed to hooks.
type HookContext struct {
	EventType          config.HookEventType `json:"event_type"`
	SessionID          string               `json:"session_id,omitempty"`
	ToolName           string               `json:"tool_name,omitempty"`
	ToolInput          map[string]any       `json:"tool_input,omitempty"`
	ToolResult         string               `json:"tool_result,omitempty"`
	ToolError          bool                 `json:"tool_error,omitempty"`
	UserPrompt         string               `json:"user_prompt,omitempty"`
	Timestamp          time.Time            `json:"timestamp"`
	WorkingDir         string               `json:"working_dir,omitempty"`
	MessageID          string               `json:"message_id,omitempty"`
	Provider           string               `json:"provider,omitempty"`
	Model              string               `json:"model,omitempty"`
	TokensUsed         int64                `json:"tokens_used,omitempty"`
	TokensInput        int64                `json:"tokens_input,omitempty"`
	PermissionAction   string               `json:"permission_action,omitempty"`
	PermissionPath     string               `json:"permission_path,omitempty"`
	PermissionParams   any                  `json:"permission_params,omitempty"`
	PermissionToolCall string               `json:"permission_tool_call,omitempty"`
}

// Executor executes hooks based on configuration.
type Executor struct {
	config     config.HookConfig
	workingDir string
	shell      *shell.Shell
}

// NewExecutor creates a new hook executor.
func NewExecutor(hookConfig config.HookConfig, workingDir string) *Executor {
	shellInst := shell.NewShell(&shell.Options{
		WorkingDir: workingDir,
	})
	return &Executor{
		config:     hookConfig,
		workingDir: workingDir,
		shell:      shellInst,
	}
}

// Execute runs all hooks matching the given event type and context.
// Returns the first error encountered, causing subsequent hooks to be skipped.
func (e *Executor) Execute(ctx context.Context, hookCtx HookContext) error {
	if e.config == nil || e.shell == nil {
		return nil
	}

	hookCtx.Timestamp = time.Now()
	hookCtx.WorkingDir = e.workingDir

	matchers, ok := e.config[hookCtx.EventType]
	if !ok || len(matchers) == 0 {
		return nil
	}

	for _, matcher := range matchers {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !e.matcherApplies(matcher, hookCtx) {
			continue
		}

		for _, hook := range matcher.Hooks {
			if err := e.executeHook(ctx, hook, hookCtx); err != nil {
				slog.Warn("Hook execution failed",
					"event", hookCtx.EventType,
					"matcher", matcher.Matcher,
					"error", err,
				)
				return err
			}
		}
	}

	return nil
}

// matcherApplies checks if a matcher applies to the given context.
func (e *Executor) matcherApplies(matcher config.HookMatcher, ctx HookContext) bool {
	if matcher.Matcher == "" || matcher.Matcher == "*" {
		return true
	}

	if ctx.EventType == config.PreToolUse || ctx.EventType == config.PostToolUse {
		return matchesToolName(matcher.Matcher, ctx.ToolName)
	}

	// For non-tool events, only empty or wildcard matchers apply
	return matcher.Matcher == "" || matcher.Matcher == "*"
}

// matchesToolName supports pipe-separated patterns like "edit|write|multiedit".
func matchesToolName(pattern, toolName string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}

	// Check for exact match first
	if pattern == toolName {
		return true
	}

	// Check if pattern contains pipes (multiple tool names)
	if !strings.Contains(pattern, "|") {
		return false
	}

	// Split by pipe and check each tool name
	for tool := range strings.SplitSeq(pattern, "|") {
		tool = strings.TrimSpace(tool)
		if tool == toolName {
			return true
		}
	}

	return false
}

// executeHook executes a single hook command.
func (e *Executor) executeHook(ctx context.Context, hook config.Hook, hookCtx HookContext) error {
	if hook.Type != "command" {
		return fmt.Errorf("unsupported hook type: %s", hook.Type)
	}

	timeout := DefaultHookTimeout
	if hook.Timeout != nil {
		timeout = time.Duration(*hook.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	contextJSON, err := json.Marshal(hookCtx)
	if err != nil {
		return fmt.Errorf("failed to marshal hook context: %w", err)
	}

	e.shell.SetEnv("CRUSH_HOOK_EVENT", string(hookCtx.EventType))
	e.shell.SetEnv("CRUSH_HOOK_CONTEXT", string(contextJSON))
	if hookCtx.SessionID != "" {
		e.shell.SetEnv("CRUSH_SESSION_ID", hookCtx.SessionID)
	}
	if hookCtx.ToolName != "" {
		e.shell.SetEnv("CRUSH_TOOL_NAME", hookCtx.ToolName)
	}

	slog.Debug("Executing hook",
		"event", hookCtx.EventType,
		"command", hook.Command,
		"timeout", timeout,
	)

	stdout, stderr, err := e.shell.ExecWithStdin(execCtx, hook.Command, string(contextJSON))
	if err != nil {
		return fmt.Errorf("hook command failed: %w: stdout=%s stderr=%s", err, stdout, stderr)
	}

	if stdout != "" || stderr != "" {
		slog.Debug("Hook output",
			"event", hookCtx.EventType,
			"stdout", stdout,
			"stderr", stderr,
		)
	}

	return nil
}
