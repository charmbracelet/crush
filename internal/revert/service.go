package revert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

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

	// 2. Determine the cut set: the checkpoint message and everything inserted
	// after it, ordered by rowid (insertion order). Using rowid instead of
	// created_at makes the cut exact even when two messages share the same
	// second-precision timestamp. File versions are correlated to this set by
	// message id.
	cutMsgs, err := s.messages.ListMessagesFromCheckpoint(ctx, sessionID, messageID)
	if err != nil {
		return Result{}, fmt.Errorf("listing messages from checkpoint: %w", err)
	}
	cutIDs := make(map[string]struct{}, len(cutMsgs))
	for _, m := range cutMsgs {
		cutIDs[m.ID] = struct{}{}
	}

	var result Result

	// 3. Restore code if requested.
	if opts.RestoreCode {
		codeResult, err := s.restoreCode(ctx, sessionID, cutIDs, cp.CreatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("restoring code: %w", err)
		}
		result.FilesRestored = codeResult.FilesRestored
		result.FilesDeleted = codeResult.FilesDeleted
	}

	// 4. Truncate conversation if requested.
	if opts.RestoreConversation {
		result.MessagesDeleted = len(cutMsgs)
		if err := s.messages.DeleteMessagesFromCheckpoint(ctx, sessionID, messageID); err != nil {
			return Result{}, fmt.Errorf("deleting messages: %w", err)
		}
	}

	return result, nil
}

// fileAction is a single planned file operation produced by planFileRevert.
type fileAction struct {
	path string
	// restore is true when the file should be rewritten to content; false
	// means the file was created in the reverted turn and should be removed.
	restore bool
	content string
	// versionIDs are the history rows produced in the reverted turn for this
	// path. They are dropped only after the on-disk operation succeeds.
	versionIDs []string
}

// planFileRevert partitions a session's file versions against the cut set and
// returns the on-disk operations needed to rewind code. A version belongs to
// the reverted turn when its producing message id is in cutIDs (or, for legacy
// rows with no message id, when its timestamp is at or after the checkpoint).
//
// For each path that has at least one reverted version, the file is restored
// to the newest version that is NOT in the cut set, or deleted if no such
// version exists (the agent created it during the reverted turn). The function
// is pure so the correlation logic can be tested without a database or disk.
func planFileRevert(versions []history.File, cutIDs map[string]struct{}, checkpointCreatedAt int64) []fileAction {
	reverted := func(v history.File) bool {
		if v.MessageID.Valid {
			_, ok := cutIDs[v.MessageID.String]
			return ok
		}
		// Legacy row with no message id: fall back to the timestamp.
		return v.CreatedAt >= checkpointCreatedAt
	}

	byPath := make(map[string][]history.File)
	paths := make([]string, 0)
	for _, v := range versions {
		if _, seen := byPath[v.Path]; !seen {
			paths = append(paths, v.Path)
		}
		byPath[v.Path] = append(byPath[v.Path], v)
	}
	sort.Strings(paths)

	var actions []fileAction
	for _, path := range paths {
		var revertedIDs []string
		var best *history.File
		for i := range byPath[path] {
			v := byPath[path][i]
			if reverted(v) {
				revertedIDs = append(revertedIDs, v.ID)
				continue
			}
			// Track the newest pre-checkpoint version as the restore source.
			if best == nil || v.Version > best.Version ||
				(v.Version == best.Version && v.CreatedAt > best.CreatedAt) {
				vv := v
				best = &vv
			}
		}
		if len(revertedIDs) == 0 {
			continue // path untouched by the reverted turn
		}
		if best != nil {
			actions = append(actions, fileAction{
				path:       path,
				restore:    true,
				content:    best.Content,
				versionIDs: revertedIDs,
			})
		} else {
			actions = append(actions, fileAction{
				path:       path,
				restore:    false,
				versionIDs: revertedIDs,
			})
		}
	}
	return actions
}

// restoreCode restores all files touched by the reverted turn to their
// pre-checkpoint state. Files that didn't exist before the checkpoint are
// deleted (agent-created files). History rows for a path are dropped only
// after its on-disk operation succeeds, so a failed write never strands the
// file at post-checkpoint content with no recoverable version.
func (s *Service) restoreCode(ctx context.Context, sessionID string, cutIDs map[string]struct{}, checkpointCreatedAt int64) (Result, error) {
	var result Result

	versions, err := s.history.ListBySession(ctx, sessionID)
	if err != nil {
		return result, fmt.Errorf("listing file versions: %w", err)
	}

	actions := planFileRevert(versions, cutIDs, checkpointCreatedAt)

	var orphanedVersionIDs []string
	for _, a := range actions {
		if a.restore {
			// Preserve the existing file's permissions where possible; the
			// history table stores content only, not mode.
			mode := os.FileMode(0o644)
			if fi, statErr := os.Stat(a.path); statErr == nil {
				mode = fi.Mode().Perm()
			}
			if mkErr := os.MkdirAll(filepath.Dir(a.path), 0o755); mkErr != nil {
				slog.Warn("Failed to create directory during revert", "path", a.path, "error", mkErr)
				continue
			}
			if writeErr := os.WriteFile(a.path, []byte(a.content), mode); writeErr != nil {
				slog.Warn("Failed to restore file during revert", "path", a.path, "error", writeErr)
				continue
			}
			result.FilesRestored = append(result.FilesRestored, a.path)
		} else {
			if removeErr := os.Remove(a.path); removeErr != nil && !os.IsNotExist(removeErr) {
				slog.Warn("Failed to delete agent-created file during revert", "path", a.path, "error", removeErr)
				continue
			}
			result.FilesDeleted = append(result.FilesDeleted, a.path)
		}
		orphanedVersionIDs = append(orphanedVersionIDs, a.versionIDs...)
	}

	// Drop the now-orphaned versions produced by the reverted turn.
	for _, id := range orphanedVersionIDs {
		if delErr := s.history.Delete(ctx, id); delErr != nil {
			slog.Warn("Failed to delete orphaned file version during revert", "id", id, "error", delErr)
		}
	}

	return result, nil
}
