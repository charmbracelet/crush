//go:build windows

package workspace

import (
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
)

// TestClientWorkspace_RespawnsDeadPipeServer is the Windows twin of
// TestClientWorkspace_RespawnsDeadServer. A server ended from Task Manager
// takes its named pipe with it, so the client's dial fails with
// ERROR_FILE_NOT_FOUND; that must bring a server back too.
func TestClientWorkspace_RespawnsDeadPipeServer(t *testing.T) {
	addr := "//./pipe/crush-test-" + uuid.NewString() + ".sock"
	testRespawnsDeadServer(t, "npipe", addr, func() (net.Listener, error) {
		return winio.ListenPipe(addr, nil)
	})
}
