package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestCoderPromptRespectsDisabledEditTools verifies that the coder system
// prompt only advertises edit tools that are actually enabled for the agent.
func TestCoderPromptRespectsDisabledEditTools(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)

	buildPrompt := func(t *testing.T, disabled []string) string {
		t.Helper()
		cfg.Config().Options.DisabledTools = disabled
		cfg.SetupAgents()

		c := &coordinator{cfg: cfg, interactive: true}
		coderCfg := cfg.Config().Agents[config.AgentCoder]
		p, err := loadPrompt(cfg, "coder",
			prompt.WithWorkingDir(env.workingDir),
			prompt.WithToolNames(c.effectiveToolNames(coderCfg, false)),
		)
		require.NoError(t, err)
		systemPrompt, err := p.Build(t.Context(), "mock", "mock-model", cfg)
		require.NoError(t, err)
		return systemPrompt
	}

	// Default: all edit tools are listed.
	defaultPrompt := buildPrompt(t, nil)
	require.Contains(t, defaultPrompt, "**Available edit tools:**")
	require.Contains(t, defaultPrompt, "- `edit` - Single find/replace in a file (exact text matching)")
	require.Contains(t, defaultPrompt, "- `multiedit` - Multiple find/replace operations in one file")
	require.Contains(t, defaultPrompt, "- `write` - Create/overwrite entire file")
	require.Contains(t, defaultPrompt, "- `lsp_replace_symbol` -")
	require.Contains(t, defaultPrompt, "- `lsp_rename` -")
	require.Contains(t, defaultPrompt, "Use 'edit' or 'multiedit' instead.")
	require.Contains(t, defaultPrompt, "Fall back to `edit`/`multiedit` for:")
	require.Contains(t, defaultPrompt, "<whitespace_and_exact_matching>")

	// Disabling edit and multiedit removes them from the prompt while keeping
	// the remaining edit tools.
	noEditOrMulti := buildPrompt(t, []string{"edit", "multiedit"})
	require.Contains(t, noEditOrMulti, "**Available edit tools:**")
	require.NotContains(t, noEditOrMulti, "- `edit` -")
	require.NotContains(t, noEditOrMulti, "- `multiedit` -")
	require.Contains(t, noEditOrMulti, "- `write` - Create/overwrite entire file")
	require.Contains(t, noEditOrMulti, "- `lsp_replace_symbol` -")
	require.Contains(t, noEditOrMulti, "- `lsp_rename` -")
	require.NotContains(t, noEditOrMulti, "Use 'edit' or 'multiedit' instead.")
	require.NotContains(t, noEditOrMulti, "Fall back to `edit`")
	require.NotContains(t, noEditOrMulti, "<whitespace_and_exact_matching>")

	// Disabling only edit keeps multiedit references.
	onlyMulti := buildPrompt(t, []string{"edit"})
	require.NotContains(t, onlyMulti, "- `edit` -")
	require.Contains(t, onlyMulti, "- `multiedit` - Multiple find/replace operations in one file")
	require.Contains(t, onlyMulti, "Use 'multiedit' instead.")
	require.Contains(t, onlyMulti, "Fall back to `multiedit` for:")
	require.Contains(t, onlyMulti, "<whitespace_and_exact_matching>")

	// Disabling all edit tools omits the available edit tools header.
	noEditTools := buildPrompt(t, []string{"edit", "multiedit", "write", "lsp_replace_symbol", "lsp_rename"})
	require.NotContains(t, noEditTools, "**Available edit tools:**")
	require.NotContains(t, noEditTools, "- `write` -")

	// With LSP off, the LSP tools are never attached, so the prompt must not
	// advertise them.
	cfg.Config().Options.AutoLSP = new(false)
	noLSP := buildPrompt(t, nil)
	require.NotContains(t, noLSP, "- `lsp_replace_symbol` -")
	require.NotContains(t, noLSP, "**Prefer LSP tools when available:**")
	require.Contains(t, noLSP, "- `edit` - Single find/replace in a file (exact text matching)")
}
