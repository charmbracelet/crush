package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/fsnotify/fsnotify"
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

	// Create a real fsnotify watcher for testing
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	gw := &global{
		watcher:      watcher,
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Test that watching the same workspace multiple times is safe (idempotent)
	err1 := gw.addDirectoryToWatcher(tempDir)
	if err1 != nil {
		t.Fatalf("First addDirectoryToWatcher call failed: %v", err1)
	}

	err2 := gw.addDirectoryToWatcher(tempDir)
	if err2 != nil {
		t.Fatalf("Second addDirectoryToWatcher call failed: %v", err2)
	}

	err3 := gw.addDirectoryToWatcher(tempDir)
	if err3 != nil {
		t.Fatalf("Third addDirectoryToWatcher call failed: %v", err3)
	}

	// All calls should succeed - fsnotify handles deduplication internally
	// This test verifies that multiple WatchWorkspace calls are safe
}

func TestGlobalWatcherOnlyWatchesDirectories(t *testing.T) {
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

	// Create a real fsnotify watcher for testing
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	gw := &global{
		watcher:      watcher,
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Watch the workspace
	err = gw.addDirectoryToWatcher(tempDir)
	if err != nil {
		t.Fatalf("addDirectoryToWatcher failed: %v", err)
	}

	// Verify that our expected directories exist and can be watched
	expectedDirs := []string{tempDir, subDir}

	for _, expectedDir := range expectedDirs {
		info, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("Expected directory %s doesn't exist: %v", expectedDir, err)
		}
		if !info.IsDir() {
			t.Fatalf("Expected %s to be a directory, but it's not", expectedDir)
		}

		// Try to add it again - fsnotify should handle this gracefully
		err = gw.addDirectoryToWatcher(expectedDir)
		if err != nil {
			t.Fatalf("Failed to add directory %s to watcher: %v", expectedDir, err)
		}
	}

	// Verify that files exist but we don't try to watch them directly
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
}

func TestFsnotifyDeduplication(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a real fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	// Add the same directory multiple times
	err1 := watcher.Add(tempDir)
	if err1 != nil {
		t.Fatalf("First Add failed: %v", err1)
	}

	err2 := watcher.Add(tempDir)
	if err2 != nil {
		t.Fatalf("Second Add failed: %v", err2)
	}

	err3 := watcher.Add(tempDir)
	if err3 != nil {
		t.Fatalf("Third Add failed: %v", err3)
	}

	// All should succeed - fsnotify handles deduplication internally
	// This test verifies the fsnotify behavior we're relying on
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

	// Create a real fsnotify watcher for testing
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("Failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	gw := &global{
		watcher:      watcher,
		watchers:     csync.NewMap[string, *Client](),
		debounceTime: 300 * time.Millisecond,
		debounceMap:  csync.NewMap[string, *time.Timer](),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Watch the workspace
	err = gw.addDirectoryToWatcher(tempDir)
	if err != nil {
		t.Fatalf("addDirectoryToWatcher failed: %v", err)
	}

	// This test verifies that the watcher can successfully add directories to fsnotify
	// The actual ignore logic is tested in the fsext package
	// Here we just verify that the watcher integration works
}

func TestGlobalWatcherShutdown(t *testing.T) {
	t.Parallel()

	// Create a new context for this test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary global watcher for testing
	gw := &global{
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
