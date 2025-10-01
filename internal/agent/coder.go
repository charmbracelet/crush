package agent

import (
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
)

//go:embed templates/coder.gotmpl
var coderPromptTmpl []byte

func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}
