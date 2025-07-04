package models

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type ModelListComponent struct {
	list      list.ListModel
	modelType int
}

func NewModelListComponent(keyMap list.KeyMap, inputStyle lipgloss.Style) *ModelListComponent {
	modelList := list.New(
		list.WithFilterable(true),
		list.WithKeyMap(keyMap),
		list.WithInputStyle(inputStyle),
		list.WithWrapNavigation(true),
	)

	return &ModelListComponent{
		list:      modelList,
		modelType: LargeModelType,
	}
}

func (m *ModelListComponent) Init() tea.Cmd {
	return tea.Batch(m.list.Init(), m.SetModelType(m.modelType))
}

func (m *ModelListComponent) Update(msg tea.Msg) (*ModelListComponent, tea.Cmd) {
	u, cmd := m.list.Update(msg)
	m.list = u.(list.ListModel)
	return m, cmd
}

func (m *ModelListComponent) View() tea.View {
	return m.list.View()
}

func (m *ModelListComponent) SetSize(width, height int) tea.Cmd {
	return m.list.SetSize(width, height)
}

func (m *ModelListComponent) Items() []util.Model {
	return m.list.Items()
}

func (m *ModelListComponent) SelectedIndex() int {
	return m.list.SelectedIndex()
}

func (m *ModelListComponent) SetModelType(modelType int) tea.Cmd {
	m.modelType = modelType

	providers := config.Providers()
	modelItems := []util.Model{}
	selectIndex := 0

	cfg := config.Get()
	var currentModel config.PreferredModel
	if m.modelType == LargeModelType {
		currentModel = cfg.Models.Large
	} else {
		currentModel = cfg.Models.Small
	}

	addedProviders := make(map[provider.InferenceProvider]bool)

	knownProviders := provider.KnownProviders()
	for providerID, providerConfig := range cfg.Providers {
		if providerConfig.Disabled {
			continue
		}

		// Check if this provider is not in the known providers list
		if !slices.Contains(knownProviders, providerID) {
			configProvider := provider.Provider{
				Name:   string(providerID),
				ID:     providerID,
				Models: make([]provider.Model, len(providerConfig.Models)),
			}

			for i, model := range providerConfig.Models {
				configProvider.Models[i] = provider.Model{
					ID:                     model.ID,
					Name:                   model.Name,
					CostPer1MIn:            model.CostPer1MIn,
					CostPer1MOut:           model.CostPer1MOut,
					CostPer1MInCached:      model.CostPer1MInCached,
					CostPer1MOutCached:     model.CostPer1MOutCached,
					ContextWindow:          model.ContextWindow,
					DefaultMaxTokens:       model.DefaultMaxTokens,
					CanReason:              model.CanReason,
					HasReasoningEffort:     model.HasReasoningEffort,
					DefaultReasoningEffort: model.ReasoningEffort,
					SupportsImages:         model.SupportsImages,
				}
			}

			name := configProvider.Name
			if name == "" {
				name = string(configProvider.ID)
			}
			modelItems = append(modelItems, commands.NewItemSection(name))
			for _, model := range configProvider.Models {
				modelItems = append(modelItems, completions.NewCompletionItem(model.Name, ModelOption{
					Provider: configProvider,
					Model:    model,
				}))
				if model.ID == currentModel.ModelID && configProvider.ID == currentModel.Provider {
					selectIndex = len(modelItems) - 1
				}
			}
			addedProviders[providerID] = true
		}
	}

	for _, provider := range providers {
		if addedProviders[provider.ID] {
			continue
		}

		if providerConfig, exists := cfg.Providers[provider.ID]; exists && providerConfig.Disabled {
			continue
		}

		name := provider.Name
		if name == "" {
			name = string(provider.ID)
		}
		modelItems = append(modelItems, commands.NewItemSection(name))
		for _, model := range provider.Models {
			modelItems = append(modelItems, completions.NewCompletionItem(model.Name, ModelOption{
				Provider: provider,
				Model:    model,
			}))
			if model.ID == currentModel.ModelID && provider.ID == currentModel.Provider {
				selectIndex = len(modelItems) - 1
			}
		}
	}

	return tea.Sequence(m.list.SetItems(modelItems), m.list.SetSelected(selectIndex))
}

// GetModelType returns the current model type
func (m *ModelListComponent) GetModelType() int {
	return m.modelType
}

