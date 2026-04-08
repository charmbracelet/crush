package agent

import (
	"sync"
	"time"

	"charm.land/fantasy"
)

type ToolExecution struct {
	ToolCallID    string
	ToolName      string
	StartTime     time.Time
	LastUpdate    time.Time
	Input         string
	Output        string
	Status        ExecutionStatus
	RetryCount    int
	Progress      float64
}

type ExecutionStatus int

const (
	StatusPending ExecutionStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusTimedOut
)

type streamingMonitor struct {
	mu         sync.RWMutex
	executions map[string]*ToolExecution
	timeouts   map[string]time.Duration
	maxRetries int
}

func newStreamingMonitor() *streamingMonitor {
	return &streamingMonitor{
		executions: make(map[string]*ToolExecution),
		timeouts:   make(map[string]time.Duration),
		maxRetries: 3,
	}
}

func (sm *streamingMonitor) StartTool(toolCallID, toolName, input string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.executions[toolCallID] = &ToolExecution{
		ToolCallID: toolCallID,
		ToolName:   toolName,
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
		Input:      input,
		Status:     StatusRunning,
		Progress:   0.0,
	}
}

func (sm *streamingMonitor) UpdateProgress(toolCallID string, progress float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if exec, ok := sm.executions[toolCallID]; ok {
		exec.Progress = progress
		exec.LastUpdate = time.Now()
	}
}

func (sm *streamingMonitor) CompleteTool(toolCallID, output string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if exec, ok := sm.executions[toolCallID]; ok {
		exec.Status = StatusCompleted
		exec.Output = output
		exec.LastUpdate = time.Now()
		exec.Progress = 1.0
	}
}

func (sm *streamingMonitor) FailTool(toolCallID string, err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if exec, ok := sm.executions[toolCallID]; ok {
		exec.Status = StatusFailed
		exec.LastUpdate = time.Now()
	}
}

func (sm *streamingMonitor) TimeoutTool(toolCallID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if exec, ok := sm.executions[toolCallID]; ok {
		exec.Status = StatusTimedOut
		exec.LastUpdate = time.Now()
	}
}

func (sm *streamingMonitor) SetTimeout(toolName string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.timeouts[toolName] = duration
}

func (sm *streamingMonitor) CheckTimeouts() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var timedOut []string
	now := time.Now()

	for id, exec := range sm.executions {
		if exec.Status != StatusRunning {
			continue
		}

		if timeout, ok := sm.timeouts[exec.ToolName]; ok {
			if now.Sub(exec.LastUpdate) > timeout {
				exec.Status = StatusTimedOut
				timedOut = append(timedOut, id)
			}
		} else {
			if now.Sub(exec.StartTime) > 5*time.Minute {
				exec.Status = StatusTimedOut
				timedOut = append(timedOut, id)
			}
		}
	}
	return timedOut
}

func (sm *streamingMonitor) GetActiveExecutions() []*ToolExecution {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var active []*ToolExecution
	for _, exec := range sm.executions {
		if exec.Status == StatusRunning {
			active = append(active, exec)
		}
	}
	return active
}

func (sm *streamingMonitor) GetExecutionStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := map[string]interface{}{
		"total":     len(sm.executions),
		"active":    0,
		"completed": 0,
		"failed":    0,
		"timed_out": 0,
	}

	for _, exec := range sm.executions {
		switch exec.Status {
		case StatusRunning:
			stats["active"] = stats["active"].(int) + 1
		case StatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case StatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case StatusTimedOut:
			stats["timed_out"] = stats["timed_out"].(int) + 1
		}
	}
	return stats
}

func (sm *streamingMonitor) OnToolCall(tc fantasy.ToolCallContent) error {
	sm.StartTool(tc.ToolCallID, tc.ToolName, tc.Input)
	return nil
}

func (sm *streamingMonitor) OnToolResult(result fantasy.ToolResultContent) error {
	isError := false
	content := ""

	switch result.Result.GetType() {
	case fantasy.ToolResultContentTypeError:
		isError = true
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); ok {
			content = r.Error.Error()
		}
	case fantasy.ToolResultContentTypeText:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); ok {
			content = r.Text
		}
	case fantasy.ToolResultContentTypeMedia:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result.Result); ok {
			content = r.Text
		}
	}

	if isError {
		sm.FailTool(result.ToolCallID, nil)
	} else {
		sm.CompleteTool(result.ToolCallID, content)
	}
	return nil
}

func (sm *streamingMonitor) OnRetry(err *fantasy.ProviderError, delay time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, exec := range sm.executions {
		if exec.Status == StatusRunning {
			exec.RetryCount++
			if exec.RetryCount >= sm.maxRetries {
				exec.Status = StatusFailed
			}
		}
	}
}
