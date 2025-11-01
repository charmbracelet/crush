package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHelpHandler(t *testing.T) {
	registry := NewRegistry(".")
	handler := NewHelpHandler(registry)

	assert.NotNil(t, handler)
	assert.Equal(t, registry, handler.registry)
}

func TestHelpHandler_GenerateHelp_EmptyRegistry(t *testing.T) {
	registry := NewRegistry(".")
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	handler := NewHelpHandler(registry)
	output := handler.GenerateHelp()

	// Help command should always be shown, even when registry is empty
	assert.Contains(t, output, "Available Commands")
	assert.Contains(t, output, "\\help")
	assert.Contains(t, output, "Show a list of all available commands and their descriptions")
}

func TestGroupCommandsByNamespace(t *testing.T) {
	commands := []Command{
		{Name: "cmd1", Namespace: ""},
		{Name: "cmd2", Namespace: ""},
		{Name: "frontend:cmd3", Namespace: "frontend"},
		{Name: "frontend:cmd4", Namespace: "frontend"},
		{Name: "backend:cmd5", Namespace: "backend"},
	}

	grouped := groupCommandsByNamespace(commands)

	// Verify root commands
	require.Contains(t, grouped, "")
	assert.Len(t, grouped[""], 2)
	assert.Equal(t, "cmd1", grouped[""][0].Name)
	assert.Equal(t, "cmd2", grouped[""][1].Name)

	// Verify frontend namespace
	require.Contains(t, grouped, "frontend")
	assert.Len(t, grouped["frontend"], 2)
	assert.Equal(t, "frontend:cmd3", grouped["frontend"][0].Name)
	assert.Equal(t, "frontend:cmd4", grouped["frontend"][1].Name)

	// Verify backend namespace
	require.Contains(t, grouped, "backend")
	assert.Len(t, grouped["backend"], 1)
	assert.Equal(t, "backend:cmd5", grouped["backend"][0].Name)
}

func TestGroupCommandsByNamespace_Sorted(t *testing.T) {
	commands := []Command{
		{Name: "z-cmd", Namespace: ""},
		{Name: "a-cmd", Namespace: ""},
		{Name: "m-cmd", Namespace: ""},
	}

	grouped := groupCommandsByNamespace(commands)

	rootCmds := grouped[""]
	assert.Len(t, rootCmds, 3)
	// Verify sorted order
	assert.Equal(t, "a-cmd", rootCmds[0].Name)
	assert.Equal(t, "m-cmd", rootCmds[1].Name)
	assert.Equal(t, "z-cmd", rootCmds[2].Name)
}

func TestHelpHandler_GenerateHelp_WithCommands(t *testing.T) {
	// Create a mock registry with test commands
	mockRegistry := &mockRegistryForHelp{
		commands: []Command{
			{
				Name:         "help",
				Description:  "Show help",
				ArgumentHint: "",
				Source:       "project",
			},
			{
				Name:         "frontend:review-pr",
				Namespace:    "frontend",
				Description:  "Review PR",
				ArgumentHint: "[pr-number]",
				Source:       "project:frontend",
			},
			{
				Name:         "backend:deploy",
				Namespace:    "backend",
				Description:  "Deploy backend",
				ArgumentHint: "",
				Source:       "user",
			},
		},
	}

	handler := NewHelpHandler(mockRegistry)
	output := handler.GenerateHelp()

	// Verify header
	assert.Contains(t, output, "Available Commands:")

	// Verify root command
	assert.Contains(t, output, "\\help")
	assert.Contains(t, output, "Show help")
	assert.Contains(t, output, "(project)")

	// Verify namespaced commands
	assert.Contains(t, output, "frontend:")
	assert.Contains(t, output, "\\frontend:review-pr")
	assert.Contains(t, output, "[pr-number]")
	assert.Contains(t, output, "(project:frontend)")

	assert.Contains(t, output, "backend:")
	assert.Contains(t, output, "\\backend:deploy")
	assert.Contains(t, output, "(user)")
}

func TestHelpHandler_FormatCommand(t *testing.T) {
	handler := &HelpHandler{}

	var output strings.Builder
	cmd := Command{
		Name:         "test-cmd",
		Description:  "Test command",
		ArgumentHint: "[arg1] [arg2]",
		Source:       "project",
	}

	handler.formatCommand(&output, cmd)
	result := output.String()

	assert.Contains(t, result, "\\test-cmd")
	assert.Contains(t, result, "Test command")
	assert.Contains(t, result, "[arg1] [arg2]")
	assert.Contains(t, result, "(project)")
}

func TestHelpHandler_FormatCommand_NoDescription(t *testing.T) {
	handler := &HelpHandler{}

	var output strings.Builder
	cmd := Command{
		Name:  "test-cmd",
		Source: "user",
	}

	handler.formatCommand(&output, cmd)
	result := output.String()

	assert.Contains(t, result, "\\test-cmd")
	assert.NotContains(t, result, " - ")
	assert.Contains(t, result, "(user)")
}

// mockRegistryForHelp is a simple mock for testing help handler
type mockRegistryForHelp struct {
	commands []Command
}

func (m *mockRegistryForHelp) LoadCommands() ([]Command, error) {
	return m.commands, nil
}

func (m *mockRegistryForHelp) FindCommand(name string) (*Command, error) {
	for i := range m.commands {
		if m.commands[i].Name == name {
			return &m.commands[i], nil
		}
	}
	return nil, fmt.Errorf("command not found: %s", name)
}

func (m *mockRegistryForHelp) ListCommands() []Command {
	return m.commands
}

func (m *mockRegistryForHelp) Reload() error {
	return nil
}

