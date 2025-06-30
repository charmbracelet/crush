package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
)

// JSONSchema represents a JSON Schema
type JSONSchema struct {
	Schema               string                 `json:"$schema,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties any                    `json:"additionalProperties,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	Default              any                    `json:"default,omitempty"`
	Definitions          map[string]*JSONSchema `json:"definitions,omitempty"`
	Ref                  string                 `json:"$ref,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	AllOf                []*JSONSchema          `json:"allOf,omitempty"`
	Not                  *JSONSchema            `json:"not,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	ExclusiveMinimum     *float64               `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     *float64               `json:"exclusiveMaximum,omitempty"`
	MultipleOf           *float64               `json:"multipleOf,omitempty"`
	MinItems             *int                   `json:"minItems,omitempty"`
	MaxItems             *int                   `json:"maxItems,omitempty"`
	UniqueItems          *bool                  `json:"uniqueItems,omitempty"`
	MinProperties        *int                   `json:"minProperties,omitempty"`
	MaxProperties        *int                   `json:"maxProperties,omitempty"`
}

// SchemaGenerator generates JSON schemas from Go types
type SchemaGenerator struct {
	definitions map[string]*JSONSchema
	visited     map[reflect.Type]bool
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		definitions: make(map[string]*JSONSchema),
		visited:     make(map[reflect.Type]bool),
	}
}

func main() {
	// Enable mock providers to avoid API calls during schema generation
	config.UseMockProviders = true

	generator := NewSchemaGenerator()
	schema := generator.GenerateSchema()

	// Pretty print the schema
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
		os.Exit(1)
	}
}

// GenerateSchema generates the complete JSON schema for the Crush configuration
func (g *SchemaGenerator) GenerateSchema() *JSONSchema {
	// Generate schema for the main Config struct
	configType := reflect.TypeOf(config.Config{})
	configSchema := g.generateTypeSchema(configType)

	// Create the root schema
	schema := &JSONSchema{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Title:       "Crush Configuration",
		Description: "Configuration schema for the Crush application",
		Type:        configSchema.Type,
		Properties:  configSchema.Properties,
		Required:    configSchema.Required,
		Definitions: g.definitions,
	}

	// Add custom enhancements
	g.enhanceSchema(schema)

	return schema
}

// generateTypeSchema generates a JSON schema for a given Go type
func (g *SchemaGenerator) generateTypeSchema(t reflect.Type) *JSONSchema {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		return g.generateTypeSchema(t.Elem())
	}

	// Check if we've already processed this type
	if g.visited[t] {
		// Return a reference to avoid infinite recursion
		return &JSONSchema{
			Ref: fmt.Sprintf("#/definitions/%s", t.Name()),
		}
	}

	switch t.Kind() {
	case reflect.String:
		return &JSONSchema{Type: "string"}
	case reflect.Bool:
		return &JSONSchema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &JSONSchema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &JSONSchema{Type: "number"}
	case reflect.Slice, reflect.Array:
		itemSchema := g.generateTypeSchema(t.Elem())
		return &JSONSchema{
			Type:  "array",
			Items: itemSchema,
		}
	case reflect.Map:
		valueSchema := g.generateTypeSchema(t.Elem())
		return &JSONSchema{
			Type:                 "object",
			AdditionalProperties: valueSchema,
		}
	case reflect.Struct:
		return g.generateStructSchema(t)
	case reflect.Interface:
		// For interface{} types, allow any value
		return &JSONSchema{}
	default:
		// Fallback for unknown types
		return &JSONSchema{}
	}
}

// generateStructSchema generates a JSON schema for a struct type
func (g *SchemaGenerator) generateStructSchema(t reflect.Type) *JSONSchema {
	// Mark as visited to prevent infinite recursion
	g.visited[t] = true

	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
	}

	var required []string

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		// Parse JSON tag
		jsonName, options := parseJSONTag(jsonTag)
		if jsonName == "" {
			jsonName = strings.ToLower(field.Name)
		}

		// Generate field schema
		fieldSchema := g.generateTypeSchema(field.Type)

		// Add description from field name if not present
		if fieldSchema.Description == "" {
			fieldSchema.Description = generateFieldDescription(field.Name, field.Type)
		}

		// Check if field is required (not omitempty and not a pointer)
		if !slices.Contains(options, "omitempty") && field.Type.Kind() != reflect.Ptr {
			required = append(required, jsonName)
		}

		schema.Properties[jsonName] = fieldSchema
	}

	if len(required) > 0 {
		schema.Required = required
	}

	// Store in definitions if it's a named type
	if t.Name() != "" {
		g.definitions[t.Name()] = schema
	}

	return schema
}

// parseJSONTag parses a JSON struct tag
func parseJSONTag(tag string) (name string, options []string) {
	if tag == "" {
		return "", nil
	}

	parts := strings.Split(tag, ",")
	name = parts[0]
	if len(parts) > 1 {
		options = parts[1:]
	}
	return name, options
}

// generateFieldDescription generates a description for a field based on its name and type
func generateFieldDescription(fieldName string, fieldType reflect.Type) string {
	// Convert camelCase to words
	words := camelCaseToWords(fieldName)
	description := strings.Join(words, " ")

	// Add type-specific information
	switch fieldType.Kind() {
	case reflect.Bool:
		if !strings.Contains(strings.ToLower(description), "enable") &&
			!strings.Contains(strings.ToLower(description), "disable") {
			description = "Enable " + strings.ToLower(description)
		}
	case reflect.Slice:
		if !strings.HasSuffix(description, "s") {
			description = description + " list"
		}
	case reflect.Map:
		description = description + " configuration"
	}

	return description
}

// camelCaseToWords converts camelCase to separate words
func camelCaseToWords(s string) []string {
	var words []string
	var currentWord strings.Builder

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if currentWord.Len() > 0 {
				words = append(words, currentWord.String())
				currentWord.Reset()
			}
		}
		currentWord.WriteRune(r)
	}

	if currentWord.Len() > 0 {
		words = append(words, currentWord.String())
	}

	return words
}

// enhanceSchema adds custom enhancements to the generated schema
func (g *SchemaGenerator) enhanceSchema(schema *JSONSchema) {
	// Add provider enums
	g.addProviderEnums(schema)

	// Add model enums
	g.addModelEnums(schema)

	// Add agent enums
	g.addAgentEnums(schema)

	// Add tool enums
	g.addToolEnums(schema)

	// Add MCP type enums
	g.addMCPTypeEnums(schema)

	// Add model type enums
	g.addModelTypeEnums(schema)

	// Add default values
	g.addDefaultValues(schema)

	// Add custom descriptions
	g.addCustomDescriptions(schema)
}

// addProviderEnums adds provider enums to the schema
func (g *SchemaGenerator) addProviderEnums(schema *JSONSchema) {
	providers := config.Providers()
	var providerIDs []any
	for _, p := range providers {
		providerIDs = append(providerIDs, string(p.ID))
	}

	// Add to PreferredModel provider field
	if preferredModelDef, exists := schema.Definitions["PreferredModel"]; exists {
		if providerProp, exists := preferredModelDef.Properties["provider"]; exists {
			providerProp.Enum = providerIDs
		}
	}

	// Add to ProviderConfig ID field
	if providerConfigDef, exists := schema.Definitions["ProviderConfig"]; exists {
		if idProp, exists := providerConfigDef.Properties["id"]; exists {
			idProp.Enum = providerIDs
		}
	}
}

// addModelEnums adds model enums to the schema
func (g *SchemaGenerator) addModelEnums(schema *JSONSchema) {
	providers := config.Providers()
	var modelIDs []any
	for _, p := range providers {
		for _, m := range p.Models {
			modelIDs = append(modelIDs, m.ID)
		}
	}

	// Add to PreferredModel model_id field
	if preferredModelDef, exists := schema.Definitions["PreferredModel"]; exists {
		if modelIDProp, exists := preferredModelDef.Properties["model_id"]; exists {
			modelIDProp.Enum = modelIDs
		}
	}
}

// addAgentEnums adds agent ID enums to the schema
func (g *SchemaGenerator) addAgentEnums(schema *JSONSchema) {
	agentIDs := []any{
		string(config.AgentCoder),
		string(config.AgentTask),
	}

	if agentDef, exists := schema.Definitions["Agent"]; exists {
		if idProp, exists := agentDef.Properties["id"]; exists {
			idProp.Enum = agentIDs
		}
	}
}

// addToolEnums adds tool enums to the schema
func (g *SchemaGenerator) addToolEnums(schema *JSONSchema) {
	tools := []any{
		"bash", "edit", "fetch", "glob", "grep", "ls", "sourcegraph", "view", "write", "agent",
	}

	if agentDef, exists := schema.Definitions["Agent"]; exists {
		if allowedToolsProp, exists := agentDef.Properties["allowed_tools"]; exists {
			if allowedToolsProp.Items != nil {
				allowedToolsProp.Items.Enum = tools
			}
		}
	}
}

// addMCPTypeEnums adds MCP type enums to the schema
func (g *SchemaGenerator) addMCPTypeEnums(schema *JSONSchema) {
	mcpTypes := []any{
		string(config.MCPStdio),
		string(config.MCPSse),
	}

	if mcpDef, exists := schema.Definitions["MCP"]; exists {
		if typeProp, exists := mcpDef.Properties["type"]; exists {
			typeProp.Enum = mcpTypes
		}
	}
}

// addModelTypeEnums adds model type enums to the schema
func (g *SchemaGenerator) addModelTypeEnums(schema *JSONSchema) {
	modelTypes := []any{
		string(config.LargeModel),
		string(config.SmallModel),
	}

	if agentDef, exists := schema.Definitions["Agent"]; exists {
		if modelProp, exists := agentDef.Properties["model"]; exists {
			modelProp.Enum = modelTypes
		}
	}
}

// addDefaultValues adds default values to the schema
func (g *SchemaGenerator) addDefaultValues(schema *JSONSchema) {
	// Add default context paths
	if optionsDef, exists := schema.Definitions["Options"]; exists {
		if contextPathsProp, exists := optionsDef.Properties["context_paths"]; exists {
			contextPathsProp.Default = []any{
				".github/copilot-instructions.md",
				".cursorrules",
				".cursor/rules/",
				"CLAUDE.md",
				"CLAUDE.local.md",
				"GEMINI.md",
				"gemini.md",
				"crush.md",
				"crush.local.md",
				"Crush.md",
				"Crush.local.md",
				"CRUSH.md",
				"CRUSH.local.md",
			}
		}
		if dataDirProp, exists := optionsDef.Properties["data_directory"]; exists {
			dataDirProp.Default = ".crush"
		}
		if debugProp, exists := optionsDef.Properties["debug"]; exists {
			debugProp.Default = false
		}
		if debugLSPProp, exists := optionsDef.Properties["debug_lsp"]; exists {
			debugLSPProp.Default = false
		}
		if disableAutoSummarizeProp, exists := optionsDef.Properties["disable_auto_summarize"]; exists {
			disableAutoSummarizeProp.Default = false
		}
	}

	// Add default MCP type
	if mcpDef, exists := schema.Definitions["MCP"]; exists {
		if typeProp, exists := mcpDef.Properties["type"]; exists {
			typeProp.Default = string(config.MCPStdio)
		}
	}

	// Add default TUI options
	if tuiOptionsDef, exists := schema.Definitions["TUIOptions"]; exists {
		if compactModeProp, exists := tuiOptionsDef.Properties["compact_mode"]; exists {
			compactModeProp.Default = false
		}
	}

	// Add default provider disabled
	if providerConfigDef, exists := schema.Definitions["ProviderConfig"]; exists {
		if disabledProp, exists := providerConfigDef.Properties["disabled"]; exists {
			disabledProp.Default = false
		}
	}

	// Add default agent disabled
	if agentDef, exists := schema.Definitions["Agent"]; exists {
		if disabledProp, exists := agentDef.Properties["disabled"]; exists {
			disabledProp.Default = false
		}
	}

	// Add default LSP disabled
	if lspConfigDef, exists := schema.Definitions["LSPConfig"]; exists {
		if disabledProp, exists := lspConfigDef.Properties["enabled"]; exists {
			disabledProp.Default = true
		}
	}
}

// addCustomDescriptions adds custom descriptions to improve the schema
func (g *SchemaGenerator) addCustomDescriptions(schema *JSONSchema) {
	// Enhance main config descriptions
	if schema.Properties != nil {
		if modelsProp, exists := schema.Properties["models"]; exists {
			modelsProp.Description = "Preferred model configurations for large and small model types"
		}
		if providersProp, exists := schema.Properties["providers"]; exists {
			providersProp.Description = "LLM provider configurations"
		}
		if agentsProp, exists := schema.Properties["agents"]; exists {
			agentsProp.Description = "Agent configurations for different tasks"
		}
		if mcpProp, exists := schema.Properties["mcp"]; exists {
			mcpProp.Description = "Model Control Protocol server configurations"
		}
		if lspProp, exists := schema.Properties["lsp"]; exists {
			lspProp.Description = "Language Server Protocol configurations"
		}
		if optionsProp, exists := schema.Properties["options"]; exists {
			optionsProp.Description = "General application options and settings"
		}
	}

	// Enhance specific field descriptions
	if providerConfigDef, exists := schema.Definitions["ProviderConfig"]; exists {
		if apiKeyProp, exists := providerConfigDef.Properties["api_key"]; exists {
			apiKeyProp.Description = "API key for authenticating with the provider"
		}
		if baseURLProp, exists := providerConfigDef.Properties["base_url"]; exists {
			baseURLProp.Description = "Base URL for the provider API (required for custom providers)"
		}
		if extraHeadersProp, exists := providerConfigDef.Properties["extra_headers"]; exists {
			extraHeadersProp.Description = "Additional HTTP headers to send with requests"
		}
		if extraParamsProp, exists := providerConfigDef.Properties["extra_params"]; exists {
			extraParamsProp.Description = "Additional provider-specific parameters"
		}
	}

	if agentDef, exists := schema.Definitions["Agent"]; exists {
		if allowedToolsProp, exists := agentDef.Properties["allowed_tools"]; exists {
			allowedToolsProp.Description = "List of tools this agent is allowed to use (if nil, all tools are allowed)"
		}
		if allowedMCPProp, exists := agentDef.Properties["allowed_mcp"]; exists {
			allowedMCPProp.Description = "Map of MCP servers this agent can use and their allowed tools"
		}
		if allowedLSPProp, exists := agentDef.Properties["allowed_lsp"]; exists {
			allowedLSPProp.Description = "List of LSP servers this agent can use (if nil, all LSPs are allowed)"
		}
		if contextPathsProp, exists := agentDef.Properties["context_paths"]; exists {
			contextPathsProp.Description = "Custom context paths for this agent (additive to global context paths)"
		}
	}

	if mcpDef, exists := schema.Definitions["MCP"]; exists {
		if commandProp, exists := mcpDef.Properties["command"]; exists {
			commandProp.Description = "Command to execute for stdio MCP servers"
		}
		if urlProp, exists := mcpDef.Properties["url"]; exists {
			urlProp.Description = "URL for SSE MCP servers"
		}
		if headersProp, exists := mcpDef.Properties["headers"]; exists {
			headersProp.Description = "HTTP headers for SSE MCP servers"
		}
	}
}
