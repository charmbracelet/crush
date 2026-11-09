package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

// TestMCPToolInfoDescribesEveryParameter covers MCP servers that publish a
// bare inputSchema. Without the fallback those properties reach the provider
// undescribed and the whole request is rejected.
func TestMCPToolInfoDescribesEveryParameter(t *testing.T) {
	t.Parallel()

	tool := &Tool{
		mcpName: "demo",
		tool: &mcp.Tool{
			Name:        "create_issue",
			Description: "Create an issue",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"title":  map[string]any{"type": "string"},
					"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"body":   map[string]any{"type": "string", "description": "Issue body"},
				},
				"required": []any{"title"},
			},
		},
	}

	info := tool.Info()

	require.Equal(t, []string{"title"}, info.Required)

	title, ok := info.Parameters["title"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "title", title["description"])

	body, ok := info.Parameters["body"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Issue body", body["description"])

	// Existing schema keywords must not be dropped when a description is added.
	labels, ok := info.Parameters["labels"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "labels", labels["description"])
	require.Equal(t, "array", labels["type"])
	items, ok := labels["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "string", items["type"])
	require.Equal(t, "labels item", items["description"])
}

// TestMCPToolInfoDereferencesSchema ensures that $defs/$ref references are
// inlined so the schema survives the properties/required round trip.
func TestMCPToolInfoDereferencesSchema(t *testing.T) {
	t.Parallel()

	// Test case 1: $defs + $ref (the set_workflow_slot shape)
	tool := &Tool{
		mcpName: "comfy",
		tool: &mcp.Tool{
			Name:        "set_workflow_slot",
			Description: "Set a workflow slot",
			InputSchema: map[string]any{
				"$defs": map[string]any{
					"SlotOverride": map[string]any{
						"type": "object",
						"title": "SlotOverride",
						"properties": map[string]any{
							"address": map[string]any{"type": "string"},
							"value":   map[string]any{"type": "string"},
						},
						"required": []any{"address", "value"},
					},
				},
				"type": "object",
				"properties": map[string]any{
					"workflow_path": map[string]any{"type": "string"},
					"overrides": map[string]any{
						"type": "array",
						"items": map[string]any{
							"anyOf": []any{
								map[string]any{"type": "string"},
								map[string]any{"$ref": "#/$defs/SlotOverride"},
							},
						},
					},
					"stdout": map[string]any{"type": "boolean"},
				},
				"required": []any{"workflow_path", "overrides"},
			},
		},
	}

	info := tool.Info()

	// The overrides parameter should contain inlined objects, not $ref strings
	overrides, ok := info.Parameters["overrides"].(map[string]any)
	require.True(t, ok)
	items, ok := overrides["items"].(map[string]any)
	require.True(t, ok)
	anyOf, ok := items["anyOf"].([]any)
	require.True(t, ok)

	// The second item should be an inlined object, not a $ref string
	require.NotEqual(t, map[string]any{"$ref": "#/$defs/SlotOverride"}, anyOf[1])
	require.Contains(t, anyOf[1], "type") // should have type field from inlined schema

	// No $ref strings should remain in the parameters
	require.NotContains(t, info.Parameters, "$ref")

	// Test case 2: no $defs (returns input unchanged)
	tool2 := &Tool{
		mcpName: "simple",
		tool: &mcp.Tool{
			Name:        "simple_tool",
			Description: "A simple tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
		},
	}
	info2 := tool2.Info()
	require.NotContains(t, info2.Parameters, "$defs")
	require.NotContains(t, info2.Parameters, "$ref")

	// Test case 3: self-referential $ref (no hang, no panic)
	tool3 := &Tool{
		mcpName: "recursive",
		tool: &mcp.Tool{
			Name:        "recursive_tool",
			Description: "A tool with self-referential schema",
			InputSchema: map[string]any{
				"$defs": map[string]any{
					"Node": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"children": map[string]any{
								"type": "array",
								"items": map[string]any{"$ref": "#/$defs/Node"},
							},
						},
					},
				},
				"type": "object",
				"properties": map[string]any{
					"root": map[string]any{"$ref": "#/$defs/Node"},
				},
				"required": []any{"root"},
			},
		},
	}
	// This should not hang or panic - the depth limit cuts cycles
	info3 := tool3.Info()
	require.NotContains(t, info3.Parameters, "$ref")
}
