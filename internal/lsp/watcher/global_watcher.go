package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/rjeczalik/notify"
)

// global manages file watching shared across all LSP clients.
//
// IMPORTANT: This implementation uses github.com/rjeczalik/notify which provides
// recursive watching on all platforms. On macOS it uses FSEvents, on Linux it
// uses inotify (with recursion handled by the library), and on Windows it uses
// ReadDirectoryChangesW.
//
// Key benefits:
// - Single watch point for entire directory tree
// - Automatic recursive watching without manually adding subdirectories
// - No file descriptor exhaustion issues
type global struct {
	// Channel for receiving file system events
	events chan notify.EventInfo

	// Map of workspace watchers by client name
	watchers *csync.Map[string, *Client]

	// Single workspace root directory for ignore checking
	root string

	started atomic.Bool

	// Debouncing for file events (shared across all clients)
	debounceTime time.Duration
	debounceMap  *csync.Map[string, *time.Timer]

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for cleanup
	wg sync.WaitGroup
}

// instance returns the singleton global watcher instance
var instance = sync.OnceValue(func() *global {
	ctx, cancel := context.WithCancel(context.Background())
	gw := &global{
		events:       make(chan notify.EventInfo, 4096), // Large buffer to prevent dropping events
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	return gw
})

// register registers a workspace watcher with the global watcher
func (gw *global) register(name string, watcher *Client) {
	gw.watchers.Set(name, watcher)
	slog.Debug("lsp watcher: Registered workspace watcher", "name", name)
}

// unregister removes a workspace watcher from the global watcher
func (gw *global) unregister(name string) {
	gw.watchers.Del(name)
	slog.Debug("lsp watcher: Unregistered workspace watcher", "name", name)
}

// Start sets up recursive watching on the workspace root.
//
// Note: We use github.com/rjeczalik/notify which provides recursive watching
// with a single watch point. The "..." suffix means watch recursively.
// This is much more efficient than manually walking and watching each directory.
func Start() error {
	gw := instance()

	// technically workspace root is always the same...
	if gw.started.Load() {
		slog.Debug("lsp watcher: watcher already set up, skipping")
		return nil
	}

	cfg := config.Get()
	root := cfg.WorkingDir()
	slog.Debug("lsp watcher: set workspace directory to global watcher", "path", root)

	// Store the workspace root for hierarchical ignore checking
	gw.root = root
	gw.started.Store(true)

	// Start the event processing goroutine
	gw.wg.Add(1)
	go gw.processEvents()

	// Set up recursive watching on the root directory
	// The "..." suffix tells notify to watch recursively
	watchPath := filepath.Join(root, "...")

	// Watch for all event types we care about
	events := notify.Create | notify.Write | notify.Remove | notify.Rename

	if err := notify.Watch(watchPath, gw.events, events); err != nil {
		return fmt.Errorf("lsp watcher: error setting up recursive watch on %s: %w", root, err)
	}

	slog.Info("lsp watcher: Started recursive watching", "root", root)
	return nil
}

// processEvents processes file system events from the notify library.
// Since notify handles recursive watching for us, we don't need to manually
// add new directories - they're automatically included.
func (gw *global) processEvents() {
	defer gw.wg.Done()
	cfg := config.Get()

	if !gw.started.Load() {
		slog.Error("lsp watcher: Global watcher not initialized")
		return
	}

	for {
		select {
		case <-gw.ctx.Done():
			return

		case event, ok := <-gw.events:
			if !ok {
				return
			}

			path := event.Path()

			// Skip ignored files
			if fsext.ShouldExcludeFile(gw.root, path) {
				continue
			}

			if cfg != nil && cfg.Options.DebugLSP {
				slog.Debug("lsp watcher: Global watcher received event", "path", path, "event", event.Event().String())
			}

			// Convert notify event to our internal format and handle it
			gw.handleFileEvent(event)
		}
	}
}

// handleFileEvent processes a file system event and distributes notifications to relevant clients
func (gw *global) handleFileEvent(event notify.EventInfo) {
	cfg := config.Get()
	path := event.Path()
	uri := string(protocol.URIFromPath(path))

	// Map notify events to our change types
	var changeType protocol.FileChangeType
	var watchKindNeeded protocol.WatchKind

	switch event.Event() {
	case notify.Create:
		changeType = protocol.FileChangeType(protocol.Created)
		watchKindNeeded = protocol.WatchCreate
		// Handle file creation for all relevant clients
		if !isDir(path) && !fsext.ShouldExcludeFile(gw.root, path) {
			gw.openMatchingFileForClients(path)
		}
	case notify.Write:
		changeType = protocol.FileChangeType(protocol.Changed)
		watchKindNeeded = protocol.WatchChange
	case notify.Remove:
		changeType = protocol.FileChangeType(protocol.Deleted)
		watchKindNeeded = protocol.WatchDelete
	case notify.Rename:
		// Treat rename as delete + create
		// First handle as delete
		for _, watcher := range gw.watchers.Seq2() {
			if !watcher.client.HandlesFile(path) {
				continue
			}
			if watched, watchKind := watcher.isPathWatched(path); watched {
				if watchKind&protocol.WatchDelete != 0 {
					gw.handleFileEventForClient(watcher, uri, protocol.FileChangeType(protocol.Deleted))
				}
			}
		}
		// Then check if renamed file exists and treat as create
		if !isDir(path) {
			changeType = protocol.FileChangeType(protocol.Created)
			watchKindNeeded = protocol.WatchCreate
		} else {
			return // Already handled delete, nothing more to do for directories
		}
	default:
		// Unknown event type, skip
		return
	}

	// Process the event for each relevant client
	for client, watcher := range gw.watchers.Seq2() {
		if !watcher.client.HandlesFile(path) {
			continue // client doesn't handle this filetype
		}

		// Debug logging per client
		if cfg.Options.DebugLSP {
			matched, kind := watcher.isPathWatched(path)
			slog.Debug("lsp watcher: File event for client",
				"path", path,
				"event", event.Event().String(),
				"watched", matched,
				"kind", kind,
				"client", client,
			)
		}

		// Check if this path should be watched according to server registrations
		if watched, watchKind := watcher.isPathWatched(path); watched {
			if watchKind&watchKindNeeded != 0 {
				// Skip directory events for non-delete operations
				if changeType != protocol.FileChangeType(protocol.Deleted) && isDir(path) {
					continue
				}

				if changeType == protocol.FileChangeType(protocol.Deleted) {
					// Don't debounce deletes
					gw.handleFileEventForClient(watcher, uri, changeType)
				} else {
					// Debounce creates and changes
					gw.debounceHandleFileEventForClient(watcher, uri, changeType)
				}
			}
		}
	}
}

// isDir checks if a path is a directory
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// openMatchingFileForClients opens a newly created file for all clients that handle it (only once per file)
func (gw *global) openMatchingFileForClients(path string) {
	// Skip directories
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}

	// Skip excluded files
	if fsext.ShouldExcludeFile(gw.root, path) {
		return
	}

	// Open the file for each client that handles it and has matching patterns
	for _, watcher := range gw.watchers.Seq2() {
		if watcher.client.HandlesFile(path) {
			watcher.openMatchingFile(gw.ctx, path)
		}
	}
}

// debounceHandleFileEventForClient handles file events with debouncing for a specific client
func (gw *global) debounceHandleFileEventForClient(watcher *Client, uri string, changeType protocol.FileChangeType) {
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
func (gw *global) handleFileEventForClient(watcher *Client, uri string, changeType protocol.FileChangeType) {
	// If the file is open and it's a change event, use didChange notification
	filePath, err := protocol.DocumentURI(uri).Path()
	if err != nil {
		slog.Error("lsp watcher: Error converting URI to path", "uri", uri, "error", err)
		return
	}

	if changeType == protocol.FileChangeType(protocol.Deleted) {
		watcher.client.ClearDiagnosticsForURI(protocol.DocumentURI(uri))
	} else if changeType == protocol.FileChangeType(protocol.Changed) && watcher.client.IsFileOpen(filePath) {
		err := watcher.client.NotifyChange(gw.ctx, filePath)
		if err != nil {
			slog.Error("lsp watcher: Error notifying change", "error", err)
		}
		return
	}

	// Notify LSP server about the file event using didChangeWatchedFiles
	if err := watcher.notifyFileEvent(gw.ctx, uri, changeType); err != nil {
		slog.Error("lsp watcher: Error notifying LSP server about file event", "error", err)
	}
}

// shutdown gracefully shuts down the global watcher
func (gw *global) shutdown() {
	if gw.cancel != nil {
		gw.cancel()
	}

	// Stop watching and close the event channel
	notify.Stop(gw.events)
	close(gw.events)

	gw.wg.Wait()
	slog.Debug("lsp watcher: Global watcher shutdown complete")
}

// Shutdown shuts down the singleton global watcher
func Shutdown() {
	instance().shutdown()
}
