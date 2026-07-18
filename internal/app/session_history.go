package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/crush/internal/history"
)

// ListSessionHistory returns the file history for sessionID, plus the file
// history of its direct child (subagent) sessions, so the "Modified Files"
// UI reflects edits made by a subagent after it finishes and control
// returns to the parent. It does not recurse into grandchildren (nested
// subagents) — there is currently no code path that lets a subagent spawn
// its own subagent, so one level is sufficient.
func (app *App) ListSessionHistory(ctx context.Context, sessionID string) ([]history.File, error) {
	files, err := app.History.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	children, err := app.Sessions.ListChildSessions(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing child sessions for %s: %w", sessionID, err)
	}
	for _, child := range children {
		childFiles, err := app.History.ListBySession(ctx, child.ID)
		if err != nil {
			return nil, fmt.Errorf("listing history for child session %s: %w", child.ID, err)
		}
		files = append(files, childFiles...)
	}
	return files, nil
}
