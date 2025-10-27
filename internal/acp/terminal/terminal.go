package terminal

import (
	"context"
	"github.com/coder/acp-go-sdk"
	"github.com/google/uuid"
	"log/slog"
)

type ID string

// Terminal wraps one ACP terminal plus local metadata.
type Terminal struct {
	ID        ID
	SessionID acp.SessionId
	Cmd       string
	Args      []string
	Env       []acp.EnvVariable
	Cwd       string
	ByteLim   *int
}

// New creates a Terminal value, but does NOT start it.
func New(cmd string, args []string, env []acp.EnvVariable, cwd string, byteLim *int) *Terminal {
	return &Terminal{
		Cmd:     cmd,
		Args:    args,
		Env:     env,
		Cwd:     cwd,
		ByteLim: byteLim,
	}
}

// Start starts a command in a new terminal
func (t *Terminal) Start(ctx context.Context, conn *acp.AgentSideConnection, sessionID acp.SessionId) error {
	req := acp.CreateTerminalRequest{
		SessionId:       sessionID,
		Command:         t.Cmd,
		Args:            t.Args,
		Env:             t.Env,
		OutputByteLimit: t.ByteLim,
	}
	if t.Cwd != "" {
		req.Cwd = &t.Cwd
	}

	resp, err := conn.CreateTerminal(ctx, req)
	if err != nil {
		return err
	}
	t.ID = ID(resp.TerminalId)
	t.SessionID = sessionID
	return nil
}

// Output retrieves the current terminal output without waiting for the command to complete
func (t *Terminal) Output(ctx context.Context, conn *acp.AgentSideConnection) (acp.TerminalOutputResponse, error) {
	return conn.TerminalOutput(ctx, acp.TerminalOutputRequest{
		SessionId:  t.SessionID,
		TerminalId: string(t.ID),
	})
}

// WaitForExit returns once the command completes
func (t *Terminal) WaitForExit(ctx context.Context, conn *acp.AgentSideConnection) (acp.WaitForTerminalExitResponse, error) {
	return conn.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{
		SessionId:  t.SessionID,
		TerminalId: string(t.ID),
	})
}

// Kill terminates a command without releasing the terminal
func (t *Terminal) Kill(ctx context.Context, conn *acp.AgentSideConnection) (acp.KillTerminalCommandResponse, error) {
	return conn.KillTerminalCommand(ctx, acp.KillTerminalCommandRequest{
		SessionId:  t.SessionID,
		TerminalId: string(t.ID),
	})
}

// Release kills the command if still running and releases all resources
func (t *Terminal) Release(ctx context.Context, conn *acp.AgentSideConnection) error {
	_, err := conn.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{
		SessionId:  t.SessionID,
		TerminalId: string(t.ID),
	})
	if err != nil {
		slog.Error("could not release terminal", "err", err)
	}

	return err
}

// EmbedInToolCalls produces the SessionUpdate that advertises the terminal via tool calls
func (t *Terminal) EmbedInToolCalls(ctx context.Context, conn *acp.AgentSideConnection) error {
	return conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: t.SessionID,
		Update: acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: acp.ToolCallId("terminal_call_" + uuid.New().String()),
				Kind:       acp.ToolKindExecute,
				Status:     acp.ToolCallStatusInProgress,
				Content: []acp.ToolCallContent{{
					Terminal: &acp.ToolCallContentTerminal{TerminalId: string(t.ID)},
				}},
			},
		},
	})
}
