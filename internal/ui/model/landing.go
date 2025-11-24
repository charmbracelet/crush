package model

import (
	"cmp"
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/ui/common"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (m *UI) selectedLargeModel() *agent.Model {
	if m.com.App.AgentCoordinator != nil {
		model := m.com.App.AgentCoordinator.Model()
		return &model
	}
	return nil
}

func (m *UI) landingView() string {
	t := m.com.Styles
	width := m.layout.main.Dx()
	cwd := common.PrettyPath(t, m.com.Config().WorkingDir(), width)

	parts := []string{
		cwd,
	}

	model := m.selectedLargeModel()
	if model != nil && model.CatwalkCfg.CanReason {
		reasoningInfo := ""
		providerConfig, ok := m.com.Config().Providers.Get(model.ModelCfg.Provider)
		if ok {
			switch providerConfig.Type {
			case catwalk.TypeAnthropic:
				if model.ModelCfg.Think {
					reasoningInfo = "Thinking On"
				} else {
					reasoningInfo = "Thinking Off"
				}
			default:
				formatter := cases.Title(language.English, cases.NoLower)
				reasoningEffort := cmp.Or(model.ModelCfg.ReasoningEffort, model.CatwalkCfg.DefaultReasoningEffort)
				reasoningInfo = formatter.String(fmt.Sprintf("Reasoning %s", reasoningEffort))

			}
			parts = append(parts, "", common.ModelInfo(t, model.CatwalkCfg.Name, reasoningInfo, nil, width))
		}
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy() - 1).
		PaddingTop(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}
