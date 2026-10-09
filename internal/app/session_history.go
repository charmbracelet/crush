package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/crush/internal/history"
)

// ListSessionHistory returns the file history for sessionID, plus the file
// history of its direct child (subagent) sessions, so the "Modified Files"
// UI reflects edits made by a subagent after it finishes and control
// returns to the parent. It does not recurse into grandchildren. A custom
// subagent can call agentic_fetch, which creates a grandchild session, but
// agentic_fetch's tools are read-only, so grandchildren never record file
// history and one level is sufficient. Uses a single query rather
// than one round trip per child, since session switches pay this cost
// synchronously before the Modified Files panel populates.
func (app *App) ListSessionHistory(ctx context.Context, sessionID string) ([]history.File, error) {
	files, err := app.History.ListBySessionWithChildren(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing history for %s and its children: %w", sessionID, err)
	}
	return files, nil
}
