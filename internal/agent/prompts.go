package agent

import (
	"context"
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

func coderPrompt(modelName string, opts ...prompt.Option) (*prompt.Prompt, error) {
	family := DetectModelFamily(modelName)
	opts = append(opts, prompt.WithModelFamily(string(family)))
	return prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	return prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
}

func InitializePrompt(cfg config.Config) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
