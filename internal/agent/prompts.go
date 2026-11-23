package agent

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filepathext"
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

// promptText returns a built-in prompt, or the file that replaces it.
func promptText(store *config.ConfigStore, name string) (string, error) {
	builtin, ok := builtinPrompts[name]
	if !ok {
		return "", fmt.Errorf("unknown prompt %q", name)
	}
	path := promptPath(store, name)
	if path == "" {
		return string(builtin), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s prompt: %w", name, err)
	}
	return string(data), nil
}

// promptPath returns the file replacing the named prompt, or "" for the
// built-in. A --prompt flag beats options.prompts. Config loading already
// anchors relative paths to the config file that set them; anything still
// relative (a $VAR that expands to one) resolves against the workspace.
func promptPath(store *config.ConfigStore, name string) string {
	if path := store.Overrides().Prompts[name]; path != "" {
		return path
	}
	opts := store.Config().Options
	if opts == nil || opts.Prompts[name] == "" {
		return ""
	}
	return filepathext.SmartJoin(store.WorkingDir(), prompt.ExpandPath(opts.Prompts[name], store))
}

// loadPromptTexts reads every prompt once, built-in or replaced, so a
// missing file or a broken template fails at startup, and a file moved or
// deleted mid-session cannot later break compaction, titles, or sub-agents.
func loadPromptTexts(store *config.ConfigStore) (map[string]string, error) {
	texts := make(map[string]string, len(config.PromptNames))
	for _, name := range config.PromptNames {
		text, err := promptText(store, name)
		if err != nil {
			return nil, err
		}
		if templatedPrompts[name] {
			if _, err := prompt.NewPrompt(name, text); err != nil {
				return nil, err
			}
		}
		texts[name] = text
	}
	return texts, nil
}

// promptOrBuiltin returns the named prompt from texts, as loaded by
// loadPromptTexts, or the built-in when texts lacks it, as it does for
// agents built by hand in tests.
func promptOrBuiltin(texts map[string]string, name string) string {
	if text, ok := texts[name]; ok {
		return text
	}
	return string(builtinPrompts[name])
}

// newPrompt builds a templated prompt from the text read at startup.
func (c *coordinator) newPrompt(name string, opts ...prompt.Option) (*prompt.Prompt, error) {
	return prompt.NewPrompt(name, promptOrBuiltin(c.promptTexts, name), opts...)
}

// templatedPrompts are the prompts rendered as Go templates. Title and
// summary are sent verbatim, so a "{{" in them is just text.
var templatedPrompts = map[string]bool{
	"coder":      true,
	"task":       true,
	"plan":       true,
	"initialize": true,
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
