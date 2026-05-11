package backend

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/fork"
)

// ErrForksDisabled is returned when fork service is not available.
var ErrForksDisabled = errors.New("fork service not available")

// ForkConversation creates a fork of a conversation.
func (b *Backend) ForkConversation(ctx context.Context, workspaceID string, params fork.ForkParams) (*fork.ForkResult, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Forks == nil {
		return nil, ErrForksDisabled
	}
	return ws.Forks.Fork(ctx, params)
}
