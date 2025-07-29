package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:    "schema",
	Short:  "Generate JSON schema for configuration",
	Long:   "Generate JSON schema for the crush configuration file",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		reflector := jsonschema.Reflector{
			// Custom type mapper to handle csync.Map
			Mapper: func(t reflect.Type) *jsonschema.Schema {
				// Handle csync.Map[string, ProviderConfig] specifically
				if t.String() == "csync.Map[string,github.com/charmbracelet/crush/internal/config.ProviderConfig]" {
					return &jsonschema.Schema{
						Type:        "object",
						Description: "AI provider configurations",
						AdditionalProperties: &jsonschema.Schema{
							Ref: "#/$defs/ProviderConfig",
						},
					}
				}
				return nil
			},
		}
		
		// First reflect the config to get the main schema
		schema := reflector.Reflect(&config.Config{})
		
		// Now manually add the ProviderConfig definition that might be missing
		providerConfigSchema := reflector.ReflectFromType(reflect.TypeOf(config.ProviderConfig{}))
		if schema.Definitions == nil {
			schema.Definitions = make(map[string]*jsonschema.Schema)
		}
		
		// Extract the actual definition from the nested schema
		if providerConfigSchema.Definitions != nil && providerConfigSchema.Definitions["ProviderConfig"] != nil {
			schema.Definitions["ProviderConfig"] = providerConfigSchema.Definitions["ProviderConfig"]
			// Also add any other definitions from the provider config schema
			for k, v := range providerConfigSchema.Definitions {
				if k != "ProviderConfig" {
					schema.Definitions[k] = v
				}
			}
		} else {
			// Fallback: use the schema itself if it's not nested
			schema.Definitions["ProviderConfig"] = providerConfigSchema
		}

		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}

		fmt.Println(string(schemaJSON))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
