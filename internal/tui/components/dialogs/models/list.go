package models

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
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
			if hasAPIKeyEnv && p.ID != catwalk.InferenceProviderAzure {
				filteredProviders = append(filteredProviders, p)
			}
		}

		m.providers = filteredProviders
		if err != nil {
			cmds = append(cmds, util.ReportError(err))
		}
	}
	cmds = append(cmds, m.list.Init(), m.SetModelType(m.modelType, ""))
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

func (m *ModelListComponent) SetModelType(modelType int, selectedID string) tea.Cmd {
	t := styles.CurrentTheme()
	m.modelType = modelType

	favGroup := list.Group[list.CompletionItem[ModelOption]]{
		Section: list.NewItemSection("Favorites"),
	}
	hasPreselectedID := selectedID != ""
	selectedItemID := selectedID
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

	allFavoritedModels := config.Get().FavoritedModels
	favoriteModelsByType := allFavoritedModels[selectedType]
	favoriteModels := make([]string, len(favoriteModelsByType))
	for i, fm := range favoriteModelsByType {
		favoriteModels[i] = fm.Model
	}
	favoriteModelsProviders := make([]string, len(favoriteModelsByType))
	for i, fmp := range favoriteModelsByType {
		favoriteModelsProviders[i] = fmp.Provider
	}

	configuredIcon := t.S().Base.Foreground(t.Success).Render(styles.CheckIcon)
	configuredInfo := fmt.Sprintf("%s %s", configuredIcon, t.S().Subtle.Render("Configured"))

	var configuredProviderGroups []list.Group[list.CompletionItem[ModelOption]]
	var unconfiguredProviderGroups []list.Group[list.CompletionItem[ModelOption]]
	addedProviders := make(map[string]bool)

	knownProviders, err := config.Providers(cfg)
	if err != nil {
		return util.ReportError(err)
	}
	for providerID, providerConfig := range cfg.Providers.Seq2() {
		if providerConfig.Disable {
			continue
		}

		isCustomProvider := !slices.ContainsFunc(knownProviders, func(p catwalk.Provider) bool { return p.ID == catwalk.InferenceProvider(providerID) })

		if isCustomProvider {
			configProvider := catwalk.Provider{Name: providerConfig.Name, ID: catwalk.InferenceProvider(providerID), Models: providerConfig.Models}
			name := cmp.Or(configProvider.Name, string(configProvider.ID))
			section := list.NewItemSection(name)
			section.SetInfo(configuredInfo)
			group := list.Group[list.CompletionItem[ModelOption]]{Section: section}
			favoriteCount := 0

			for _, model := range configProvider.Models {
				isFavorite := slices.Contains(favoriteModels, model.ID) && slices.Contains(favoriteModelsProviders, string(configProvider.ID))
				modelName := model.Name
				if isFavorite {
					modelName = " ✦ " + modelName
				}
				modelOption := ModelOption{Provider: configProvider, Model: model}
				key := modelKey(string(configProvider.ID), model.ID)
				item := list.NewCompletionItem(modelName, modelOption, list.WithCompletionID(key))
				itemsByKey[key] = item

				if isFavorite {
					favoriteCount++
					favGroup.Items = append(favGroup.Items, item)
				} else {
					group.Items = append(group.Items, item)
				}

				if !hasPreselectedID && selectedItemID == "" && model.ID == currentModel.Model && string(configProvider.ID) == currentModel.Provider {
					selectedItemID = item.ID()
				}
			}

			// Only add the group if not all models were favorites
			if favoriteCount < len(configProvider.Models) {
				configuredProviderGroups = append(configuredProviderGroups, group)
			}
			addedProviders[providerID] = true
		}
	}

	for _, provider := range m.providers {
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
				if idx, ok := modelIndex[model.ID]; !ok {
					displayProvider.Models = append(displayProvider.Models, model)
				} else if model.Name != "" {
					displayProvider.Models[idx].Name = model.Name
				}
			}
		}

		name := cmp.Or(displayProvider.Name, string(displayProvider.ID))
		section := list.NewItemSection(name)
		group := list.Group[list.CompletionItem[ModelOption]]{Section: section}
		favoriteCount := 0

		for _, model := range displayProvider.Models {
			isFavorite := slices.Contains(favoriteModels, model.ID) && slices.Contains(favoriteModelsProviders, string(displayProvider.ID))
			modelName := model.Name
			if isFavorite {
				modelName = " ✦ " + modelName
			}
			modelOption := ModelOption{Provider: displayProvider, Model: model}
			key := modelKey(string(displayProvider.ID), model.ID)
			item := list.NewCompletionItem(modelName, modelOption, list.WithCompletionID(key))
			itemsByKey[key] = item

			if isFavorite {
				favoriteCount++
				favGroup.Items = append(favGroup.Items, item)
			} else {
				group.Items = append(group.Items, item)
			}

			if !hasPreselectedID && selectedItemID == "" && model.ID == currentModel.Model && string(displayProvider.ID) == currentModel.Provider {
				selectedItemID = item.ID()
			}
		}

		if favoriteCount < len(displayProvider.Models) {
			if providerConfigured {
				section.SetInfo(configuredInfo)
				configuredProviderGroups = append(configuredProviderGroups, group)
			} else {
				unconfiguredProviderGroups = append(unconfiguredProviderGroups, group)
			}
		}
	}

	var finalGroups []list.Group[list.CompletionItem[ModelOption]]

	if len(recentItems) > 0 {
		recentGroup := list.Group[list.CompletionItem[ModelOption]]{Section: list.NewItemSection("Recently used")}
		var validRecentItems []config.SelectedModel
		for _, recent := range recentItems {
			if option, ok := itemsByKey[modelKey(recent.Provider, recent.Model)]; ok {
				validRecentItems = append(validRecentItems, recent)
				recentID := fmt.Sprintf("recent::%s", modelKey(recent.Provider, recent.Model)) // Keep recent:: prefix for unique ID
				modelOption := option.Value()
				providerName := cmp.Or(modelOption.Provider.Name, string(modelOption.Provider.ID))
				item := list.NewCompletionItem(modelOption.Model.Name, option.Value(), list.WithCompletionID(recentID), list.WithCompletionShortcut(providerName))
				recentGroup.Items = append(recentGroup.Items, item)
				if !hasPreselectedID && recent.Model == currentModel.Model && recent.Provider == currentModel.Provider {
					selectedItemID = recentID
				}
			}
		}
		if len(validRecentItems) != len(recentItems) {
			_ = cfg.SetConfigField(fmt.Sprintf("recent_models.%s", selectedType), validRecentItems)
		}
		if len(recentGroup.Items) > 0 {
			finalGroups = append(finalGroups, recentGroup)
		}
	}

	if len(favGroup.Items) > 0 {
		finalGroups = append(finalGroups, favGroup)
	}

	if len(configuredProviderGroups) > 0 {
		finalGroups = append(finalGroups, configuredProviderGroups...)
	}

	if len(unconfiguredProviderGroups) > 0 {
		finalGroups = append(finalGroups, unconfiguredProviderGroups...)
	}

	var cmds []tea.Cmd
	cmd := m.list.SetGroups(finalGroups)
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
