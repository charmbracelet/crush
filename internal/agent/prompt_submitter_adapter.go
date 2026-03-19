package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// PromptSubmitterAdapter adapts coordinator prompt submission to plugin.PromptSubmitter.
type PromptSubmitterAdapter struct {
	coordinator Coordinator
	sessions    session.Service
	permissions permission.Service

	mu        sync.RWMutex
	sessionID string
}

// NewPromptSubmitterAdapter creates a new adapter for prompt submission.
func NewPromptSubmitterAdapter(sessions session.Service, permissions permission.Service) *PromptSubmitterAdapter {
	return &PromptSubmitterAdapter{sessions: sessions, permissions: permissions}
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

// SubmitPrompt sends a prompt to the agent in a new session.
func (a *PromptSubmitterAdapter) SubmitPrompt(ctx context.Context, prompt string) error {
	a.mu.RLock()
	coordinator := a.coordinator
	a.mu.RUnlock()

	if coordinator == nil {
		return errors.New("coordinator not initialized")
	}

	if a.sessions == nil {
		return errors.New("no session service available")
	}
	sess, err := a.sessions.Create(ctx, "ACP Session")
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if a.permissions != nil {
		a.permissions.AutoApproveSession(sess.ID)
	}

	_, err = coordinator.Run(ctx, sess.ID, prompt)
	return err
}

// SubmitPromptToSession sends a prompt to a specific existing session.
// If the session exists, the prompt is appended preserving conversation history.
// If it doesn't exist, a new session is created with that title.
func (a *PromptSubmitterAdapter) SubmitPromptToSession(ctx context.Context, sessionID, prompt string) error {
	a.mu.RLock()
	coordinator := a.coordinator
	a.mu.RUnlock()

	if coordinator == nil {
		return errors.New("coordinator not initialized")
	}

	if a.sessions == nil {
		return errors.New("no session service available")
	}

	// Check if session exists; if not, create one.
	_, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		_, err = a.sessions.Create(ctx, "ACP Session")
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	if a.permissions != nil {
		a.permissions.AutoApproveSession(sessionID)
	}

	_, err = coordinator.Run(ctx, sessionID, prompt)
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
