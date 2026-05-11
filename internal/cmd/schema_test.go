package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestSchemaNoBrokenRefs(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)

	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(bts, &schema))
	require.NotEmpty(t, schema.Defs, "schema should have definitions")

	for name := range schema.Defs {
		require.NotContains(t, name, "/", "schema $def key %q contains '/' which breaks JSON Pointer $ref resolution", name)
	}
}

func TestSchemaProvidersHasAdditionalProperties(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)

	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(bts, &schema))

	var cfg struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schema.Defs["Config"], &cfg))

	providersRaw, ok := cfg.Properties["providers"]
	require.True(t, ok, "Config should have a providers property")

	var providers struct {
		Type                 string          `json:"type"`
		AdditionalProperties json.RawMessage `json:"additionalProperties"`
	}
	require.NoError(t, json.Unmarshal(providersRaw, &providers))
	require.Equal(t, "object", providers.Type)
	require.True(t, strings.Contains(string(providers.AdditionalProperties), "ProviderConfig"),
		"providers should use additionalProperties with a ProviderConfig ref, got: %s", string(providers.AdditionalProperties))
}

func TestSchemaWebSearchIsOptional(t *testing.T) {
	t.Parallel()

	reflector := new(jsonschema.Reflector)
	bts, err := json.Marshal(reflector.Reflect(&config.Config{}))
	require.NoError(t, err)

	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(bts, &schema))

	var tools struct {
		Required []string `json:"required"`
	}
	require.NoError(t, json.Unmarshal(schema.Defs["Tools"], &tools))

	// Only assert the invariant this test cares about: web_search must remain
	// optional. github.com/invopop/jsonschema v0.13.0 does not understand the
	// json "omitzero" tag option, so it marks the sibling ls and grep fields as
	// required unless they are also removed explicitly. Newer jsonschema versions
	// understand omitzero and may produce no required fields here at all. If this
	// is restored to require.ElementsMatch(t, []string{"ls", "grep"},
	// tools.Required), first bump github.com/invopop/jsonschema to a version whose
	// omitzero handling is intentionally relied on, review the generated
	// schema.json diff, and decide whether Tools.JSONSchemaExtend in
	// internal/config/config.go is still needed.
	require.NotContains(t, tools.Required, "web_search")
}
