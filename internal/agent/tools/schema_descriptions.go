package tools

import (
	"context"
	"reflect"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/schema"
)

// NewAgentTool creates a typed tool from a function, guaranteeing a
// description on every parameter of the schema sent to the provider.
//
// It wraps fantasy.NewAgentTool because the schema generator only applies the
// description tag to a field's own schema: array element schemas go out bare,
// and so do fields without a tag. Strict OpenAI-compatible providers (DeepSeek,
// Ollama, TokenRouter, Nebius) reject the whole request with
// "Missing required parameter: description" when they see one, which fails
// every turn before a tool can run.
func NewAgentTool[T any](
	name string,
	description string,
	fn func(ctx context.Context, input T, call fantasy.ToolCall) (fantasy.ToolResponse, error),
) fantasy.AgentTool {
	return describeTool(fantasy.NewAgentTool(name, description, fn), reflect.TypeFor[T]())
}

// NewParallelAgentTool is NewAgentTool for tools that may run in parallel with
// other tools.
func NewParallelAgentTool[T any](
	name string,
	description string,
	fn func(ctx context.Context, input T, call fantasy.ToolCall) (fantasy.ToolResponse, error),
) fantasy.AgentTool {
	return describeTool(fantasy.NewParallelAgentTool(name, description, fn), reflect.TypeFor[T]())
}

// DescribeTools returns the tools with a description on every parameter they
// advertise.
//
// Tools built through NewAgentTool already describe themselves from their
// struct tags. This covers the ones Crush does not build from a parameter
// struct, such as MCP tools and the agent delegation tools.
func DescribeTools(agentTools []fantasy.AgentTool) []fantasy.AgentTool {
	described := make([]fantasy.AgentTool, 0, len(agentTools))
	for _, tool := range agentTools {
		// Only wrap tools that haven't been described yet (don't double-wrap)
		// describedTool.Info() will use parametersFor if params != nil, else describeParameters
		// If we already have a describedTool, skip it to avoid double-wrapping
		if _, ok := tool.(*describedTool); ok {
			described = append(described, tool)
			continue
		}
		described = append(described, describeTool(tool, nil))
	}
	return described
}

// describeTool wraps a tool so that its Info reports described parameters.
// params is the tool's parameter type, or nil when it is not known.
func describeTool(tool fantasy.AgentTool, params reflect.Type) fantasy.AgentTool {
	return &describedTool{AgentTool: tool, params: params}
}

// describedTool decorates a tool with parameter descriptions.
type describedTool struct {
	fantasy.AgentTool
	params reflect.Type
}

// Info returns the wrapped tool's info with every parameter described.
func (t *describedTool) Info() fantasy.ToolInfo {
	info := t.AgentTool.Info()
	if t.params != nil {
		info.Parameters = parametersFor(t.params)
		return info
	}
	info.Parameters = describeParameters(info.Parameters)
	return info
}

// parametersFor generates the parameters for a tool input struct, describing
// every property and array item.
func parametersFor(t reflect.Type) map[string]any {
	s := schema.Generate(t)
	describeSchema(t, &s)
	return schema.ToParameters(s)
}

// describeSchema fills in a description on every property and array item of s,
// recursing into nested structs. The description and items_description tags
// win; a field without one is described by its name, which is what upstream
// fantasy does once it falls back to the field name.
func describeSchema(t reflect.Type, s *schema.Schema) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || s == nil {
		return
	}

	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}

		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			name = toSnakeCaseField(field.Name)
		}

		prop, ok := s.Properties[name]
		if !ok || prop == nil {
			continue
		}

		if prop.Description == "" {
			prop.Description = field.Name
		}

		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		switch fieldType.Kind() {
		case reflect.Slice, reflect.Array:
			if prop.Items == nil {
				continue
			}
			switch desc := field.Tag.Get("items_description"); {
			case desc != "":
				prop.Items.Description = desc
			case prop.Items.Description == "":
				prop.Items.Description = field.Name + " item"
			}
			describeSchema(fieldType.Elem(), prop.Items)
		case reflect.Struct:
			describeSchema(fieldType, prop)
		}
	}
}

// describeParameters returns the parameters with a description on every
// property and array item, falling back to the property name.
func describeParameters(parameters map[string]any) map[string]any {
	described := make(map[string]any, len(parameters))
	for name, raw := range parameters {
		prop, ok := raw.(map[string]any)
		if !ok {
			described[name] = raw
			continue
		}
		described[name] = describeProperty(name, prop)
	}
	return described
}

// describeProperty copies a property schema, adding the descriptions it lacks.
// Other schema keywords are preserved untouched.
func describeProperty(name string, prop map[string]any) map[string]any {
	described := make(map[string]any, len(prop)+1)
	for k, v := range prop {
		described[k] = v
	}

	if desc, ok := described["description"].(string); !ok || desc == "" {
		described["description"] = name
	}

	if nested, ok := described["properties"].(map[string]any); ok {
		described["properties"] = describeParameters(nested)
	}

	if items, ok := described["items"].(map[string]any); ok {
		described["items"] = describeProperty(name+" item", items)
	}

	return described
}

// toSnakeCaseField mirrors the generator's naming for untagged fields so
// lookups line up with the property names it produces.
func toSnakeCaseField(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return string(result.String())
}
