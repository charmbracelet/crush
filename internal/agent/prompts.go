package agent

import (
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
)

//go:embed templates/coder.gotmpl
var coderPromptTmpl []byte

//go:embed templates/task.gotmpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md
var initializePrompt []byte

func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func InitializePrompt() string {
	return string(initializePrompt)
}
