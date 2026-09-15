package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestSetPermissionMode_ReportedInWorkspaceProto pins the invariant
// that the mode a client is shown is the mode the server enforces.
// The workspace snapshot used to be built from the startup flag, so
// any runtime toggle left every client reading a stale value forever.
// The dangerous direction is a client displaying "normal" while the
// server auto-approves everything, so both directions are asserted.
func TestSetPermissionMode_ReportedInWorkspaceProto(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	// Start in yolo so that toggling down to normal is just as much a
	// real assertion as toggling up; a workspace started in normal
	// would make the down-toggle pass by accident.
	args := protoWS(t.TempDir(), t.TempDir(), uuid.New().String())
	args.PermissionMode = proto.WorkspacePermissionModeYolo
	ws, created, err := b.CreateWorkspace(args)
	require.NoError(t, err)
	require.Equal(t, proto.WorkspacePermissionModeYolo, created.PermissionMode)

	for _, tc := range []struct {
		name string
		mode permission.PermissionMode
		want proto.WorkspacePermissionMode
	}{
		{"yolo to normal", permission.PermissionModeNormal, proto.WorkspacePermissionModeNormal},
		{"normal to sysadmin", permission.PermissionModeSysadmin, proto.WorkspacePermissionModeSysadmin},
		{"sysadmin to yolo", permission.PermissionModeYolo, proto.WorkspacePermissionModeYolo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, b.SetPermissionMode(ws.ID, tc.mode))
			require.Equal(t, tc.mode, ws.Permissions.PermissionMode(),
				"the server must enforce the mode it was just given")

			got, err := b.GetWorkspaceProto(ws.ID)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.PermissionMode,
				"workspace snapshot must report the enforced mode")

			// ListWorkspaces is a second client-facing path onto the
			// same snapshot; a fix that misses it still lies to clients.
			list := b.ListWorkspaces()
			require.Len(t, list, 1)
			require.Equal(t, tc.want, list[0].PermissionMode,
				"workspace listing must report the enforced mode")
		})
	}
}
