package agent

import (
	"context"
	_ "embed"
	"os"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

func coderPrompt(cfg *config.Config, opts ...prompt.Option) (*prompt.Prompt, error) {
	templateContent := string(coderPromptTmpl)
	if cfg != nil && cfg.Options != nil && cfg.Options.CustomPromptPath != "" {
		data, err := os.ReadFile(home.Long(cfg.Options.CustomPromptPath))
		if err != nil {
			return nil, err
		}
		templateContent = string(data)
	}
	systemPrompt, err := prompt.NewPrompt("coder", templateContent, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(cfg *config.Config, opts ...prompt.Option) (*prompt.Prompt, error) {
	templateContent := string(taskPromptTmpl)
	if cfg != nil && cfg.Options != nil && cfg.Options.CustomPromptPath != "" {
		data, err := os.ReadFile(home.Long(cfg.Options.CustomPromptPath))
		if err != nil {
			return nil, err
		}
		templateContent = string(data)
	}
	systemPrompt, err := prompt.NewPrompt("task", templateContent, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
