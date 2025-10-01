package agent

import (
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
)

//go:embed templates/coder.gotmpl
var coderPromptTmpl []byte

func coderPrompt() (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl))
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}
