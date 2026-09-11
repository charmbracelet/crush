package workspace

import (
	"context"
	"github.com/charmbracelet/crush/internal/filehistory"
)

// RewindWorkspace is supported by local and remote native workspaces.
type RewindWorkspace interface {
	Rewind(context.Context, string, filehistory.Request) (filehistory.Result, error)
}

func (w *AppWorkspace) Rewind(ctx context.Context, sessionID string, request filehistory.Request) (filehistory.Result, error) {
	return w.app.Rewind(ctx, sessionID, request)
}
func (w *ClientWorkspace) Rewind(ctx context.Context, sessionID string, request filehistory.Request) (filehistory.Result, error) {
	return w.client.Rewind(ctx, w.workspaceID(), sessionID, request)
}
