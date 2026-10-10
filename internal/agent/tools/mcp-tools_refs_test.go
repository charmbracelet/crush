package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
)

func TestInfoInlinesDefsRefs(t *testing.T) {
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tasks": map[string]any{
				"type":  "array",
				"items": map[string]any{"$ref": "#/$defs/OpenTask"},
			},
			"filter": map[string]any{"$ref": "#/$defs/Filter"},
			"legacy": map[string]any{"$ref": "#/definitions/Legacy"},
		},
		"required": []any{"tasks"},
		"$defs": map[string]any{
			"OpenTask": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string"},
					"tags": map[string]any{
						"type":  "array",
						"items": map[string]any{"$ref": "#/$defs/Tag"},
					},
				},
			},
			"Tag":    map[string]any{"type": "string"},
			"Filter": map[string]any{"type": "string"},
		},
		"definitions": map[string]any{
			"Legacy": map[string]any{"type": "integer"},
		},
	}

	tool := &Tool{
		mcpName: "test",
		tool:    &mcp.Tool{Name: "batch_add_tasks", InputSchema: inputSchema},
	}
	info := tool.Info()

	tasks, ok := info.Parameters["tasks"].(map[string]any)
	if !ok {
		t.Fatalf("tasks property missing or wrong type: %#v", info.Parameters["tasks"])
	}
	items, ok := tasks["items"].(map[string]any)
	if !ok {
		t.Fatalf("tasks.items missing or wrong type: %#v", tasks["items"])
	}
	if _, isRef := items["$ref"]; isRef {
		t.Fatalf("tasks.items still contains an unresolved $ref: %#v", items)
	}
	if items["type"] != "object" {
		t.Fatalf("tasks.items should be the inlined OpenTask object, got %#v", items)
	}
	props, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("inlined OpenTask missing properties: %#v", items)
	}
	tagArray, ok := props["tags"].(map[string]any)
	if !ok {
		t.Fatalf("inlined OpenTask missing tags: %#v", props)
	}
	if _, isRef := tagArray["items"].(map[string]any)["$ref"]; isRef {
		t.Fatalf("nested $ref inside inlined def was not resolved: %#v", tagArray["items"])
	}

	if filter, ok := info.Parameters["filter"].(map[string]any); !ok || filter["type"] != "string" {
		t.Fatalf("$defs reference not inlined: %#v", info.Parameters["filter"])
	}
	if legacy, ok := info.Parameters["legacy"].(map[string]any); !ok || legacy["type"] != "integer" {
		t.Fatalf("legacy definitions reference not inlined: %#v", info.Parameters["legacy"])
	}

	if len(info.Required) != 1 || info.Required[0] != "tasks" {
		t.Fatalf("required mismatch: %#v", info.Required)
	}

	// The shared original schema must not be mutated.
	origItems := inputSchema["properties"].(map[string]any)["tasks"].(map[string]any)["items"].(map[string]any)
	if _, hasRef := origItems["$ref"]; !hasRef {
		t.Fatalf("original cached schema was mutated: %#v", origItems)
	}
}

func TestResolveRefsHandlesCycles(t *testing.T) {
	defs := map[string]any{
		"Node": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"child": map[string]any{"$ref": "#/$defs/Node"},
			},
		},
	}
	node := map[string]any{"$ref": "#/$defs/Node"}
	_ = resolveRefs(node, defs, 0) // must terminate
}

func TestResolveRefsLeavesExternalRefsAlone(t *testing.T) {
	defs := map[string]any{"Known": map[string]any{"type": "string"}}
	node := map[string]any{
		"external": map[string]any{"$ref": "https://example.com/schema.json"},
		"missing":  map[string]any{"$ref": "#/$defs/Unknown"},
		"described": map[string]any{
			"$ref":        "#/$defs/Known",
			"description": "kept alongside ref",
		},
	}
	result := resolveRefs(node, defs, 0)
	if ext, ok := result["external"].(map[string]any); !ok || ext["$ref"] != "https://example.com/schema.json" {
		t.Fatalf("external ref should be untouched: %#v", result["external"])
	}
	if miss, ok := result["missing"].(map[string]any); !ok || miss["$ref"] != "#/$defs/Unknown" {
		t.Fatalf("unresolvable local ref should be left as-is: %#v", result["missing"])
	}
	// A ref with sibling keywords is not a pure reference; leave it untouched.
	if d, ok := result["described"].(map[string]any); !ok || d["$ref"] != "#/$defs/Known" {
		t.Fatalf("ref with siblings should be untouched: %#v", result["described"])
	}
}
