package backend

import "sync"

// clientTracker manages the mapping between clients, workspaces, and data
// directories. It ensures that multiple clients connecting to the same data
// directory share a single workspace to prevent SQLite contention.
type clientTracker struct {
	mu sync.Mutex
	// byDataDir maps data directory paths to workspace IDs.
	byDataDir map[string]string
	// byWorkspace maps workspace IDs to their connected client IDs.
	byWorkspace map[string]map[string]struct{}
	// byClient maps client IDs to their workspace ID.
	byClient map[string]string
}

func newClientTracker() *clientTracker {
	return &clientTracker{
		byDataDir:   make(map[string]string),
		byWorkspace: make(map[string]map[string]struct{}),
		byClient:    make(map[string]string),
	}
}

// workspaceForDataDir returns the workspace ID for a data directory, if one exists.
func (t *clientTracker) workspaceForDataDir(dataDir string) (workspaceID string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	workspaceID, ok = t.byDataDir[dataDir]
	return
}

// workspaceForClient returns the workspace ID for a client ID.
func (t *clientTracker) workspaceForClient(clientID string) (workspaceID string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	workspaceID, ok = t.byClient[clientID]
	return
}

// addClient registers a new client for a workspace and data directory.
// If this is the first client for the data directory, isNew is true.
func (t *clientTracker) addClient(clientID, workspaceID, dataDir string) (clientCount int, isNew bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, existed := t.byDataDir[dataDir]
	isNew = !existed

	t.byDataDir[dataDir] = workspaceID
	t.byClient[clientID] = workspaceID

	if t.byWorkspace[workspaceID] == nil {
		t.byWorkspace[workspaceID] = make(map[string]struct{})
	}
	t.byWorkspace[workspaceID][clientID] = struct{}{}

	return len(t.byWorkspace[workspaceID]), isNew
}

// removeClient unregisters a client and returns the workspace ID it belonged to.
// If this was the last client for the workspace, lastClient is true and dataDir
// is returned for cleanup.
func (t *clientTracker) removeClient(clientID string) (workspaceID, dataDir string, lastClient bool, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	workspaceID, ok = t.byClient[clientID]
	if !ok {
		return "", "", false, false
	}

	delete(t.byClient, clientID)

	clients := t.byWorkspace[workspaceID]
	delete(clients, clientID)

	if len(clients) > 0 {
		return workspaceID, "", false, true
	}

	// Last client - find and clean up data directory mapping.
	for dir, wsID := range t.byDataDir {
		if wsID == workspaceID {
			dataDir = dir
			delete(t.byDataDir, dir)
			break
		}
	}
	delete(t.byWorkspace, workspaceID)

	return workspaceID, dataDir, true, true
}

// clientCount returns the number of clients connected to a workspace.
func (t *clientTracker) clientCount(workspaceID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.byWorkspace[workspaceID])
}

// cleanupStaleWorkspace removes tracking for a workspace that no longer exists.
func (t *clientTracker) cleanupStaleWorkspace(workspaceID, dataDir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byDataDir, dataDir)
	delete(t.byWorkspace, workspaceID)
}
