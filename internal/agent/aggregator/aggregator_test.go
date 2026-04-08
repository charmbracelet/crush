package aggregator

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestResultAggregator_Submit(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:         5 * time.Second,
		MaxResults:      10,
		MinResults:      1,
		PriorityEnabled: true,
	})

	result := &TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Data:     map[string]string{"key": "value"},
		Priority: 10,
		Duration: time.Second,
	}

	err := ra.Submit(result)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	count := ra.GetResultCount("task-123")
	if count != 1 {
		t.Fatalf("Expected 1 result, got %d", count)
	}
}

func TestResultAggregator_SubmitNil(t *testing.T) {
	ra := New(DefaultConfig())

	err := ra.Submit(nil)
	if err == nil {
		t.Fatal("Expected error for nil result")
	}
}

func TestResultAggregator_SubmitEmptyTaskID(t *testing.T) {
	ra := New(DefaultConfig())

	result := &TaskResult{
		AgentID: "agent-1",
		TaskID:  "",
	}

	err := ra.Submit(result)
	if err == nil {
		t.Fatal("Expected error for empty task ID")
	}
}

func TestResultAggregator_SubmitMaxResults(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    5 * time.Second,
		MaxResults: 2,
		MinResults: 1,
	})

	// Submit first result
	err := ra.Submit(&TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 10,
	})
	if err != nil {
		t.Fatalf("First submit failed: %v", err)
	}

	// Submit second result
	err = ra.Submit(&TaskResult{
		AgentID:  "agent-2",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 20,
	})
	if err != nil {
		t.Fatalf("Second submit failed: %v", err)
	}

	// Submit third result - should fail
	err = ra.Submit(&TaskResult{
		AgentID:  "agent-3",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 30,
	})
	if err != ErrMaxResultsExceeded {
		t.Fatalf("Expected ErrMaxResultsExceeded, got %v", err)
	}
}

func TestResultAggregator_Aggregate(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:         5 * time.Second,
		MaxResults:      10,
		MinResults:      1,
		PriorityEnabled: true,
	})

	// Submit multiple results
	ra.Submit(&TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Data:     "result-1",
		Priority: 10,
	})

	ra.Submit(&TaskResult{
		AgentID:  "agent-2",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Data:     "result-2",
		Priority: 20,
	})

	aggregated, err := ra.Aggregate("task-123")
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}

	if aggregated.TaskID != "task-123" {
		t.Fatalf("Expected task ID 'task-123', got '%s'", aggregated.TaskID)
	}

	if aggregated.Status != ResultStatusSuccess {
		t.Fatalf("Expected status Success, got %s", aggregated.Status)
	}

	// With priority enabled, agent-2 should be primary
	if aggregated.Primary.AgentID != "agent-2" {
		t.Fatalf("Expected primary agent 'agent-2', got '%s'", aggregated.Primary.AgentID)
	}
}

func TestResultAggregator_AggregateNoResults(t *testing.T) {
	ra := New(DefaultConfig())

	_, err := ra.Aggregate("nonexistent")
	if err != ErrNoResults {
		t.Fatalf("Expected ErrNoResults, got %v", err)
	}
}

func TestResultAggregator_AggregateMixedStatus(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    5 * time.Second,
		MaxResults: 10,
		MinResults: 1,
	})

	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-123",
		Status:  ResultStatusSuccess,
	})

	ra.Submit(&TaskResult{
		AgentID: "agent-2",
		TaskID:  "task-123",
		Status:  ResultStatusFailed,
	})

	aggregated, _ := ra.Aggregate("task-123")
	if aggregated.Status != ResultStatusPartial {
		t.Fatalf("Expected status Partial, got %s", aggregated.Status)
	}
}

func TestResultAggregator_GetBest(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    5 * time.Second,
		MaxResults: 10,
		MinResults: 1,
	})

	ra.Submit(&TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 10,
	})

	ra.Submit(&TaskResult{
		AgentID:  "agent-2",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 20,
	})

	ra.Submit(&TaskResult{
		AgentID:  "agent-3",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Priority: 5,
	})

	best, err := ra.GetBest("task-123")
	if err != nil {
		t.Fatalf("GetBest failed: %v", err)
	}

	if best.AgentID != "agent-2" {
		t.Fatalf("Expected best agent 'agent-2', got '%s'", best.AgentID)
	}

	if best.Priority != 20 {
		t.Fatalf("Expected priority 20, got %d", best.Priority)
	}
}

func TestResultAggregator_GetBestNoResults(t *testing.T) {
	ra := New(DefaultConfig())

	_, err := ra.GetBest("nonexistent")
	if err != ErrNoResults {
		t.Fatalf("Expected ErrNoResults, got %v", err)
	}
}

func TestResultAggregator_WaitAll(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    100 * time.Millisecond,
		MaxResults: 10,
		MinResults: 2,
	})

	// Submit one result immediately
	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-123",
		Status:  ResultStatusSuccess,
	})

	// Submit second result after a delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		ra.Submit(&TaskResult{
			AgentID: "agent-2",
			TaskID:  "task-123",
			Status:  ResultStatusSuccess,
		})
	}()

	results, err := ra.WaitAll("task-123")
	if err != nil {
		t.Fatalf("WaitAll failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
}

func TestResultAggregator_WaitAllTimeout(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    50 * time.Millisecond,
		MaxResults: 10,
		MinResults: 5, // Require 5, but only submit 1
	})

	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-123",
		Status:  ResultStatusSuccess,
	})

	results, _ := ra.WaitAll("task-123")

	// Should return what we have after timeout
	if len(results) != 1 {
		t.Fatalf("Expected 1 result after timeout, got %d", len(results))
	}
}

func TestResultAggregator_Cancel(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    5 * time.Second,
		MaxResults: 10,
		MinResults: 1,
	})

	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-123",
		Status:  ResultStatusSuccess,
	})

	ra.Cancel("task-123")

	count := ra.GetResultCount("task-123")
	if count != 0 {
		t.Fatalf("Expected 0 results after cancel, got %d", count)
	}
}

func TestResultAggregator_OnTimeout(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    20 * time.Millisecond,
		MaxResults: 10,
		MinResults: 5,
	})

	callbackCalled := false
	ra.OnTimeout("task-123", func() {
		callbackCalled = true
	})

	time.Sleep(50 * time.Millisecond)

	if !callbackCalled {
		t.Fatal("Expected timeout callback to be called")
	}
}

func TestResultAggregator_Concurrent(t *testing.T) {
	ra := New(AggregatorConfig{
		Timeout:    5 * time.Second,
		MaxResults: 100,
		MinResults: 1,
	})

	var wg sync.WaitGroup
	numAgents := 10
	numTasks := 5

	for a := 0; a < numAgents; a++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()
			for taskIdx := 0; taskIdx < numTasks; taskIdx++ {
				taskIDStr := fmt.Sprintf("task-%d", taskIdx)
				ra.Submit(&TaskResult{
					AgentID:  "agent",
					TaskID:   taskIDStr,
					Status:   ResultStatusSuccess,
					Priority: agentID,
				})
			}
		}(a)
	}

	wg.Wait()

	for taskIdx := 0; taskIdx < numTasks; taskIdx++ {
		taskIDStr := fmt.Sprintf("task-%d", taskIdx)
		count := ra.GetResultCount(taskIDStr)
		if count != numAgents {
			t.Fatalf("Task %d: expected %d results, got %d", taskIdx, numAgents, count)
		}
	}
}

func TestResultAggregator_Clear(t *testing.T) {
	ra := New(DefaultConfig())

	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-123",
		Status:  ResultStatusSuccess,
	})

	ra.Submit(&TaskResult{
		AgentID: "agent-1",
		TaskID:  "task-456",
		Status:  ResultStatusSuccess,
	})

	ra.Clear()

	if ra.GetResultCount("task-123") != 0 {
		t.Fatal("Expected 0 results for task-123 after clear")
	}

	if ra.GetResultCount("task-456") != 0 {
		t.Fatal("Expected 0 results for task-456 after clear")
	}
}

func TestResultAggregator_Close(t *testing.T) {
	ra := New(DefaultConfig())

	// Should not panic
	ra.Close()
}

func TestResultStatus_String(t *testing.T) {
	tests := []struct {
		status   ResultStatus
		expected string
	}{
		{ResultStatusPending, "pending"},
		{ResultStatusSuccess, "success"},
		{ResultStatusPartial, "partial"},
		{ResultStatusFailed, "failed"},
		{ResultStatusTimeout, "timeout"},
		{ResultStatus(100), "unknown"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.status.String())
		}
	}
}

func TestAggregatedResult_ToJSON(t *testing.T) {
	ar := &AggregatedResult{
		TaskID: "task-123",
		Status: ResultStatusSuccess,
		Primary: &TaskResult{
			AgentID: "agent-1",
			TaskID:  "task-123",
			Status:  ResultStatusSuccess,
			Data:    "test",
		},
		Metadata: map[string]interface{}{
			"total_results": 1,
		},
	}

	data, err := ar.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON data")
	}
}

func TestTaskResultToJSON(t *testing.T) {
	result := &TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Data:     "test",
		Priority: 10,
	}

	data, err := TaskResultToJSON(result)
	if err != nil {
		t.Fatalf("TaskResultToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON data")
	}
}

func TestTaskResultFromJSON(t *testing.T) {
	result := &TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusSuccess,
		Data:     "test",
		Priority: 10,
	}

	data, _ := TaskResultToJSON(result)

	parsed, err := TaskResultFromJSON(data)
	if err != nil {
		t.Fatalf("TaskResultFromJSON failed: %v", err)
	}

	if parsed.AgentID != result.AgentID {
		t.Errorf("Expected AgentID %s, got %s", result.AgentID, parsed.AgentID)
	}

	if parsed.TaskID != result.TaskID {
		t.Errorf("Expected TaskID %s, got %s", result.TaskID, parsed.TaskID)
	}
}

func TestTaskResultFromJSONInvalid(t *testing.T) {
	_, err := TaskResultFromJSON([]byte("invalid json"))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestResultAggregator_MergeResults(t *testing.T) {
	tests := []struct {
		name     string
		config   AggregatorConfig
		submitFn func(*ResultAggregator)
		taskID   string
	}{
		{
			name: "MergeStrategyFirst",
			config: AggregatorConfig{
				MergeStrategy: MergeStrategyFirst,
			},
			submitFn: func(ra *ResultAggregator) {
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "a", Data: "first", Priority: 5})
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "b", Data: "second", Priority: 10})
			},
			taskID: "task",
		},
		{
			name: "MergeStrategyLast",
			config: AggregatorConfig{
				MergeStrategy: MergeStrategyLast,
			},
			submitFn: func(ra *ResultAggregator) {
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "a", Data: "first", Priority: 5})
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "b", Data: "second", Priority: 10})
			},
			taskID: "task",
		},
		{
			name: "MergeStrategyPriority",
			config: AggregatorConfig{
				MergeStrategy:   MergeStrategyPriority,
				PriorityEnabled: true,
			},
			submitFn: func(ra *ResultAggregator) {
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "a", Data: "first", Priority: 5})
				ra.Submit(&TaskResult{TaskID: "task", AgentID: "b", Data: "second", Priority: 10})
			},
			taskID: "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ra := New(tt.config)
			tt.submitFn(ra)

			result, err := ra.MergeResults(tt.taskID)
			if err != nil {
				t.Fatalf("MergeResults failed: %v", err)
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}
		})
	}
}

func TestResultAggregator_MergeResultsNoResults(t *testing.T) {
	ra := New(DefaultConfig())

	_, err := ra.MergeResults("nonexistent")
	if err != ErrNoResults {
		t.Fatalf("Expected ErrNoResults, got %v", err)
	}
}

// Helper functions

func DefaultConfig() AggregatorConfig {
	return AggregatorConfig{
		Timeout:         30 * time.Second,
		MaxResults:      100,
		MinResults:      1,
		PriorityEnabled: false,
		ConflictEnabled: false,
		MergeStrategy:   MergeStrategyFirst,
	}
}

// Test for error handling in TaskResult with error field
func TestResultAggregator_SubmitWithError(t *testing.T) {
	ra := New(DefaultConfig())

	result := &TaskResult{
		AgentID:  "agent-1",
		TaskID:   "task-123",
		Status:   ResultStatusFailed,
		Err:      errors.New("test error"),
		Priority: 10,
	}

	err := ra.Submit(result)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	aggregated, _ := ra.Aggregate("task-123")
	if aggregated.Status != ResultStatusFailed {
		t.Fatalf("Expected status Failed, got %s", aggregated.Status)
	}
}
