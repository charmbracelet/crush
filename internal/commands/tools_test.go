package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildFilteredTools_EmptyAllowedTools(t *testing.T) {
	// When allowedTools is empty, should return all tools
	result := buildFilteredTools([]string{})

	allTools := AllAvailableTools()
	assert.Equal(t, allTools, result)
	assert.NotEmpty(t, result) // Should have some tools
}

func TestBuildFilteredTools_SpecificTools(t *testing.T) {
	allTools := AllAvailableTools()
	if len(allTools) == 0 {
		t.Skip("No tools available")
	}

	// Filter to first two tools
	allowed := []string{allTools[0]}
	if len(allTools) > 1 {
		allowed = append(allowed, allTools[1])
	}

	result := buildFilteredTools(allowed)

	assert.Len(t, result, len(allowed))
	for _, tool := range allowed {
		assert.Contains(t, result, tool)
	}
}

func TestBuildFilteredTools_AllTools(t *testing.T) {
	allTools := AllAvailableTools()
	if len(allTools) == 0 {
		t.Skip("No tools available")
	}

	// Allow all tools
	result := buildFilteredTools(allTools)

	assert.Equal(t, allTools, result)
}

func TestBuildFilteredTools_InvalidTools(t *testing.T) {
	allTools := AllAvailableTools()
	if len(allTools) == 0 {
		t.Skip("No tools available")
	}

	// Include invalid tool names
	allowed := []string{allTools[0], "InvalidTool", "AnotherInvalid"}

	result := buildFilteredTools(allowed)

	// Should only include valid tools
	assert.Len(t, result, 1)
	assert.Contains(t, result, allTools[0])
	assert.NotContains(t, result, "InvalidTool")
	assert.NotContains(t, result, "AnotherInvalid")
}

func TestBuildFilteredTools_CaseSensitive(t *testing.T) {
	allTools := AllAvailableTools()
	if len(allTools) == 0 {
		t.Skip("No tools available")
	}

	// Use lowercase version of a tool name
	originalTool := allTools[0]
	lowercaseTool := ""
	for _, r := range originalTool {
		if r >= 'A' && r <= 'Z' {
			lowercaseTool += string(r + 32)
		} else {
			lowercaseTool += string(r)
		}
	}

	allowed := []string{lowercaseTool}

	result := buildFilteredTools(allowed)

	// Should be case-sensitive - lowercase version won't match
	if lowercaseTool != originalTool {
		assert.NotContains(t, result, originalTool)
	}
}

