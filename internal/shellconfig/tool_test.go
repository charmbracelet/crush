package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolAdd(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tools.sh")
	data, err := LoadShellConfig(t.Context(), path, []byte(`tool add jira_search \
  --description "Search Jira" \
  --command 'jq -r .jql' \
  --param jql "JQL query" \
  --required jql \
  --timeout 30 \
  --env JIRA_URL https://jira.example`))
	require.NoError(t, err)
	require.JSONEq(t, `{"custom_tools":{"jira_search":{
		"description":"Search Jira",
		"command":"jq -r .jql",
		"params":{"jql":"JQL query"},
		"required":["jql"],
		"timeout":30,
		"env":{"JIRA_URL":"https://jira.example"},
		"source":`+quote(path)+`}}}`, string(data))
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestToolAddSchemaAndUpdate(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `tool add x --description d --command c --schema '{"type":"object","properties":{"a":{"type":"number"}}}'
tool add x --timeout 5`)
	x := result["custom_tools"].(map[string]any)["x"].(map[string]any)
	require.Equal(t, "c", x["command"])
	require.Equal(t, float64(5), x["timeout"])
	require.Contains(t, x["schema"].(map[string]any)["properties"], "a")
}

func TestToolAddPartialDoesNotClaimSource(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `tool add x --timeout 5`)
	x := result["custom_tools"].(map[string]any)["x"].(map[string]any)
	require.NotContains(t, x, "source", "an override that sets no command must not redirect CRUSH_PLUGIN_FILE")
}

func TestToolRemove(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `tool add a --description d --command c
tool add b --description d --command c
tool rm a`)
	tools := result["custom_tools"].(map[string]any)
	require.NotContains(t, tools, "a")
	require.Contains(t, tools, "b")
}

func TestToolAddRejectsBadFlags(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`tool add x --descripton d`))
	require.ErrorContains(t, err, "unknown flag")

	_, err = LoadShellConfig(t.Context(), path, []byte(`tool add x --schema '[1]'`))
	require.ErrorContains(t, err, "expects a JSON object")
}
