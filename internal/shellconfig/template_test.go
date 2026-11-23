package shellconfig

import (
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// trickyValue holds every character the config loader's expansion treats
// specially, plus quotes it must leave alone.
const trickyValue = `p@$$w0rd $(echo nope) ` + "`id`" + ` ${HOME} "q" 'x' \n \`

// TestTemplateFieldsSurviveExpansion checks that crushrc values reach
// consumers verbatim even though the loader shell-expands these fields.
func TestTemplateFieldsSurviveExpansion(t *testing.T) {
	t.Setenv("V", trickyValue)

	result := loadScript(t, `provider add p --api-key "$V" --base-url "$V" --extra-header H "$V"
mcp add m --command "$V" --args "$V" --env E "$V" --url "$V" --header H "$V" \
  --oauth-client-id "$V" --oauth-client-secret "$V"
lsp add l --command "$V" --args "$V" --env E "$V"`)

	p := result["providers"].(map[string]any)["p"].(map[string]any)
	m := result["mcp"].(map[string]any)["m"].(map[string]any)
	l := result["lsp"].(map[string]any)["l"].(map[string]any)

	fields := map[string]any{
		"provider api_key":        p["api_key"],
		"provider base_url":       p["base_url"],
		"provider extra_headers":  p["extra_headers"].(map[string]any)["H"],
		"mcp command":             m["command"],
		"mcp args":                m["args"].([]any)[0],
		"mcp env":                 m["env"].(map[string]any)["E"],
		"mcp url":                 m["url"],
		"mcp headers":             m["headers"].(map[string]any)["H"],
		"mcp oauth_client_id":     m["oauth_client_id"],
		"mcp oauth_client_secret": m["oauth_client_secret"],
		"lsp command":             l["command"],
		"lsp args":                l["args"].([]any)[0],
		"lsp env":                 l["env"].(map[string]any)["E"],
	}
	for name, stored := range fields {
		got, err := shell.ExpandValue(t.Context(), stored.(string), nil)
		require.NoError(t, err, name)
		require.Equal(t, trickyValue, got, name)
	}
}

// TestNonTemplateFieldsAreNotEscaped guards against escaping fields the
// loader never expands, which would leave the backslashes in place.
func TestNonTemplateFieldsAreNotEscaped(t *testing.T) {
	t.Setenv("V", trickyValue)

	result := loadScript(t, `provider add p --name "$V" --system-prompt-prefix "$V" --aws-auth-refresh "$V"`)

	p := result["providers"].(map[string]any)["p"].(map[string]any)
	for _, key := range []string{"name", "system_prompt_prefix", "aws_auth_refresh"} {
		require.Equal(t, trickyValue, p[key], key)
	}
}
