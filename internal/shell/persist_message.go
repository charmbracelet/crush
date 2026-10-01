package shell

import (
	"context"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// PersistOutput stores a bang-mode shell command result as a user message.
// Oversized output is truncated first, with the full text spilled to a file
// under spillDir, so a single noisy command cannot swamp the context window
// or the session.
// If the target session no longer exists (deleted before or during the
// command), persistence is skipped without surfacing an error.
func PersistOutput(
	ctx context.Context,
	messages message.Service,
	sessionID, command, output string,
	exitCode int,
	spillDir string,
) error {
	if sessionID == "" {
		return nil
	}

	_, err := messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{message.ShellCommand{
			Command:  command,
			Output:   TruncateForPersist(output, spillDir),
			ExitCode: exitCode,
		}},
	})
	// The messages table has a single foreign key (session_id), so an FK
	// failure here can only mean the session is gone. We match on the error
	// text because the codebase builds against two swappable SQLite drivers
	// (modernc and ncruces) and this is the one signal stable across both.
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		slog.Debug(
			"Skipping shell command persistence: session no longer exists",
			"session_id", sessionID,
		)
		return nil
	}
	return err
}
