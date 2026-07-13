package tools

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/skills"
)

const SkillToolName = "skill"

//go:embed skill.md
var skillDescription string

type SkillParams struct {
	Name string `json:"name" description:"Exact skill name from the available_skills metadata"`
}

type SkillResponseMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func NewSkillTool(activeSkills []*skills.Skill, tracker *skills.Tracker) fantasy.AgentTool {
	available := make(map[string]*skills.Skill, len(activeSkills))
	names := make([]string, 0, len(activeSkills))
	for _, skill := range activeSkills {
		if skill == nil || skill.Name == "" || skill.Instructions == "" || skill.DisableModelInvocation {
			continue
		}
		available[skill.Name] = skill
		names = append(names, skill.Name)
	}
	slices.Sort(names)

	return fantasy.NewAgentTool(
		SkillToolName,
		skillDescription,
		func(_ context.Context, params SkillParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			name := strings.TrimSpace(params.Name)
			if name == "" {
				return fantasy.NewTextErrorResponse("name is required; use an exact name from available_skills"), nil
			}

			skill, ok := available[name]
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"skill %q is not available; available skills: %s",
					name,
					strings.Join(names, ", "),
				)), nil
			}

			tracker.MarkLoaded(skill.Name)
			metadata := SkillResponseMetadata{
				Name:        skill.Name,
				Description: skill.Description,
			}
			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(skill.FormatInvocation()),
				metadata,
			), nil
		},
	)
}
