package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

const jiraPlugin = `tool add jira_search --description "Search Jira" --command 'jq -r .jql' --param jql "JQL" --required jql`

// TestPluginAddsTool is the tool half of the plugin system: a script in the
// plugins directory hands the coder a new tool, and only the coder.
func TestPluginAddsTool(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()
	writePlugin(t, workDir, "jira.sh", jiraPlugin)

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	store.SetupAgents()

	cfg := store.Config()
	tool, ok := cfg.CustomTools["jira_search"]
	require.True(t, ok)
	require.Equal(t, filepath.Join(workDir, ".crush", "plugins", "jira.sh"), tool.Source)
	require.Contains(t, cfg.Agents[config.AgentCoder].AllowedTools, "jira_search")
	require.NotContains(t, cfg.Agents[config.AgentTask].AllowedTools, "jira_search")
	require.NotContains(t, cfg.Agents[config.AgentPlan].AllowedTools, "jira_search")
}

func TestPluginToolHonorsDisabledTools(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()
	writePlugin(t, workDir, "jira.sh", jiraPlugin)
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"), []byte(`permissions deny jira_search`), 0o644))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	store.SetupAgents()
	require.NotContains(t, store.Config().Agents[config.AgentCoder].AllowedTools, "jira_search")
}

// TestProjectCrushrcOverridesPluginTool keeps the precedence providers have:
// a project can tune or drop a plugin's tool without editing the plugin.
func TestProjectCrushrcOverridesPluginTool(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()
	writePlugin(t, workDir, "jira.sh", jiraPlugin+"\n"+`tool add other --description o --command 'echo o'`)
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"), []byte(`tool add jira_search --timeout 5
tool add other --disabled true`), 0o644))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	store.SetupAgents()
	tools := store.Config().CustomTools
	require.Equal(t, 5, tools["jira_search"].Timeout)
	require.Equal(t, "jq -r .jql", tools["jira_search"].Command)
	require.Equal(t, filepath.Join(workDir, ".crush", "plugins", "jira.sh"), tools["jira_search"].Source)
	require.True(t, tools["other"].Disabled)
	require.NotContains(t, store.Config().Agents[config.AgentCoder].AllowedTools, "other")
}

func TestInvalidPluginToolFailsLoad(t *testing.T) {
	for name, script := range map[string]string{
		"builtin clash":     `tool add bash --description d --command c`,
		"mcp prefix":        `tool add mcp_x --description d --command c`,
		"bad name":          `tool add "has space" --description d --command c`,
		"no command":        `tool add x --description d`,
		"no description":    `tool add x --command c`,
		"unknown require":   `tool add x --description d --command c --param a A --required b`,
		"schema and params": `tool add x --description d --command c --param a A --schema '{"properties":{}}'`,
	} {
		t.Run(name, func(t *testing.T) {
			isolateConfig(t)
			workDir := t.TempDir()
			writePlugin(t, workDir, "bad.sh", script)
			_, err := config.Load(workDir, t.TempDir(), false)
			require.ErrorContains(t, err, "invalid tool configuration")
		})
	}
}

// TestCrushrcCanReinstatePromptForPluginAction lets a user overrule a plugin
// that skips the prompt for an action.
func TestCrushrcCanReinstatePromptForPluginAction(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()
	writePlugin(t, workDir, "gh.sh", `tool add GitHub --description "GitHub"
tool action GitHub pr_list --description "List" --command 'gh pr list' --skip-permission true`)
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"), []byte(`tool action GitHub pr_list --skip-permission false`), 0o644))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)
	a := store.Config().CustomTools["GitHub"].Actions["pr_list"]
	require.False(t, a.SkipPermission)
	require.Equal(t, "gh pr list", a.Command)
}

func TestInvalidToolActionFailsLoad(t *testing.T) {
	for name, script := range map[string]string{
		"no command anywhere": `tool add x --description d
tool action x a --description a`,
		"no description": `tool add x --description d --command c
tool action x a`,
		"reserved param": `tool add x --description d --command c
tool action x a --description a --param action A`,
		"undeclared require": `tool add x --description d --command c
tool action x a --description a --required n`,
		"uppercase param": `tool add x --description d --command c --param PATH p`,
	} {
		t.Run(name, func(t *testing.T) {
			isolateConfig(t)
			workDir := t.TempDir()
			writePlugin(t, workDir, "bad.sh", script)
			_, err := config.Load(workDir, t.TempDir(), false)
			require.ErrorContains(t, err, "invalid tool configuration")
		})
	}
}

func TestBadDisplayTemplateFailsLoad(t *testing.T) {
	isolateConfig(t)
	workDir := t.TempDir()
	writePlugin(t, workDir, "bad.sh", `tool add x --description d --command c
tool action x a --description a --summary '{{.n'`)
	_, err := config.Load(workDir, t.TempDir(), false)
	require.ErrorContains(t, err, "invalid summary template")
}

func TestCustomToolTemplateTruncate(t *testing.T) {
	tpl, err := config.CustomToolTemplate(`{{truncate 10 .}}`)
	require.NoError(t, err)
	var b strings.Builder
	require.NoError(t, tpl.Execute(&b, "Looks good\nto me, merging"))
	require.Equal(t, "Looks goo…", b.String())
	b.Reset()
	require.NoError(t, tpl.Execute(&b, "LGTM"))
	require.Equal(t, "LGTM", b.String())
}
