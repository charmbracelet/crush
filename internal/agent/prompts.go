package agent

import (
	"context"
	_ "embed"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/subagents"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

//go:embed templates/subagent.md.tpl
var subagentPromptTmpl []byte

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

func resolvePreloadedSkillsXML(skillNames []string, activeSkills []*skills.Skill) string {
	if len(skillNames) == 0 {
		return ""
	}
	byName := make(map[string]*skills.Skill, len(activeSkills))
	for _, s := range activeSkills {
		byName[s.Name] = s
	}
	var parts []string
	for _, name := range skillNames {
		s, ok := byName[name]
		if !ok {
			slog.Warn("Subagent references unknown skill", "skill", name)
			continue
		}
		if s.DisableModelInvocation {
			slog.Warn("Subagent references skill with disable-model-invocation, skipping", "skill", name)
			continue
		}
		parts = append(parts, s.FormatInvocation())
	}
	return strings.Join(parts, "\n")
}

func subagentPrompt(sa *subagents.Subagent, activeSkills []*skills.Skill, opts ...prompt.Option) (*prompt.Prompt, error) {
	preloadedXML := resolvePreloadedSkillsXML(sa.Skills, activeSkills)
	allOpts := make([]prompt.Option, 0, len(opts)+2)
	allOpts = append(allOpts, prompt.WithSubagentBody(sa.Body), prompt.WithPreloadedSkillsXML(preloadedXML))
	allOpts = append(allOpts, opts...)
	return prompt.NewPrompt("subagent", string(subagentPromptTmpl), allOpts...)
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
