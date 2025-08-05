package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

func TestContextTool(t *testing.T) {
	// Create temporary directory with context files
	tmpDir := t.TempDir()
	
	// Create test context files
	claudeFile := filepath.Join(tmpDir, "CLAUDE.md")
	claudeContent := "# Claude Instructions\nYou are a helpful coding assistant.\nAlways write tests."
	if err := os.WriteFile(claudeFile, []byte(claudeContent), 0644); err != nil {
		t.Fatalf("Failed to create CLAUDE.md: %v", err)
	}
	
	cursorrules := filepath.Join(tmpDir, ".cursorrules")
	cursorContent := "Follow the single responsibility principle.\nUse meaningful variable names."
	if err := os.WriteFile(cursorrules, []byte(cursorContent), 0644); err != nil {
		t.Fatalf("Failed to create .cursorrules: %v", err)
	}
	
	// Create mock permission service
	permissions := &mockPermissionService{}
	
	// Create context tool
	tool := NewContextTool(permissions, tmpDir)
	
	t.Run("loads specified context files", func(t *testing.T) {
		input := ContextInput{
			Paths: []string{claudeFile, cursorrules},
		}
		inputJSON, _ := json.Marshal(input)
		
		call := ToolCall{
			ID:    "test-1",
			Name:  "context",
			Input: string(inputJSON),
		}
		
		response, err := tool.Run(context.Background(), call)
		if err != nil {
			t.Fatalf("Tool run failed: %v", err)
		}
		
		if response.IsError {
			t.Fatalf("Tool returned error: %s", response.Content)
		}
		
		// Check that both files' content is included
		if !strings.Contains(response.Content, claudeContent) {
			t.Error("Expected response to contain CLAUDE.md content")
		}
		if !strings.Contains(response.Content, cursorContent) {
			t.Error("Expected response to contain .cursorrules content")
		}
		
		// Check metadata
		if response.Metadata == "" {
			t.Error("Expected response metadata")
		} else {
			// Parse metadata JSON
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(response.Metadata), &metadata); err != nil {
				t.Errorf("Failed to parse metadata JSON: %v", err)
			} else {
				if pathsLoaded, ok := metadata["paths_loaded"].(float64); ok {
					if int(pathsLoaded) != 2 {
						t.Errorf("Expected 2 paths loaded, got %d", int(pathsLoaded))
					}
				} else {
					t.Error("Expected paths_loaded in metadata")
				}
			}
		}
	})
	
	t.Run("filters content by query", func(t *testing.T) {
		input := ContextInput{
			Paths: []string{claudeFile, cursorrules},
			Query: "responsibility",
		}
		inputJSON, _ := json.Marshal(input)
		
		call := ToolCall{
			ID:    "test-2",
			Name:  "context",
			Input: string(inputJSON),
		}
		
		response, err := tool.Run(context.Background(), call)
		if err != nil {
			t.Fatalf("Tool run failed: %v", err)
		}
		
		if response.IsError {
			t.Fatalf("Tool returned error: %s", response.Content)
		}
		
		// Should contain the line with "responsibility" but not all content
		if !strings.Contains(response.Content, "single responsibility principle") {
			t.Error("Expected response to contain filtered content with 'responsibility'")
		}
		
		// Should not contain content from CLAUDE.md since it doesn't match query
		if strings.Contains(response.Content, "coding assistant") {
			t.Error("Expected response to not contain unrelated content")
		}
	})
	
	t.Run("handles empty paths gracefully", func(t *testing.T) {
		input := ContextInput{
			Paths: []string{"/nonexistent/file.md"},
		}
		inputJSON, _ := json.Marshal(input)
		
		call := ToolCall{
			ID:    "test-3", 
			Name:  "context",
			Input: string(inputJSON),
		}
		
		response, err := tool.Run(context.Background(), call)
		if err != nil {
			t.Fatalf("Tool run failed: %v", err)
		}
		
		if response.IsError {
			t.Error("Expected tool to handle missing files gracefully")
		}
		
		if !strings.Contains(response.Content, "No context files found") {
			t.Error("Expected message about no context files found")
		}
	})
}

// Mock permission service for testing
type mockPermissionService struct{}

func (m *mockPermissionService) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	// Return empty channel for testing
	ch := make(chan pubsub.Event[permission.PermissionRequest])
	close(ch)
	return ch
}

func (m *mockPermissionService) GrantPersistent(permission.PermissionRequest) {
	// No-op for testing
}

func (m *mockPermissionService) Grant(permission.PermissionRequest) {
	// No-op for testing
}

func (m *mockPermissionService) Deny(permission.PermissionRequest) {
	// No-op for testing
}

func (m *mockPermissionService) Request(permission.CreatePermissionRequest) bool {
	return true // Always allow for testing
}

func (m *mockPermissionService) AutoApproveSession(string) {
	// No-op for testing
}

func (m *mockPermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	// Return empty channel for testing
	ch := make(chan pubsub.Event[permission.PermissionNotification])
	close(ch)
	return ch
}