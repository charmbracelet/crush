package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestBashTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewBashTool(&allowAllPerms{}, dir)
	require.NotEmpty(t, tool.Name())
	require.NotEmpty(t, tool.Info().Description)
	require.NotEmpty(t, tool.Info().Name)
	require.NotEmpty(t, tool.Info().Parameters)
	require.NotEmpty(t, tool.Info().Required)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, "1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "1")

	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "doc.txt"), []byte(`hello world`), 0o644))
	resp, err := tool.Run(ctx, ToolCall{
		ID:    "1",
		Name:  "some name",
		Input: fmt.Sprintf(`{"command":%q,"working_dir":%q}`, "cat doc.txt", dir2),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "hello world", resp.Content)
}

type allowAllPerms struct{}

func (a *allowAllPerms) AutoApproveSession(sessionID string)                     {}
func (a *allowAllPerms) Deny(permission permission.PermissionRequest)            {}
func (a *allowAllPerms) Grant(permission permission.PermissionRequest)           {}
func (a *allowAllPerms) GrantPersistent(permission permission.PermissionRequest) {}
func (a *allowAllPerms) SetSkipRequests(skip bool)                               {}

func (a *allowAllPerms) Request(opts permission.CreatePermissionRequest) bool { return true }
func (a *allowAllPerms) SkipRequests() bool                                   { return true }

func (a *allowAllPerms) Subscribe(context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return nil
}

func (a *allowAllPerms) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return nil
}
