package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/invopop/jsonschema"
)

func main() {
	// Create a new reflector
	r := &jsonschema.Reflector{
		// Use anonymous schemas to avoid ID conflicts
		Anonymous: true,
		// Expand the root struct instead of referencing it
		ExpandedStruct:            true,
		AllowAdditionalProperties: true,
	}

	// Generate schema for the main Config struct
	schema := r.Reflect(&config.Config{})

	// Enhance the schema with additional information
	enhanceSchema(schema)

	// Set the schema metadata
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.Title = "Crush Configuration"
	schema.Description = "Configuration schema for the Crush application"

	// Pretty print the schema
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
		os.Exit(1)
	}
}

// enhanceSchema adds additional enhancements to the generated schema
func enhanceSchema(schema *jsonschema.Schema) {
	// Add provider enums
	addProviderEnums(schema)

	// Add model enums
	addModelEnums(schema)

	// Add tool enums
	addToolEnums(schema)

	// Add default context paths
	addDefaultContextPaths(schema)
}

// addProviderEnums adds provider enums to the schema
func addProviderEnums(schema *jsonschema.Schema) {
	providers := config.Providers()
	var providerIDs []any
	for _, p := range providers {
		providerIDs = append(providerIDs, string(p.ID))
	}

	// Add to PreferredModel provider field
	if schema.Definitions != nil {
		if preferredModelDef, exists := schema.Definitions["PreferredModel"]; exists {
			if providerProp, exists := preferredModelDef.Properties.Get("provider"); exists {
				providerProp.Enum = providerIDs
			}
		}

		// Add to ProviderConfig ID field
		if providerConfigDef, exists := schema.Definitions["ProviderConfig"]; exists {
			if idProp, exists := providerConfigDef.Properties.Get("id"); exists {
				idProp.Enum = providerIDs
			}
		}
	}
}

// addModelEnums adds model enums to the schema
func addModelEnums(schema *jsonschema.Schema) {
	providers := config.Providers()
	var modelIDs []any
	for _, p := range providers {
		for _, m := range p.Models {
			modelIDs = append(modelIDs, m.ID)
		}
	}

	// Add to PreferredModel model_id field
	if schema.Definitions != nil {
		if preferredModelDef, exists := schema.Definitions["PreferredModel"]; exists {
			if modelIDProp, exists := preferredModelDef.Properties.Get("model_id"); exists {
				modelIDProp.Enum = modelIDs
			}
		}
	}
}

// addToolEnums adds tool enums to the schema
func addToolEnums(schema *jsonschema.Schema) {
	tools := []any{
		"bash", "edit", "fetch", "glob", "grep", "ls", "sourcegraph", "view", "write", "agent",
	}

	if schema.Definitions != nil {
		if agentDef, exists := schema.Definitions["Agent"]; exists {
			if allowedToolsProp, exists := agentDef.Properties.Get("allowed_tools"); exists {
				if allowedToolsProp.Items != nil {
					allowedToolsProp.Items.Enum = tools
				}
			}
		}
	}
}

// addDefaultContextPaths adds default context paths to the schema
func addDefaultContextPaths(schema *jsonschema.Schema) {
	defaultContextPaths := []any{
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

	if schema.Definitions != nil {
		if optionsDef, exists := schema.Definitions["Options"]; exists {
			if contextPathsProp, exists := optionsDef.Properties.Get("context_paths"); exists {
				contextPathsProp.Default = defaultContextPaths
			}
		}
	}

	// Also add to root properties if they exist
	if schema.Properties != nil {
		if optionsProp, exists := schema.Properties.Get("options"); exists {
			if optionsProp.Properties != nil {
				if contextPathsProp, exists := optionsProp.Properties.Get("context_paths"); exists {
					contextPathsProp.Default = defaultContextPaths
				}
			}
		}
	}
}
