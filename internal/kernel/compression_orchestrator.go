package kernel

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"charm.land/fantasy"
	kctx "github.com/charmbracelet/crushcl/internal/kernel/context"
)

// CompressionOrchestrator coordinates between different compression layers
// It manages the 4-tier compression system and determines optimal compression strategy
type CompressionOrchestrator struct {
	// Components
	compactor *kctx.ContextCompactor
	hookPipeline *HookPipeline
	usageTracker *EnhancedUsageTracker

	// Configuration
	config OrchestratorConfig

	// State
	mu sync.RWMutex
	currentLevel kctx.CompressionLevel
	totalCompactions int
	lastCompactTime time.Time
	lastCompactDuration time.Duration
}

// OrchestratorConfig holds configuration for the orchestrator
type OrchestratorConfig struct {
	// Token budgets
	MaxTokenBudget int

	// Threshold ratios
	L1Threshold float64 // Tool count threshold (default: 20)
	L2Threshold float64 // Token ratio threshold (default: 0.85)
	L3Threshold float64 // Token ratio threshold (default: 0.95)

	// Timeout settings
	L1Timeout time.Duration
	L2Timeout time.Duration
	L3Timeout time.Duration
	L4Timeout time.Duration

	// Feature flags
	EnableL1 bool
	EnableL2 bool
	EnableL3 bool
	EnableL4 bool

	// SM Compression settings
	EnableSMCompression bool
	SMPoolSize int
	SMPoolBlockSize int
}

// DefaultOrchestratorConfig returns the default configuration
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxTokenBudget: 200000,
		L1Threshold:    20,
		L2Threshold:    0.85,
		L3Threshold:    0.95,
		L1Timeout:      1 * time.Millisecond,
		L2Timeout:      100 * time.Millisecond,
		L3Timeout:      30 * time.Second,
		L4Timeout:      10 * time.Millisecond,
		EnableL1:       true,
		EnableL2:       true,
		EnableL3:       true,
		EnableL4:       true,
		EnableSMCompression: true,
		SMPoolSize:     50,
		SMPoolBlockSize: 100,
	}
}

// NewCompressionOrchestrator creates a new compression orchestrator
func NewCompressionOrchestrator(config OrchestratorConfig) *CompressionOrchestrator {
	if config.MaxTokenBudget == 0 {
		config = DefaultOrchestratorConfig()
	}

	co := &CompressionOrchestrator{
		config:         config,
		hookPipeline:   NewHookPipeline(),
		usageTracker:   NewEnhancedUsageTracker(),
	}

	// Initialize compactor with SM components
	co.compactor = kctx.NewContextCompactor(config.MaxTokenBudget)

	// Register default hooks
	co.registerDefaultHooks()

	return co
}

// registerDefaultHooks registers the default compression hooks
func (co *CompressionOrchestrator) registerDefaultHooks() {
	// Pre-compact hook
	co.hookPipeline.RegisterHook(&Hook{
		Name:     "pre-compact-logger",
		Phase:    HookPhasePreCompact,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			slog.Debug("Pre-compact hook triggered",
				"level", hookCtx.CompressionLevel,
				"tokens_before", hookCtx.TokensBefore,
				"tokens_after", hookCtx.TokensAfter,
			)
			return nil
		},
	})

	// Post-compact hook
	co.hookPipeline.RegisterHook(&Hook{
		Name:     "post-compact-logger",
		Phase:    HookPhasePostCompact,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			slog.Debug("Post-compact hook triggered",
				"level", hookCtx.CompressionLevel,
				"compression_ratio", func() float64 {
					if hookCtx.TokensBefore > 0 {
						return 1.0 - float64(hookCtx.TokensAfter)/float64(hookCtx.TokensBefore)
					}
					return 0
				}(),
			)
			return nil
		},
	})

	// Pre-tool-use logging hook
	co.hookPipeline.RegisterHook(PreToolUse("tool-pre-logger", func(ctx context.Context, toolName string, input interface{}) error {
		slog.Debug("Pre-tool hook", "tool", toolName)
		return nil
	}))

	// Post-tool-use logging hook
	co.hookPipeline.RegisterHook(PostToolUse("tool-post-logger", func(ctx context.Context, toolName string, input, output interface{}, err error) error {
		if err != nil {
			slog.Debug("Post-tool hook with error", "tool", toolName, "error", err)
		}
		return nil
	}))
}

// Compact determines the appropriate compression level and executes it
func (co *CompressionOrchestrator) Compact(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, error) {
	// Estimate current token count
	tokens := co.estimateTokens(messages)

	// Determine compression level
	level := co.determineLevel(tokens, len(messages))

	// Execute pre-compact hooks
	hookCtx := &HookContext{
		SessionID: sessionID,
		TurnCount: co.totalCompactions,
		CompressionLevel: int(level),
		TokensBefore: tokens,
	}
	co.hookPipeline.ExecutePhase(ctx, HookPhasePreCompact, hookCtx)

	var result []fantasy.Message
	var err error
	var duration time.Duration

	startTime := time.Now()

	switch level {
	case kctx.L1Microcompact:
		result, err = co.executeL1(ctx, messages)
	case kctx.L2AutoCompact:
		result, err = co.executeL2(ctx, messages, sessionID)
	case kctx.L3FullCompact:
		result, err = co.executeL3(ctx, messages, sessionID)
	case kctx.L4SessionMemory:
		result, err = co.executeL4(ctx, messages, sessionID)
	default:
		result = messages
	}

	duration = time.Since(startTime)

	co.mu.Lock()
	co.currentLevel = level
	co.totalCompactions++
	co.lastCompactTime = time.Now()
	co.lastCompactDuration = duration
	co.mu.Unlock()

	// Record compression metrics
	tokensAfter := co.estimateTokens(result)
	hookCtx.TokensAfter = tokensAfter
	co.usageTracker.RecordCompression(sessionID, int(level), tokens, tokensAfter)

	// Execute post-compact hooks
	co.hookPipeline.ExecutePhase(ctx, HookPhasePostCompact, hookCtx)

	if err != nil {
		slog.Warn("Compression failed", "level", level, "error", err)
		return messages, err
	}

	slog.Info("Compression completed",
		"level", level,
		"tokens_before", tokens,
		"tokens_after", tokensAfter,
		"duration_ms", duration.Milliseconds(),
	)

	return result, nil
}

// determineLevel determines the appropriate compression level
func (co *CompressionOrchestrator) determineLevel(tokens int, toolCount int) kctx.CompressionLevel {
	ratio := float64(tokens) / float64(co.config.MaxTokenBudget)

	// Check L4 first (session memory)
	if co.config.EnableL4 && ratio >= co.config.L2Threshold {
		if co.compactor.GetContextManager().GetCollapses() != nil {
			return kctx.L4SessionMemory
		}
	}

	// Check L3 (full compact)
	if co.config.EnableL3 && ratio >= co.config.L3Threshold {
		return kctx.L3FullCompact
	}

	// Check L2 (auto compact)
	if co.config.EnableL2 && ratio >= co.config.L2Threshold {
		return kctx.L2AutoCompact
	}

	// Check L1 (microcompact)
	if co.config.EnableL1 && toolCount > int(co.config.L1Threshold) {
		return kctx.L1Microcompact
	}

	return 0 // No compression needed
}

// executeL1 performs L1 microcompaction
func (co *CompressionOrchestrator) executeL1(ctx context.Context, messages []fantasy.Message) ([]fantasy.Message, error) {
	return co.compactor.L1Microcompact(messages), nil
}

// executeL2 performs L2 auto compaction
func (co *CompressionOrchestrator) executeL2(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, error) {
	return co.compactor.L2AutoCompact(messages, sessionID), nil
}

// executeL3 performs L3 full compaction with fork summarization
func (co *CompressionOrchestrator) executeL3(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, error) {
	// Use fork summarization if available
	compressed, summary, err := co.compactor.TriggerForkSummarize(ctx, messages, sessionID)
	if err != nil {
		return messages, err
	}

	// Record the collapse
	co.compactor.L3FullCompact(compressed, sessionID, summary)

	return compressed, nil
}

// executeL4 performs L4 session memory compression
func (co *CompressionOrchestrator) executeL4(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, error) {
	// Use SM compression if enabled
	if co.config.EnableSMCompression {
		compressed, _, err := co.compactor.SMCompact(messages, nil)
		return compressed, err
	}

	// Fallback to basic session memory
	return co.compactor.L4SessionMemory(messages), nil
}

// estimateTokens estimates token count (rough approximation)
func (co *CompressionOrchestrator) estimateTokens(messages []fantasy.Message) int {
	total := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				total += len(textPart.Text) / 4 // Rough estimate: 4 chars per token
			}
		}
	}
	return total
}

// GetCurrentLevel returns the current compression level
func (co *CompressionOrchestrator) GetCurrentLevel() kctx.CompressionLevel {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.currentLevel
}

// GetTotalCompactions returns the total number of compactions performed
func (co *CompressionOrchestrator) GetTotalCompactions() int {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.totalCompactions
}

// GetLastCompactDuration returns the duration of the last compaction
func (co *CompressionOrchestrator) GetLastCompactDuration() time.Duration {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.lastCompactDuration
}

// ShouldCompact checks if compaction should be triggered
func (co *CompressionOrchestrator) ShouldCompact(tokens int, toolCount int) bool {
	return co.determineLevel(tokens, toolCount) > 0
}

// GetCompactor returns the underlying compactor
func (co *CompressionOrchestrator) GetCompactor() *kctx.ContextCompactor {
	return co.compactor
}

// GetHookPipeline returns the hook pipeline
func (co *CompressionOrchestrator) GetHookPipeline() *HookPipeline {
	return co.hookPipeline
}

// GetUsageTracker returns the usage tracker
func (co *CompressionOrchestrator) GetUsageTracker() *EnhancedUsageTracker {
	return co.usageTracker
}

// RegisterHook registers a custom hook
func (co *CompressionOrchestrator) RegisterHook(hook *Hook) error {
	return co.hookPipeline.RegisterHook(hook)
}

// UnregisterHook removes a hook by name
func (co *CompressionOrchestrator) UnregisterHook(name string) bool {
	return co.hookPipeline.UnregisterHook(name)
}

// Metrics returns comprehensive metrics
func (co *CompressionOrchestrator) Metrics() map[string]interface{} {
	co.mu.RLock()
	defer co.mu.RUnlock()

	return map[string]interface{}{
		"current_level":        co.currentLevel.String(),
		"total_compactions":    co.totalCompactions,
		"last_compact_time":    co.lastCompactTime,
		"last_compact_ms":     co.lastCompactDuration.Milliseconds(),
		"config": map[string]interface{}{
			"max_token_budget": co.config.MaxTokenBudget,
			"l1_threshold":     co.config.L1Threshold,
			"l2_threshold":     co.config.L2Threshold,
			"l3_threshold":     co.config.L3Threshold,
			"enable_sm":        co.config.EnableSMCompression,
		},
		"compactor_metrics": co.compactor.Metrics(),
		"hook_metrics":      co.hookPipeline.Metrics(),
		"usage_metrics":      co.usageTracker.Metrics(),
	}
}

// Reset resets the orchestrator state
func (co *CompressionOrchestrator) Reset() {
	co.mu.Lock()
	defer co.mu.Unlock()

	co.currentLevel = 0
	co.totalCompactions = 0
	co.lastCompactTime = time.Time{}
	co.lastCompactDuration = 0

	co.compactor.FullReset()
	co.hookPipeline.ClearExecutionLog()
}
