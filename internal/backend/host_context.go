package backend

import (
	"github.com/charmbracelet/crush/internal/proto"
)

// SetHostContext records what the host application has in view for the
// workspace's agent. The zero context clears it.
func (b *Backend) SetHostContext(workspaceID string, hc proto.HostContext) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	coord := ws.Coordinator()
	if coord == nil {
		return ErrAgentNotInitialized
	}
	coord.SetHostContext(hc)
	return nil
}

// GetHostContext returns what the host application has in view for the
// workspace, the zero context when none is set.
func (b *Backend) GetHostContext(workspaceID string) (proto.HostContext, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.HostContext{}, err
	}
	coord := ws.Coordinator()
	if coord == nil {
		return proto.HostContext{}, nil
	}
	return coord.HostContext(), nil
}
