//go:build windows

package workspace

import (
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
)

// TestClientWorkspace_RespawnsDeadPipeServer is the named-pipe twin of
// TestClientWorkspace_RespawnsDeadServer: a missing pipe must respawn too.
func TestClientWorkspace_RespawnsDeadPipeServer(t *testing.T) {
	addr := "//./pipe/crush-test-" + uuid.NewString() + ".sock"
	testRespawnsDeadServer(t, "npipe", addr, func() (net.Listener, error) {
		return winio.ListenPipe(addr, nil)
	})
}
