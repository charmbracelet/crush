package completions

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_CommandCompletionsFlow(t *testing.T) {
	// Create temporary directory for test commands
	tmpDir := t.TempDir()
	projectDir := tmpDir

	// Setup project commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create test commands
	testCommands := []struct {
		path        string
		content     string
		expectedName string
	}{
		{
			path:        "help.md",
			content:     "---\ndescription: Show help\n---\n# Help\n",
			expectedName: "help",
		},
		{
			path:        "frontend/review-pr.md",
			content:     "---\ndescription: Review frontend PR\n---\n# Review PR\n",
			expectedName: "frontend:review-pr",
		},
		{
			path:        "frontend/components/button.md",
			content:     "---\ndescription: Button component\n---\n# Button\n",
			expectedName: "frontend:components:button",
		},
	}

	for _, tc := range testCommands {
		cmdPath := filepath.Join(commandsDir, tc.path)
		cmdDir := filepath.Dir(cmdPath)
		require.NoError(t, os.MkdirAll(cmdDir, 0o755))
		require.NoError(t, os.WriteFile(cmdPath, []byte(tc.content), 0o644))
	}

	// Create registry and load commands
	registry := commands.NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create completion provider
	provider := NewCommandCompletionProvider(registry)

	// Test: Load completions
	items := provider.loadCommandCompletions()
	require.Len(t, items, len(testCommands), "Should load all test commands")

	// Test: Verify command names
	loadedNames := make(map[string]bool)
	for _, item := range items {
		cmd := item.Value()
		loadedNames[cmd.Name] = true
		assert.NotEmpty(t, item.Text(), "Completion item should have display text")
		assert.NotEmpty(t, item.FilterValue(), "Completion item should have filter value")
	}

	for _, tc := range testCommands {
		assert.True(t, loadedNames[tc.expectedName],
			"Command %s should be loaded", tc.expectedName)
	}

	// Test: Verify filtering works
	// This simulates typing "\hel" which should match "help"
	filteredItems := filterCompletions(items, "hel")
	assert.NotEmpty(t, filteredItems, "Filtering 'hel' should match 'help'")
	assert.Equal(t, "help", filteredItems[0].Value().Name, "Filtered result should be 'help'")

	// Test: Verify fuzzy matching across namespace
	// Typing "\cbut" should match "frontend:components:button"
	filteredItems = filterCompletions(items, "cbut")
	assert.NotEmpty(t, filteredItems, "Filtering 'cbut' should match 'frontend:components:button'")
	found := false
	for _, item := range filteredItems {
		if item.Value().Name == "frontend:components:button" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find 'frontend:components:button' when filtering 'cbut'")

	// Test: Verify namespace filtering
	filteredItems = filterCompletions(items, "frontend:")
	assert.GreaterOrEqual(t, len(filteredItems), 2,
		"Filtering 'frontend:' should match at least 2 commands")
	for _, item := range filteredItems {
		assert.Contains(t, item.Value().Name, "frontend:",
			"Filtered commands should have 'frontend:' namespace")
	}

	// Test: Verify empty query shows all commands
	filteredItems = filterCompletions(items, "")
	assert.Len(t, filteredItems, len(testCommands),
		"Empty query should show all commands")
}

func TestIntegration_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Test: Very long command name
	longCmdPath := filepath.Join(commandsDir, "very-long-command-name-that-might-cause-display-issues.md")
	longCmdContent := "---\ndescription: Very long command\n---\n# Long Command\n"
	require.NoError(t, os.WriteFile(longCmdPath, []byte(longCmdContent), 0o644))

	// Test: Special characters in command name
	specialCmdPath := filepath.Join(commandsDir, "test-command-with-special-chars.md")
	specialCmdContent := "---\ndescription: Command with special chars\n---\n# Special\n"
	require.NoError(t, os.WriteFile(specialCmdPath, []byte(specialCmdContent), 0o644))

	// Test: Command with no description
	noDescCmdPath := filepath.Join(commandsDir, "no-desc.md")
	noDescContent := "# No Description\n"
	require.NoError(t, os.WriteFile(noDescCmdPath, []byte(noDescContent), 0o644))

	// Create registry and load commands
	registry := commands.NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create completion provider
	provider := NewCommandCompletionProvider(registry)
	items := provider.loadCommandCompletions()

	// Verify all commands loaded (including edge cases)
	assert.GreaterOrEqual(t, len(items), 3, "Should load all commands including edge cases")

	// Test: Very long command name handled gracefully
	var longCmdFound bool
	for _, item := range items {
		cmd := item.Value()
		if cmd.Name == "very-long-command-name-that-might-cause-display-issues" {
			longCmdFound = true
			// Verify FilterValue works with long names
			filterValue := item.FilterValue()
			assert.Contains(t, filterValue, cmd.Name, "FilterValue should include command name")
			assert.NotEmpty(t, item.Text(), "Display text should not be empty")
		}
	}
	assert.True(t, longCmdFound, "Long command name should be loaded")

	// Test: Command with no description handled
	var noDescFound bool
	for _, item := range items {
		cmd := item.Value()
		if cmd.Name == "no-desc" {
			noDescFound = true
			// Text should just be the command name (no description)
			text := item.Text()
			assert.Equal(t, "no-desc", text, "Command without description should show only name")
		}
	}
	assert.True(t, noDescFound, "Command without description should be loaded")

	// Test: Empty registry handled
	emptyRegistry := commands.NewRegistry(t.TempDir())
	_, err = emptyRegistry.LoadCommands()
	require.NoError(t, err, "Loading empty registry should not error")

	emptyProvider := NewCommandCompletionProvider(emptyRegistry)
	emptyItems := emptyProvider.loadCommandCompletions()
	assert.Empty(t, emptyItems, "Empty registry should return empty completions")
}

func TestIntegration_RapidFiltering(t *testing.T) {
	// Test that rapid filtering doesn't cause crashes or race conditions
	tmpDir := t.TempDir()
	projectDir := tmpDir
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create multiple commands
	for i := 0; i < 10; i++ {
		cmdPath := filepath.Join(commandsDir, fmt.Sprintf("cmd%d.md", i))
		content := fmt.Sprintf("---\ndescription: Command %d\n---\n# Command %d\n", i, i)
		require.NoError(t, os.WriteFile(cmdPath, []byte(content), 0o644))
	}

	registry := commands.NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	provider := NewCommandCompletionProvider(registry)
	items := provider.loadCommandCompletions()

	// Simulate rapid filtering with different queries
	queries := []string{"cmd", "1", "2", "3", "cmd1", "cmd2", ""}
	for _, query := range queries {
		filtered := filterCompletions(items, query)
		// Just verify it doesn't panic and returns valid results
		assert.NotNil(t, filtered, "Filtering should not return nil")
		for _, item := range filtered {
			assert.NotNil(t, item, "Filtered items should not be nil")
			assert.NotEmpty(t, item.FilterValue(), "FilterValue should not be empty")
		}
	}
}

// filterCompletions simulates the filtering that happens in the completion system
// Uses the same fuzzy matching library as the actual implementation
func filterCompletions(items []list.CompletionItem[commands.Command], query string) []list.CompletionItem[commands.Command] {
	if query == "" {
		return items
	}

	// Extract filter values for fuzzy matching
	filterValues := make([]string, len(items))
	for i, item := range items {
		filterValues[i] = item.FilterValue()
	}

	// Use fuzzy matching (same as actual implementation)
	matches := fuzzy.Find(query, filterValues)

	var filtered []list.CompletionItem[commands.Command]
	for _, match := range matches {
		filtered = append(filtered, items[match.Index])
	}

	return filtered
}

