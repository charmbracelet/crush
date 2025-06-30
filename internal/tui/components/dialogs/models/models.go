package models

import (
	"slices"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	ModelsDialogID dialogs.DialogID = "models"

	defaultWidth = 60
)

const (
	LargeModelType int = iota
	SmallModelType
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model     config.PreferredModel
	ModelType config.ModelType
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	dialogs.DialogModel
}

type ModelOption struct {
	Provider provider.Provider
	Model    provider.Model
}

type modelDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	modelList list.ListModel
	keyMap    KeyMap
	help      help.Model
	modelType int
}

func NewModelDialogCmp() ModelDialog {
	listKeyMap := list.DefaultKeyMap()
	keyMap := DefaultKeyMap()

	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)

	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.Padding(0, 1, 0, 1)
	modelList := list.New(
		list.WithFilterable(true),
		list.WithKeyMap(listKeyMap),
		list.WithInputStyle(inputStyle),
		list.WithWrapNavigation(true),
	)
	help := help.New()
	help.Styles = t.S().Help

	return &modelDialogCmp{
		modelList: modelList,
		width:     defaultWidth,
		keyMap:    DefaultKeyMap(),
		help:      help,
		modelType: LargeModelType,
	}
}

func (m *modelDialogCmp) Init() tea.Cmd {
	m.SetModelType(m.modelType)
	return m.modelList.Init()
}

func (m *modelDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.SetModelType(m.modelType)
		return m, m.modelList.SetSize(m.listWidth(), m.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Select):
			selectedItemInx := m.modelList.SelectedIndex()
			if selectedItemInx == list.NoSelection {
				return m, nil
			}
			items := m.modelList.Items()
			selectedItem := items[selectedItemInx].(completions.CompletionItem).Value().(ModelOption)

			var modelType config.ModelType
			if m.modelType == LargeModelType {
				modelType = config.LargeModel
			} else {
				modelType = config.SmallModel
			}

			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ModelSelectedMsg{
					Model: config.PreferredModel{
						ModelID:  selectedItem.Model.ID,
						Provider: selectedItem.Provider.ID,
					},
					ModelType: modelType,
				}),
			)
		case key.Matches(msg, m.keyMap.Tab):
			if m.modelType == LargeModelType {
				return m, m.SetModelType(SmallModelType)
			} else {
				return m, m.SetModelType(LargeModelType)
			}
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := m.modelList.Update(msg)
			m.modelList = u.(list.ListModel)
			return m, cmd
		}
	}
	return m, nil
}

func (m *modelDialogCmp) View() tea.View {
	t := styles.CurrentTheme()
	listView := m.modelList.View()
	radio := m.modelTypeRadio()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Model", m.width-lipgloss.Width(radio)-5)+" "+radio),
		listView.String(),
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	v := tea.NewView(m.style().Render(content))
	if listView.Cursor() != nil {
		c := m.moveCursor(listView.Cursor())
		v.SetCursor(c)
	}
	return v
}

func (m *modelDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *modelDialogCmp) listWidth() int {
	return defaultWidth - 2 // 4 for padding
}

func (m *modelDialogCmp) listHeight() int {
	listHeigh := len(m.modelList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeigh, m.wHeight/2)
}

func (m *modelDialogCmp) Position() (int, int) {
	row := m.wHeight/4 - 2 // just a bit above the center
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *modelDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := m.Position()
	offset := row + 3 // Border + title
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (m *modelDialogCmp) ID() dialogs.DialogID {
	return ModelsDialogID
}

func (m *modelDialogCmp) modelTypeRadio() string {
	t := styles.CurrentTheme()
	choices := []string{"Large Task", "Small Task"}
	iconSelected := "◉"
	iconUnselected := "○"
	if m.modelType == LargeModelType {
		return t.S().Base.Foreground(t.FgHalfMuted).Render(iconSelected + " " + choices[0] + "  " + iconUnselected + " " + choices[1])
	}
	return t.S().Base.Foreground(t.FgHalfMuted).Render(iconUnselected + " " + choices[0] + "  " + iconSelected + " " + choices[1])
}

func (m *modelDialogCmp) SetModelType(modelType int) tea.Cmd {
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

	// Create a map to track which providers we've already added
	addedProviders := make(map[provider.InferenceProvider]bool)

	// First, add any configured providers that are not in the known providers list
	// These should appear at the top of the list
	knownProviders := provider.KnownProviders()
	for providerID, providerConfig := range cfg.Providers {
		if providerConfig.Disabled {
			continue
		}

		// Check if this provider is not in the known providers list
		if !slices.Contains(knownProviders, providerID) {
			// Convert config provider to provider.Provider format
			configProvider := provider.Provider{
				Name:   string(providerID), // Use provider ID as name for unknown providers
				ID:     providerID,
				Models: make([]provider.Model, len(providerConfig.Models)),
			}

			// Convert models
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

			// Add this unknown provider to the list
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
					selectIndex = len(modelItems) - 1 // Set the selected index to the current model
				}
			}
			addedProviders[providerID] = true
		}
	}

	// Then add the known providers from the predefined list
	for _, provider := range providers {
		// Skip if we already added this provider as an unknown provider
		if addedProviders[provider.ID] {
			continue
		}

		// Check if this provider is configured and not disabled
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
				selectIndex = len(modelItems) - 1 // Set the selected index to the current model
			}
		}
	}

	return tea.Sequence(m.modelList.SetItems(modelItems), m.modelList.SetSelected(selectIndex))
}
