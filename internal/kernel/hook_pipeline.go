package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
)

// HookPhase represents the phase at which a hook executes
type HookPhase string

const (
	HookPhasePreToolUse    HookPhase = "pre_tool_use"
	HookPhasePostToolUse   HookPhase = "post_tool_use"
	HookPhasePreCompact    HookPhase = "pre_compact"
	HookPhasePostCompact   HookPhase = "post_compact"
	HookPhasePreAgent      HookPhase = "pre_agent"
	HookPhasePostAgent     HookPhase = "post_agent"
	HookPhaseOnError       HookPhase = "on_error"
	HookPhaseOnLoopDetected HookPhase = "on_loop_detected"
)

// HookPriority determines hook execution order within a phase
type HookPriority int

const (
	HookPriorityHigh   HookPriority = 100 // System hooks
	HookPriorityMedium HookPriority = 50  // Core functionality
	HookPriorityLow    HookPriority = 10  // User/custom hooks
)

// HookFunc is the function signature for a hook
type HookFunc func(ctx context.Context, hookCtx *HookContext) error

// Hook defines a single hook instance
type Hook struct {
	Name     string
	Phase    HookPhase
	Priority HookPriority
	Fn       HookFunc
	Enabled  bool
}

// HookContext holds context information passed to hooks
type HookContext struct {
	// Session info
	SessionID string
	TurnCount int

	// Tool info (for tool hooks)
	ToolName    string
	ToolInput   interface{}
	ToolOutput  interface{}
	ToolError   error

	// Message info (for agent hooks)
	Messages []fantasy.Message

	// Compact info (for compact hooks)
	CompressionLevel int
	TokensBefore    int
	TokensAfter     int

	// Error info (for error hooks)
	Error error

	// Loop info (for loop detection hooks)
	LoopSignature string
	LoopCount     int

	// Metadata
	Metadata map[string]interface{}

	mu sync.RWMutex
}

// SetMetadata sets a metadata value
func (hc *HookContext) SetMetadata(key string, value interface{}) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.Metadata == nil {
		hc.Metadata = make(map[string]interface{})
	}
	hc.Metadata[key] = value
}

// GetMetadata gets a metadata value
func (hc *HookContext) GetMetadata(key string) (interface{}, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if hc.Metadata == nil {
		return nil, false
	}
	val, ok := hc.Metadata[key]
	return val, ok
}

// HookPipeline manages all hooks and their execution
type HookPipeline struct {
	hooks map[HookPhase][]*Hook
	mu    sync.RWMutex

	// Execution tracking
	executionLog []HookExecution
	maxLogSize  int
}

// HookExecution records a single hook execution
type HookExecution struct {
	HookName    string
	Phase       HookPhase
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Error       error
	Success     bool
}

// NewHookPipeline creates a new hook pipeline
func NewHookPipeline() *HookPipeline {
	return &HookPipeline{
		hooks:       make(map[HookPhase][]*Hook),
		executionLog: make([]HookExecution, 0),
		maxLogSize:  1000,
	}
}

// RegisterHook registers a new hook
func (hp *HookPipeline) RegisterHook(hook *Hook) error {
	if hook.Name == "" {
		return fmt.Errorf("hook name cannot be empty")
	}
	if hook.Fn == nil {
		return fmt.Errorf("hook function cannot be nil")
	}
	if hook.Phase == "" {
		return fmt.Errorf("hook phase cannot be empty")
	}

	hp.mu.Lock()
	defer hp.mu.Unlock()

	hook.Enabled = true
	hp.hooks[hook.Phase] = append(hp.hooks[hook.Phase], hook)

	// Sort by priority (descending)
	hp.sortHooks(hook.Phase)

	return nil
}

// UnregisterHook removes a hook by name
func (hp *HookPipeline) UnregisterHook(name string) bool {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	for phase, hooks := range hp.hooks {
		for i, h := range hooks {
			if h.Name == name {
				hp.hooks[phase] = append(hooks[:i], hooks[i+1:]...)
				return true
			}
		}
	}
	return false
}

// EnableHook enables a hook by name
func (hp *HookPipeline) EnableHook(name string) bool {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	for _, hooks := range hp.hooks {
		for _, h := range hooks {
			if h.Name == name {
				h.Enabled = true
				return true
			}
		}
	}
	return false
}

// DisableHook disables a hook by name
func (hp *HookPipeline) DisableHook(name string) bool {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	for _, hooks := range hp.hooks {
		for _, h := range hooks {
			if h.Name == name {
				h.Enabled = false
				return true
			}
		}
	}
	return false
}

// ExecutePhase executes all hooks for a given phase
func (hp *HookPipeline) ExecutePhase(ctx context.Context, phase HookPhase, hookCtx *HookContext) []error {
	hp.mu.RLock()
	hooks := hp.hooks[phase]
	hp.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	var errors []error

	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}

		exec := HookExecution{
			HookName:  hook.Name,
			Phase:     phase,
			StartTime: time.Now(),
		}

		// Create a hook-specific context with timeout
		hookTimeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := hook.Fn(hookTimeoutCtx, hookCtx)
		exec.EndTime = time.Now()
		exec.Duration = exec.EndTime.Sub(exec.StartTime)

		if err != nil {
			exec.Error = err
			exec.Success = false
			errors = append(errors, err)
		} else {
			exec.Success = true
		}

		// Log execution
		hp.logExecution(exec)
	}

	return errors
}

// ExecuteToolHooks executes pre/post tool hooks
func (hp *HookPipeline) ExecuteToolHooks(ctx context.Context, hookCtx *HookContext, toolName string, input interface{}, output interface{}, toolErr error) []error {
	hookCtx.ToolName = toolName
	hookCtx.ToolInput = input
	hookCtx.ToolOutput = output
	hookCtx.ToolError = toolErr

	var errors []error

	// Pre-tool hooks
	if preErrors := hp.ExecutePhase(ctx, HookPhasePreToolUse, hookCtx); len(preErrors) > 0 {
		errors = append(errors, preErrors...)
	}

	// Post-tool hooks (only if pre-tool succeeded or no pre-tool errors)
	if len(errors) == 0 || hp.shouldContinueOnError(HookPhasePreToolUse) {
		if postErrors := hp.ExecutePhase(ctx, HookPhasePostToolUse, hookCtx); len(postErrors) > 0 {
			errors = append(errors, postErrors...)
		}
	}

	return errors
}

// ExecuteCompactHooks executes pre/post compact hooks
func (hp *HookPipeline) ExecuteCompactHooks(ctx context.Context, hookCtx *HookContext, level int, tokensBefore, tokensAfter int) []error {
	hookCtx.CompressionLevel = level
	hookCtx.TokensBefore = tokensBefore
	hookCtx.TokensAfter = tokensAfter

	var errors []error

	// Pre-compact hooks
	if preErrors := hp.ExecutePhase(ctx, HookPhasePreCompact, hookCtx); len(preErrors) > 0 {
		errors = append(errors, preErrors...)
	}

	// Post-compact hooks
	if postErrors := hp.ExecutePhase(ctx, HookPhasePostCompact, hookCtx); len(postErrors) > 0 {
		errors = append(errors, postErrors...)
	}

	return errors
}

// ExecuteAgentHooks executes pre/post agent hooks
func (hp *HookPipeline) ExecuteAgentHooks(ctx context.Context, hookCtx *HookContext, messages []fantasy.Message) []error {
	hookCtx.Messages = messages

	var errors []error

	if preErrors := hp.ExecutePhase(ctx, HookPhasePreAgent, hookCtx); len(preErrors) > 0 {
		errors = append(errors, preErrors...)
	}

	if postErrors := hp.ExecutePhase(ctx, HookPhasePostAgent, hookCtx); len(postErrors) > 0 {
		errors = append(errors, postErrors...)
	}

	return errors
}

// ExecuteErrorHook executes the error hook
func (hp *HookPipeline) ExecuteErrorHook(ctx context.Context, hookCtx *HookContext, err error) {
	hookCtx.Error = err
	hp.ExecutePhase(ctx, HookPhaseOnError, hookCtx)
}

// ExecuteLoopDetectedHook executes the loop detection hook
func (hp *HookPipeline) ExecuteLoopDetectedHook(ctx context.Context, hookCtx *HookContext, signature string, count int) {
	hookCtx.LoopSignature = signature
	hookCtx.LoopCount = count
	hp.ExecutePhase(ctx, HookPhaseOnLoopDetected, hookCtx)
}

// GetHookByName returns a hook by name
func (hp *HookPipeline) GetHookByName(name string) *Hook {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	for _, hooks := range hp.hooks {
		for _, h := range hooks {
			if h.Name == name {
				return h
			}
		}
	}
	return nil
}

// ListHooks returns all registered hooks
func (hp *HookPipeline) ListHooks() []*Hook {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	var result []*Hook
	for _, hooks := range hp.hooks {
		result = append(result, hooks...)
	}
	return result
}

// GetExecutionLog returns the recent execution log
func (hp *HookPipeline) GetExecutionLog() []HookExecution {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	log := make([]HookExecution, len(hp.executionLog))
	copy(log, hp.executionLog)
	return log
}

// ClearExecutionLog clears the execution log
func (hp *HookPipeline) ClearExecutionLog() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.executionLog = make([]HookExecution, 0)
}

// sortHooks sorts hooks by priority (descending)
func (hp *HookPipeline) sortHooks(phase HookPhase) {
	hooks := hp.hooks[phase]
	for i := 0; i < len(hooks); i++ {
		for j := i + 1; j < len(hooks); j++ {
			if hooks[j].Priority > hooks[i].Priority {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			}
		}
	}
}

// logExecution records a hook execution
func (hp *HookPipeline) logExecution(exec HookExecution) {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	hp.executionLog = append(hp.executionLog, exec)

	// Trim log if too large
	if len(hp.executionLog) > hp.maxLogSize {
		hp.executionLog = hp.executionLog[len(hp.executionLog)-hp.maxLogSize:]
	}
}

// shouldContinueOnError checks if we should continue execution despite pre-hook errors
func (hp *HookPipeline) shouldContinueOnError(phase HookPhase) bool {
	// Default: don't continue on error for most phases
	return false
}

// Metrics returns hook pipeline metrics
func (hp *HookPipeline) Metrics() map[string]interface{} {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	phaseCounts := make(map[string]int)
	enabledCounts := make(map[string]int)

	for phase, hooks := range hp.hooks {
		phaseCounts[string(phase)] = len(hooks)
		enabled := 0
		for _, h := range hooks {
			if h.Enabled {
				enabled++
			}
		}
		enabledCounts[string(phase)] = enabled
	}

	// Calculate recent success rate
	recentExecutions := hp.executionLog
	if len(recentExecutions) > 100 {
		recentExecutions = recentExecutions[len(recentExecutions)-100:]
	}

	successCount := 0
	totalCount := len(recentExecutions)
	for _, exec := range recentExecutions {
		if exec.Success {
			successCount++
		}
	}

	successRate := 0.0
	if totalCount > 0 {
		successRate = float64(successCount) / float64(totalCount) * 100
	}

	return map[string]interface{}{
		"total_hooks":     len(hp.hooks),
		"phase_counts":    phaseCounts,
		"enabled_counts":  enabledCounts,
		"execution_count": len(hp.executionLog),
		"success_rate":    successRate,
	}
}

// Common Hook Factories

// NewLoggingHook creates a logging hook
func NewLoggingHook(name string, phase HookPhase) *Hook {
	return &Hook{
		Name:     name,
		Phase:    phase,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			// Simple logging - in production, use proper logging
			return nil
		},
	}
}

// NewMetricsHook creates a hook that records metrics
func NewMetricsHook(name string, phase HookPhase) *Hook {
	return &Hook{
		Name:     name,
		Phase:    phase,
		Priority: HookPriorityHigh,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			// Record metrics
			return nil
		},
	}
}

// NewValidationHook creates a hook that validates tool input/output
func NewValidationHook(name string, toolName string, validator func(interface{}) error) *Hook {
	return &Hook{
		Name:     name,
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityHigh,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			if hookCtx.ToolName == toolName {
				return validator(hookCtx.ToolInput)
			}
			return nil
		},
	}
}

// NewFilteringHook creates a hook that can filter/modify messages
func NewFilteringHook(name string, phase HookPhase, filter func(context.Context, *HookContext) error) *Hook {
	return &Hook{
		Name:     name,
		Phase:    phase,
		Priority: HookPriorityMedium,
		Fn:       filter,
	}
}

// HookBuilder provides a fluent API for building hooks
type HookBuilder struct {
	hook *Hook
}

// NewHookBuilder creates a new hook builder
func NewHookBuilder(name string, phase HookPhase) *HookBuilder {
	return &HookBuilder{
		hook: &Hook{
			Name:     name,
			Phase:    phase,
			Priority: HookPriorityMedium,
			Enabled:  true,
		},
	}
}

// WithPriority sets the hook priority
func (hb *HookBuilder) WithPriority(priority HookPriority) *HookBuilder {
	hb.hook.Priority = priority
	return hb
}

// WithFunc sets the hook function
func (hb *HookBuilder) WithFunc(fn HookFunc) *HookBuilder {
	hb.hook.Fn = fn
	return hb
}

// Build returns the configured hook
func (hb *HookBuilder) Build() *Hook {
	return hb.hook
}

// PreToolUse creates a pre-tool-use hook with a simple function
func PreToolUse(name string, fn func(ctx context.Context, toolName string, input interface{}) error) *Hook {
	return NewHookBuilder(name, HookPhasePreToolUse).
		WithFunc(func(ctx context.Context, hookCtx *HookContext) error {
			return fn(ctx, hookCtx.ToolName, hookCtx.ToolInput)
		}).Build()
}

// PostToolUse creates a post-tool-use hook with a simple function
func PostToolUse(name string, fn func(ctx context.Context, toolName string, input, output interface{}, err error) error) *Hook {
	return NewHookBuilder(name, HookPhasePostToolUse).
		WithFunc(func(ctx context.Context, hookCtx *HookContext) error {
			return fn(ctx, hookCtx.ToolName, hookCtx.ToolInput, hookCtx.ToolOutput, hookCtx.ToolError)
		}).Build()
}

// OnError creates an error hook with a simple function
func OnError(name string, fn func(ctx context.Context, err error) error) *Hook {
	return NewHookBuilder(name, HookPhaseOnError).
		WithFunc(func(ctx context.Context, hookCtx *HookContext) error {
			return fn(ctx, hookCtx.Error)
		}).Build()
}

// ToolNameMatcher is a helper for matching tool names
func ToolNameMatcher(ctx *HookContext, names ...string) bool {
	for _, name := range names {
		if ctx.ToolName == name {
			return true
		}
	}
	return false
}

// ExtractTopicsFromMessages extracts simple topics from messages
func ExtractTopicsFromMessages(messages []fantasy.Message) []string {
	topicSet := make(map[string]bool)

	keywords := []string{
		"code", "test", "bug", "feature", "api", "file", "error",
		"build", "run", "debug", "refactor", "optimize", "security",
		"database", "cache", "config", "deploy", "monitor", "log",
	}

	for _, msg := range messages {
		text := extractTextFromMessage(msg)
		lowerText := strings.ToLower(text)

		for _, kw := range keywords {
			if strings.Contains(lowerText, kw) {
				topicSet[kw] = true
			}
		}
	}

	topics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topics = append(topics, t)
	}
	return topics
}

// extractTextFromMessage extracts text from a fantasy message
func extractTextFromMessage(msg fantasy.Message) string {
	var sb strings.Builder
	for _, part := range msg.Content {
		if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
			sb.WriteString(textPart.Text)
		}
	}
	return sb.String()
}
