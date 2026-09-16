package backend

import (
	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/charmbracelet/crush/internal/proto"
)

// AnswerPinentry resolves the pending integrated pinentry prompt with
// the user's secret. The returned bool reports whether this call
// resolved the pending prompt (true) or found it already resolved by a
// previous caller (false).
func (b *Backend) AnswerPinentry(workspaceID string, req proto.PinentryAnswer) (bool, error) {
	if _, err := b.GetWorkspace(workspaceID); err != nil {
		return false, err
	}
	return pinentry.DefaultPrompts().Respond(req.RequestID, req.Secret), nil
}

// CancelPinentry dismisses the pending integrated pinentry prompt.
// Returns true if a prompt was cancelled, false if none was pending.
func (b *Backend) CancelPinentry(workspaceID string, req proto.PinentryCancel) (bool, error) {
	if _, err := b.GetWorkspace(workspaceID); err != nil {
		return false, err
	}
	return pinentry.DefaultPrompts().Cancel(req.RequestID), nil
}
