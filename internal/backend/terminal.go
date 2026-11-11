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

// AttachTerminalHost records that an event stream of clientID comes from a
// TUI that can open the agent's interactive terminals. Only such clients
// are routed terminal calls.
func (b *Backend) AttachTerminalHost(workspaceID, clientID string) {
	b.addTerminalStreams(workspaceID, clientID, 1)
}

// DetachTerminalHost undoes [Backend.AttachTerminalHost] when the stream
// ends.
func (b *Backend) DetachTerminalHost(workspaceID, clientID string) {
	b.addTerminalStreams(workspaceID, clientID, -1)
}

func (b *Backend) addTerminalStreams(workspaceID, clientID string, delta int) {
	ws, ok := b.workspaces.Get(workspaceID)
	if !ok {
		return
	}
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	if cs, ok := ws.clients[clientID]; ok {
		cs.terminalStreams = max(cs.terminalStreams+delta, 0)
	}
}

// terminalClient picks the client whose machine runs the agent's terminal
// calls for a chat session: the one that served the session last while it
// stays connected, since the terminals it started only exist there, or
// else a client viewing the session. Only TUIs qualify: an API client that
// follows the session, such as an editor plugin, would never answer. Ties
// go to the lowest client ID so the pick is stable.
func (ws *Workspace) terminalClient(sessionID, preferred string) (string, error) {
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	if cs, ok := ws.clients[preferred]; ok && cs.terminalStreams > 0 {
		return preferred, nil
	}
	var picked string
	for id, cs := range ws.clients {
		if cs.terminalStreams > 0 && cs.currentSessionID == sessionID && (picked == "" || id < picked) {
			picked = id
		}
	}
	if picked == "" {
		return "", terminal.ErrUnavailable
	}
	return picked, nil
}
