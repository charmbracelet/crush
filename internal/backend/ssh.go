package backend

import (
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/sshaskpass"
)

// AnswerSSH resolves the pending integrated SSH prompt with the user's
// secret. The returned bool reports whether this call resolved the
// pending prompt (true) or found it already resolved by a previous
// caller (false).
func (b *Backend) AnswerSSH(workspaceID string, req proto.SSHAnswer) (bool, error) {
	if _, err := b.GetWorkspace(workspaceID); err != nil {
		return false, err
	}
	return sshaskpass.DefaultPrompts().Respond(req.RequestID, req.Secret), nil
}

// CancelSSH dismisses the pending integrated SSH prompt. Returns true
// if a prompt was cancelled, false if none was pending.
func (b *Backend) CancelSSH(workspaceID string, req proto.SSHCancel) (bool, error) {
	if _, err := b.GetWorkspace(workspaceID); err != nil {
		return false, err
	}
	return sshaskpass.DefaultPrompts().Cancel(req.RequestID), nil
}
