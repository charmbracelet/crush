package agent

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/plan.md.tpl
var planPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

//go:embed templates/title.md
var titlePromptTmpl []byte

//go:embed templates/summary.md
var summaryPromptTmpl []byte

// builtinPrompts holds every prompt a user may replace via options.prompts.
// Its keys must cover config.PromptNames, which TestBuiltinPromptsCoverNames
// enforces.
var builtinPrompts = map[string][]byte{
	"coder":      coderPromptTmpl,
	"task":       taskPromptTmpl,
	"plan":       planPromptTmpl,
	"initialize": initializePromptTmpl,
	"title":      titlePromptTmpl,
	"summary":    summaryPromptTmpl,
}

// promptText returns a built-in prompt, or the file options.prompts named in
// its place.
func promptText(store *config.ConfigStore, name string) (string, error) {
	builtin, ok := builtinPrompts[name]
	if !ok {
		return "", fmt.Errorf("unknown prompt %q", name)
	}
	path := store.Config().Options.Prompts[name]
	if path == "" {
		return string(builtin), nil
	}
	data, err := os.ReadFile(prompt.ExpandPath(path, store))
	if err != nil {
		return "", fmt.Errorf("reading %s prompt: %w", name, err)
	}
	return string(data), nil
}

// loadPrompt builds a templated prompt by name, honoring options.prompts.
func loadPrompt(store *config.ConfigStore, name string, opts ...prompt.Option) (*prompt.Prompt, error) {
	text, err := promptText(store, name)
	if err != nil {
		return nil, err
	}
	return prompt.NewPrompt(name, text, opts...)
}

func InitializePrompt(store *config.ConfigStore) (string, error) {
	systemPrompt, err := loadPrompt(store, "initialize")
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", store)
}
