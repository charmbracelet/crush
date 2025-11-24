package model

import (
	"cmp"
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
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

func (m *UI) lspInfo(t *styles.Styles, width, height int) string {
	var lsps []common.LSPInfo

	for _, state := range m.lspStates {
		client, ok := m.com.App.LSPClients.Get(state.Name)
		if !ok {
			continue
		}
		lspErrs := map[protocol.DiagnosticSeverity]int{
			protocol.SeverityError:       0,
			protocol.SeverityWarning:     0,
			protocol.SeverityHint:        0,
			protocol.SeverityInformation: 0,
		}

		for _, diagnostics := range client.GetDiagnostics() {
			for _, diagnostic := range diagnostics {
				if severity, ok := lspErrs[diagnostic.Severity]; ok {
					lspErrs[diagnostic.Severity] = severity + 1
				}
			}
		}

		lsps = append(lsps, common.LSPInfo{LSPClientInfo: state, Diagnostics: lspErrs})
	}
	title := t.Subtle.Render("LSPs")
	list := t.Subtle.Render("None")
	if len(lsps) > 0 {
		height = max(0, height-2) // remove title and space
		list = common.LspList(t, lsps, width, height)
	}

	return fmt.Sprintf("%s\n\n%s", title, list)
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
	infoSection := lipgloss.JoinVertical(lipgloss.Left, parts...)

	_, remainingHeightArea := uv.SplitVertical(m.layout.main, uv.Fixed(lipgloss.Height(infoSection)+1))

	mcpLspSectionWidth := min(30, width/2)

	lspSection := m.lspInfo(t, mcpLspSectionWidth, remainingHeightArea.Dy())

	content := lipgloss.JoinHorizontal(lipgloss.Left, lspSection)

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy() - 1).
		PaddingTop(1).
		Render(
			lipgloss.JoinVertical(lipgloss.Left, infoSection, "", content),
		)
}
