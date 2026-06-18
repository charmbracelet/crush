package revert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
)

var (
	// ErrSessionBusy is returned when the session is currently processing a request.
	ErrSessionBusy = errors.New("session is busy")
	// ErrMessageNotFound is returned when the checkpoint message doesn't exist.
	ErrMessageNotFound = errors.New("message not found")
)

// Options controls what parts of the session to revert.
type Options struct {
	// RestoreCode restores agent-edited/created files to their pre-checkpoint state.
	RestoreCode bool
	// RestoreConversation truncates the conversation after the checkpoint.
	RestoreConversation bool
}

// Result describes the outcome of a revert operation.
type Result struct {
	MessagesDeleted int
	FilesRestored   []string
	FilesDeleted    []string
}

// Service orchestrates the revert operation.
type Service struct {
	history  history.Service
	messages message.Service
}

// NewService creates a new revert Service.
func NewService(history history.Service, messages message.Service) *Service {
	return &Service{
		history:  history,
		messages: messages,
	}
}

// RevertToMessage reverts the session to the state just before the given
// checkpoint message. Files are restored to their pre-checkpoint content,
// and messages at or after the checkpoint are deleted.
func (s *Service) RevertToMessage(
	ctx context.Context,
	sessionID string,
	messageID string,
	isBusy bool,
	opts Options,
) (Result, error) {
	if isBusy {
		return Result{}, ErrSessionBusy
	}

	// 1. Get the checkpoint message.
	cp, err := s.messages.Get(ctx, messageID)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s", ErrMessageNotFound, err)
	}
	if cp.SessionID != sessionID {
		return Result{}, fmt.Errorf("%w: message belongs to session %s", ErrMessageNotFound, cp.SessionID)
	}

	var result Result

	// 2. Restore code if requested.
	if opts.RestoreCode {
		codeResult, err := s.restoreCode(ctx, sessionID, cp.CreatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("restoring code: %w", err)
		}
		result.FilesRestored = codeResult.FilesRestored
		result.FilesDeleted = codeResult.FilesDeleted
	}

	// 3. Truncate conversation if requested.
	if opts.RestoreConversation {
		msgs, err := s.messages.ListMessagesAfter(ctx, sessionID, cp.CreatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("listing messages to delete: %w", err)
		}
		result.MessagesDeleted = len(msgs)

		if err := s.messages.DeleteMessagesAfter(ctx, sessionID, cp.CreatedAt); err != nil {
			return Result{}, fmt.Errorf("deleting messages: %w", err)
		}
	}

	return result, nil
}

// restoreCode restores all files touched at or after the checkpoint to their
// pre-checkpoint state. Files that didn't exist before the checkpoint are
// deleted (agent-created files).
func (s *Service) restoreCode(ctx context.Context, sessionID string, checkpointCreatedAt int64) (Result, error) {
	var result Result

	// Find all distinct paths that have versions at or after the checkpoint.
	paths, err := s.history.ListDistinctPathsAfterCheckpoint(ctx, sessionID, checkpointCreatedAt)
	if err != nil {
		return result, fmt.Errorf("listing paths after checkpoint: %w", err)
	}

	for _, path := range paths {
		// Try to get the latest version of this file before the checkpoint.
		prev, err := s.history.GetFileVersionBeforeCheckpoint(ctx, path, sessionID, checkpointCreatedAt)
		if err != nil {
			// No version before checkpoint — this file was agent-created in the
			// reverted turn. Delete it from disk.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				slog.Warn("Failed to delete agent-created file during revert", "path", path, "error", removeErr)
			}
			result.FilesDeleted = append(result.FilesDeleted, path)
			continue
		}

		// Restore the file to its pre-checkpoint content.
		if writeErr := os.WriteFile(path, []byte(prev.Content), 0o644); writeErr != nil {
			slog.Warn("Failed to restore file during revert", "path", path, "error", writeErr)
			continue
		}
		result.FilesRestored = append(result.FilesRestored, path)
	}

	// Clean up orphaned file versions at or after the checkpoint.
	if err := s.history.DeleteFileVersionsAfterCheckpoint(ctx, sessionID, checkpointCreatedAt); err != nil {
		return result, fmt.Errorf("cleaning up file versions: %w", err)
	}

	return result, nil
}
