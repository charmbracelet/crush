package dialog

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
)

// ModelType represents the type of model to select.
type ModelType int

const (
	ModelTypeLarge ModelType = iota
	ModelTypeSmall
)

// String returns the string representation of the [ModelType].
func (mt ModelType) String() string {
	switch mt {
	case ModelTypeLarge:
		return "Large Task"
	case ModelTypeSmall:
		return "Small Task"
	default:
		return "Unknown"
	}
}

// Config returns the corresponding config model type.
func (mt ModelType) Config() config.SelectedModelType {
	switch mt {
	case ModelTypeLarge:
		return config.SelectedModelTypeLarge
	case ModelTypeSmall:
		return config.SelectedModelTypeSmall
	default:
		return ""
	}
}

// Placeholder returns the input placeholder for the model type.
func (mt ModelType) Placeholder() string {
	switch mt {
	case ModelTypeLarge:
		return largeModelInputPlaceholder
	case ModelTypeSmall:
		return smallModelInputPlaceholder
	default:
		return ""
	}
}

const (
	largeModelInputPlaceholder = "Choose a model for large, complex tasks"
	smallModelInputPlaceholder = "Choose a model for small, simple tasks"
)

// ModelsID is the identifier for the model selection dialog.
const ModelsID = "models"

const defaultModelsDialogMaxWidth = 70

// Models represents a model selection dialog.
type Models struct {
	com *common.Common

	modelType ModelType
	providers []catwalk.Provider

	keyMap struct {
		Tab      key.Binding
		UpDown   key.Binding
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
	list  *ModelsList
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*Models)(nil)

// NewModels creates a new Models dialog.
func NewModels(com *common.Common) (*Models, error) {
	t := com.Styles
	m := &Models{}
	m.com = com
	help := help.New()
	help.Styles = t.DialogHelpStyles()

	m.help = help
	m.list = NewModelsList(t)
	m.list.Focus()
	m.list.SetSelected(0)

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = largeModelInputPlaceholder
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "toggle type"),
	)
	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.Close = CloseKey

	providers, err := getFilteredProviders(com.Config())
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}

	m.providers = providers
	if err := m.setProviderItems(); err != nil {
		return nil, fmt.Errorf("failed to set provider items: %w", err)
	}

	return m, nil
}

// ID implements Dialog.
func (m *Models) ID() string {
	return ModelsID
}

// HandleMsg implements Dialog.
func (m *Models) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}

			modelItem, ok := selectedItem.(*ModelItem)
			if !ok {
				break
			}

			return ActionSelectModel{
				Provider:  modelItem.prov,
				Model:     modelItem.SelectedModel(),
				ModelType: modelItem.SelectedModelType(),
			}
		case key.Matches(msg, m.keyMap.Tab):
			if m.modelType == ModelTypeLarge {
				m.modelType = ModelTypeSmall
			} else {
				m.modelType = ModelTypeLarge
			}
			if err := m.setProviderItems(); err != nil {
				return uiutil.ReportError(err)
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			m.list.Focus()
			m.list.SetFilter(value)
			m.list.SelectFirst()
			m.list.ScrollToTop()
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor for the dialog.
func (m *Models) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// modelTypeRadioView returns the radio view for model type selection.
func (m *Models) modelTypeRadioView() string {
	t := m.com.Styles
	textStyle := t.HalfMuted
	largeRadioStyle := t.RadioOff
	smallRadioStyle := t.RadioOff
	if m.modelType == ModelTypeLarge {
		largeRadioStyle = t.RadioOn
	} else {
		smallRadioStyle = t.RadioOn
	}

	largeRadio := largeRadioStyle.Padding(0, 1).Render()
	smallRadio := smallRadioStyle.Padding(0, 1).Render()

	return fmt.Sprintf("%s%s  %s%s",
		largeRadio, textStyle.Render(ModelTypeLarge.String()),
		smallRadio, textStyle.Render(ModelTypeSmall.String()))
}

// Draw implements [Dialog].
func (m *Models) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultModelsDialogMaxWidth, area.Dx()))
	height := max(0, min(defaultDialogHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	m.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding
	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Model"
	rc.TitleInfo = m.modelTypeRadioView()
	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	rc.AddPart(listView)
	rc.Help = m.help.View(m)

	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp returns the short help view.
func (m *Models) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Tab,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp returns the full help view.
func (m *Models) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Select,
			m.keyMap.Next,
			m.keyMap.Previous,
			m.keyMap.Tab,
		},
		{
			m.keyMap.Close,
		},
	}
}

// setProviderItems sets the provider items in the list.
func (m *Models) setProviderItems() error {
	t := m.com.Styles
	cfg := m.com.Config()

	var selectedItemID string
	selectedType := m.modelType.Config()
	currentModel := cfg.Models[selectedType]
	recentItems := cfg.RecentModels[selectedType]

	// Track providers already added to avoid duplicates
	addedProviders := make(map[string]bool)

	// Get a list of known providers to compare against
	knownProviders, err := config.Providers(cfg)
	if err != nil {
		return fmt.Errorf("failed to get providers: %w", err)
	}

	containsProviderFunc := func(id string) func(p catwalk.Provider) bool {
		return func(p catwalk.Provider) bool {
			return p.ID == catwalk.InferenceProvider(id)
		}
	}

	// itemsMap contains the keys of added model items.
	itemsMap := make(map[string]*ModelItem)
	groups := []ModelGroup{}
	for id, p := range cfg.Providers.Seq2() {
		if p.Disable {
			continue
		}

		// Check if this provider is not in the known providers list
		if !slices.ContainsFunc(knownProviders, containsProviderFunc(id)) ||
			!slices.ContainsFunc(m.providers, containsProviderFunc(id)) {
			provider := p.ToProvider()

			// Add this unknown provider to the list
			name := cmp.Or(p.Name, id)

			addedProviders[id] = true

			group := NewModelGroup(t, name, true)
			for _, model := range p.Models {
				item := NewModelItem(t, provider, model, m.modelType, false)
				group.AppendItems(item)
				itemsMap[item.ID()] = item
				if model.ID == currentModel.Model && string(provider.ID) == currentModel.Provider {
					selectedItemID = item.ID()
				}
			}
			if len(group.Items) > 0 {
				groups = append(groups, group)
			}
		}
	}

	// Now add known providers from the predefined list
	for _, provider := range m.providers {
		providerID := string(provider.ID)
		if addedProviders[providerID] {
			continue
		}

		providerConfig, providerConfigured := cfg.Providers.Get(providerID)
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
			name = providerID
		}

		group := NewModelGroup(t, name, providerConfigured)
		for _, model := range displayProvider.Models {
			item := NewModelItem(t, provider, model, m.modelType, false)
			group.AppendItems(item)
			itemsMap[item.ID()] = item
			if model.ID == currentModel.Model && string(provider.ID) == currentModel.Provider {
				selectedItemID = item.ID()
			}
		}

		groups = append(groups, group)
	}

	if len(recentItems) > 0 {
		recentGroup := NewModelGroup(t, "Recently used", false)

		var validRecentItems []config.SelectedModel
		for _, recent := range recentItems {
			key := modelKey(recent.Provider, recent.Model)
			item, ok := itemsMap[key]
			if !ok {
				continue
			}

			// Show provider for recent items
			item = NewModelItem(t, item.prov, item.model, m.modelType, true)
			item.showProvider = true

			validRecentItems = append(validRecentItems, recent)
			recentGroup.AppendItems(item)
			if recent.Model == currentModel.Model && recent.Provider == currentModel.Provider {
				selectedItemID = item.ID()
			}
		}

		if len(validRecentItems) != len(recentItems) {
			// FIXME: Does this need to be here? Is it mutating the config during a read?
			if err := cfg.SetConfigField(fmt.Sprintf("recent_models.%s", selectedType), validRecentItems); err != nil {
				return fmt.Errorf("failed to update recent models: %w", err)
			}
		}

		if len(recentGroup.Items) > 0 {
			groups = append([]ModelGroup{recentGroup}, groups...)
		}
	}

	// Set model groups in the list.
	m.list.SetGroups(groups...)
	m.list.SetSelectedItem(selectedItemID)
	m.list.ScrollToSelected()

	// Update placeholder based on model type
	m.input.Placeholder = m.modelType.Placeholder()

	return nil
}

func getFilteredProviders(cfg *config.Config) ([]catwalk.Provider, error) {
	providers, err := config.Providers(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}
	var filteredProviders []catwalk.Provider
	for _, p := range providers {
		var (
			isAzure         = p.ID == catwalk.InferenceProviderAzure
			isCopilot       = p.ID == catwalk.InferenceProviderCopilot
			isHyper         = string(p.ID) == "hyper"
			hasAPIKeyEnv    = strings.HasPrefix(p.APIKey, "$")
			_, isConfigured = cfg.Providers.Get(string(p.ID))
		)
		if isAzure || isCopilot || isHyper || hasAPIKeyEnv || isConfigured {
			filteredProviders = append(filteredProviders, p)
		}
	}
	return filteredProviders, nil
}

func modelKey(providerID, modelID string) string {
	if providerID == "" || modelID == "" {
		return ""
	}
	return providerID + ":" + modelID
}
