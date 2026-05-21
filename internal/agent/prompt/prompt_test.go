package prompt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPromptBuildDefaultSystemPrompt(t *testing.T) {
	workingDir := t.TempDir()
	store := loadPromptTestConfig(t, workingDir, "")
	p := newPromptForTest(t, workingDir, "default body")

	rendered, err := p.Build(context.Background(), "provider", "model", store)
	require.NoError(t, err)
	require.Contains(t, rendered, "default body")
	require.Contains(t, rendered, "<env>"+filepath.ToSlash(workingDir)+"</env>")
}

func TestPromptBuildConfiguredSystemPrompt(t *testing.T) {
	workingDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "custom.md"), []byte("configured body"), 0o644))
	store := loadPromptTestConfig(t, workingDir, `"system_prompt_path":"custom.md"`)
	p := newPromptForTest(t, workingDir, "default body")

	rendered, err := p.Build(context.Background(), "provider", "model", store)
	require.NoError(t, err)
	require.Contains(t, rendered, "configured body")
	require.NotContains(t, rendered, "default body")
	require.Contains(t, rendered, "<env>"+filepath.ToSlash(workingDir)+"</env>")
}

func TestPromptBuildRuntimeSystemPromptOverrideWins(t *testing.T) {
	workingDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "configured.md"), []byte("configured body"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "override.md"), []byte("override body"), 0o644))
	store := loadPromptTestConfig(t, workingDir, `"system_prompt_path":"configured.md"`)
	store.Overrides().SystemPromptPath = "override.md"
	p := newPromptForTest(t, workingDir, "default body")

	rendered, err := p.Build(context.Background(), "provider", "model", store)
	require.NoError(t, err)
	require.Contains(t, rendered, "override body")
	require.NotContains(t, rendered, "configured body")
	require.NotContains(t, rendered, "default body")
}

func TestPromptBuildSystemPromptPathUsesPromptWorkingDir(t *testing.T) {
	storeWorkingDir := t.TempDir()
	promptWorkingDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(promptWorkingDir, "custom.md"), []byte("prompt working dir body"), 0o644))
	store := loadPromptTestConfig(t, storeWorkingDir, `"system_prompt_path":"custom.md"`)
	p := newPromptForTest(t, promptWorkingDir, "default body")

	rendered, err := p.Build(context.Background(), "provider", "model", store)
	require.NoError(t, err)
	require.Contains(t, rendered, "prompt working dir body")
	require.NotContains(t, rendered, "default body")
	require.Contains(t, rendered, "<env>"+filepath.ToSlash(promptWorkingDir)+"</env>")
}

func newPromptForTest(t *testing.T, workingDir, defaultBody string) *Prompt {
	t.Helper()

	p, err := NewPrompt(
		"test",
		"{{.SystemPrompt}}\n<env>{{.WorkingDir}}</env>",
		WithSystemPrompt(defaultBody),
		WithWorkingDir(workingDir),
	)
	require.NoError(t, err)
	return p
}

func loadPromptTestConfig(t *testing.T, workingDir, options string) *config.ConfigStore {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	t.Setenv("CRUSH_DISABLE_METRICS", "1")

	optionFields := []string{
		`"context_paths":[]`,
		`"disabled_skills":["crush-config","crush-hooks","jq"]`,
	}
	if options != "" {
		optionFields = append(optionFields, options)
	}
	content := `{"providers":{"test":{"type":"openai-compat","base_url":"http://localhost:1/v1","models":[{"id":"test-model","name":"Test Model"}]}},"models":{"large":{"provider":"test","model":"test-model"},"small":{"provider":"test","model":"test-model"}},"options":{` + strings.Join(optionFields, ",") + `}}`
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(content), 0o644))

	store, err := config.Load(workingDir, filepath.Join(workingDir, ".crush"), false)
	require.NoError(t, err)
	return store
}
