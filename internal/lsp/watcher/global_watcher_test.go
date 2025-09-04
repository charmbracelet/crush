package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestGlobalWatcher(t *testing.T) {
	t.Parallel()

	// Test that we can get the global watcher instance
	gw1 := GetGlobalWatcher()
	if gw1 == nil {
		t.Fatal("Expected global watcher instance, got nil")
	}

	// Test that subsequent calls return the same instance (singleton)
	gw2 := GetGlobalWatcher()
	if gw1 != gw2 {
		t.Fatal("Expected same global watcher instance, got different instances")
	}

	// Test registration and unregistration
	mockWatcher := &WorkspaceWatcher{
		name: "test-watcher",
	}

	gw1.RegisterWorkspaceWatcher("test", mockWatcher)

	// Check that it was registered
	gw1.watchersMu.RLock()
	registered := gw1.workspaceWatchers["test"]
	gw1.watchersMu.RUnlock()

	if registered != mockWatcher {
		t.Fatal("Expected workspace watcher to be registered")
	}

	// Test unregistration
	gw1.UnregisterWorkspaceWatcher("test")

	gw1.watchersMu.RLock()
	unregistered := gw1.workspaceWatchers["test"]
	gw1.watchersMu.RUnlock()

	if unregistered != nil {
		t.Fatal("Expected workspace watcher to be unregistered")
	}
}

func TestGlobalWatcherWorkspaceDeduplication(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a new global watcher instance for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gw := &GlobalWatcher{
		workspaceWatchers: make(map[string]*WorkspaceWatcher),
		watchedDirs:       make(map[string]bool),
		workspacePaths:    make(map[string]bool),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Test that watching the same workspace twice doesn't duplicate
	err1 := gw.WatchWorkspace(tempDir)
	if err1 != nil {
		t.Fatalf("First WatchWorkspace call failed: %v", err1)
	}

	err2 := gw.WatchWorkspace(tempDir)
	if err2 != nil {
		t.Fatalf("Second WatchWorkspace call failed: %v", err2)
	}

	// Check that the workspace is only tracked once
	gw.workspacesMu.RLock()
	count := 0
	for path := range gw.workspacePaths {
		if path == tempDir {
			count++
		}
	}
	gw.workspacesMu.RUnlock()

	if count != 1 {
		t.Fatalf("Expected workspace to be tracked exactly once, got %d times", count)
	}
}

func TestGlobalWatcherDirectoryDeduplication(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create a new global watcher instance for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a real fsnotify watcher for testing
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	gw := &GlobalWatcher{
		watcher:           watcher,
		workspaceWatchers: make(map[string]*WorkspaceWatcher),
		watchedDirs:       make(map[string]bool),
		workspacePaths:    make(map[string]bool),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Test that watching the same directory twice doesn't duplicate
	err1 := gw.watchDirectory(tempDir)
	if err1 != nil {
		t.Fatalf("First watchDirectory call failed: %v", err1)
	}

	err2 := gw.watchDirectory(tempDir)
	if err2 != nil {
		t.Fatalf("Second watchDirectory call failed: %v", err2)
	}

	// Check that the directory is only tracked once
	gw.watchedMu.RLock()
	count := 0
	for dir := range gw.watchedDirs {
		if dir == tempDir {
			count++
		}
	}
	gw.watchedMu.RUnlock()

	if count != 1 {
		t.Fatalf("Expected directory to be tracked exactly once, got %d times", count)
	}
}

func TestGlobalWatcherShutdown(t *testing.T) {
	t.Parallel()

	// Create a new context for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary global watcher for testing
	gw := &GlobalWatcher{
		workspaceWatchers: make(map[string]*WorkspaceWatcher),
		watchedDirs:       make(map[string]bool),
		workspacePaths:    make(map[string]bool),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Test shutdown doesn't panic
	gw.Shutdown()

	// Verify context was cancelled
	select {
	case <-gw.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected context to be cancelled after shutdown")
	}
}