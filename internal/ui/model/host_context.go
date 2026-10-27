package model

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/workspace"
)

// hostOpenFiles returns a function that lists the files the host application
// has open, for the file completions to list first. It asks the workspace
// each time it is called, and returns none when that fails or is slow.
func hostOpenFiles(ws workspace.Workspace) func() []string {
	return func() []string {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hc, err := ws.HostContext(ctx)
		if err != nil {
			return nil
		}
		return hc.OpenFiles
	}
}
