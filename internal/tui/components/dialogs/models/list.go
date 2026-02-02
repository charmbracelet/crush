package models

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type listModel = list.FilterableGroupList[list.CompletionItem[ModelOption]]

type ModelListComponent struct {
	list      listModel
	modelType int
	providers []catwalk.Provider
}

func modelKey(providerID, modelID string) string {
	if providerID == "" || modelID == "" {
		return ""
	}
	return providerID + ":" + modelID
}

func NewModelListComponent(keyMap list.KeyMap, inputPlaceholder string, shouldResize bool) *ModelListComponent {
	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	options := []list.ListOption{
		list.WithKeyMap(keyMap),
		list.WithWrapNavigation(),
	}
	if shouldResize {
		options = append(options, list.WithResizeByList())
	}
	modelList := list.NewFilterableGroupedList(
		[]list.Group[list.CompletionItem[ModelOption]]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterPlaceholder(inputPlaceholder),
		list.WithFilterListOptions(
			options...,
		),
	)

	return &ModelListComponent{
		list:      modelList,
		modelType: LargeModelType,
	}
}

func (m *ModelListComponent) Init() tea.Cmd {
	var cmds []tea.Cmd
	if len(m.providers) == 0 {
		cfg := config.Get()
		providers, err := config.Providers(cfg)
		filteredProviders := []catwalk.Provider{}
		for _, p := range providers {
			hasAPIKeyEnv := strings.HasPrefix(p.APIKey, "$")
			isHyper := p.ID == "hyper"
			isCopilot := p.ID == catwalk.InferenceProviderCopilot
			if (hasAPIKeyEnv && p.ID != catwalk.InferenceProviderAzure) || isHyper || isCopilot {
				filteredProviders = append(filteredProviders, p)
			}
		}

		m.providers = filteredProviders
		if err != nil {
			cmds = append(cmds, util.ReportError(err))
		}
	}
	cmds = append(cmds, m.list.Init(), m.SetModelType(m.modelType))
	return tea.Batch(cmds...)
}

func (m *ModelListComponent) Update(msg tea.Msg) (*ModelListComponent, tea.Cmd) {
	u, cmd := m.list.Update(msg)
	m.list = u.(listModel)
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

func (m *ModelListComponent) SelectedModel() *ModelOption {
	s := m.list.SelectedItem()
	if s == nil {
		return nil
	}
	sv := *s
	model := sv.Value()
	return &model
}

func (m *ModelListComponent) SetModelType(modelType int) tea.Cmd {
	t := styles.CurrentTheme()
	m.modelType = modelType

	var groups []list.Group[list.CompletionItem[ModelOption]]
	// first none section
	selectedItemID := ""
	itemsByKey := make(map[string]list.CompletionItem[ModelOption])

	cfg := config.Get()
	var currentModel config.SelectedModel
	selectedType := config.SelectedModelTypeLarge
	if m.modelType == LargeModelType {
		currentModel = cfg.Models[config.SelectedModelTypeLarge]
		selectedType = config.SelectedModelTypeLarge
	} else {
		currentModel = cfg.Models[config.SelectedModelTypeSmall]
		selectedType = config.SelectedModelTypeSmall
	}
	recentItems := cfg.RecentModels[selectedType]

	configuredIcon := t.S().Base.Foreground(t.Success).Render(styles.CheckIcon)
	configured := fmt.Sprintf("%s %s", configuredIcon, t.S().Subtle.Render("Configured"))

	// Create a map to track which providers we've already added
	addedProviders := make(map[string]bool)

	// First, add any configured providers that are not in the known providers list
	// These should appear at the top of the list
	knownProviders, err := config.Providers(cfg)
	if err != nil {
		return util.ReportError(err)
	}
	for providerID, providerConfig := range cfg.Providers.Seq2() {
		if providerConfig.Disable {
			continue
		}

		// Check if this provider is not in the known providers list
		if !slices.ContainsFunc(knownProviders, func(p catwalk.Provider) bool { return p.ID == catwalk.InferenceProvider(providerID) }) ||
			!slices.ContainsFunc(m.providers, func(p catwalk.Provider) bool { return p.ID == catwalk.InferenceProvider(providerID) }) {
			// Convert config provider to provider.Provider format
			configProvider := providerConfig.ToProvider()

			// Add this unknown provider to the list
			name := configProvider.Name
			if name == "" {
				name = string(configProvider.ID)
			}
			section := list.NewItemSection(name)
			section.SetInfo(configured)
			group := list.Group[list.CompletionItem[ModelOption]]{
				Section: section,
			}
			for _, model := range configProvider.Models {
				modelOption := ModelOption{
					Provider: configProvider,
					Model:    model,
				}
				key := modelKey(string(configProvider.ID), model.ID)
				item := list.NewCompletionItem(
					model.Name,
					modelOption,
					list.WithCompletionID(key),
				)
				itemsByKey[key] = item

				group.Items = append(group.Items, item)
				if model.ID == currentModel.Model && string(configProvider.ID) == currentModel.Provider {
					selectedItemID = item.ID()
				}
			}
			groups = append(groups, group)

			addedProviders[providerID] = true
		}
	}

	// Move "Charm Hyper" to first position
	// (but still after recent models and custom providers).
	slices.SortStableFunc(m.providers, func(a, b catwalk.Provider) int {
		switch {
		case a.ID == "hyper":
			return -1
		case b.ID == "hyper":
			return 1
		default:
			return 0
		}
	})

	// Then add the known providers from the predefined list
	for _, provider := range m.providers {
		// Skip if we already added this provider as an unknown provider
		if addedProviders[string(provider.ID)] {
			continue
		}

		providerConfig, providerConfigured := cfg.Providers.Get(string(provider.ID))
		if providerConfigured && providerConfig.Disable {
			continue
		}

		displayProvider := provider
		if providerConfigured {
			displayProvider.Name = cmp.Or(providerConfig.Name, displayProvider.Name)
			modelIndex := make(map[string]int, len(displayProvider.Models))
			for i, model := range displayProvider.Models {
				modelIndex[model.ID] = i
			}
			for _, model := range providerConfig.Models {
				if model.ID == "" {
					continue
				}
				if idx, ok := modelIndex[model.ID]; ok {
					if model.Name != "" {
						displayProvider.Models[idx].Name = model.Name
					}
					continue
				}
				if model.Name == "" {
					model.Name = model.ID
				}
				displayProvider.Models = append(displayProvider.Models, model)
				modelIndex[model.ID] = len(displayProvider.Models) - 1
			}
		}

		name := displayProvider.Name
		if name == "" {
			name = string(displayProvider.ID)
		}

		section := list.NewItemSection(name)
		if providerConfigured {
			section.SetInfo(configured)
		}
		group := list.Group[list.CompletionItem[ModelOption]]{
			Section: section,
		}
		for _, model := range displayProvider.Models {
			modelOption := ModelOption{
				Provider: displayProvider,
				Model:    model,
			}
			key := modelKey(string(displayProvider.ID), model.ID)
			item := list.NewCompletionItem(
				model.Name,
				modelOption,
				list.WithCompletionID(key),
			)
			itemsByKey[key] = item
			group.Items = append(group.Items, item)
			if model.ID == currentModel.Model && string(displayProvider.ID) == currentModel.Provider {
				selectedItemID = item.ID()
			}
		}
		groups = append(groups, group)
	}

	if len(recentItems) > 0 {
		recentSection := list.NewItemSection("Recently used")
		recentGroup := list.Group[list.CompletionItem[ModelOption]]{
			Section: recentSection,
		}
		var validRecentItems []config.SelectedModel
		for _, recent := range recentItems {
			key := modelKey(recent.Provider, recent.Model)
			option, ok := itemsByKey[key]
			if !ok {
				continue
			}
			validRecentItems = append(validRecentItems, recent)
			recentID := fmt.Sprintf("recent::%s", key)
			modelOption := option.Value()
			providerName := modelOption.Provider.Name
			if providerName == "" {
				providerName = string(modelOption.Provider.ID)
			}
			item := list.NewCompletionItem(
				modelOption.Model.Name,
				option.Value(),
				list.WithCompletionID(recentID),
				list.WithCompletionShortcut(providerName),
			)
			recentGroup.Items = append(recentGroup.Items, item)
			if recent.Model == currentModel.Model && recent.Provider == currentModel.Provider {
				selectedItemID = recentID
			}
		}

		if len(validRecentItems) != len(recentItems) {
			if err := cfg.SetConfigField(fmt.Sprintf("recent_models.%s", selectedType), validRecentItems); err != nil {
				return util.ReportError(err)
			}
		}

		if len(recentGroup.Items) > 0 {
			groups = append([]list.Group[list.CompletionItem[ModelOption]]{recentGroup}, groups...)
		}
	}

	var cmds []tea.Cmd

	cmd := m.list.SetGroups(groups)

	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmd = m.list.SetSelected(selectedItemID)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Sequence(cmds...)
}

// GetModelType returns the current model type
func (m *ModelListComponent) GetModelType() int {
	return m.modelType
}

func (m *ModelListComponent) SetInputPlaceholder(placeholder string) {
	m.list.SetInputPlaceholder(placeholder)
}
