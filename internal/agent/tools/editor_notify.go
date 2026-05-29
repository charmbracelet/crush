package tools

import (
	"context"
	"log/slog"

	"github.com/taigrr/crush/internal/editor"
)

// notifyEditor fires the editor.Bridge hooks after Crush successfully
// writes a file. It is best-effort: any error is logged at debug level
// and never propagated to the agent. Safe to call with a nil bridge.
func notifyEditor(ctx context.Context, bridge editor.Bridge, path, oldContent, newContent string) {
	if bridge == nil || !bridge.Available() {
		return
	}

	startLine, endLine := editor.EditedRange(oldContent, newContent)
	if endLine > startLine {
		if err := bridge.FlashEdit(ctx, path, startLine, endLine); err != nil {
			slog.Debug("Editor flash edit failed", "path", path, "error", err)
		}
	}

	if err := bridge.NotifyFileChanged(ctx, path); err != nil {
		slog.Debug("Editor file-changed notify failed", "path", path, "error", err)
	}
}
