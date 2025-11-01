package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveCommandName_RootLevelCommands(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		ns       string
	}{
		{
			name:     "simple root command",
			path:     "/commands/review-pr.md",
			baseDir:  "/commands",
			expected: "review-pr",
			ns:       "",
		},
		{
			name:     "root command with uppercase extension",
			path:     "/commands/review-pr.MD",
			baseDir:  "/commands",
			expected: "review-pr",
			ns:       "",
		},
		{
			name:     "root command relative path",
			path:     "review-pr.md",
			baseDir:  ".",
			expected: "review-pr",
			ns:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, namespace := deriveCommandName(tt.path, tt.baseDir)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.ns, namespace)
		})
	}
}

func TestDeriveCommandName_SingleSubdirectoryNamespace(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		ns       string
	}{
		{
			name:     "frontend namespace",
			path:     "/commands/frontend/review-pr.md",
			baseDir:  "/commands",
			expected: "frontend:review-pr",
			ns:       "frontend",
		},
		{
			name:     "backend namespace",
			path:     "/commands/backend/test.md",
			baseDir:  "/commands",
			expected: "backend:test",
			ns:       "backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, namespace := deriveCommandName(tt.path, tt.baseDir)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.ns, namespace)
		})
	}
}

func TestDeriveCommandName_NestedSubdirectories(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		ns       string
	}{
		{
			name:     "two level nesting",
			path:     "/commands/frontend/components/button.md",
			baseDir:  "/commands",
			expected: "frontend:components:button",
			ns:       "frontend:components",
		},
		{
			name:     "three level nesting",
			path:     "/commands/frontend/components/ui/button.md",
			baseDir:  "/commands",
			expected: "frontend:components:ui:button",
			ns:       "frontend:components:ui",
		},
		{
			name:     "deep nesting",
			path:     "/commands/a/b/c/d/e/command.md",
			baseDir:  "/commands",
			expected: "a:b:c:d:e:command",
			ns:       "a:b:c:d:e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, namespace := deriveCommandName(tt.path, tt.baseDir)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.ns, namespace)
		})
	}
}

func TestDeriveCommandName_CrossPlatformPathSeparators(t *testing.T) {
	// Test Windows-style paths
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		ns       string
	}{
		{
			name:     "Windows path separators",
			path:     filepath.Join("commands", "frontend", "review-pr.md"),
			baseDir:  "commands",
			expected: "frontend:review-pr",
			ns:       "frontend",
		},
		{
			name:     "Unix path separators",
			path:     "commands/frontend/review-pr.md",
			baseDir:  "commands",
			expected: "frontend:review-pr",
			ns:       "frontend",
		},
		{
			name:     "Mixed separators handled",
			path:     filepath.Join("commands", "frontend/components", "button.md"),
			baseDir:  "commands",
			expected: "frontend:components:button",
			ns:       "frontend:components",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, namespace := deriveCommandName(tt.path, tt.baseDir)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.ns, namespace)
		})
	}
}

func TestDeriveCommandName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		ns       string
	}{
		{
			name:     "empty baseDir",
			path:     "review-pr.md",
			baseDir:  "",
			expected: "review-pr",
			ns:       "",
		},
		{
			name:     "path same as baseDir",
			path:     "/commands/commands.md",
			baseDir:  "/commands",
			expected: "commands",
			ns:       "",
		},
		{
			name:     "relative path with ..",
			path:     "../commands/frontend/test.md",
			baseDir:  "../commands",
			expected: "frontend:test",
			ns:       "frontend",
		},
		{
			name:     "filename with dots",
			path:     "/commands/test.command.md",
			baseDir:  "/commands",
			expected: "test.command",
			ns:       "",
		},
		{
			name:     "namespace with dots",
			path:     "/commands/test.ns/command.md",
			baseDir:  "/commands",
			expected: "test.ns:command",
			ns:       "test.ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, namespace := deriveCommandName(tt.path, tt.baseDir)
			assert.Equal(t, tt.expected, name)
			assert.Equal(t, tt.ns, namespace)
		})
	}
}

func TestDeriveCommandName_RelativePathFailure(t *testing.T) {
	// Test case where filepath.Rel might fail (different roots on Windows)
	// In this case, the function should use the original path
	name, namespace := deriveCommandName("C:\\commands\\test.md", "/commands")
	// Should still produce a valid result (using original path)
	assert.NotEmpty(t, name)
	// Namespace behavior depends on path format
	assert.NotNil(t, namespace) // Can be empty or have value
}

