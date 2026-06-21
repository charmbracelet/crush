package backend

import (
	"context"
	"time"
)

// FileTrackerRecordRead records a file read for a session.
func (b *Backend) FileTrackerRecordRead(ctx context.Context, workspaceID, sessionID, path string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.FileTracker.RecordRead(ctx, sessionID, path)
	return nil
}

// FileTrackerLastReadTime returns the last read time for a file in a session.
func (b *Backend) FileTrackerLastReadTime(ctx context.Context, workspaceID, sessionID, path string) (time.Time, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return time.Time{}, err
	}

	return ws.FileTracker.LastReadTime(ctx, sessionID, path), nil
}

// FileTrackerListReadFiles returns the list of read files for a session.
func (b *Backend) FileTrackerListReadFiles(ctx context.Context, workspaceID, sessionID string) ([]string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.FileTracker.ListReadFiles(ctx, sessionID)
}
