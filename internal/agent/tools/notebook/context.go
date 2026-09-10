package notebook

import (
	"context"

	"github.com/charmbracelet/crush/internal/agent/tools"
)

// getSessionID retrieves the session ID from the context.
func getSessionID(ctx context.Context) string {
	return tools.GetSessionFromContext(ctx)
}
