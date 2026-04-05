package dialog

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// ModelType represents the type of model to select.
type ModelType int

const (
	ModelTypeLarge ModelType = iota
	ModelTypeSmall
	ModelTypeBackground
	ModelTypeAutoClassifier
)

// String returns the string representation of the [ModelType].
func (mt ModelType) String() string {
	switch mt {
	case ModelTypeLarge:
		return "Large"
	case ModelTypeSmall:
		return "Small"
	case ModelTypeBackground:
		return "Background"
	case ModelTypeAutoClassifier:
		return "Auto Classifier"
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
	case ModelTypeBackground:
		return config.SelectedModelTypeBackground
	case ModelTypeAutoClassifier:
		return config.SelectedModelTypeAutoClassifier
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
	case ModelTypeBackground:
		return backgroundModelInputPlaceholder
	case ModelTypeAutoClassifier:
		return autoClassifierModelInputPlaceholder
	default:
		return ""
	}
}

const (
	onboardingModelInputPlaceholder     = "Find your fave"
	largeModelInputPlaceholder          = "Choose a model for large, complex tasks"
	smallModelInputPlaceholder          = "Choose a model for small, simple tasks"
	backgroundModelInputPlaceholder     = "Choose a model for background tasks (memory extraction, handoff generation)"
	autoClassifierModelInputPlaceholder = "Choose a model for Auto Mode permission review"
)

// ModelsID is the identifier for the model selection dialog.
const ModelsID = "models"

const defaultModelsDialogMaxWidth = 73

const modelSummaryContentHeight = 4

// Models represents a model selection dialog.
type Models struct {
	com          *common.Common
	isOnboarding bool

	modelType ModelType
	providers []catwalk.Provider

	keyMap struct {
		Tab           key.Binding
		UpDown        key.Binding
		Apply         key.Binding
		ApplyAndClose key.Binding
		Edit          key.Binding
		Next          key.Binding
		Previous      key.Binding
		Close         key.Binding
	}
	list  *ModelsList
	input textinput.Model
	help  help.Model

	spinner spinner.Model
	loading bool
}

var (
	_ Dialog        = (*Models)(nil)
	_ LoadingDialog = (*Models)(nil)
)

// NewModels creates a new Models dialog.
func NewModels(com *common.Common, isOnboarding bool) (*Models, error) {
	t := com.Styles
	m := &Models{}
	m.com = com
	m.isOnboarding = isOnboarding

	help := help.New()
	help.Styles = t.DialogHelpStyles()

	m.help = help
	m.list = NewModelsList(t)
	m.list.Focus()
	m.list.SetSelected(0)

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = onboardingModelInputPlaceholder
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = com.Styles.Dialog.Spinner
	m.spinner = s
	m.modelType = m.firstSelectableModelType()

	m.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "toggle type"),
	)
	m.keyMap.Apply = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply"),
	)
	m.keyMap.ApplyAndClose = key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", "done"),
	)
	m.keyMap.Edit = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "edit"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("up/down", "choose"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("down", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("up", "previous item"),
	)
	m.keyMap.Close = CloseKey

	var err error
	m.providers, err = config.Providers(m.com.Config())
	if err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}

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
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		if m.loading {
			if key.Matches(msg, m.keyMap.Close) {
				return ActionClose{}
			}
			return nil
		}
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
			} else {
				m.list.SelectPrev()
			}
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
			} else {
				m.list.SelectNext()
			}
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Apply, m.keyMap.ApplyAndClose, m.keyMap.Edit):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}

			modelItem, ok := selectedItem.(*ModelItem)
			if !ok {
				break
			}

			isEdit := key.Matches(msg, m.keyMap.Edit)
			closeDialog := key.Matches(msg, m.keyMap.ApplyAndClose)

			return ActionSelectModel{
				Provider:       modelItem.prov,
				Model:          modelItem.SelectedModel(),
				ModelType:      modelItem.SelectedModelType(),
				ReAuthenticate: isEdit,
				CloseDialog:    closeDialog,
			}
		case key.Matches(msg, m.keyMap.Tab):
			if m.isOnboarding {
				break
			}
			m.modelType = m.nextModelType(m.modelType)
			if err := m.setProviderItems(); err != nil {
				return util.ReportError(err)
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
	if m.loading {
		return nil
	}
	cur := InputCursor(m.com.Styles, m.input.Cursor())
	if cur != nil && !m.isOnboarding {
		cur.Y += modelSummaryContentHeight
	}
	return cur
}

// modelTypeRadioView returns the radio view for model type selection.
func (m *Models) modelTypeRadioView() string {
	t := m.com.Styles
	textStyle := t.HalfMuted
	largeRadioStyle := t.RadioOff
	smallRadioStyle := t.RadioOff
	handoffRadioStyle := t.RadioOff
	autoClassifierRadioStyle := t.RadioOff
	if m.modelType == ModelTypeLarge {
		largeRadioStyle = t.RadioOn
	} else if m.modelType == ModelTypeSmall {
		smallRadioStyle = t.RadioOn
	} else if m.modelType == ModelTypeBackground {
		handoffRadioStyle = t.RadioOn
	} else {
		autoClassifierRadioStyle = t.RadioOn
	}

	largeRadio := largeRadioStyle.Padding(0, 1).Render()
	smallRadio := smallRadioStyle.Padding(0, 1).Render()
	handoffRadio := handoffRadioStyle.Padding(0, 1).Render()
	autoClassifierRadio := autoClassifierRadioStyle.Padding(0, 1).Render()

	return fmt.Sprintf("%s%s  %s%s  %s%s  %s%s",
		largeRadio, textStyle.Render(ModelTypeLarge.String()),
		smallRadio, textStyle.Render(ModelTypeSmall.String()),
		handoffRadio, textStyle.Render(ModelTypeBackground.String()),
		autoClassifierRadio, textStyle.Render(ModelTypeAutoClassifier.String()))
}

func (m *Models) currentModelSummaryView(width int) string {
	lines := []string{
		m.currentModelSummaryLine(ModelTypeLarge, width),
		m.currentModelSummaryLine(ModelTypeSmall, width),
		m.currentModelSummaryLine(ModelTypeBackground, width),
		m.currentModelSummaryLine(ModelTypeAutoClassifier, width),
	}
	return m.com.Styles.Dialog.SecondaryText.Width(width).Render(strings.Join(lines, "\n"))
}

func (m *Models) currentModelSummaryLine(modelType ModelType, width int) string {
	t := m.com.Styles
	radioStyle := t.RadioOff
	labelStyle := t.HalfMuted
	if m.modelType == modelType {
		radioStyle = t.RadioOn
		labelStyle = t.Base
	}

	line := fmt.Sprintf(
		"%s%s: %s",
		radioStyle.Padding(0, 1).Render(),
		labelStyle.Render(modelType.String()),
		t.Subtle.Render(m.selectedModelSummary(modelType)),
	)
	return ansi.Truncate(line, max(0, width), "")
}

func (m *Models) selectedModelSummary(modelType ModelType) string {
	selectedModel := m.com.Config().Models[modelType.Config()]
	if selectedModel.Model == "" && selectedModel.Provider == "" {
		return "Not configured"
	}

	modelName, providerName := m.lookupSelectedModelDisplay(selectedModel.Provider, selectedModel.Model)
	if providerName == "" {
		return modelName
	}
	return fmt.Sprintf("%s | %s", modelName, providerName)
}

func (m *Models) lookupSelectedModelDisplay(providerID, modelID string) (string, string) {
	if providerID == "" || modelID == "" {
		return "Not configured", ""
	}

	if providerConfig, ok := m.com.Config().Providers.Get(providerID); ok {
		providerName := cmp.Or(providerConfig.Name, providerID)
		for _, model := range providerConfig.Models {
			if model.ID == modelID {
				return cmp.Or(model.Name, model.ID), providerName
			}
		}
	}

	for _, provider := range m.providers {
		if string(provider.ID) != providerID {
			continue
		}

		providerName := cmp.Or(string(provider.Name), providerID)
		for _, model := range provider.Models {
			if model.ID == modelID {
				return cmp.Or(model.Name, model.ID), providerName
			}
		}

		return modelID, providerName
	}

	return modelID, providerID
}

// Draw implements [Dialog].
func (m *Models) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultModelsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	if !m.isOnboarding {
		heightOffset += modelSummaryContentHeight
	}

	m.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding
	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Model"
	rc.TitleInfo = ""

	if m.isOnboarding {
		titleText := t.Dialog.PrimaryText.Render("To start, let's choose a provider and model.")
		rc.AddPart(titleText)
	}

	if !m.isOnboarding {
		rc.AddPart(m.currentModelSummaryView(innerWidth))
	}

	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	rc.AddPart(listView)

	rc.Help = m.help.View(m)
	if m.loading {
		rc.Help = m.spinner.View() + " Preparing model switch..."
	}

	cur := m.Cursor()

	if m.isOnboarding {
		rc.Title = ""
		rc.TitleInfo = ""
		rc.IsOnboarding = true
		view := rc.Render()
		DrawOnboardingCursor(scr, area, view, cur)

		// FIXME(@andreynering): Figure it out how to properly fix this
		if cur != nil {
			cur.Y -= 1
			cur.X -= 1
		}
	} else {
		view := rc.Render()
		DrawCenterCursor(scr, area, view, cur)
	}
	return cur
}

// ShortHelp returns the short help view.
func (m *Models) ShortHelp() []key.Binding {
	if m.isOnboarding {
		return []key.Binding{
			m.keyMap.UpDown,
			m.keyMap.Apply,
			m.keyMap.ApplyAndClose,
		}
	}
	h := []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Tab,
		m.keyMap.Apply,
		m.keyMap.ApplyAndClose,
	}
	if m.isSelectedConfigured() {
		h = append(h, m.keyMap.Edit)
	}
	h = append(h, m.keyMap.Close)
	return h
}

// FullHelp returns the full help view.
func (m *Models) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// StartLoading implements [LoadingDialog].
func (m *Models) StartLoading() tea.Cmd {
	if m.loading {
		return nil
	}
	m.loading = true
	return m.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (m *Models) StopLoading() {
	m.loading = false
}

func (m *Models) isSelectedConfigured() bool {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return false
	}
	modelItem, ok := selectedItem.(*ModelItem)
	if !ok {
		return false
	}
	providerID := string(modelItem.prov.ID)
	_, isConfigured := m.com.Config().Providers.Get(providerID)
	return isConfigured
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
				item := NewModelItem(t, provider, model, m.modelType, false, model.ID == currentModel.Model && string(provider.ID) == currentModel.Provider)
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

	// Move "Charm Hyper" to first position.
	// (But still after recent models and custom providers).
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
				model.Name = cmp.Or(model.Name, model.ID)
				displayProvider.Models = append(displayProvider.Models, model)
				modelIndex[model.ID] = len(displayProvider.Models) - 1
			}
		}

		name := cmp.Or(displayProvider.Name, providerID)

		group := NewModelGroup(t, name, providerConfigured)
		for _, model := range displayProvider.Models {
			item := NewModelItem(t, provider, model, m.modelType, false, model.ID == currentModel.Model && string(provider.ID) == currentModel.Provider)
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
			item = NewModelItem(t, item.prov, item.model, m.modelType, true, recent.Model == currentModel.Model && recent.Provider == currentModel.Provider)
			item.showProvider = true

			validRecentItems = append(validRecentItems, recent)
			recentGroup.AppendItems(item)
			if recent.Model == currentModel.Model && recent.Provider == currentModel.Provider {
				selectedItemID = item.ID()
			}
		}

		if len(validRecentItems) != len(recentItems) {
			// FIXME: Does this need to be here? Is it mutating the config during a read?
			if err := m.com.Store().SetConfigField(config.ScopeGlobal, fmt.Sprintf("recent_models.%s", selectedType), validRecentItems); err != nil {
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
	m.list.ScrollToTop()

	// Update placeholder based on model type
	if !m.isOnboarding {
		m.input.Placeholder = m.modelType.Placeholder()
	}

	return nil
}

func (m *Models) firstSelectableModelType() ModelType {
	for _, modelType := range []ModelType{ModelTypeLarge, ModelTypeSmall, ModelTypeBackground, ModelTypeAutoClassifier} {
		selected := m.com.Config().Models[modelType.Config()]
		if selected.Provider == "" || selected.Model == "" {
			return modelType
		}
	}
	return ModelTypeLarge
}

func (m *Models) nextModelType(current ModelType) ModelType {
	switch current {
	case ModelTypeLarge:
		return ModelTypeSmall
	case ModelTypeSmall:
		return ModelTypeBackground
	case ModelTypeBackground:
		return ModelTypeAutoClassifier
	default:
		return ModelTypeLarge
	}
}

func (m *Models) HandleModelApplied(modelType config.SelectedModelType) error {
	if m.modelType.Config() == modelType {
		for i := 0; i < 5; i++ {
			next := m.nextModelType(m.modelType)
			selected := m.com.Config().Models[next.Config()]
			if selected.Provider == "" || selected.Model == "" {
				m.modelType = next
				break
			}
			if next == m.modelType {
				break
			}
			m.modelType = next
		}
	}
	return m.setProviderItems()
}

func modelKey(providerID, modelID string) string {
	if providerID == "" || modelID == "" {
		return ""
	}
	return providerID + ":" + modelID
}
