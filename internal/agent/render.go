package agent

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/compaction"
)

// renderedCheckpoint builds a checkpoint from recorded session memory.
//
// It returns "" when there is nothing to render from, which is the caller's
// signal to summarize with a model instead. That fallback is the whole reason
// this can be enabled without risk: a session that has never been observed, or
// one whose memory was retired, behaves exactly as it did before.
func (a *sessionAgent) renderedCheckpoint(ctx context.Context, sessionID string, budget compaction.RenderBudget) (string, int) {
	if a.ledger == nil || !a.renderFromLedger {
		return "", 0
	}

	current, err := a.ledger.Ledger(ctx, sessionID)
	if err != nil {
		// Memory is an optimization, so an unreadable ledger means falling
		// back to the mechanism that always works rather than failing a
		// compaction the session needs.
		slog.Warn("Falling back to summarizing: memory ledger unreadable",
			"session_id", sessionID, "error", err)
		return "", 0
	}
	if current.Empty() {
		return "", 0
	}

	rendered := current.Render(budget)
	if rendered.Kept == 0 {
		return "", 0
	}
	if rendered.DroppedHigh > 0 {
		// Losing a decision to the budget is the one outcome this mechanism
		// exists to avoid, so say so rather than let a thin checkpoint look
		// like a complete one.
		slog.Warn("A rendered checkpoint left out decisions to fit its budget",
			"session_id", sessionID,
			"entries", rendered.Kept,
			"dropped_decisions", rendered.DroppedHigh,
		)
	}
	slog.Debug("Rendered a checkpoint from session memory",
		"session_id", sessionID,
		"entries", rendered.Kept,
		"dropped", rendered.Dropped,
		"tokens", rendered.Tokens,
	)
	return rendered.Text, rendered.Kept
}
