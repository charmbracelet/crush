package models

import (
	"slices"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/models"
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

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model models.Model
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	dialogs.DialogModel
}

type modelDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window

	modelList list.ListModel
	keyMap    KeyMap
	help      help.Model
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
	}
}

var ProviderPopularity = map[models.InferenceProvider]int{
	models.ProviderAnthropic:  1,
	models.ProviderOpenAI:     2,
	models.ProviderGemini:     3,
	models.ProviderGROQ:       4,
	models.ProviderOpenRouter: 5,
	models.ProviderBedrock:    6,
	models.ProviderAzure:      7,
	models.ProviderVertexAI:   8,
	models.ProviderXAI:        9,
}

var ProviderName = map[models.InferenceProvider]string{
	models.ProviderAnthropic:  "Anthropic",
	models.ProviderOpenAI:     "OpenAI",
	models.ProviderGemini:     "Gemini",
	models.ProviderGROQ:       "Groq",
	models.ProviderOpenRouter: "OpenRouter",
	models.ProviderBedrock:    "AWS Bedrock",
	models.ProviderAzure:      "Azure",
	models.ProviderVertexAI:   "VertexAI",
	models.ProviderXAI:        "xAI",
}

func (m *modelDialogCmp) Init() tea.Cmd {
	cfg := config.Get()
	enabledProviders := getEnabledProviders(cfg)

	modelItems := []util.Model{}
	for _, provider := range enabledProviders {
		name, ok := ProviderName[provider]
		if !ok {
			name = string(provider) // Fallback to provider ID if name is not defined
		}
		modelItems = append(modelItems, commands.NewItemSection(name))
		for _, model := range getModelsForProvider(provider) {
			modelItems = append(modelItems, completions.NewCompletionItem(model.Name, model))
		}
	}
	m.modelList.SetItems(modelItems)
	return m.modelList.Init()
}

func (m *modelDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		return m, m.modelList.SetSize(m.listWidth(), m.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Select):
			selectedItemInx := m.modelList.SelectedIndex()
			if selectedItemInx == list.NoSelection {
				return m, nil // No item selected, do nothing
			}
			items := m.modelList.Items()
			selectedItem := items[selectedItemInx].(completions.CompletionItem).Value().(models.Model)

			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ModelSelectedMsg{Model: selectedItem}),
			)
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
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Model", m.width-4)),
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

func GetSelectedModel(cfg *config.Config) models.Model {
	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelID := agentCfg.Model
	return models.SupportedModels[selectedModelID]
}

func getEnabledProviders(cfg *config.Config) []models.InferenceProvider {
	var providers []models.InferenceProvider
	for providerID, provider := range cfg.Providers {
		if !provider.Disabled {
			providers = append(providers, providerID)
		}
	}

	// Sort by provider popularity
	slices.SortFunc(providers, func(a, b models.InferenceProvider) int {
		rA := ProviderPopularity[a]
		rB := ProviderPopularity[b]

		// models not included in popularity ranking default to last
		if rA == 0 {
			rA = 999
		}
		if rB == 0 {
			rB = 999
		}
		return rA - rB
	})
	return providers
}

func getModelsForProvider(provider models.InferenceProvider) []models.Model {
	var providerModels []models.Model
	for _, model := range models.SupportedModels {
		if model.Provider == provider {
			providerModels = append(providerModels, model)
		}
	}

	// reverse alphabetical order (if llm naming was consistent latest would appear first)
	slices.SortFunc(providerModels, func(a, b models.Model) int {
		if a.Name > b.Name {
			return -1
		} else if a.Name < b.Name {
			return 1
		}
		return 0
	})

	return providerModels
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
