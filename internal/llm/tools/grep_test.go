package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRegexCache(t *testing.T) {
	cache := newRegexCache()

	// Test basic caching
	pattern := "test.*pattern"
	regex1, err := cache.get(pattern)
	if err != nil {
		t.Fatalf("Failed to compile regex: %v", err)
	}

	regex2, err := cache.get(pattern)
	if err != nil {
		t.Fatalf("Failed to get cached regex: %v", err)
	}

	// Should be the same instance (cached)
	if regex1 != regex2 {
		t.Error("Expected cached regex to be the same instance")
	}

	// Test that it actually works
	if !regex1.MatchString("test123pattern") {
		t.Error("Regex should match test string")
	}
}

func TestGlobToRegexCaching(t *testing.T) {
	// Test that globToRegex uses pre-compiled regex
	pattern1 := globToRegex("*.{js,ts}")

	// Should not panic and should work correctly
	regex1, err := regexp.Compile(pattern1)
	if err != nil {
		t.Fatalf("Failed to compile glob regex: %v", err)
	}

	if !regex1.MatchString("test.js") {
		t.Error("Glob regex should match .js files")
	}
	if !regex1.MatchString("test.ts") {
		t.Error("Glob regex should match .ts files")
	}
	if regex1.MatchString("test.go") {
		t.Error("Glob regex should not match .go files")
	}
}

func TestGrepWithIgnoreFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":           "hello world",
		"file2.txt":           "hello world",
		"ignored/file3.txt":   "hello world",
		"node_modules/lib.js": "hello world",
		"secret.key":          "hello world",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Create .gitignore file
	gitignoreContent := "ignored/\n*.key\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Create .crushignore file
	crushignoreContent := "node_modules/\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".crushignore"), []byte(crushignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to write .crushignore: %v", err)
	}

	// Create grep tool
	grepTool := NewGrepTool(tempDir)

	// Create grep parameters
	params := GrepParams{
		Pattern: "hello world",
		Path:    tempDir,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	// Run grep
	call := ToolCall{Input: string(paramsJSON)}
	response, err := grepTool.Run(context.Background(), call)
	if err != nil {
		t.Fatalf("Grep failed: %v", err)
	}

	// Check results - should only find file1.txt and file2.txt
	// ignored/file3.txt should be ignored by .gitignore
	// node_modules/lib.js should be ignored by .crushignore
	// secret.key should be ignored by .gitignore
	result := response.Content
	if !strings.Contains(result, "file1.txt") {
		t.Error("Expected to find file1.txt in results")
	}
	if !strings.Contains(result, "file2.txt") {
		t.Error("Expected to find file2.txt in results")
	}
	if strings.Contains(result, "file3.txt") {
		t.Error("Expected file3.txt to be ignored by .gitignore")
	}
	if strings.Contains(result, "lib.js") {
		t.Error("Expected lib.js to be ignored by .crushignore")
	}
	if strings.Contains(result, "secret.key") {
		t.Error("Expected secret.key to be ignored by .gitignore")
	}
}

// Benchmark to show performance improvement
func BenchmarkRegexCacheVsCompile(b *testing.B) {
	cache := newRegexCache()
	pattern := "test.*pattern.*[0-9]+"

	b.Run("WithCache", func(b *testing.B) {
		for b.Loop() {
			_, err := cache.get(pattern)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutCache", func(b *testing.B) {
		for b.Loop() {
			_, err := regexp.Compile(pattern)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
