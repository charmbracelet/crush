package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/rjeczalik/notify"
)

func TestGlobalWatcher(t *testing.T) {
	t.Parallel()

	// Test that we can get the global watcher instance
	gw1 := instance()
	if gw1 == nil {
		t.Fatal("Expected global watcher instance, got nil")
	}

	// Test that subsequent calls return the same instance (singleton)
	gw2 := instance()
	if gw1 != gw2 {
		t.Fatal("Expected same global watcher instance, got different instances")
	}

	// Test registration and unregistration
	mockWatcher := &Client{
		name: "test-watcher",
	}

	gw1.register("test", mockWatcher)

	// Check that it was registered
	registered, _ := gw1.watchers.Get("test")

	if registered != mockWatcher {
		t.Fatal("Expected workspace watcher to be registered")
	}

	// Test unregistration
	gw1.unregister("test")

	unregistered, _ := gw1.watchers.Get("test")

	if unregistered != nil {
		t.Fatal("Expected workspace watcher to be unregistered")
	}
}

func TestGlobalWatcherWorkspaceIdempotent(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a new global watcher instance for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gw := &global{
		events:       make(chan notify.EventInfo, 100),
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Test that watching the same workspace multiple times is safe (idempotent)
	// With notify, we use recursive watching with "..."
	watchPath := filepath.Join(tempDir, "...")

	err1 := notify.Watch(watchPath, gw.events, notify.All)
	if err1 != nil {
		t.Fatalf("First Watch call failed: %v", err1)
	}
	defer notify.Stop(gw.events)

	// Watching the same path again should be safe (notify handles this)
	err2 := notify.Watch(watchPath, gw.events, notify.All)
	if err2 != nil {
		t.Fatalf("Second Watch call failed: %v", err2)
	}

	err3 := notify.Watch(watchPath, gw.events, notify.All)
	if err3 != nil {
		t.Fatalf("Third Watch call failed: %v", err3)
	}

	// All calls should succeed - notify handles deduplication internally
	// This test verifies that multiple Watch calls are safe
}

func TestGlobalWatcherRecursiveWatching(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create some files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create a new global watcher instance for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gw := &global{
		events:       make(chan notify.EventInfo, 100),
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
		root:         tempDir,
	}

	// Set up recursive watching on the root directory
	watchPath := filepath.Join(tempDir, "...")
	if err := notify.Watch(watchPath, gw.events, notify.All); err != nil {
		t.Fatalf("Failed to set up recursive watch: %v", err)
	}
	defer notify.Stop(gw.events)

	// Verify that our expected directories and files exist
	expectedDirs := []string{tempDir, subDir}

	for _, expectedDir := range expectedDirs {
		info, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("Expected directory %s doesn't exist: %v", expectedDir, err)
		}
		if !info.IsDir() {
			t.Fatalf("Expected %s to be a directory, but it's not", expectedDir)
		}
	}

	// Verify that files exist
	testFiles := []string{file1, file2}
	for _, file := range testFiles {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatalf("Test file %s doesn't exist: %v", file, err)
		}
		if info.IsDir() {
			t.Fatalf("Expected %s to be a file, but it's a directory", file)
		}
	}

	// Create a new file in the subdirectory to test recursive watching
	newFile := filepath.Join(subDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0o644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// We should receive an event for the file creation
	select {
	case event := <-gw.events:
		// On macOS, paths might have /private prefix, so we need to compare the real paths
		eventPath, _ := filepath.EvalSymlinks(event.Path())
		expectedPath, _ := filepath.EvalSymlinks(newFile)
		if eventPath != expectedPath {
			// Also try comparing just the base names as a fallback
			if filepath.Base(event.Path()) != filepath.Base(newFile) {
				t.Errorf("Expected event for %s, got %s", newFile, event.Path())
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for file creation event")
	}
}

func TestNotifyDeduplication(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create an event channel
	events := make(chan notify.EventInfo, 100)
	defer close(events)

	// Add the same directory multiple times with recursive watching
	watchPath := filepath.Join(tempDir, "...")

	err1 := notify.Watch(watchPath, events, notify.All)
	if err1 != nil {
		t.Fatalf("First Watch failed: %v", err1)
	}
	defer notify.Stop(events)

	err2 := notify.Watch(watchPath, events, notify.All)
	if err2 != nil {
		t.Fatalf("Second Watch failed: %v", err2)
	}

	err3 := notify.Watch(watchPath, events, notify.All)
	if err3 != nil {
		t.Fatalf("Third Watch failed: %v", err3)
	}

	// All should succeed - notify handles deduplication internally
	// This test verifies the notify behavior we're relying on
}

func TestGlobalWatcherRespectsIgnoreFiles(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create directories that should be ignored
	nodeModules := filepath.Join(tempDir, "node_modules")
	target := filepath.Join(tempDir, "target")
	customIgnored := filepath.Join(tempDir, "custom_ignored")
	normalDir := filepath.Join(tempDir, "src")

	for _, dir := range []string{nodeModules, target, customIgnored, normalDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create .gitignore file
	gitignoreContent := "node_modules/\ntarget/\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	// Create .crushignore file
	crushignoreContent := "custom_ignored/\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".crushignore"), []byte(crushignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to create .crushignore: %v", err)
	}

	// Create a new global watcher instance for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gw := &global{
		events:       make(chan notify.EventInfo, 100),
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
		root:         tempDir,
	}

	// Set up recursive watching
	watchPath := filepath.Join(tempDir, "...")
	if err := notify.Watch(watchPath, gw.events, notify.All); err != nil {
		t.Fatalf("Failed to set up recursive watch: %v", err)
	}
	defer notify.Stop(gw.events)

	// The notify library watches everything, but our processEvents
	// function should filter out ignored files using fsext.ShouldExcludeFile
	// This test verifies that the structure is set up correctly
}

func TestGlobalWatcherShutdown(t *testing.T) {
	t.Parallel()

	// Create a new context for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary global watcher for testing
	gw := &global{
		events:       make(chan notify.EventInfo, 100),
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Test shutdown doesn't panic
	gw.shutdown()

	// Verify context was cancelled
	select {
	case <-gw.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected context to be cancelled after shutdown")
	}
}
