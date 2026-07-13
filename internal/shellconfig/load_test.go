package shellconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// TestLoadShellConfig_Provider verifies that the provider builtin produces
// correct JSON for a basic provider definition.
func TestLoadShellConfig_Provider(t *testing.T) {
	dir := t.TempDir()
	script := `provider openai --api-key "$OPENAI_API_KEY" --base-url "https://api.openai.com/v1"`
	path := filepath.Join(dir, "crush.sh")

	t.Setenv("OPENAI_API_KEY", "test-key-123")
	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	require.NotNil(t, jsonBytes)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers, ok := result["providers"].(map[string]any)
	require.True(t, ok)
	openai, ok := providers["openai"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "test-key-123", openai["api_key"])
	require.Equal(t, "https://api.openai.com/v1", openai["base_url"])
}

// TestLoadShellConfig_MultipleProviders verifies that multiple provider calls
// each produce separate entries.
func TestLoadShellConfig_MultipleProviders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider openai --api-key "key1"
provider anthropic --api-key "key2"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	require.Len(t, providers, 2)
	require.Equal(t, "key1", providers["openai"].(map[string]any)["api_key"])
	require.Equal(t, "key2", providers["anthropic"].(map[string]any)["api_key"])
}

// TestLoadShellConfig_Model verifies the model builtin.
func TestLoadShellConfig_Model(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `model large --provider openai --model gpt-4o --think
model small --provider anthropic --model claude-3-5-haiku`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	models := result["models"].(map[string]any)
	large := models["large"].(map[string]any)
	require.Equal(t, "openai", large["provider"])
	require.Equal(t, "gpt-4o", large["model"])
	require.Equal(t, true, large["think"])

	small := models["small"].(map[string]any)
	require.Equal(t, "anthropic", small["provider"])
	require.Equal(t, "claude-3-5-haiku", small["model"])
}

// TestLoadShellConfig_MCP verifies the mcp builtin with stdio and http types.
func TestLoadShellConfig_MCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `mcp github --type stdio --command npx --args "-y" --args "@modelcontextprotocol/server-github" --env GITHUB_TOKEN "ghp_xxx"
mcp local-server --type http --url "http://localhost:3000/mcp" --header "Authorization" "Bearer token"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	mcps := result["mcp"].(map[string]any)

	github := mcps["github"].(map[string]any)
	require.Equal(t, "stdio", github["type"])
	require.Equal(t, "npx", github["command"])
	args := github["args"].([]any)
	require.Len(t, args, 2)
	require.Equal(t, "-y", args[0])
	require.Equal(t, "@modelcontextprotocol/server-github", args[1])
	env := github["env"].(map[string]any)
	require.Equal(t, "ghp_xxx", env["GITHUB_TOKEN"])

	local := mcps["local-server"].(map[string]any)
	require.Equal(t, "http", local["type"])
	require.Equal(t, "http://localhost:3000/mcp", local["url"])
	headers := local["headers"].(map[string]any)
	require.Equal(t, "Bearer token", headers["Authorization"])
}

// TestLoadShellConfig_LSP verifies the lsp builtin.
func TestLoadShellConfig_LSP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `lsp gopls --command gopls --filetypes go --filetypes mod --root-markers go.mod --timeout 60`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	lsps := result["lsp"].(map[string]any)
	gopls := lsps["gopls"].(map[string]any)
	require.Equal(t, "gopls", gopls["command"])
	filetypes := gopls["filetypes"].([]any)
	require.Len(t, filetypes, 2)
	require.Equal(t, "go", filetypes[0])
	require.Equal(t, "mod", filetypes[1])
	markers := gopls["root_markers"].([]any)
	require.Len(t, markers, 1)
	require.Equal(t, "go.mod", markers[0])
	require.EqualValues(t, 60, gopls["timeout"])
}

// TestLoadShellConfig_Permissions verifies the permissions builtin.
func TestLoadShellConfig_Permissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `permissions --allow bash --allow view`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	perms := result["permissions"].(map[string]any)
	tools := perms["allowed_tools"].([]any)
	require.Len(t, tools, 2)
	require.Equal(t, "bash", tools[0])
	require.Equal(t, "view", tools[1])
}

// TestLoadShellConfig_Hook verifies the hook builtin.
func TestLoadShellConfig_Hook(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `hook PreToolUse --command "echo running" --matcher "bash" --timeout 10 --name "my-hook"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	hooks := result["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	require.Len(t, preToolUse, 1)
	hook := preToolUse[0].(map[string]any)
	require.Equal(t, "echo running", hook["command"])
	require.Equal(t, "bash", hook["matcher"])
	require.EqualValues(t, 10, hook["timeout"])
	require.Equal(t, "my-hook", hook["name"])
}

// TestLoadShellConfig_Options verifies the options builtin.
func TestLoadShellConfig_Options(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `options --data-directory .crush --disable-metrics true --debug true`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	opts := result["options"].(map[string]any)
	require.Equal(t, ".crush", opts["data_directory"])
	require.Equal(t, true, opts["disable_metrics"])
	require.Equal(t, true, opts["debug"])
}

// TestLoadShellConfig_SourceInclude verifies that source works for includes.
func TestLoadShellConfig_SourceInclude(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create an included file with a provider definition.
	includeContent := `provider openai --api-key "included-key"`
	includePath := filepath.Join(dir, "shared.sh")
	require.NoError(t, os.WriteFile(includePath, []byte(includeContent), 0o644))

	// Create the main script that sources the include.
	script := `source ` + includePath + `
provider anthropic --api-key "main-key"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	require.Len(t, providers, 2)
	require.Equal(t, "included-key", providers["openai"].(map[string]any)["api_key"])
	require.Equal(t, "main-key", providers["anthropic"].(map[string]any)["api_key"])
}

// TestLoadShellConfig_Conditionals verifies that bash conditionals work.
func TestLoadShellConfig_Conditionals(t *testing.T) {
	dir := t.TempDir()
	script := `if [[ "$USE_ANTHROPIC" == "1" ]]; then
  provider anthropic --api-key "ant-key"
else
  provider openai --api-key "oai-key"
fi`
	path := filepath.Join(dir, "crush.sh")

	t.Setenv("USE_ANTHROPIC", "1")
	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	require.Len(t, providers, 1)
	require.Contains(t, providers, "anthropic")
}

// TestLoadShellConfig_CommandSubstitution verifies that $(...) works in config values.
func TestLoadShellConfig_CommandSubstitution(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider openai --api-key "$(echo dynamic-key)"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	openai := providers["openai"].(map[string]any)
	require.Equal(t, "dynamic-key", openai["api_key"])
}

// TestLoadShellConfig_EnvVarExpansion verifies that $VAR expansion works.
func TestLoadShellConfig_EnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	script := `provider openai --api-key "$MY_API_KEY"`
	path := filepath.Join(dir, "crush.sh")

	t.Setenv("MY_API_KEY", "env-key-456")
	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	openai := providers["openai"].(map[string]any)
	require.Equal(t, "env-key-456", openai["api_key"])
}

// TestLoadShellConfig_UnknownFlag verifies error handling for unknown flags.
func TestLoadShellConfig_UnknownFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider openai --bogus-flag "value"`
	path := filepath.Join(dir, "crush.sh")

	_, err := LoadShellConfig(path, []byte(script))
	require.Error(t, err)
}

// TestLoadShellConfig_MissingRequiredArgs verifies error handling for missing args.
func TestLoadShellConfig_MissingRequiredArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider`
	path := filepath.Join(dir, "crush.sh")

	_, err := LoadShellConfig(path, []byte(script))
	require.Error(t, err)
}

// TestLoadShellConfig_NoBuiltins verifies that a script with no config builtins
// produces no output.
func TestLoadShellConfig_NoBuiltins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `echo "just a normal script"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	require.Nil(t, jsonBytes)
}

// TestLoadShellConfig_ExtraHeader verifies the --extra-header flag.
func TestLoadShellConfig_ExtraHeader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider custom --api-key "key" --extra-header "X-Custom" "value123"`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	custom := providers["custom"].(map[string]any)
	headers := custom["extra_headers"].(map[string]any)
	require.Equal(t, "value123", headers["X-Custom"])
}

// TestLoadShellConfig_FullConfig verifies a complete config with all builtins.
func TestLoadShellConfig_FullConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "oai-key")
	t.Setenv("ANTHROPIC_API_KEY", "ant-key")

	script := `#!/usr/bin/env bash

# Providers
provider openai --api-key "$OPENAI_API_KEY" --base-url "https://api.openai.com/v1"
provider anthropic --api-key "$ANTHROPIC_API_KEY"
provider my-llm --type openai --api-key "ollama" --base-url "http://localhost:11434/v1"

# Models
model large --provider openai --model gpt-4o --think
model small --provider anthropic --model claude-3-5-haiku

# MCP
mcp github --type stdio --command npx --args "-y" --args "@modelcontextprotocol/server-github"

# LSP
lsp gopls --command gopls --filetypes go --root-markers go.mod

# Permissions
permissions --allow bash --allow view

# Hooks
hook PreToolUse --command "echo running" --matcher "bash" --timeout 10

# Options
options --data-directory .crush --disable-metrics true`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	require.NotNil(t, jsonBytes)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	// Verify providers
	providers := result["providers"].(map[string]any)
	require.Len(t, providers, 3)
	require.Equal(t, "oai-key", providers["openai"].(map[string]any)["api_key"])
	require.Equal(t, "ant-key", providers["anthropic"].(map[string]any)["api_key"])
	myLLM := providers["my-llm"].(map[string]any)
	require.Equal(t, "ollama", myLLM["api_key"])
	require.Equal(t, "http://localhost:11434/v1", myLLM["base_url"])

	// Verify models
	models := result["models"].(map[string]any)
	large := models["large"].(map[string]any)
	require.Equal(t, "openai", large["provider"])
	require.Equal(t, "gpt-4o", large["model"])
	require.Equal(t, true, large["think"])

	// Verify MCP
	mcps := result["mcp"].(map[string]any)
	github := mcps["github"].(map[string]any)
	require.Equal(t, "npx", github["command"])

	// Verify LSP
	lsps := result["lsp"].(map[string]any)
	require.Contains(t, lsps, "gopls")

	// Verify permissions
	perms := result["permissions"].(map[string]any)
	require.Contains(t, perms, "allowed_tools")

	// Verify hooks
	hooks := result["hooks"].(map[string]any)
	require.Contains(t, hooks, "PreToolUse")

	// Verify options
	opts := result["options"].(map[string]any)
	require.Equal(t, ".crush", opts["data_directory"])
	require.Equal(t, true, opts["disable_metrics"])
}

// TestConfigBuilder_NoBuilderInContext verifies that builtins are no-ops
// when no ConfigBuilder is on the context (normal bash tool execution).
func TestConfigBuilder_NoBuilderInContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// "provider" without a ConfigBuilder should be a no-op (return nil),
	// not an error. The builtins check for the builder and silently skip.
	err := shell.Run(t.Context(), shell.RunOptions{
		Command: `provider openai --api-key "test"`,
		Cwd:     dir,
		Env:     os.Environ(),
	})
	require.NoError(t, err)
}
