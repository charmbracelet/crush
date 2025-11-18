package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
)

const DefaultHookTimeout = 30 * time.Second

// HookContext contains context information passed to hooks.
// Fields are populated based on event type - see field comments for availability.
type HookContext struct {
	// EventType is the lifecycle event that triggered this hook.
	// Available: All events
	EventType config.HookEventType `json:"event_type"`

	// SessionID identifies the Crush session.
	// Available: All events (if session exists)
	SessionID string `json:"session_id,omitempty"`

	// TranscriptPath is the path to exported session transcript JSON file.
	// Available: All events (if session exists and export succeeds)
	TranscriptPath string `json:"transcript_path,omitempty"`

	// ToolName is the name of the tool being called.
	// Available: pre_tool_use, post_tool_use, permission_requested
	ToolName string `json:"tool_name,omitempty"`

	// ToolInput contains the tool's input parameters as key-value pairs.
	// Available: pre_tool_use, post_tool_use
	ToolInput map[string]any `json:"tool_input,omitempty"`

	// ToolResult is the string result returned by the tool.
	// Available: post_tool_use
	ToolResult string `json:"tool_result,omitempty"`

	// ToolError indicates whether the tool execution failed.
	// Available: post_tool_use
	ToolError bool `json:"tool_error,omitempty"`

	// UserPrompt is the prompt submitted by the user.
	// Available: user_prompt_submit
	UserPrompt string `json:"user_prompt,omitempty"`

	// Timestamp is when the hook was triggered (RFC3339 format).
	// Available: All events
	Timestamp time.Time `json:"timestamp"`

	// WorkingDir is the project working directory.
	// Available: All events
	WorkingDir string `json:"working_dir,omitempty"`

	// MessageID identifies the assistant message being processed.
	// Available: pre_tool_use, post_tool_use, stop
	MessageID string `json:"message_id,omitempty"`

	// Provider is the LLM provider name (e.g., "anthropic").
	// Available: All events with LLM interaction
	Provider string `json:"provider,omitempty"`

	// Model is the LLM model name (e.g., "claude-3-5-sonnet-20241022").
	// Available: All events with LLM interaction
	Model string `json:"model,omitempty"`

	// TokensUsed is the total tokens consumed in this interaction.
	// Available: stop
	TokensUsed int64 `json:"tokens_used,omitempty"`

	// TokensInput is the input tokens consumed in this interaction.
	// Available: stop
	TokensInput int64 `json:"tokens_input,omitempty"`

	// PermissionAction is the permission action being requested (e.g., "read", "write").
	// Available: permission_requested
	PermissionAction string `json:"permission_action,omitempty"`

	// PermissionPath is the file path involved in the permission request.
	// Available: permission_requested
	PermissionPath string `json:"permission_path,omitempty"`

	// PermissionParams contains additional permission parameters.
	// Available: permission_requested
	PermissionRequest *permission.PermissionRequest `json:"permission_request,omitempty"`
}

type HookDecision string

const (
	HookDecisionBlock HookDecision = "block"
	HookDecisionDeny  HookDecision = "deny"
	HookDecisionAllow HookDecision = "allow"
	HookDecisionAsk   HookDecision = "ask"
)

type Service interface {
	Execute(ctx context.Context, hookCtx HookContext) ([]message.HookOutput, error)
	SetSmallModel(model fantasy.LanguageModel)
}

type service struct {
	config     config.HookConfig
	workingDir string
	regexCache *csync.Map[string, *regexp.Regexp]
	smallModel fantasy.LanguageModel
	messages   message.Service
}

// NewService creates a new hook executor.
func NewService(
	hookConfig config.HookConfig,
	workingDir string,
	smallModel fantasy.LanguageModel,
	messages message.Service,
) Service {
	return &service{
		config:     hookConfig,
		workingDir: workingDir,
		regexCache: csync.NewMap[string, *regexp.Regexp](),
		smallModel: smallModel,
		messages:   messages,
	}
}

// Execute implements Service.
func (s *service) Execute(ctx context.Context, hookCtx HookContext) ([]message.HookOutput, error) {
	if s.config == nil {
		return nil, nil
	}

	// Check if context is already cancelled - prevents race conditions during cancellation.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	hookCtx.Timestamp = time.Now()
	hookCtx.WorkingDir = s.workingDir

	hooks := s.collectMatchingHooks(hookCtx)
	if len(hooks) == 0 {
		return nil, nil
	}

	transcriptPath, cleanup, err := s.setupTranscript(ctx, hookCtx)
	if err != nil {
		slog.Warn("Failed to export transcript for hooks", "error", err)
	} else if transcriptPath != "" {
		hookCtx.TranscriptPath = transcriptPath
		// Ensure cleanup happens even on panic.
		defer func() {
			cleanup()
		}()
	}

	results := make([]message.HookOutput, len(hooks))
	var wg sync.WaitGroup

	for i, hook := range hooks {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wg.Add(1)
		go func(idx int, h config.Hook) {
			defer wg.Done()
			result, err := s.executeHook(ctx, h, hookCtx)
			if err != nil {
				slog.Warn("Hook execution failed",
					"event", hookCtx.EventType,
					"error", err,
				)
			} else {
				results[idx] = *result
			}
		}(i, hook)
	}
	wg.Wait()
	return results, nil
}

func (s *service) setupTranscript(ctx context.Context, hookCtx HookContext) (string, func(), error) {
	if hookCtx.SessionID == "" || s.messages == nil {
		return "", func() {}, nil
	}

	path, err := exportTranscript(ctx, s.messages, hookCtx.SessionID)
	if err != nil {
		return "", func() {}, err
	}

	cleanup := func() {
		if path != "" {
			cleanupTranscript(path)
		}
	}

	return path, cleanup, nil
}

func (s *service) executeHook(ctx context.Context, hook config.Hook, hookCtx HookContext) (*message.HookOutput, error) {
	var result *message.HookOutput
	var err error

	switch hook.Type {
	case "prompt":
		if hook.Prompt == "" {
			return nil, fmt.Errorf("prompt-based hook missing 'prompt' field")
		}
		if s.smallModel == nil {
			return nil, fmt.Errorf("prompt-based hook requires small model configuration")
		}
		result, err = s.executePromptHook(ctx, hook, hookCtx)
	case "", "command":
		if hook.Command == "" {
			return nil, fmt.Errorf("command-based hook missing 'command' field")
		}
		slog.Info("executing")
		result, err = s.executeCommandHook(ctx, hook, hookCtx)
	default:
		return nil, fmt.Errorf("unsupported hook type: %s", hook.Type)
	}

	if result != nil {
		result.EventType = string(hookCtx.EventType)
	}

	return result, err
}

func (s *service) executePromptHook(ctx context.Context, hook config.Hook, hookCtx HookContext) (*message.HookOutput, error) {
	contextJSON, err := json.Marshal(hookCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hook context: %w", err)
	}

	var finalPrompt string
	if strings.Contains(hook.Prompt, "$ARGUMENTS") {
		finalPrompt = strings.ReplaceAll(hook.Prompt, "$ARGUMENTS", string(contextJSON))
	} else {
		finalPrompt = fmt.Sprintf("%s\n\nContext: %s", hook.Prompt, string(contextJSON))
	}

	timeout := DefaultHookTimeout
	if hook.Timeout != nil {
		timeout = time.Duration(*hook.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type readTranscriptParams struct{}
	readTranscriptTool := fantasy.NewAgentTool(
		"read_transcript",
		"Used to read the conversation so far",
		func(ctx context.Context, params readTranscriptParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if hookCtx.TranscriptPath == "" {
				return fantasy.NewTextErrorResponse("No transcript available"), nil
			}
			data, err := os.ReadFile(hookCtx.TranscriptPath)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(string(data)), nil
		})

	var output *message.HookOutput
	outputTool := fantasy.NewAgentTool(
		"output",
		"Used to submit the output, remember you MUST call this tool at the end",
		func(ctx context.Context, params message.HookOutput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			output = &params
			return fantasy.NewTextResponse("ouptut submitted"), nil
		})
	agent := fantasy.NewAgent(
		s.smallModel,
		fantasy.WithSystemPrompt(`You are a helpful sub agent used in a larger agents conversation loop,
			your goal is to intercept the conversation and fulfill the intermediate requests, makesure to ALWAYS use the output tool at the end to output your decision`),
		fantasy.WithTools(readTranscriptTool, outputTool),
	)

	_, err = agent.Generate(execCtx, fantasy.AgentCall{
		Prompt: finalPrompt,
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (s *service) executeCommandHook(ctx context.Context, hook config.Hook, hookCtx HookContext) (*message.HookOutput, error) {
	timeout := DefaultHookTimeout
	if hook.Timeout != nil {
		timeout = time.Duration(*hook.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	contextJSON, err := json.Marshal(hookCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hook context: %w", err)
	}

	sh := shell.NewShell(&shell.Options{
		WorkingDir: s.workingDir,
	})

	sh.SetEnv("CRUSH_HOOK_EVENT", string(hookCtx.EventType))
	sh.SetEnv("CRUSH_HOOK_CONTEXT", string(contextJSON))
	sh.SetEnv("CRUSH_PROJECT_DIR", s.workingDir)
	if hookCtx.SessionID != "" {
		sh.SetEnv("CRUSH_SESSION_ID", hookCtx.SessionID)
	}
	if hookCtx.ToolName != "" {
		sh.SetEnv("CRUSH_TOOL_NAME", hookCtx.ToolName)
	}

	slog.Debug("Hook execution trace",
		"event", hookCtx.EventType,
		"command", hook.Command,
		"timeout", timeout,
		"context", hookCtx,
	)

	stdout, stderr, err := sh.ExecWithStdin(execCtx, hook.Command, string(contextJSON))

	exitCode := shell.ExitCode(err)

	var result *message.HookOutput
	switch exitCode {
	case 0:
		// if the event is  UserPromptSubmit we want the output to be added to the context
		if hookCtx.EventType == config.UserPromptSubmit {
			result = &message.HookOutput{
				AdditionalContext: stdout,
			}
		} else {
			result = &message.HookOutput{
				Message: stdout,
			}
		}
	case 2:
		result = &message.HookOutput{
			Decision: string(HookDecisionBlock),
			Error:    stderr,
		}
		return result, nil
	default:
		result = &message.HookOutput{
			Error: stderr,
		}
		return result, nil
	}

	jsonOutput := parseHookOutput(stdout)
	if jsonOutput == nil {
		return result, nil
	}

	result.Message = jsonOutput.Message
	result.Stop = jsonOutput.Stop
	result.Decision = jsonOutput.Decision
	result.AdditionalContext = jsonOutput.AdditionalContext
	result.UpdatedInput = jsonOutput.UpdatedInput

	// Trace output in debug mode
	slog.Debug("Hook execution output",
		"event", hookCtx.EventType,
		"exit_code", exitCode,
		"stdout_length", len(stdout),
		"stderr_length", len(stderr),
		"stdout", stdout,
		"stderr", stderr,
	)
	return result, nil
}

func parseHookOutput(stdout string) *message.HookOutput {
	stdout = strings.TrimSpace(stdout)
	slog.Info(stdout)
	if stdout == "" {
		return nil
	}

	var output message.HookOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		// Failed to parse as HookOutput
		return nil
	}

	return &output
}

func (s *service) SetSmallModel(model fantasy.LanguageModel) {
	s.smallModel = model
}

func (s *service) collectMatchingHooks(hookCtx HookContext) []config.Hook {
	matchers, ok := s.config[hookCtx.EventType]
	if !ok || len(matchers) == 0 {
		return nil
	}

	var hooks []config.Hook
	for _, matcher := range matchers {
		if !s.matcherApplies(matcher, hookCtx) {
			continue
		}
		hooks = append(hooks, matcher.Hooks...)
	}
	return hooks
}

func (s *service) matcherApplies(matcher config.HookMatcher, ctx HookContext) bool {
	if ctx.EventType == config.PreToolUse || ctx.EventType == config.PostToolUse {
		return s.matchesToolName(matcher.Matcher, ctx.ToolName)
	}

	return matcher.Matcher == "" || matcher.Matcher == "*"
}

func (s *service) matchesToolName(pattern, toolName string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}

	if pattern == toolName {
		return true
	}

	if strings.Contains(pattern, "|") {
		for tool := range strings.SplitSeq(pattern, "|") {
			tool = strings.TrimSpace(tool)
			if tool == toolName {
				return true
			}
		}

		return s.matchesRegex(pattern, toolName)
	}

	return s.matchesRegex(pattern, toolName)
}

func (s *service) matchesRegex(pattern, text string) bool {
	re, ok := s.regexCache.Get(pattern)
	if !ok {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			// Not a valid regex, don't cache failures.
			return false
		}
		re = s.regexCache.GetOrSet(pattern, func() *regexp.Regexp {
			return compiled
		})
	}

	if re == nil {
		return false
	}

	return re.MatchString(text)
}
