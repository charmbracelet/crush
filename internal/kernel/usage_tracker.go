package kernel

import (
	"sync"
	"time"

	"charm.land/fantasy"
)

// UsageRecord represents a single usage record
type UsageRecord struct {
	SessionID    string
	Timestamp    time.Time
	Model        string
	Provider     string

	// Token counts
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64

	// Derived metrics
	TotalTokens int64
	Cost        float64

	// Context metrics
	ContextTokensBefore int
	ContextTokensAfter  int
	CompressionRatio    float64

	// Tool metrics
	ToolCallsCount   int
	ToolCallsTime    time.Duration
	MostUsedTool     string
	ToolUsageByName  map[string]int

	// Turn metrics
	TurnNumber int
	TurnDuration time.Duration

	// Error tracking
	HasError   bool
	ErrorCount int
}

// EnhancedUsageTracker provides comprehensive token and cost tracking
type EnhancedUsageTracker struct {
	mu sync.RWMutex

	// Per-session tracking
	sessions map[string]*SessionUsage

	// Global aggregates
	globalStats UsageStats

	// History for trend analysis
	history []UsageRecord
	maxHistory int

	// Budget management
	budgets map[string]*Budget
}

// SessionUsage tracks usage for a single session
type SessionUsage struct {
	SessionID     string
	StartTime     time.Time
	LastUpdated   time.Time
	Records       []UsageRecord

	// Running totals
	TotalInputTokens         int64
	TotalOutputTokens        int64
	TotalCacheReadTokens     int64
	TotalCacheCreationTokens int64
	TotalCost                float64
	TotalToolCalls           int
	TotalTurns               int

	// Compression stats
	TotalCompactions     int
	CompressionRatioSum   float64
	TokensSavedByCompression int64

	// Error tracking
	ErrorCount    int
	LastError     error

	// Tool usage
	ToolUsage map[string]int

	// Turn timing
	TurnDurations []time.Duration
}

// UsageStats holds global statistics
type UsageStats struct {
	TotalSessions        int
	ActiveSessions       int
	TotalInputTokens     int64
	TotalOutputTokens    int64
	TotalCacheTokens     int64
	TotalCost            float64
	TotalToolCalls       int
	TotalCompactions     int
	AverageContextRatio  float64
	PeakContextTokens    int
	MostUsedModel        string
	MostUsedProvider     string
}

// Budget represents a usage budget
type Budget struct {
	Name         string
	MaxTokens    int64
	MaxCost      float64
	CurrentUsage int64
	CurrentCost  float64
	ResetTime   time.Time
	AlertAt     float64 // Percentage at which to alert
}

// NewEnhancedUsageTracker creates a new usage tracker
func NewEnhancedUsageTracker() *EnhancedUsageTracker {
	return &EnhancedUsageTracker{
		sessions:    make(map[string]*SessionUsage),
		history:     make([]UsageRecord, 0),
		maxHistory:  10000,
		budgets:    make(map[string]*Budget),
	}
}

// StartSession initializes tracking for a new session
func (t *EnhancedUsageTracker) StartSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sessions[sessionID] = &SessionUsage{
		SessionID:   sessionID,
		StartTime:   time.Now(),
		LastUpdated: time.Now(),
		Records:     make([]UsageRecord, 0),
		ToolUsage:   make(map[string]int),
	}

	t.globalStats.TotalSessions++
	t.globalStats.ActiveSessions++
}

// EndSession finalizes tracking for a session
func (t *EnhancedUsageTracker) EndSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return
	}

	session.LastUpdated = time.Now()
	t.globalStats.ActiveSessions--

	// Move session stats to history
	t.globalStats.TotalInputTokens += session.TotalInputTokens
	t.globalStats.TotalOutputTokens += session.TotalOutputTokens
	t.globalStats.TotalCacheTokens += session.TotalCacheReadTokens + session.TotalCacheCreationTokens
	t.globalStats.TotalCost += session.TotalCost
	t.globalStats.TotalToolCalls += session.TotalToolCalls
}

// RecordUsage records token usage for a session
func (t *EnhancedUsageTracker) RecordUsage(sessionID string, model, provider string, usage fantasy.Usage, cost float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		session = &SessionUsage{
			SessionID: sessionID,
			StartTime: time.Now(),
			ToolUsage: make(map[string]int),
		}
		t.sessions[sessionID] = session
	}

	record := UsageRecord{
		SessionID:           sessionID,
		Timestamp:           time.Now(),
		Model:               model,
		Provider:            provider,
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		TotalTokens:        usage.TotalTokens,
		Cost:               cost,
	}

	// Update session totals
	session.TotalInputTokens += usage.InputTokens
	session.TotalOutputTokens += usage.OutputTokens
	session.TotalCacheReadTokens += usage.CacheReadTokens
	session.TotalCacheCreationTokens += usage.CacheCreationTokens
	session.TotalCost += cost
	session.LastUpdated = time.Now()

	// Add to history
	session.Records = append(session.Records, record)
	t.history = append(t.history, record)

	// Trim history if too large
	if len(t.history) > t.maxHistory {
		t.history = t.history[len(t.history)-t.maxHistory:]
	}
}

// RecordToolCall records a tool call
func (t *EnhancedUsageTracker) RecordToolCall(sessionID, toolName string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return
	}

	session.TotalToolCalls++
	session.ToolUsage[toolName]++

	// Update record with tool info
	if len(session.Records) > 0 {
		record := &session.Records[len(session.Records)-1]
		record.ToolCallsCount++
		record.ToolCallsTime += duration
		record.MostUsedTool = toolName
		if record.ToolUsageByName == nil {
			record.ToolUsageByName = make(map[string]int)
		}
		record.ToolUsageByName[toolName]++
	}
}

// RecordTurn records a turn
func (t *EnhancedUsageTracker) RecordTurn(sessionID string, duration time.Duration, contextBefore, contextAfter int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return
	}

	session.TotalTurns++
	session.TurnDurations = append(session.TurnDurations, duration)

	record := &UsageRecord{
		SessionID:           sessionID,
		Timestamp:          time.Now(),
		TurnNumber:         session.TotalTurns,
		TurnDuration:       duration,
		ContextTokensBefore: contextBefore,
		ContextTokensAfter:  contextAfter,
	}

	// Calculate compression ratio
	if contextBefore > 0 {
		record.CompressionRatio = 1.0 - float64(contextAfter)/float64(contextBefore)
	}

	session.Records = append(session.Records, *record)
}

// RecordCompression records a compression event
func (t *EnhancedUsageTracker) RecordCompression(sessionID string, level int, tokensBefore, tokensAfter int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return
	}

	session.TotalCompactions++

	if tokensBefore > 0 && tokensAfter >= 0 {
		ratio := 1.0 - float64(tokensAfter)/float64(tokensBefore)
		session.CompressionRatioSum += ratio
		session.TokensSavedByCompression += int64(tokensBefore - tokensAfter)
	}

	// Update global stats
	t.globalStats.TotalCompactions++
}

// RecordError records an error
func (t *EnhancedUsageTracker) RecordError(sessionID string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return
	}

	session.ErrorCount++
	session.LastError = err
}

// GetSessionUsage returns usage stats for a session
func (t *EnhancedUsageTracker) GetSessionUsage(sessionID string) *SessionUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	session, ok := t.sessions[sessionID]
	if !ok {
		return nil
	}

	// Return a copy to avoid race conditions
	usage := *session
	usage.Records = make([]UsageRecord, len(session.Records))
	copy(usage.Records, session.Records)
	usage.TurnDurations = make([]time.Duration, len(session.TurnDurations))
	copy(usage.TurnDurations, session.TurnDurations)

	return &usage
}

// GetGlobalStats returns global statistics
func (t *EnhancedUsageTracker) GetGlobalStats() UsageStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := t.globalStats

	// Calculate averages
	if t.globalStats.TotalSessions > 0 {
		stats.AverageContextRatio = float64(stats.TotalInputTokens) / float64(stats.TotalSessions)
	}

	// Find peak
	for _, session := range t.sessions {
		total := session.TotalInputTokens + session.TotalOutputTokens
		if int(total) > stats.PeakContextTokens {
			stats.PeakContextTokens = int(total)
		}
	}

	return stats
}

// GetHistory returns recent usage history
func (t *EnhancedUsageTracker) GetHistory(limit int) []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.history) {
		limit = len(t.history)
	}

	history := make([]UsageRecord, limit)
	copy(history, t.history[len(t.history)-limit:])

	return history
}

// SetBudget creates or updates a budget
func (t *EnhancedUsageTracker) SetBudget(budget *Budget) {
	t.mu.Lock()
	defer t.mu.Unlock()

	budget.ResetTime = time.Now().Add(24 * time.Hour) // Default daily reset
	t.budgets[budget.Name] = budget
}

// CheckBudget checks if a budget would be exceeded
func (t *EnhancedUsageTracker) CheckBudget(name string, additionalTokens int64, additionalCost float64) (bool, *Budget) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	budget, ok := t.budgets[name]
	if !ok {
		return true, nil // No budget = unlimited
	}

	// Check if reset time passed
	if time.Now().After(budget.ResetTime) {
		budget.CurrentUsage = 0
		budget.CurrentCost = 0
		budget.ResetTime = time.Now().Add(24 * time.Hour)
	}

	// Check limits
	willExceedTokens := budget.CurrentUsage+additionalTokens > budget.MaxTokens
	willExceedCost := budget.CurrentCost+additionalCost > budget.MaxCost

	if willExceedTokens || willExceedCost {
		return false, budget
	}

	return true, budget
}

// UpdateBudgetUsage updates the current budget usage
func (t *EnhancedUsageTracker) UpdateBudgetUsage(name string, tokens int64, cost float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	budget, ok := t.budgets[name]
	if !ok {
		return
	}

	budget.CurrentUsage += tokens
	budget.CurrentCost += cost
}

// GetBudgetStatus returns the status of all budgets
func (t *EnhancedUsageTracker) GetBudgetStatus() map[string]*Budget {
	t.mu.RLock()
	defer t.mu.RUnlock()

	status := make(map[string]*Budget)
	for name, budget := range t.budgets {
		status[name] = budget
	}
	return status
}

// Metrics returns comprehensive metrics
func (t *EnhancedUsageTracker) Metrics() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := t.GetGlobalStats()

	// Calculate session metrics
	sessionMetrics := make([]map[string]interface{}, 0, len(t.sessions))
	for _, session := range t.sessions {
		avgTurnDuration := time.Duration(0)
		if len(session.TurnDurations) > 0 {
			total := time.Duration(0)
			for _, d := range session.TurnDurations {
				total += d
			}
			avgTurnDuration = total / time.Duration(len(session.TurnDurations))
		}

		avgCompression := 0.0
		if session.TotalCompactions > 0 {
			avgCompression = session.CompressionRatioSum / float64(session.TotalCompactions)
		}

		sessionMetrics = append(sessionMetrics, map[string]interface{}{
			"session_id":                   session.SessionID,
			"total_turns":                  session.TotalTurns,
			"total_tool_calls":             session.TotalToolCalls,
			"total_input_tokens":           session.TotalInputTokens,
			"total_output_tokens":          session.TotalOutputTokens,
			"total_cost":                   session.TotalCost,
			"total_compactions":            session.TotalCompactions,
			"tokens_saved_by_compression":  session.TokensSavedByCompression,
			"avg_compression_ratio":        avgCompression,
			"avg_turn_duration_ms":         avgTurnDuration.Milliseconds(),
			"error_count":                  session.ErrorCount,
			"duration_minutes":             time.Since(session.StartTime).Minutes(),
		})
	}

	return map[string]interface{}{
		"global_stats": map[string]interface{}{
			"total_sessions":        stats.TotalSessions,
			"active_sessions":       stats.ActiveSessions,
			"total_input_tokens":    stats.TotalInputTokens,
			"total_output_tokens":    stats.TotalOutputTokens,
			"total_cache_tokens":    stats.TotalCacheTokens,
			"total_cost":            stats.TotalCost,
			"total_tool_calls":      stats.TotalToolCalls,
			"total_compactions":     stats.TotalCompactions,
			"peak_context_tokens":   stats.PeakContextTokens,
		},
		"budgets": t.GetBudgetStatus(),
		"sessions": sessionMetrics,
		"history_count": len(t.history),
	}
}
