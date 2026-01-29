package agent

import (
	"context"
	"errors"
	"sync"
)

// PromptSubmitterAdapter adapts coordinator prompt submission to plugin.PromptSubmitter.
type PromptSubmitterAdapter struct {
	coordinator Coordinator

	mu        sync.RWMutex
	sessionID string
}

// NewPromptSubmitterAdapter creates a new adapter for prompt submission.
func NewPromptSubmitterAdapter() *PromptSubmitterAdapter {
	return &PromptSubmitterAdapter{}
}

// SetCoordinator sets the coordinator reference.
// This is called after the coordinator is fully initialized.
func (a *PromptSubmitterAdapter) SetCoordinator(c Coordinator) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.coordinator = c
}

// SetSessionID sets the current session ID.
func (a *PromptSubmitterAdapter) SetSessionID(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionID = sessionID
}

// SubmitPrompt sends a prompt to the agent.
func (a *PromptSubmitterAdapter) SubmitPrompt(ctx context.Context, prompt string) error {
	a.mu.RLock()
	coordinator := a.coordinator
	sessionID := a.sessionID
	a.mu.RUnlock()

	if coordinator == nil {
		return errors.New("coordinator not initialized")
	}
	if sessionID == "" {
		return errors.New("no active session")
	}

	// Run the prompt through the coordinator.
	// The coordinator handles queuing if the agent is busy.
	_, err := coordinator.Run(ctx, sessionID, prompt)
	return err
}

// CurrentSessionID returns the ID of the currently active session.
func (a *PromptSubmitterAdapter) CurrentSessionID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessionID
}

// IsSessionBusy returns true if the current session is busy processing.
func (a *PromptSubmitterAdapter) IsSessionBusy() bool {
	a.mu.RLock()
	coordinator := a.coordinator
	sessionID := a.sessionID
	a.mu.RUnlock()

	if coordinator == nil || sessionID == "" {
		return false
	}
	return coordinator.IsSessionBusy(sessionID)
}
