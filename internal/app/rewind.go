package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/charmbracelet/crush/internal/message"
	"path/filepath"
)

// Rewind coordinates the authoritative conversation database with workspace files.
func (a *App) Rewind(ctx context.Context, sessionID string, request filehistory.Request) (filehistory.Result, error) {
	if a.AgentCoordinator != nil && a.AgentCoordinator.IsBusy() {
		return filehistory.Result{}, fmt.Errorf("wait for active agents to finish before rewinding")
	}
	if _, err := a.Sessions.Get(ctx, sessionID); err != nil {
		return filehistory.Result{}, err
	}
	store := filehistory.New(a.config.WorkingDir(), filepath.Join(filepath.Dir(config.GlobalConfigData()), "file-history"), a.config.Config().Options.FilesnapBinary)
	return store.Navigate(ctx, sessionID, request, func() (json.RawMessage, error) {
		sess, err := a.Sessions.Get(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		return message.SnapshotConversation(ctx, a.Messages, sess)
	}, func(data json.RawMessage) error {
		return message.RestoreConversation(context.WithoutCancel(ctx), a.Messages, a.conn, sessionID, data)
	})
}
