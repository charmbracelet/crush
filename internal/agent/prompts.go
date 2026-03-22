package agent

import (
	"context"
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/explore.md.tpl
var explorePromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

const generalPromptSuffix = `

<subagent_mode>
You are running as a delegated implementation subagent, not as the primary orchestrator.

Subagent rules:
- Execute the assigned bounded task directly with the tools available to you.
- Do not behave like the session planner or claim that you will spin up other subagents.
- Focus on concrete execution, verification, and a concise handoff for the parent agent.
- Keep your final response short and report the key files changed, findings, and verification performed.
</subagent_mode>`

func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func generalPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("general", string(coderPromptTmpl)+generalPromptSuffix, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func explorePrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("explore", string(explorePromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func promptForAgent(agentCfg config.Agent, isSubAgent bool, opts ...prompt.Option) (*prompt.Prompt, error) {
	if !isSubAgent {
		return coderPrompt(opts...)
	}

	switch agentCfg.ID {
	case config.AgentExplore:
		return explorePrompt(opts...)
	case config.AgentCoder, config.AgentGeneral:
		return generalPrompt(opts...)
	default:
		if isReadOnlyAgent(agentCfg) {
			return explorePrompt(opts...)
		}
		return generalPrompt(opts...)
	}
}

func isReadOnlyAgent(agentCfg config.Agent) bool {
	if len(agentCfg.AllowedTools) == 0 {
		return false
	}
	readOnlyTools := map[string]struct{}{
		"glob":        {},
		"grep":        {},
		"ls":          {},
		"sourcegraph": {},
		"view":        {},
	}
	for _, tool := range agentCfg.AllowedTools {
		if _, ok := readOnlyTools[tool]; !ok {
			return false
		}
	}
	return true
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
