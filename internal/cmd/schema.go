package cmd

import (
	"encoding/json"
	"fmt"

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
		reflector := jsonschema.Reflector{}
		schema := reflector.Reflect(&config.Config{})

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
