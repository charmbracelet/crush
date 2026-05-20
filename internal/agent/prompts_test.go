package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// promptStore returns a config store whose options.prompts is `prompts`.
func promptStore(t *testing.T, prompts map[string]string) *config.ConfigStore {
	t.Helper()
	dir := t.TempDir()
	store, err := config.Init(dir, "", false)
	require.NoError(t, err)
	store.Config().Options.Prompts = prompts
	return store
}

// The map in prompts.go and the list config validates against must agree, or a
// name passes validation and then resolves to nothing.
func TestBuiltinPromptsCoverNames(t *testing.T) {
	t.Parallel()

	require.Len(t, builtinPrompts, len(config.PromptNames))
	for _, name := range config.PromptNames {
		require.Contains(t, builtinPrompts, name, "config.PromptNames lists %q but builtinPrompts has no template for it", name)
		require.NotEmpty(t, builtinPrompts[name], "built-in %q prompt is empty", name)
	}
}

func TestPromptTextUsesBuiltinByDefault(t *testing.T) {
	t.Parallel()

	store := promptStore(t, nil)
	for _, name := range config.PromptNames {
		text, err := promptText(store, name)
		require.NoError(t, err)
		require.Equal(t, string(builtinPrompts[name]), text)
	}
}

func TestPromptTextUsesOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "coder.md.tpl")
	require.NoError(t, os.WriteFile(path, []byte("be brief"), 0o644))

	store := promptStore(t, map[string]string{"coder": path})

	text, err := promptText(store, "coder")
	require.NoError(t, err)
	require.Equal(t, "be brief", text)

	// Overriding one prompt leaves the others on their built-in.
	task, err := promptText(store, "task")
	require.NoError(t, err)
	require.Equal(t, string(taskPromptTmpl), task)
}

func TestPromptTextExpandsPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "summary.md"), []byte("sum it up"), 0o644))
	t.Setenv("CRUSH_TEST_PROMPT_DIR", dir)

	store := promptStore(t, map[string]string{"summary": "$CRUSH_TEST_PROMPT_DIR/summary.md"})

	text, err := promptText(store, "summary")
	require.NoError(t, err)
	require.Equal(t, "sum it up", text)
}

func TestPromptTextMissingFileFails(t *testing.T) {
	t.Parallel()

	store := promptStore(t, map[string]string{"coder": filepath.Join(t.TempDir(), "nope.md")})

	_, err := promptText(store, "coder")
	require.ErrorContains(t, err, "reading coder prompt")
}

func TestPromptTextUnknownNameFails(t *testing.T) {
	t.Parallel()

	_, err := promptText(promptStore(t, nil), "codr")
	require.ErrorContains(t, err, `unknown prompt "codr"`)
}

// A malformed override must fail when the prompt is loaded, not on every turn
// that builds it.
func TestLoadPromptRejectsMalformedTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "coder.md.tpl")
	require.NoError(t, os.WriteFile(path, []byte("hi {{.WorkingDir"), 0o644))

	_, err := loadPrompt(promptStore(t, map[string]string{"coder": path}), "coder")
	require.ErrorContains(t, err, "parsing coder prompt template")
}

func TestValidatePromptsRejectsUnknownName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{
		Prompts: config.PromptPaths{"codr": "/tmp/x.md"},
	}}
	require.ErrorContains(t, cfg.ValidatePrompts(), `unknown prompt "codr"`)

	cfg.Options.Prompts = config.PromptPaths{"coder": "/tmp/x.md"}
	require.NoError(t, cfg.ValidatePrompts())
}
