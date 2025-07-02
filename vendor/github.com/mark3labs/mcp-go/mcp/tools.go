package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
)

var errToolSchemaConflict = errors.New("provide either InputSchema or RawInputSchema, not both")

// ListToolsRequest is sent from the client to request a list of tools the
// server has.
type ListToolsRequest struct {
	PaginatedRequest
}

// ListToolsResult is the server's response to a tools/list request from the
// client.
type ListToolsResult struct {
	PaginatedResult
	Tools []Tool `json:"tools"`
}

// CallToolResult is the server's response to a tool call.
//
// Any errors that originate from the tool SHOULD be reported inside the result
// object, with `isError` set to true, _not_ as an MCP protocol-level error
// response. Otherwise, the LLM would not be able to see that an error occurred
// and self-correct.
//
// However, any errors in _finding_ the tool, an error indicating that the
// server does not support tool calls, or any other exceptional conditions,
// should be reported as an MCP error response.
type CallToolResult struct {
	Result
	Content []Content `json:"content"` // Can be TextContent, ImageContent, or      EmbeddedResource
	// Whether the tool call ended in an error.
	//
	// If not set, this is assumed to be false (the call was successful).
	IsError bool `json:"isError,omitempty"`
}

// CallToolRequest is used by the client to invoke a tool provided by the server.
type CallToolRequest struct {
	Request
	Params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
		Meta      *struct {
			// If specified, the caller is requesting out-of-band progress
			// notifications for this request (as represented by
			// notifications/progress). The value of this parameter is an
			// opaque token that will be attached to any subsequent
			// notifications. The receiver is not obligated to provide these
			// notifications.
			ProgressToken ProgressToken `json:"progressToken,omitempty"`
		} `json:"_meta,omitempty"`
	} `json:"params"`
}

// ToolListChangedNotification is an optional notification from the server to
// the client, informing it that the list of tools it offers has changed. This may
// be issued by servers without any previous subscription from the client.
type ToolListChangedNotification struct {
	Notification
}

// Tool represents the definition for a tool the client can call.
type Tool struct {
	// The name of the tool.
	Name string `json:"name"`
	// A human-readable description of the tool.
	Description string `json:"description,omitempty"`
	// A JSON Schema object defining the expected parameters for the tool.
	InputSchema ToolInputSchema `json:"inputSchema"`
	// Alternative to InputSchema - allows arbitrary JSON Schema to be provided
	RawInputSchema json.RawMessage `json:"-"` // Hide this from JSON marshaling
}

// MarshalJSON implements the json.Marshaler interface for Tool.
// It handles marshaling either InputSchema or RawInputSchema based on which is set.
func (t Tool) MarshalJSON() ([]byte, error) {
	// Create a map to build the JSON structure
	m := make(map[string]interface{}, 3)

	// Add the name and description
	m["name"] = t.Name
	if t.Description != "" {
		m["description"] = t.Description
	}

	// Determine which schema to use
	if t.RawInputSchema != nil {
		if t.InputSchema.Type != "" {
			return nil, fmt.Errorf("tool %s has both InputSchema and RawInputSchema set: %w", t.Name, errToolSchemaConflict)
		}
		m["inputSchema"] = t.RawInputSchema
	} else {
		// Use the structured InputSchema
		m["inputSchema"] = t.InputSchema
	}

	return json.Marshal(m)
}

type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// ToolOption is a function that configures a Tool.
// It provides a flexible way to set various properties of a Tool using the functional options pattern.
type ToolOption func(*Tool)

// PropertyOption is a function that configures a property in a Tool's input schema.
// It allows for flexible configuration of JSON Schema properties using the functional options pattern.
type PropertyOption func(map[string]interface{})

//
// Core Tool Functions
//

// NewTool creates a new Tool with the given name and options.
// The tool will have an object-type input schema with configurable properties.
// Options are applied in order, allowing for flexible tool configuration.
func NewTool(name string, opts ...ToolOption) Tool {
	tool := Tool{
		Name: name,
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: make(map[string]interface{}),
			Required:   nil, // Will be omitted from JSON if empty
		},
	}

	for _, opt := range opts {
		opt(&tool)
	}

	return tool
}

// NewToolWithRawSchema creates a new Tool with the given name and a raw JSON
// Schema. This allows for arbitrary JSON Schema to be used for the tool's input
// schema.
//
// NOTE a [Tool] built in such a way is incompatible with the [ToolOption] and
// runtime errors will result from supplying a [ToolOption] to a [Tool] built
// with this function.
func NewToolWithRawSchema(name, description string, schema json.RawMessage) Tool {
	tool := Tool{
		Name:           name,
		Description:    description,
		RawInputSchema: schema,
	}

	return tool
}

// WithDescription adds a description to the Tool.
// The description should provide a clear, human-readable explanation of what the tool does.
func WithDescription(description string) ToolOption {
	return func(t *Tool) {
		t.Description = description
	}
}

//
// Common Property Options
//

// Description adds a description to a property in the JSON Schema.
// The description should explain the purpose and expected values of the property.
func Description(desc string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["description"] = desc
	}
}

// Required marks a property as required in the tool's input schema.
// Required properties must be provided when using the tool.
func Required() PropertyOption {
	return func(schema map[string]interface{}) {
		schema["required"] = true
	}
}

// Title adds a display-friendly title to a property in the JSON Schema.
// This title can be used by UI components to show a more readable property name.
func Title(title string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["title"] = title
	}
}

//
// String Property Options
//

// DefaultString sets the default value for a string property.
// This value will be used if the property is not explicitly provided.
func DefaultString(value string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

// Enum specifies a list of allowed values for a string property.
// The property value must be one of the specified enum values.
func Enum(values ...string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["enum"] = values
	}
}

// MaxLength sets the maximum length for a string property.
// The string value must not exceed this length.
func MaxLength(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxLength"] = max
	}
}

// MinLength sets the minimum length for a string property.
// The string value must be at least this length.
func MinLength(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minLength"] = min
	}
}

// Pattern sets a regex pattern that a string property must match.
// The string value must conform to the specified regular expression.
func Pattern(pattern string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["pattern"] = pattern
	}
}

//
// Number Property Options
//

// DefaultNumber sets the default value for a number property.
// This value will be used if the property is not explicitly provided.
func DefaultNumber(value float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

// Max sets the maximum value for a number property.
// The number value must not exceed this maximum.
func Max(max float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maximum"] = max
	}
}

// Min sets the minimum value for a number property.
// The number value must not be less than this minimum.
func Min(min float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minimum"] = min
	}
}

// MultipleOf specifies that a number must be a multiple of the given value.
// The number value must be divisible by this value.
func MultipleOf(value float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["multipleOf"] = value
	}
}

//
// Boolean Property Options
//

// DefaultBool sets the default value for a boolean property.
// This value will be used if the property is not explicitly provided.
func DefaultBool(value bool) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

//
// Property Type Helpers
//

// WithBoolean adds a boolean property to the tool schema.
// It accepts property options to configure the boolean property's behavior and constraints.
func WithBoolean(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "boolean",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			if t.InputSchema.Required == nil {
				t.InputSchema.Required = []string{name}
			} else {
				t.InputSchema.Required = append(t.InputSchema.Required, name)
			}
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithNumber adds a number property to the tool schema.
// It accepts property options to configure the number property's behavior and constraints.
func WithNumber(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "number",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			if t.InputSchema.Required == nil {
				t.InputSchema.Required = []string{name}
			} else {
				t.InputSchema.Required = append(t.InputSchema.Required, name)
			}
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithString adds a string property to the tool schema.
// It accepts property options to configure the string property's behavior and constraints.
func WithString(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "string",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			if t.InputSchema.Required == nil {
				t.InputSchema.Required = []string{name}
			} else {
				t.InputSchema.Required = append(t.InputSchema.Required, name)
			}
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithObject adds an object property to the tool schema.
// It accepts property options to configure the object property's behavior and constraints.
func WithObject(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			if t.InputSchema.Required == nil {
				t.InputSchema.Required = []string{name}
			} else {
				t.InputSchema.Required = append(t.InputSchema.Required, name)
			}
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithArray adds an array property to the tool schema.
// It accepts property options to configure the array property's behavior and constraints.
func WithArray(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "array",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			if t.InputSchema.Required == nil {
				t.InputSchema.Required = []string{name}
			} else {
				t.InputSchema.Required = append(t.InputSchema.Required, name)
			}
		}

		t.InputSchema.Properties[name] = schema
	}
}

// Properties defines the properties for an object schema
func Properties(props map[string]interface{}) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["properties"] = props
	}
}

// AdditionalProperties specifies whether additional properties are allowed in the object
// or defines a schema for additional properties
func AdditionalProperties(schema interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["additionalProperties"] = schema
	}
}

// MinProperties sets the minimum number of properties for an object
func MinProperties(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minProperties"] = min
	}
}

// MaxProperties sets the maximum number of properties for an object
func MaxProperties(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxProperties"] = max
	}
}

// PropertyNames defines a schema for property names in an object
func PropertyNames(schema map[string]interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["propertyNames"] = schema
	}
}

// Items defines the schema for array items
func Items(schema interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["items"] = schema
	}
}

// MinItems sets the minimum number of items for an array
func MinItems(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minItems"] = min
	}
}

// MaxItems sets the maximum number of items for an array
func MaxItems(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxItems"] = max
	}
}

// UniqueItems specifies whether array items must be unique
func UniqueItems(unique bool) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["uniqueItems"] = unique
	}
}
