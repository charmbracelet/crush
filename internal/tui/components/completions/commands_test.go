package completions

import (
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandToCompletionItem_FilterValue(t *testing.T) {
	cmd := commands.Command{
		Name:        "frontend:components:button",
		Namespace:   "frontend:components",
		Description: "Button component command",
	}

	item := commandToCompletionItem(cmd)

	// FilterValue should return the text, which starts with the command name
	filterValue := item.FilterValue()
	assert.Equal(t, "frontend:components:button - Button component command", filterValue)

	// Verify the text starts with the command name for fuzzy matching
	assert.Contains(t, filterValue, cmd.Name)
}

func TestCommandToCompletionItem_DisplayText(t *testing.T) {
	tests := []struct {
		name        string
		cmd         commands.Command
		expectedPre string // Expected prefix of display text
	}{
		{
			name: "command with description",
			cmd: commands.Command{
				Name:        "review-pr",
				Description: "Review a pull request",
			},
			expectedPre: "review-pr - Review a pull request",
		},
		{
			name: "command without description",
			cmd: commands.Command{
				Name: "simple-cmd",
			},
			expectedPre: "simple-cmd",
		},
		{
			name: "namespaced command",
			cmd: commands.Command{
				Name:        "frontend:review-pr",
				Namespace:   "frontend",
				Description: "Frontend PR review",
			},
			expectedPre: "frontend:review-pr - Frontend PR review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := commandToCompletionItem(tt.cmd)
			text := item.Text()
			assert.Equal(t, tt.expectedPre, text)
		})
	}
}

func TestLoadCommandCompletions_EmptyRegistry(t *testing.T) {
	// Create a mock registry that returns empty list
	mockRegistry := &mockRegistry{
		commands: []commands.Command{},
	}

	provider := NewCommandCompletionProvider(mockRegistry)
	items := provider.loadCommandCompletions()

	assert.Empty(t, items, "Should return empty slice for empty registry")
}

func TestLoadCommandCompletions_WithCommands(t *testing.T) {
	allCommands := []commands.Command{
		{
			Name:        "cmd1",
			Description: "First command",
		},
		{
			Name:        "namespace:cmd2",
			Namespace:   "namespace",
			Description: "Second command",
		},
	}

	mockRegistry := &mockRegistry{
		commands: allCommands,
	}

	provider := NewCommandCompletionProvider(mockRegistry)
	items := provider.loadCommandCompletions()

	require.Len(t, items, 2, "Should return 2 completion items")

	// Verify first item
	item1 := items[0]
	assert.Equal(t, "cmd1 - First command", item1.Text())
	cmd1 := item1.Value()
	assert.Equal(t, "cmd1", cmd1.Name)

	// Verify second item
	item2 := items[1]
	assert.Equal(t, "namespace:cmd2 - Second command", item2.Text())
	cmd2 := item2.Value()
	assert.Equal(t, "namespace:cmd2", cmd2.Name)
}

// mockRegistry is a simple mock implementation of commands.Registry for testing
type mockRegistry struct {
	commands []commands.Command
}

func (m *mockRegistry) LoadCommands() ([]commands.Command, error) {
	return m.commands, nil
}

func (m *mockRegistry) FindCommand(name string) (*commands.Command, error) {
	for i := range m.commands {
		if m.commands[i].Name == name {
			return &m.commands[i], nil
		}
	}
	return nil, assert.AnError
}

func (m *mockRegistry) ListCommands() []commands.Command {
	return m.commands
}

func (m *mockRegistry) Reload() error {
	return nil
}

// Test fuzzy matching scenarios
func TestCommandToCompletionItem_FuzzyMatching(t *testing.T) {
	cmd := commands.Command{
		Name:        "frontend:components:button",
		Namespace:   "frontend:components",
		Description: "Button component command",
	}

	item := commandToCompletionItem(cmd)
	filterValue := item.FilterValue()

	// Test that "cbut" matches "frontend:components:button"
	// The fuzzy matcher looks for "cbut" within "frontend:components:button - Button component command"
	matches := fuzzy.Find("cbut", []string{filterValue})
	assert.NotEmpty(t, matches, "\"cbut\" should match \"frontend:components:button\"")
	if len(matches) > 0 {
		assert.Equal(t, 0, matches[0].Index, "Match should be at index 0")
	}

	// Test that "fcombut" matches "frontend:components:button"
	matches = fuzzy.Find("fcombut", []string{filterValue})
	assert.NotEmpty(t, matches, "\"fcombut\" should match \"frontend:components:button\"")
	if len(matches) > 0 {
		assert.Equal(t, 0, matches[0].Index, "Match should be at index 0")
	}

	// Test that full command name matches
	matches = fuzzy.Find("frontend:components:button", []string{filterValue})
	assert.NotEmpty(t, matches, "Full command name should match")
	if len(matches) > 0 {
		assert.Equal(t, 0, matches[0].Index, "Match should be at index 0")
	}

	// Test that non-matching query doesn't match
	matches = fuzzy.Find("xyzabc", []string{filterValue})
	assert.Empty(t, matches, "Non-matching query should not match")
}

