package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:    "schema",
	Short:  "Generate JSON schema for configuration",
	Long:   "Generate JSON schema for the crush configuration file",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var reflector jsonschema.Reflector
		csyncMapType := reflect.TypeOf(csync.Map[string, any]{})
		reflector.Mapper = func(t reflect.Type) *jsonschema.Schema {
			if t.Kind() == reflect.Struct && t.PkgPath() == csyncMapType.PkgPath() && len(t.Name()) > 4 && t.Name()[:4] == "Map[" {
				return reflector.ReflectFromType(reflect.MapOf(t.Field(0).Type, t.Field(1).Type))
			}
			return nil
		}
		bts, err := json.MarshalIndent(reflector.Reflect(&config.Config{}), "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}
		fmt.Println(string(bts))
		return nil
	},
}
