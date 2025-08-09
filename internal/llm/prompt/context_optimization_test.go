package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
)

func TestCoderPromptContextOptimization(t *testing.T) {
	// Initialize config for testing
	setupTestConfig(t)
	
	// Create temporary directory with large context files to simulate real usage
	tmpDir := t.TempDir()
	
	// Create several large context files that would cause token waste
	largeFile1 := filepath.Join(tmpDir, "CLAUDE.md")
	largeFile2 := filepath.Join(tmpDir, "project_docs.md")
	
	// Simulate large context files (this represents the actual problem)
	largeContent1 := strings.Repeat("This is important project context. ", 1000) // ~34KB
	largeContent2 := strings.Repeat("More detailed project information. ", 1000)  // ~34KB
	
	if err := os.WriteFile(largeFile1, []byte(largeContent1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(largeFile2, []byte(largeContent2), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Test old behavior when explicitly enabled (this demonstrates the problem)
	t.Run("old behavior includes all context when enabled", func(t *testing.T) {
		// Enable the old behavior via environment variable
		t.Setenv("CRUSH_INCLUDE_CONTEXT", "true")
		
		contextPaths := []string{largeFile1, largeFile2}
		result := CoderPrompt("anthropic", contextPaths...)
		
		// When enabled, the implementation includes ALL context files in EVERY request
		if !strings.Contains(result, largeContent1) {
			t.Error("Expected prompt to contain first context file content when enabled")
		}
		if !strings.Contains(result, largeContent2) {
			t.Error("Expected prompt to contain second context file content when enabled")
		}
		
		// This demonstrates the problem: prompt is massive due to including all context
		promptSize := len(result)
		t.Logf("Prompt size with context enabled: %d bytes", promptSize)
		
		// This represents a significant token waste - context files can be huge
		if promptSize < 50000 { // Less than 50KB suggests the test setup might be wrong
			t.Errorf("Expected large prompt size to demonstrate the problem, got %d bytes", promptSize)
		}
	})
	
	t.Run("optimized behavior should be more selective", func(t *testing.T) {
		// After implementing the fix, context should not be included by default
		contextPaths := []string{largeFile1, largeFile2}
		
		// By default, context should NOT be included (optimization)
		result := CoderPrompt("anthropic", contextPaths...)
		basePromptWithoutContext := CoderPrompt("anthropic") // No context paths
		
		contextOverhead := len(result) - len(basePromptWithoutContext)
		t.Logf("Context overhead: %d bytes", contextOverhead)
		
		// After optimization, overhead should be minimal (just the context tool info)
		if contextOverhead > 1000 { // More than 1KB suggests context files were included
			t.Errorf("Expected minimal context overhead after optimization, got %d bytes", contextOverhead)
		}
		
		// Verify the context tool information is mentioned
		if !strings.Contains(result, "context` tool") {
			t.Error("Expected prompt to mention context tool availability")
		}
	})
	
	t.Run("context can still be included when explicitly requested", func(t *testing.T) {
		// Set environment variable to opt-in to context inclusion
		t.Setenv("CRUSH_INCLUDE_CONTEXT", "true")
		
		contextPaths := []string{largeFile1, largeFile2}
		result := CoderPrompt("anthropic", contextPaths...)
		
		// When explicitly requested, context should be included
		if !strings.Contains(result, largeContent1) {
			t.Error("Expected prompt to contain first context file content when explicitly requested")
		}
		if !strings.Contains(result, largeContent2) {
			t.Error("Expected prompt to contain second context file content when explicitly requested")
		}
	})
}

func TestProcessContextPathsTokenUsage(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple context files of varying sizes
	files := []struct {
		name    string
		content string
	}{
		{"README.md", strings.Repeat("Documentation content. ", 500)},
		{"CLAUDE.md", strings.Repeat("AI instructions. ", 800)},
		{"API_DOCS.md", strings.Repeat("API documentation. ", 1200)},
	}
	
	var filePaths []string
	for _, file := range files {
		path := filepath.Join(tmpDir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", file.name, err)
		}
		filePaths = append(filePaths, path)
	}
	
	t.Run("all files processed by default", func(t *testing.T) {
		result := ProcessContextPaths("", filePaths)
		
		// Current behavior: ALL files are processed and included
		for _, file := range files {
			if !strings.Contains(result, file.content) {
				t.Errorf("Expected result to contain content from %s", file.name)
			}
		}
		
		// This demonstrates the token waste - everything gets included
		totalSize := len(result)
		t.Logf("Total context size: %d bytes", totalSize)
		
		// Significant overhead for token usage
		if totalSize < 20000 {
			t.Errorf("Expected large context size demonstrating waste, got %d bytes", totalSize)
		}
	})
}

func setupTestConfig(t *testing.T) {
	// Create a minimal config for testing
	tmpDir := t.TempDir()
	
	// Initialize config with the test directory
	_, err := config.Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}
}