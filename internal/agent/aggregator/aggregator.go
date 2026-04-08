// Package aggregator provides result aggregation functionality for multi-agent task coordination.
package aggregator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Result status types
type ResultStatus int

const (
	ResultStatusPending ResultStatus = iota
	ResultStatusSuccess
	ResultStatusPartial
	ResultStatusFailed
	ResultStatusTimeout
)

func (s ResultStatus) String() string {
	switch s {
	case ResultStatusPending:
		return "pending"
	case ResultStatusSuccess:
		return "success"
	case ResultStatusPartial:
		return "partial"
	case ResultStatusFailed:
		return "failed"
	case ResultStatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// Merge strategy types
type MergeStrategy int

const (
	MergeStrategyFirst MergeStrategy = iota
	MergeStrategyLast
	MergeStrategyPriority
	MergeStrategyNewest
	MergeStrategyAll
)

// Configuration for the aggregator
type AggregatorConfig struct {
	Timeout         time.Duration // Result collection timeout
	MaxResults      int           // Maximum number of results
	MinResults      int           // Minimum required results
	PriorityEnabled bool          // Enable priority sorting
	ConflictEnabled bool          // Enable conflict detection
	MergeStrategy   MergeStrategy // Merge strategy
}

// Task result from an agent
type TaskResult struct {
	AgentID   string
	TaskID    string
	Status    ResultStatus
	Data      interface{}
	Err       error
	Priority  int
	Timestamp time.Time
	Duration  time.Duration
	Metadata  map[string]interface{}
}

// Conflict represents a detected conflict between results
type Conflict struct {
	TaskID     string
	Results    []*TaskResult
	Resolution ConflictResolution
	ResolvedBy string
}

// Conflict resolution types
type ConflictResolution int

const (
	ConflictResolutionNone ConflictResolution = iota
	ConflictResolutionPriority
	ConflictResolutionMajority
	ConflictResolutionConsensus
)

// Aggregated result combining multiple task results
type AggregatedResult struct {
	TaskID       string
	Status       ResultStatus
	Primary      *TaskResult   // Primary result
	Alternatives []*TaskResult // Alternative results
	Conflicts    []*Conflict   // Conflict information
	Summary      string        // Result summary
	Metadata     map[string]interface{}
}

// Errors
var (
	ErrNoResults          = errors.New("no results available")
	ErrTimeout            = errors.New("result collection timeout")
	ErrConflictUnresolved = errors.New("result conflict could not be resolved")
	ErrMaxResultsExceeded = errors.New("maximum results exceeded")
	ErrTaskNotFound       = errors.New("task not found")
)

// ResultAggregatorInterface defines the interface for result aggregation
type ResultAggregatorInterface interface {
	Submit(result *TaskResult) error
	Aggregate(taskID string) (*AggregatedResult, error)
	WaitAll(taskID string) ([]*TaskResult, error)
	GetBest(taskID string) (*TaskResult, error)
	Cancel(taskID string)
	OnTimeout(taskID string, callback func())
}

// ResultAggregator collects and aggregates results from multiple agents
type ResultAggregator struct {
	mu         sync.RWMutex
	results    map[string][]*TaskResult // taskID -> results
	conflicts  map[string]*Conflict
	config     AggregatorConfig
	timeouts   map[string]time.Time
	timeoutCbs map[string][]func()
	done       chan struct{}
}

// New creates a new ResultAggregator
func New(config AggregatorConfig) *ResultAggregator {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxResults == 0 {
		config.MaxResults = 100
	}
	if config.MinResults == 0 {
		config.MinResults = 1
	}

	return &ResultAggregator{
		results:    make(map[string][]*TaskResult),
		conflicts:  make(map[string]*Conflict),
		config:     config,
		timeouts:   make(map[string]time.Time),
		timeoutCbs: make(map[string][]func()),
		done:       make(chan struct{}),
	}
}

// Submit submits a result for aggregation
func (ra *ResultAggregator) Submit(result *TaskResult) error {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	if result == nil {
		return errors.New("result cannot be nil")
	}

	if result.TaskID == "" {
		return errors.New("task ID cannot be empty")
	}

	// Set timestamp if not set
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now()
	}

	// Check max results limit
	if ra.config.MaxResults > 0 {
		if len(ra.results[result.TaskID]) >= ra.config.MaxResults {
			return ErrMaxResultsExceeded
		}
	}

	// Store result
	ra.results[result.TaskID] = append(ra.results[result.TaskID], result)

	return nil
}

// Aggregate aggregates all results for a task
func (ra *ResultAggregator) Aggregate(taskID string) (*AggregatedResult, error) {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	results := ra.results[taskID]
	if len(results) == 0 {
		return nil, ErrNoResults
	}

	// Sort by priority if enabled
	if ra.config.PriorityEnabled {
		sorted := make([]*TaskResult, len(results))
		copy(sorted, results)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Priority > sorted[j].Priority
		})
		results = sorted
	}

	// Detect conflicts if enabled
	conflicts := make([]*Conflict, 0)
	if ra.config.ConflictEnabled {
		if conflict := ra.detectConflict(taskID, results); conflict != nil {
			conflicts = append(conflicts, conflict)
		}
	}

	// Determine primary result
	primary := results[0]

	// Compute aggregated status
	status := ra.computeAggregatedStatus(results)

	return &AggregatedResult{
		TaskID:       taskID,
		Status:       status,
		Primary:      primary,
		Alternatives: results[1:],
		Conflicts:    conflicts,
		Summary:      ra.generateSummary(primary, conflicts),
		Metadata: map[string]interface{}{
			"total_results":  len(results),
			"conflict_count": len(conflicts),
			"avg_duration":   ra.computeAvgDuration(results),
		},
	}, nil
}

// WaitAll waits for all results for a task
func (ra *ResultAggregator) WaitAll(taskID string) ([]*TaskResult, error) {
	ra.mu.Lock()
	deadline := time.Now().Add(ra.config.Timeout)
	if existingDeadline, ok := ra.timeouts[taskID]; ok {
		deadline = existingDeadline
	}
	ra.mu.Unlock()

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		ra.mu.RLock()
		results := ra.results[taskID]
		ra.mu.RUnlock()

		// Check if we have minimum results
		if len(results) >= ra.config.MinResults {
			return results, nil
		}

		// Check if deadline passed
		if time.Now().After(deadline) {
			return results, nil
		}

		// Wait a bit before checking again
		select {
		case <-ctx.Done():
			ra.mu.RLock()
			defer ra.mu.RUnlock()
			return ra.results[taskID], ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// GetBest returns the best result for a task based on priority
func (ra *ResultAggregator) GetBest(taskID string) (*TaskResult, error) {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	results := ra.results[taskID]
	if len(results) == 0 {
		return nil, ErrNoResults
	}

	// Find highest priority result
	best := results[0]
	for _, r := range results[1:] {
		if r.Priority > best.Priority {
			best = r
		}
	}

	return best, nil
}

// Cancel cancels result collection for a task
func (ra *ResultAggregator) Cancel(taskID string) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	delete(ra.results, taskID)
	delete(ra.conflicts, taskID)
	delete(ra.timeouts, taskID)
	delete(ra.timeoutCbs, taskID)
}

// OnTimeout registers a callback for when a task times out
func (ra *ResultAggregator) OnTimeout(taskID string, callback func()) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.timeoutCbs[taskID] = append(ra.timeoutCbs[taskID], callback)

	// Set timeout
	ra.timeouts[taskID] = time.Now().Add(ra.config.Timeout)

	// Start timeout monitor
	go ra.monitorTimeout(taskID)
}

// GetResults returns all results for a task
func (ra *ResultAggregator) GetResults(taskID string) []*TaskResult {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	return ra.results[taskID]
}

// GetResultCount returns the number of results for a task
func (ra *ResultAggregator) GetResultCount(taskID string) int {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	return len(ra.results[taskID])
}

// Clear clears all results
func (ra *ResultAggregator) Clear() {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.results = make(map[string][]*TaskResult)
	ra.conflicts = make(map[string]*Conflict)
}

// Close closes the aggregator
func (ra *ResultAggregator) Close() {
	close(ra.done)
}

// detectConflict detects conflicts between results
func (ra *ResultAggregator) detectConflict(taskID string, results []*TaskResult) *Conflict {
	if len(results) < 2 {
		return nil
	}

	// Check if results are consistent by hashing their data
	hashes := make(map[string]int)
	for _, r := range results {
		h := ra.hashResult(r)
		hashes[h]++
	}

	// Multiple different hashes indicate conflict
	if len(hashes) > 1 {
		return &Conflict{
			TaskID:     taskID,
			Results:    results,
			Resolution: ConflictResolutionMajority,
		}
	}

	return nil
}

// hashResult creates a hash of a result for comparison
func (ra *ResultAggregator) hashResult(r *TaskResult) string {
	dataStr := fmt.Sprintf("%v", r.Data)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%s", r.AgentID, dataStr, r.Status, r.Timestamp.Format(time.RFC3339))))
	return hex.EncodeToString(hash[:8])
}

// computeAggregatedStatus computes the overall status from results
func (ra *ResultAggregator) computeAggregatedStatus(results []*TaskResult) ResultStatus {
	if len(results) == 0 {
		return ResultStatusFailed
	}

	successCount := 0
	failedCount := 0
	timeoutCount := 0

	for _, r := range results {
		switch r.Status {
		case ResultStatusSuccess:
			successCount++
		case ResultStatusFailed:
			failedCount++
		case ResultStatusTimeout:
			timeoutCount++
		}
	}

	total := len(results)

	// All successful
	if successCount == total {
		return ResultStatusSuccess
	}

	// All failed
	if failedCount == total {
		return ResultStatusFailed
	}

	// All timeout
	if timeoutCount == total {
		return ResultStatusTimeout
	}

	// Mixed results
	return ResultStatusPartial
}

// computeAvgDuration computes the average duration of results
func (ra *ResultAggregator) computeAvgDuration(results []*TaskResult) time.Duration {
	if len(results) == 0 {
		return 0
	}

	var total time.Duration
	for _, r := range results {
		total += r.Duration
	}

	return total / time.Duration(len(results))
}

// generateSummary generates a human-readable summary
func (ra *ResultAggregator) generateSummary(primary *TaskResult, conflicts []*Conflict) string {
	summary := fmt.Sprintf("Primary result from %s with status %s", primary.AgentID, primary.Status)
	if len(conflicts) > 0 {
		summary += fmt.Sprintf(" (detected %d conflicts)", len(conflicts))
	}
	return summary
}

// monitorTimeout monitors for task timeout
func (ra *ResultAggregator) monitorTimeout(taskID string) {
	ra.mu.Lock()
	deadline, ok := ra.timeouts[taskID]
	if !ok {
		ra.mu.Unlock()
		return
	}
	ra.mu.Unlock()

	select {
	case <-time.After(time.Until(deadline)):
		ra.mu.Lock()
		callbacks := ra.timeoutCbs[taskID]
		delete(ra.timeoutCbs, taskID)
		ra.mu.Unlock()

		// Execute callbacks
		for _, cb := range callbacks {
			cb()
		}
	case <-ra.done:
		return
	}
}

// MergeResults merges multiple results based on the configured strategy
func (ra *ResultAggregator) MergeResults(taskID string) (interface{}, error) {
	ra.mu.RLock()
	results := ra.results[taskID]
	ra.mu.RUnlock()

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	switch ra.config.MergeStrategy {
	case MergeStrategyFirst:
		return results[0].Data, nil
	case MergeStrategyLast:
		return results[len(results)-1].Data, nil
	case MergeStrategyPriority:
		best, _ := ra.GetBest(taskID)
		if best != nil {
			return best.Data, nil
		}
		return nil, ErrNoResults
	case MergeStrategyNewest:
		var newest *TaskResult
		for _, r := range results {
			if newest == nil || r.Timestamp.After(newest.Timestamp) {
				newest = r
			}
		}
		if newest != nil {
			return newest.Data, nil
		}
		return nil, ErrNoResults
	case MergeStrategyAll:
		// Return all data as slice
		data := make([]interface{}, len(results))
		for i, r := range results {
			data[i] = r.Data
		}
		return data, nil
	default:
		return results[0].Data, nil
	}
}

// ToJSON converts the aggregated result to JSON
func (ar *AggregatedResult) ToJSON() ([]byte, error) {
	return json.Marshal(ar)
}

// TaskResultToJSON converts a task result to JSON
func TaskResultToJSON(r *TaskResult) ([]byte, error) {
	return json.Marshal(r)
}

// TaskResultFromJSON creates a TaskResult from JSON
func TaskResultFromJSON(data []byte) (*TaskResult, error) {
	var result TaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
