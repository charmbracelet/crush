package backend

import (
	"context"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
)

// RespondTerminal resolves a terminal call the agent forwarded to
// clientID with the result of running it on the client's machine. The
// returned bool reports whether such a call was waiting.
func (b *Backend) RespondTerminal(workspaceID, clientID string, req proto.TerminalResult) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}
	if ws.App == nil || ws.Terminal == nil {
		return false, nil
	}
	return ws.Terminal.Respond(req.CallID, clientID, terminal.Result{
		Content:  req.Content,
		Metadata: req.Metadata,
		IsError:  req.IsError,
	}), nil
}

// RecordShellCommand stores a shell command the user ran on the client's
// machine as a message in the session, the way commands run here are.
func (b *Backend) RecordShellCommand(ctx context.Context, workspaceID, sessionID string, req proto.ShellCommandRecord) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.App == nil || ws.Messages == nil {
		return nil
	}
	var dataDir string
	if ws.Cfg != nil {
		dataDir = ws.Cfg.Config().Options.DataDirectory
	}
	return shell.PersistOutput(ctx, ws.Messages, sessionID, req.Command, req.Output, req.ExitCode, dataDir)
}

// terminalClient picks the client whose machine runs the agent's terminal
// calls for a chat session: the one that served the session last while it
// stays connected, since the terminals it started only exist there, or
// else a client viewing the session. Ties go to the lowest client ID so
// the pick is stable.
func (ws *Workspace) terminalClient(sessionID, preferred string) (string, error) {
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	if cs, ok := ws.clients[preferred]; ok && cs.streams > 0 {
		return preferred, nil
	}
	var picked string
	for id, cs := range ws.clients {
		if cs.streams > 0 && cs.currentSessionID == sessionID && (picked == "" || id < picked) {
			picked = id
		}
	}
	if picked == "" {
		return "", terminal.ErrUnavailable
	}
	return picked, nil
}
