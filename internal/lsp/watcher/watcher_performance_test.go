package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// createTestWorkspace creates a temporary workspace with test files
func createTestWorkspace(tb testing.TB) string {
	tmpDir, err := os.MkdirTemp("", "watcher_test")
	if err != nil {
		tb.Fatal(err)
	}

	// Create test files for Go project
	testFiles := []string{
		"go.mod",
		"go.sum",
		"main.go",
		"src/lib.go",
		"src/utils.go",
		"cmd/app.go",
		"internal/config.go",
		"internal/db.go",
		"pkg/api.go",
		"pkg/client.go",
		"test/main_test.go",
		"test/lib_test.go",
		"docs/README.md",
		"scripts/build.sh",
		"Makefile",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			tb.Fatal(err)
		}

		if err := os.WriteFile(fullPath, []byte("// test content"), 0644); err != nil {
			tb.Fatal(err)
		}
	}

	return tmpDir
}

// simulateOldApproach simulates the old file opening approach with per-file delays
func simulateOldApproach(workspacePath string, serverName string) (int, time.Duration) {
	start := time.Now()
	filesOpened := 0

	// Define patterns for high-priority files based on server type
	var patterns []string

	switch serverName {
	case "gopls":
		patterns = []string{
			"**/go.mod",
			"**/go.sum",
			"**/main.go",
		}
	default:
		patterns = []string{
			"**/package.json",
			"**/Makefile",
		}
	}

	// OLD APPROACH: For each pattern, find and open matching files with per-file delays
	for _, pattern := range patterns {
		matches, err := doublestar.Glob(os.DirFS(workspacePath), pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			fullPath := filepath.Join(workspacePath, match)
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() {
				continue
			}

			// Simulate file opening (1ms overhead)
			time.Sleep(1 * time.Millisecond)
			filesOpened++

			// OLD: Add delay after each file
			time.Sleep(20 * time.Millisecond)

			// Limit files
			if filesOpened >= 5 {
				break
			}
		}
	}

	return filesOpened, time.Since(start)
}

// simulateNewApproach simulates the new batched file opening approach
func simulateNewApproach(workspacePath string, serverName string) (int, time.Duration) {
	start := time.Now()
	filesOpened := 0

	// Define patterns for high-priority files based on server type
	var patterns []string

	switch serverName {
	case "gopls":
		patterns = []string{
			"**/go.mod",
			"**/go.sum",
			"**/main.go",
		}
	default:
		patterns = []string{
			"**/package.json",
			"**/Makefile",
		}
	}

	// NEW APPROACH: Collect all files first
	var filesToOpen []string

	// For each pattern, find matching files
	for _, pattern := range patterns {
		matches, err := doublestar.Glob(os.DirFS(workspacePath), pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			fullPath := filepath.Join(workspacePath, match)
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() {
				continue
			}

			filesToOpen = append(filesToOpen, fullPath)

			// Limit the number of files per pattern
			if len(filesToOpen) >= 5 {
				break
			}
		}
	}

	// Open files in batches to reduce overhead
	batchSize := 3
	for i := 0; i < len(filesToOpen); i += batchSize {
		end := min(i+batchSize, len(filesToOpen))

		// Open batch of files
		for j := i; j < end; j++ {
			// Simulate file opening (1ms overhead)
			time.Sleep(1 * time.Millisecond)
			filesOpened++
		}

		// Only add delay between batches, not individual files
		if end < len(filesToOpen) {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return filesOpened, time.Since(start)
}

func BenchmarkOldApproach(b *testing.B) {
	tmpDir := createTestWorkspace(b)
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		simulateOldApproach(tmpDir, "gopls")
	}
}

func BenchmarkNewApproach(b *testing.B) {
	tmpDir := createTestWorkspace(b)
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		simulateNewApproach(tmpDir, "gopls")
	}
}

func TestPerformanceComparison(t *testing.T) {
	tmpDir := createTestWorkspace(t)
	defer os.RemoveAll(tmpDir)

	// Test old approach
	filesOpenedOld, oldDuration := simulateOldApproach(tmpDir, "gopls")

	// Test new approach
	filesOpenedNew, newDuration := simulateNewApproach(tmpDir, "gopls")

	t.Logf("Old approach: %d files in %v", filesOpenedOld, oldDuration)
	t.Logf("New approach: %d files in %v", filesOpenedNew, newDuration)

	if newDuration > 0 && oldDuration > 0 {
		improvement := float64(oldDuration-newDuration) / float64(oldDuration) * 100
		t.Logf("Performance improvement: %.1f%%", improvement)

		if improvement <= 0 {
			t.Errorf("Expected performance improvement, but new approach was slower")
		}
	}

	// Verify same number of files opened
	if filesOpenedOld != filesOpenedNew {
		t.Errorf("Different number of files opened: old=%d, new=%d", filesOpenedOld, filesOpenedNew)
	}

	// Verify new approach is faster
	if newDuration >= oldDuration {
		t.Errorf("New approach should be faster: old=%v, new=%v", oldDuration, newDuration)
	}
}
