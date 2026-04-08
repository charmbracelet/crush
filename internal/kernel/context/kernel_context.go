package context

import (
	"sync"
	"time"

	"charm.land/fantasy"
)

// CompressionLevel represents the tier of compression being applied
// Aligned with Claude Code's 4-tier compression system
type CompressionLevel int

const (
	L1Microcompact CompressionLevel = iota + 1 // Rule-based cleanup, <1ms
	L2AutoCompact                            // Threshold-triggered (85%), uses LLM or SM
	L3FullCompact                            // Fork agent summarization, 5-30s
	L4SessionMemory                          // Use existing summaries, <10ms
)

// Tier represents the tier classification of tool results
type Tier string

const (
	TierMustReapply Tier = "mustReapply"
	TierFrozen      Tier = "frozen"
	TierFresh       Tier = "fresh"
)

// ToolResult represents a tool execution result with tier management
type ToolResult struct {
	ID       string
	ToolName string
	Input    string
	Output   string
	Tier     Tier
	Frozen   bool
}

// CollapseCommit represents a collapsed conversation segment
type CollapseCommit struct {
	ID        string
	Timestamp time.Time
	Messages  []fantasy.Message
	Summary   string
	ParentID  string
}

// ContextManager implements Claude Code's core compression engine
// It manages tool result budgets, frozen results, and conversation collapses
type ContextManager struct {
	mu sync.RWMutex

	toolBudget    map[string]*ToolResult
	frozenResults map[string]*ToolResult
	collapses     []CollapseCommit

	maxToolBudget     int
	maxFrozenResults  int
	maxCollapses      int

	autoCompactThreshold float64
	blockingLimit       int
	consecutiveFailures int
	maxFailures         int

	onCompactHook func([]fantasy.Message) []fantasy.Message
}

// Config holds configuration for the ContextManager
type Config struct {
	MaxToolBudget       int
	MaxFrozenResults    int
	MaxCollapses        int
	AutoCompactThresh   float64
	BlockingLimit       int
	MaxFailures         int
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MaxToolBudget:       20,
		MaxFrozenResults:    50,
		MaxCollapses:        10,
		AutoCompactThresh:   0.85,
		BlockingLimit:       128 * 1024,
		MaxFailures:         3,
	}
}

// New creates a new ContextManager with the given configuration
func New(cfg Config) *ContextManager {
	if cfg.MaxToolBudget == 0 {
		cfg = DefaultConfig()
	}
	return &ContextManager{
		toolBudget:            make(map[string]*ToolResult),
		frozenResults:         make(map[string]*ToolResult),
		collapses:             make([]CollapseCommit, 0, cfg.MaxCollapses),
		maxToolBudget:         cfg.MaxToolBudget,
		maxFrozenResults:      cfg.MaxFrozenResults,
		maxCollapses:          cfg.MaxCollapses,
		autoCompactThreshold:   cfg.AutoCompactThresh,
		blockingLimit:         cfg.BlockingLimit,
		maxFailures:           cfg.MaxFailures,
	}
}

// AddToolResult adds a tool result to the budget
func (cm *ContextManager) AddToolResult(result *ToolResult) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if result.Tier == TierFrozen {
		cm.frozenResults[result.ID] = result
		if len(cm.frozenResults) > cm.maxFrozenResults {
			cm.evictOldestFrozen()
		}
		return
	}

	cm.toolBudget[result.ID] = result
	if len(cm.toolBudget) > cm.maxToolBudget {
		cm.evictOldestToolBudget()
	}
}

// evictOldestToolBudget removes the oldest tool result from budget
func (cm *ContextManager) evictOldestToolBudget() {
	var oldest *ToolResult
	for _, tr := range cm.toolBudget {
		if oldest == nil || tr.ID < oldest.ID {
			oldest = tr
		}
	}
	if oldest != nil {
		delete(cm.toolBudget, oldest.ID)
	}
}

// evictOldestFrozen removes the oldest frozen result
func (cm *ContextManager) evictOldestFrozen() {
	var oldest *ToolResult
	for _, tr := range cm.frozenResults {
		if oldest == nil || tr.ID < oldest.ID {
			oldest = tr
		}
	}
	if oldest != nil {
		delete(cm.frozenResults, oldest.ID)
	}
}

// GetToolResult retrieves a tool result by ID
func (cm *ContextManager) GetToolResult(id string) (*ToolResult, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if tr, ok := cm.frozenResults[id]; ok {
		return tr, true
	}
	if tr, ok := cm.toolBudget[id]; ok {
		return tr, true
	}
	return nil, false
}

// Freeze marks a tool result as frozen (protected from eviction)
func (cm *ContextManager) Freeze(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if tr, ok := cm.toolBudget[id]; ok {
		tr.Tier = TierFrozen
		tr.Frozen = true
		cm.frozenResults[id] = tr
		delete(cm.toolBudget, id)
	}
}

// Unfreeze releases a frozen tool result back to budget
func (cm *ContextManager) Unfreeze(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if tr, ok := cm.frozenResults[id]; ok {
		tr.Tier = TierFresh
		tr.Frozen = false
		cm.toolBudget[id] = tr
		delete(cm.frozenResults, id)
	}
}

// AddCollapse records a collapsed conversation segment
func (cm *ContextManager) AddCollapse(commit CollapseCommit) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.collapses) >= cm.maxCollapses {
		cm.collapses = cm.collapses[1:]
	}
	cm.collapses = append(cm.collapses, commit)
}

// GetCollapses returns all recorded collapses (read-only)
func (cm *ContextManager) GetCollapses() []CollapseCommit {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.collapses
}

// DrainCollapses returns and clears all collapses
func (cm *ContextManager) DrainCollapses() []CollapseCommit {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	collapses := cm.collapses
	cm.collapses = make([]CollapseCommit, 0, cm.maxCollapses)
	return collapses
}

// ShouldAutoCompact checks if automatic compaction should be triggered
// usage is a ratio (0.0 to 1.0) representing context usage
func (cm *ContextManager) ShouldAutoCompact(usage float64) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return usage >= cm.autoCompactThreshold
}

// RecordFailure increments the failure counter
// Returns true if circuit breaker threshold is reached
func (cm *ContextManager) RecordFailure() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.consecutiveFailures++
	return cm.consecutiveFailures >= cm.maxFailures
}

// RecordSuccess resets the failure counter
func (cm *ContextManager) RecordSuccess() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.consecutiveFailures = 0
}

// CircuitTripped returns true if the circuit breaker has tripped
func (cm *ContextManager) CircuitTripped() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.consecutiveFailures >= cm.maxFailures
}

// SetOnCompactHook sets the hook function called during auto-compaction
func (cm *ContextManager) SetOnCompactHook(hook func([]fantasy.Message) []fantasy.Message) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onCompactHook = hook
}

// RunCompactHook executes the registered compaction hook
func (cm *ContextManager) RunCompactHook(messages []fantasy.Message) []fantasy.Message {
	cm.mu.RLock()
	hook := cm.onCompactHook
	cm.mu.RUnlock()

	if hook != nil {
		return hook(messages)
	}
	return messages
}

// ProjectView creates a projected view of messages using collapse history
func (cm *ContextManager) ProjectView(messages []fantasy.Message, collapse bool) []fantasy.Message {
	if !collapse {
		return messages
	}

	collapses := cm.GetCollapses()
	if len(collapses) == 0 {
		return messages
	}

	var projected []fantasy.Message
	var summaryMsgs []fantasy.Message

	for _, msg := range messages {
		projected = append(projected, msg)
	}

	for _, collapse := range collapses {
		summaryMsgs = append(summaryMsgs, fantasy.NewSystemMessage(collapse.Summary))
	}

	projected = append(summaryMsgs, projected...)
	return projected
}

// ApplyToolBudget filters messages based on tool budget management
// Tool results not in budget (and not frozen) are removed
// Note: This is a simplified version; full implementation requires tool call ID tracking
func (cm *ContextManager) ApplyToolBudget(messages []fantasy.Message) []fantasy.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Simple budget-based filtering: if too many tool results, remove older ones
	if len(cm.toolBudget) < cm.maxToolBudget {
		return messages
	}

	// Budget exceeded, apply filtering
	toolResultCount := 0
	var result []fantasy.Message
	for _, msg := range messages {
		if msg.Role == fantasy.MessageRoleTool {
			toolResultCount++
			// Skip tool results beyond budget (keep recent ones)
			if toolResultCount > cm.maxToolBudget {
				continue
			}
		}
		result = append(result, msg)
	}
	return result
}

// Metrics returns current state metrics for monitoring
func (cm *ContextManager) Metrics() map[string]any {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return map[string]any{
		"tool_budget_count":     len(cm.toolBudget),
		"frozen_results_count":  len(cm.frozenResults),
		"collapses_count":       len(cm.collapses),
		"consecutive_failures":  cm.consecutiveFailures,
		"circuit_tripped":       cm.consecutiveFailures >= cm.maxFailures,
		"auto_compact_thresh":   cm.autoCompactThreshold,
		"blocking_limit":        cm.blockingLimit,
	}
}
