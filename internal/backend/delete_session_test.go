package backend

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// sessionStateCoordinator báo một session đang chạy và một session có prompt xếp hàng.
type sessionStateCoordinator struct {
	*blockingCoordinator
	busy, queued string
}

func (c *sessionStateCoordinator) IsSessionBusy(id string) bool { return id == c.busy }
func (c *sessionStateCoordinator) QueuedPrompts(id string) int {
	if id == c.queued {
		return 1
	}
	return 0
}

// Xoá session còn lượt đang chạy hay xếp hàng bị từ chối (409): run đó sẽ
// ghi tin nhắn vào một session đã mất.
func TestDeleteSession_RefusesSessionWithRun(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	coord := &sessionStateCoordinator{blockingCoordinator: newBlockingCoordinator(), busy: "running", queued: "waiting"}
	ws := insertAgentWorkspace(t, b, coord)

	for _, id := range []string{"running", "waiting"} {
		require.ErrorIs(t, b.DeleteSession(t.Context(), ws.ID, id), ErrSessionBusy, id)
	}
}
