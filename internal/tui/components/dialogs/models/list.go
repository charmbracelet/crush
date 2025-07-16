package models

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type ModelListComponent struct {
	list      list.ListModel
	modelType int
	providers []provider.Provider
}

func NewModelListComponent(keyMap list.KeyMap, inputStyle lipgloss.Style, inputPlaceholder string) *ModelListComponent {
	modelList := list.New(
		list.WithFilterable(true),
		list.WithKeyMap(keyMap),
		list.WithInputStyle(inputStyle),
		list.WithFilterPlaceholder(inputPlaceholder),
		list.WithWrapNavigation(true),
	)

	return &ModelListComponent{
		list:      modelList,
		modelType: LargeModelType,
	}
}

func (m *ModelListComponent) Init() tea.Cmd {
	var cmds []tea.Cmd
	if len(m.providers) == 0 {
		providers, err := config.Providers()
		m.providers = providers
		if err != nil {
			cmds = append(cmds, util.ReportError(err))
		}
	}
	cmds = append(cmds, m.list.Init(), m.SetModelType(m.modelType))
	return tea.Batch(cmds...)
}

func (m *ModelListComponent) Update(msg tea.Msg) (*ModelListComponent, tea.Cmd) {
	u, cmd := m.list.Update(msg)
	m.list = u.(list.ListModel)
	return m, cmd
}

func (m *ModelListComponent) View() string {
	return m.list.View()
}

func (m *ModelListComponent) Cursor() *tea.Cursor {
	return m.list.Cursor()
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
	t := styles.CurrentTheme()
	m.modelType = modelType

	modelItems := []util.Model{}
	selectIndex := 0

	cfg := config.Get()
	var currentModel config.SelectedModel
	if m.modelType == LargeModelType {
		currentModel = cfg.Models[config.SelectedModelTypeLarge]
	} else {
		currentModel = cfg.Models[config.SelectedModelTypeSmall]
	}

	configuredIcon := t.S().Base.Foreground(t.Success).Render(styles.CheckIcon)
	configured := fmt.Sprintf("%s %s", configuredIcon, t.S().Subtle.Render("Configured"))

	// Create a map to track which providers we've already added
	addedProviders := make(map[string]bool)

	// First, add any configured providers that are not in the known providers list
	// These should appear at the top of the list
	knownProviders, err := config.Providers()
	if err != nil {
		return util.ReportError(err)
	}
	for providerID, providerConfig := range cfg.Providers {
		if providerConfig.Disable {
			continue
		}

		// Check if this provider is not in the known providers list
		if !slices.ContainsFunc(knownProviders, func(p provider.Provider) bool { return p.ID == provider.InferenceProvider(providerID) }) {
			// Convert config provider to provider.Provider format
			configProvider := provider.Provider{
				Name:   providerConfig.Name,
				ID:     provider.InferenceProvider(providerID),
				Models: make([]provider.Model, len(providerConfig.Models)),
			}

			// Convert models
			for i, model := range providerConfig.Models {
				configProvider.Models[i] = provider.Model{
					ID:                     model.ID,
					Model:                  model.Model,
					CostPer1MIn:            model.CostPer1MIn,
					CostPer1MOut:           model.CostPer1MOut,
					CostPer1MInCached:      model.CostPer1MInCached,
					CostPer1MOutCached:     model.CostPer1MOutCached,
					ContextWindow:          model.ContextWindow,
					DefaultMaxTokens:       model.DefaultMaxTokens,
					CanReason:              model.CanReason,
					HasReasoningEffort:     model.HasReasoningEffort,
					DefaultReasoningEffort: model.DefaultReasoningEffort,
					SupportsImages:         model.SupportsImages,
				}
			}

			// Add this unknown provider to the list
			name := configProvider.Name
			if name == "" {
				name = string(configProvider.ID)
			}
			section := commands.NewItemSection(name)
			section.SetInfo(configured)
			modelItems = append(modelItems, section)
			for _, model := range configProvider.Models {
				modelItems = append(modelItems, completions.NewCompletionItem(model.Model, ModelOption{
					Provider: configProvider,
					Model:    model,
				}))
				if model.ID == currentModel.Model && string(configProvider.ID) == currentModel.Provider {
					selectIndex = len(modelItems) - 1 // Set the selected index to the current model
				}
			}
			addedProviders[providerID] = true
		}
	}

	// Then add the known providers from the predefined list
	for _, provider := range m.providers {
		// Skip if we already added this provider as an unknown provider
		if addedProviders[string(provider.ID)] {
			continue
		}

		// Check if this provider is configured and not disabled
		if providerConfig, exists := cfg.Providers[string(provider.ID)]; exists && providerConfig.Disable {
			continue
		}

		name := provider.Name
		if name == "" {
			name = string(provider.ID)
		}

		section := commands.NewItemSection(name)
		if _, ok := cfg.Providers[string(provider.ID)]; ok {
			section.SetInfo(configured)
		}
		modelItems = append(modelItems, section)
		for _, model := range provider.Models {
			modelItems = append(modelItems, completions.NewCompletionItem(model.Model, ModelOption{
				Provider: provider,
				Model:    model,
			}))
			if model.ID == currentModel.Model && string(provider.ID) == currentModel.Provider {
				selectIndex = len(modelItems) - 1 // Set the selected index to the current model
			}
		}
	}

	return tea.Sequence(m.list.SetItems(modelItems), m.list.SetSelected(selectIndex))
}

// GetModelType returns the current model type
func (m *ModelListComponent) GetModelType() int {
	return m.modelType
}

func (m *ModelListComponent) SetInputPlaceholder(placeholder string) {
	m.list.SetFilterPlaceholder(placeholder)
}

func (m *ModelListComponent) SetProviders(providers []provider.Provider) {
	m.providers = providers
}
