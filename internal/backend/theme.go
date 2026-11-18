package backend

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// hostThemeState returns the workspace's current host theme.
func (w *Workspace) hostThemeState() proto.HostThemeState {
	w.themeMu.Lock()
	defer w.themeMu.Unlock()
	return proto.HostThemeState{WorkspaceID: w.ID, Theme: w.hostTheme}
}

// GetHostTheme returns the host theme of the given workspace. A nil Theme
// means none is set.
func (b *Backend) GetHostTheme(workspaceID string) (proto.HostThemeState, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.HostThemeState{}, err
	}
	return ws.hostThemeState(), nil
}

// SetHostTheme sets the theme a host application supplies for every client
// of the workspace, or clears it when t is nil, and publishes a host_theme
// event (updated or deleted) to all of them. Setting a theme equal to the
// current one is a no-op that publishes nothing. Invalid colors and an
// unknown base theme fail with [ErrInvalidTheme]; unknown keys are ignored.
// It returns the state after the call.
func (b *Backend) SetHostTheme(workspaceID string, t *proto.HostTheme) (proto.HostThemeState, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.HostThemeState{}, err
	}
	if t != nil {
		// Validation may read user theme files for the base, so it runs
		// before the lock is taken.
		ignored, err := styles.ValidateHostTheme(t.Base, t.Palette, t.Syntax)
		if err != nil {
			return proto.HostThemeState{}, fmt.Errorf("%w: %w", ErrInvalidTheme, err)
		}
		if len(ignored) > 0 {
			slog.Debug("Ignoring unknown host theme keys", "workspace", workspaceID, "keys", ignored)
		}
		t = t.Clone()
	}

	ws.themeMu.Lock()
	defer ws.themeMu.Unlock()
	st := proto.HostThemeState{WorkspaceID: ws.ID, Theme: t}
	if ws.hostTheme.Equal(t) {
		st.Theme = ws.hostTheme
		return st, nil
	}
	ws.hostTheme = t
	// Publish under the lock so concurrent sets reach subscribers in the
	// order they were stored. Publishing never blocks.
	if ws.App != nil { // Synthetic test workspaces have no App.
		typ := pubsub.UpdatedEvent
		if t == nil {
			typ = pubsub.DeletedEvent
		}
		ws.SendEvent(pubsub.Event[proto.HostThemeState]{Type: typ, Payload: st})
	}
	return st, nil
}
