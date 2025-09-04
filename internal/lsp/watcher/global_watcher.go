package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/fsnotify/fsnotify"
	ignore "github.com/sabhiram/go-gitignore"
)

// GlobalWatcher manages a single fsnotify.Watcher instance shared across all LSP clients.
//
// IMPORTANT: This implementation only watches directories, not individual files.
// The fsnotify library automatically provides events for all files within watched
// directories, making this approach much more efficient than watching individual files.
//
// Key benefits of directory-only watching:
// - Significantly fewer file descriptors used
// - Automatic coverage of new files created in watched directories
// - Better performance with large codebases
// - fsnotify handles deduplication internally (no need to track watched dirs)
type GlobalWatcher struct {
	watcher *fsnotify.Watcher

	// Map of workspace watchers by client name
	workspaceWatchers map[string]*WorkspaceWatcher
	watchersMu        sync.RWMutex

	// Single workspace root directory for ignore checking
	workspaceRoot string

	// Debouncing for file events (shared across all clients)
	debounceTime time.Duration
	debounceMap  *csync.Map[string, *time.Timer]

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for cleanup
	wg sync.WaitGroup
}

// GetGlobalWatcher returns the singleton global watcher instance
var GetGlobalWatcher = sync.OnceValue(func() *GlobalWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	gw := &GlobalWatcher{
		workspaceWatchers: make(map[string]*WorkspaceWatcher),
		debounceTime:      300 * time.Millisecond,
		debounceMap:       csync.NewMap[string, *time.Timer](),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Initialize the fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to create global file watcher", "error", err)
		return gw
	}

	gw.watcher = watcher

	// Start the event processing goroutine
	gw.wg.Add(1)
	go gw.processEvents()

	return gw
})

// RegisterWorkspaceWatcher registers a workspace watcher with the global watcher
func (gw *GlobalWatcher) RegisterWorkspaceWatcher(name string, watcher *WorkspaceWatcher) {
	gw.watchersMu.Lock()
	defer gw.watchersMu.Unlock()

	gw.workspaceWatchers[name] = watcher
	slog.Debug("Registered workspace watcher", "name", name)
}

// UnregisterWorkspaceWatcher removes a workspace watcher from the global watcher
func (gw *GlobalWatcher) UnregisterWorkspaceWatcher(name string) {
	gw.watchersMu.Lock()
	defer gw.watchersMu.Unlock()

	delete(gw.workspaceWatchers, name)
	slog.Debug("Unregistered workspace watcher", "name", name)
}

// WatchWorkspace adds a workspace to be watched.
// Note: We only watch directories, not individual files. fsnotify automatically provides
// events for all files within watched directories. Multiple calls with the same workspace
// are safe since fsnotify handles directory deduplication internally.
func (gw *GlobalWatcher) WatchWorkspace(workspacePath string) error {
	cfg := config.Get()
	slog.Debug("Adding workspace to global watcher", "path", workspacePath)

	// Store the workspace root for hierarchical ignore checking
	gw.workspaceRoot = workspacePath

	// Walk the workspace and add only directories to the watcher
	// fsnotify will automatically provide events for all files within these directories
	// Multiple calls with the same directories are safe (fsnotify deduplicates)
	err := filepath.WalkDir(workspacePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process directories - we don't watch individual files
		if !d.IsDir() {
			return nil
		}

		// Check if directory should be skipped based on hierarchical .gitignore/.crushignore
		if gw.shouldIgnoreDirectory(path) {
			if cfg != nil && cfg.Options.DebugLSP {
				slog.Debug("Skipping ignored directory", "path", path)
			}
			return filepath.SkipDir
		}

		// Add directory to watcher (fsnotify handles deduplication automatically)
		if err := gw.addDirectoryToWatcher(path); err != nil {
			slog.Error("Error watching directory", "path", path, "error", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking workspace %s: %w", workspacePath, err)
	}

	return nil
}

// shouldIgnoreDirectory checks if a directory should be ignored based on hierarchical
// .gitignore/.crushignore files. It checks for ignore files in each directory from
// the target directory up to the workspace root.
func (gw *GlobalWatcher) shouldIgnoreDirectory(dirPath string) bool {
	if gw.workspaceRoot == "" {
		return false
	}

	if strings.HasPrefix(dirPath, gw.workspaceRoot) {
		return gw.isIgnoredInWorkspace(dirPath, gw.workspaceRoot)
	}

	return false
}

// isIgnoredInWorkspace checks if a path is ignored by walking up the directory tree
// from the target path to the workspace root, checking for .gitignore/.crushignore
// files in each directory.
func (gw *GlobalWatcher) isIgnoredInWorkspace(targetPath, workspaceRoot string) bool {
	// Get relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, targetPath)
	if err != nil {
		return false
	}

	// Don't ignore the workspace root itself
	if relPath == "." {
		return false
	}

	// Check each directory from workspace root to target path for ignore files
	currentPath := workspaceRoot
	pathComponents := strings.Split(relPath, string(filepath.Separator))

	for i, component := range pathComponents {
		currentPath = filepath.Join(currentPath, component)

		// Build the relative path from workspace root to current path
		currentRelPath := strings.Join(pathComponents[:i+1], "/")

		// Check ignore files in each parent directory
		checkPath := workspaceRoot
		for j := 0; j <= i; j++ {
			if j > 0 {
				checkPath = filepath.Join(checkPath, pathComponents[j-1])
			}

			// Check .gitignore in this directory
			if gw.checkIgnoreFile(filepath.Join(checkPath, ".gitignore"), currentRelPath) {
				return true
			}

			// Check .crushignore in this directory
			if gw.checkIgnoreFile(filepath.Join(checkPath, ".crushignore"), currentRelPath) {
				return true
			}
		}
	}

	return false
}

// checkIgnoreFile checks if a path matches patterns in an ignore file
func (gw *GlobalWatcher) checkIgnoreFile(ignoreFilePath, relPath string) bool {
	content, err := os.ReadFile(ignoreFilePath)
	if err != nil {
		return false // File doesn't exist or can't be read
	}

	lines := strings.Split(string(content), "\n")
	ignoreParser := ignore.CompileIgnoreLines(lines...)

	// Check both with and without trailing slash for directories
	if ignoreParser.MatchesPath(relPath) {
		return true
	}

	// For directories, also check with trailing slash
	if ignoreParser.MatchesPath(relPath + "/") {
		return true
	}

	return false
}

// addDirectoryToWatcher adds a directory to the fsnotify watcher.
// fsnotify handles deduplication internally, so we don't need to track watched directories.
func (gw *GlobalWatcher) addDirectoryToWatcher(dirPath string) error {
	if gw.watcher == nil {
		return fmt.Errorf("global watcher not initialized")
	}

	// Add directory to fsnotify watcher - fsnotify handles deduplication
	// "A path can only be watched once; watching it more than once is a no-op"
	err := gw.watcher.Add(dirPath)
	if err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dirPath, err)
	}

	slog.Debug("Added directory to global watcher", "path", dirPath)
	return nil
}

// processEvents processes file system events and handles them centrally.
// Since we only watch directories, we automatically get events for all files
// within those directories. When new directories are created, we add them
// to the watcher to ensure complete coverage.
func (gw *GlobalWatcher) processEvents() {
	defer gw.wg.Done()
	cfg := config.Get()

	if gw.watcher == nil {
		slog.Error("Global watcher not initialized")
		return
	}

	for {
		select {
		case <-gw.ctx.Done():
			return

		case event, ok := <-gw.watcher.Events:
			if !ok {
				return
			}

			// Handle directory creation globally (only once)
			// When new directories are created, we need to add them to the watcher
			// to ensure we get events for files created within them
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !gw.shouldIgnoreDirectory(event.Name) {
						if err := gw.addDirectoryToWatcher(event.Name); err != nil {
							slog.Error("Error adding new directory to watcher", "path", event.Name, "error", err)
						}
					} else if cfg != nil && cfg.Options.DebugLSP {
						slog.Debug("Skipping ignored new directory", "path", event.Name)
					}
				}
			}

			if cfg != nil && cfg.Options.DebugLSP {
				slog.Debug("Global watcher received event", "path", event.Name, "op", event.Op.String())
			}

			// Process the event centrally
			gw.handleFileEvent(event)

		case err, ok := <-gw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Global watcher error", "error", err)
		}
	}
}

// handleFileEvent processes a file system event and distributes notifications to relevant clients
func (gw *GlobalWatcher) handleFileEvent(event fsnotify.Event) {
	cfg := config.Get()
	uri := string(protocol.URIFromPath(event.Name))

	// Handle file creation for all relevant clients (only once)
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && !info.IsDir() {
			if !shouldExcludeFile(event.Name) {
				gw.openMatchingFileForClients(event.Name)
			}
		}
	}

	// Get all workspace watchers that might be interested in this file
	gw.watchersMu.RLock()
	watchers := make(map[string]*WorkspaceWatcher, len(gw.workspaceWatchers))
	maps.Copy(watchers, gw.workspaceWatchers)
	gw.watchersMu.RUnlock()

	// Process the event for each relevant client
	for clientName, watcher := range watchers {
		if !watcher.client.HandlesFile(event.Name) {
			continue // client doesn't handle this filetype
		}

		// Debug logging per client
		if cfg.Options.DebugLSP {
			matched, kind := watcher.isPathWatched(event.Name)
			slog.Debug("File event for client",
				"path", event.Name,
				"operation", event.Op.String(),
				"watched", matched,
				"kind", kind,
				"client", clientName,
			)
		}

		// Check if this path should be watched according to server registrations
		if watched, watchKind := watcher.isPathWatched(event.Name); watched {
			switch {
			case event.Op&fsnotify.Write != 0:
				if watchKind&protocol.WatchChange != 0 {
					gw.debounceHandleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Changed))
				}
			case event.Op&fsnotify.Create != 0:
				// File creation was already handled globally above
				// Just send the notification if needed
				info, err := os.Stat(event.Name)
				if err != nil {
					if !os.IsNotExist(err) {
						slog.Debug("Error getting file info", "path", event.Name, "error", err)
					}
					continue
				}
				if !info.IsDir() && watchKind&protocol.WatchCreate != 0 {
					gw.debounceHandleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Created))
				}
			case event.Op&fsnotify.Remove != 0:
				if watchKind&protocol.WatchDelete != 0 {
					gw.handleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Deleted))
				}
			case event.Op&fsnotify.Rename != 0:
				// For renames, first delete
				if watchKind&protocol.WatchDelete != 0 {
					gw.handleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Deleted))
				}

				// Then check if the new file exists and create an event
				if info, err := os.Stat(event.Name); err == nil && !info.IsDir() {
					if watchKind&protocol.WatchCreate != 0 {
						gw.debounceHandleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Created))
					}
				}
			}
		}
	}
}

// openMatchingFileForClients opens a newly created file for all clients that handle it (only once per file)
func (gw *GlobalWatcher) openMatchingFileForClients(path string) {
	// Skip directories
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}

	// Skip excluded files
	if shouldExcludeFile(path) {
		return
	}

	gw.watchersMu.RLock()
	watchers := make(map[string]*WorkspaceWatcher, len(gw.workspaceWatchers))
	maps.Copy(watchers, gw.workspaceWatchers)
	gw.watchersMu.RUnlock()

	// Open the file for each client that handles it and has matching patterns
	for _, watcher := range watchers {
		if watcher.client.HandlesFile(path) {
			watcher.openMatchingFile(gw.ctx, path)
		}
	}
}

// debounceHandleFileEventForClient handles file events with debouncing for a specific client
func (gw *GlobalWatcher) debounceHandleFileEventForClient(watcher *WorkspaceWatcher, uri string, changeType protocol.FileChangeType) {
	// Create a unique key based on URI, change type, and client name
	key := fmt.Sprintf("%s:%d:%s", uri, changeType, watcher.name)

	// Cancel existing timer if any
	if timer, exists := gw.debounceMap.Get(key); exists {
		timer.Stop()
	}

	// Create new timer
	gw.debounceMap.Set(key, time.AfterFunc(gw.debounceTime, func() {
		gw.handleFileEventForClient(watcher, uri, changeType)

		// Cleanup timer after execution
		gw.debounceMap.Del(key)
	}))
}

// handleFileEventForClient sends file change notifications to a specific client
func (gw *GlobalWatcher) handleFileEventForClient(watcher *WorkspaceWatcher, uri string, changeType protocol.FileChangeType) {
	// If the file is open and it's a change event, use didChange notification
	filePath, err := protocol.DocumentURI(uri).Path()
	if err != nil {
		slog.Error("Error converting URI to path", "uri", uri, "error", err)
		return
	}

	if changeType == protocol.FileChangeType(protocol.Deleted) {
		watcher.client.ClearDiagnosticsForURI(protocol.DocumentURI(uri))
	} else if changeType == protocol.FileChangeType(protocol.Changed) && watcher.client.IsFileOpen(filePath) {
		err := watcher.client.NotifyChange(gw.ctx, filePath)
		if err != nil {
			slog.Error("Error notifying change", "error", err)
		}
		return
	}

	// Notify LSP server about the file event using didChangeWatchedFiles
	if err := watcher.notifyFileEvent(gw.ctx, uri, changeType); err != nil {
		slog.Error("Error notifying LSP server about file event", "error", err)
	}
}

// Shutdown gracefully shuts down the global watcher
func (gw *GlobalWatcher) Shutdown() {
	if gw.cancel != nil {
		gw.cancel()
	}

	if gw.watcher != nil {
		gw.watcher.Close()
		gw.watcher = nil
	}

	gw.wg.Wait()
	slog.Debug("Global watcher shutdown complete")
}

// ShutdownGlobalWatcher shuts down the singleton global watcher
func ShutdownGlobalWatcher() {
	if gw := GetGlobalWatcher(); gw != nil {
		gw.Shutdown()
	}
}
