package dialog

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
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
	onboardingModelInputPlaceholder = "Find your fave"
	largeModelInputPlaceholder      = "Choose a model for large, complex tasks"
	smallModelInputPlaceholder      = "Choose a model for small, simple tasks"
)

// ModelsID is the identifier for the model selection dialog.
const ModelsID = "models"

const defaultModelsDialogMaxWidth = 73

// Models represents a model selection dialog.
type Models struct {
	com          *common.Common
	isOnboarding bool

	modelType ModelType
	providers []catwalk.Provider
	// providerID khác rỗng: bảng chỉ hiện model của provider này (alt+m).
	providerID string

	keyMap struct {
		Tab       key.Binding
		UpDown    key.Binding
		Select    key.Binding
		Edit      key.Binding
		Providers key.Binding
		Next      key.Binding
		Previous  key.Binding
		Close     key.Binding
	}
	list  *ModelsList
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*Models)(nil)

// ErrNoProviderModels: provider được lọc không có model nào (bị xoá khỏi cấu
// hình, bị tắt, hoặc ID không có trong catalog).
var ErrNoProviderModels = errors.New("provider has no models")

// NewModels creates a new Models dialog.
func NewModels(com *common.Common, isOnboarding bool) (*Models, error) {
	return newModels(com, isOnboarding, "")
}

// NewModelsForProvider tạo bảng model chỉ gồm model của providerID: đổi model
// trong provider đang dùng không phải lội qua cả catalog.
func NewModelsForProvider(com *common.Common, providerID string) (*Models, error) {
	return newModels(com, false, providerID)
}

func newModels(com *common.Common, isOnboarding bool, providerID string) (*Models, error) {
	t := com.Styles
	m := &Models{}
	m.com = com
	m.isOnboarding = isOnboarding
	m.providerID = providerID

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

	m.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "toggle type"),
	)
	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.Edit = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "edit"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Providers = key.NewBinding(
		key.WithKeys("alt+p"),
		key.WithHelp("alt+p", "provider"),
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

	// A stale catalog must not keep this dialog from opening: it is the
	// only way for the user to choose a model.
	var err error
	m.providers, err = config.Providers(m.com.Config())
	if err != nil {
		if len(m.providers) == 0 {
			return nil, fmt.Errorf("failed to get providers: %w", err)
		}
		slog.Warn("Listing the previously known providers", "error", err)
	}

	if err := m.setProviderItems(); err != nil {
		return nil, fmt.Errorf("failed to set provider items: %w", err)
	}
	if providerID != "" && m.list.Len() == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoProviderModels, providerID)
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
		case !m.isOnboarding && key.Matches(msg, m.keyMap.Providers):
			return ActionOpenProviders{}
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
		case key.Matches(msg, m.keyMap.Select, m.keyMap.Edit):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}

			modelItem, ok := selectedItem.(*ModelItem)
			if !ok {
				break
			}

			isEdit := key.Matches(msg, m.keyMap.Edit)

			return ActionSelectModel{
				Provider:       modelItem.prov,
				Model:          modelItem.SelectedModel(),
				ModelType:      modelItem.SelectedModelType(),
				ReAuthenticate: isEdit,
			}
		case key.Matches(msg, m.keyMap.Tab):
			if m.isOnboarding {
				break
			}
			if m.modelType == ModelTypeLarge {
				m.modelType = ModelTypeSmall
			} else {
				m.modelType = ModelTypeLarge
			}
			if err := m.setProviderItems(); err != nil {
				return util.ReportError(err)
			}
		default:
			prevValue := m.input.Value()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			if value != prevValue {
				m.list.Focus()
				m.list.SetFilter(value)
				m.list.SelectFirst()
				m.list.ScrollToTop()
			}
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
	textStyle := t.Radio.Label
	largeRadioStyle := t.Radio.Off
	smallRadioStyle := t.Radio.Off
	if m.modelType == ModelTypeLarge {
		largeRadioStyle = t.Radio.On
	} else {
		smallRadioStyle = t.Radio.On
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
	width := max(0, min(defaultModelsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	m.input.SetWidth(dialogInputTextWidth(t, m.input, innerWidth))

	listHeight, listTotalHeight, _ := sizeDialogList(t, m.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Model"
	if name := m.providerName(); name != "" {
		rc.Title = "Switch Model · " + name
	}
	rc.TitleInfo = m.modelTypeRadioView()

	if m.isOnboarding {
		titleText := t.Dialog.PrimaryText.Render("To start, let's choose a provider and model.")
		rc.AddPart(titleText)
	}

	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	cur := m.Cursor()

	if m.isOnboarding {
		rc.Title = ""
		rc.TitleInfo = ""
		rc.IsOnboarding = true
		view := rc.Render()
		cur = adjustOnboardingInputCursor(t, cur)
		DrawOnboardingCursor(scr, area, view, cur)
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
			m.keyMap.Select,
		}
	}
	h := []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Tab,
		m.keyMap.Select,
	}
	if m.isSelectedConfigured() {
		h = append(h, m.keyMap.Edit)
	}
	h = append(h, m.keyMap.Providers, m.keyMap.Close)
	return h
}

// FullHelp returns the full help view.
func (m *Models) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
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
	// The config merge concatenates arrays, so a project-scoped list can
	// repeat models; keep the first occurrence of each.
	var recentItems []config.SelectedModel
	seenRecent := make(map[string]bool)
	for _, r := range cfg.RecentModels[selectedType] {
		if m.providerID != "" && r.Provider != m.providerID {
			continue
		}
		if k := modelKey(r.Provider, r.Model); !seenRecent[k] {
			seenRecent[k] = true
			recentItems = append(recentItems, r)
		}
	}

	// Track providers already added to avoid duplicates
	addedProviders := make(map[string]bool)

	// Get a list of known providers to compare against
	knownProviders, err := config.Providers(cfg)
	if err != nil && len(knownProviders) == 0 {
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
		if p.Disable || (m.providerID != "" && id != m.providerID) {
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

	// Now add known providers from the predefined list.
	// Providers already has Hyper at the front of the list.
	for _, provider := range m.providers {
		providerID := string(provider.ID)
		if addedProviders[providerID] || (m.providerID != "" && providerID != m.providerID) {
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

		// The OpenAI and xAI providers each hold exactly one credential.
		// Signed in with ChatGPT or Grok, only the models the
		// subscription grants are usable, so they are all the section
		// lists; the API catalog would only 404. Without a login the
		// section is the API catalog.
		if providerConfig.OAuthToken != nil {
			var subscriptionModels []catwalk.Model
			switch provider.ID {
			case catwalk.InferenceProviderOpenAI:
				subscriptionModels = providerConfig.ChatGPTModels
			case catwalk.InferenceProviderXAI:
				subscriptionModels = providerConfig.GrokModels
			}
			if subscriptionModels != nil {
				group := NewModelGroup(t, name, true)
				for _, model := range subscriptionModels {
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
				continue
			}
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

		// Đang lọc theo provider thì danh sách chỉ có một phần: không được coi
		// model gần đây của provider khác là hỏng mà xoá khỏi cấu hình.
		if m.providerID == "" && len(validRecentItems) != len(recentItems) {
			// FIXME: Does this need to be here? Is it mutating the config during a read?
			if err := m.com.Workspace.SetConfigField(config.ScopeGlobal, fmt.Sprintf("recent_models.%s", selectedType), validRecentItems); err != nil {
				return fmt.Errorf("failed to update recent models: %w", err)
			}
		}

		if len(recentGroup.Items) > 0 {
			groups = append([]ModelGroup{recentGroup}, groups...)
		}
	}

	// Ranh giới model (AGENT_CLI_ALLOWED_MODELS, execpolicy): chỉ hiện model
	// được phép; chọn model khác thì coordinator cũng từ chối.
	if pol, err := execpolicy.Current(); err == nil && pol.Restricted() {
		groups = allowedModelGroups(t, groups, pol)
		if selectedItemID != "" && !modelGroupsContain(groups, selectedItemID) {
			selectedItemID = ""
		}
	}

	// Set model groups in the list.
	m.list.SetGroups(groups...)
	if selectedItemID != "" {
		m.list.SetSelectedItem(selectedItemID)
		m.list.ScrollToSelected()
	} else {
		// Model hiện tại không có trong danh sách (vd Tab sang loại model mà
		// provider khác provider đang lọc): chọn lại từ đầu, không giữ chỉ số
		// cũ trỏ vào một dòng bất kỳ như thể đó là model đang dùng.
		m.list.SelectFirst()
		m.list.ScrollToTop()
	}

	// Update placeholder based on model type
	if !m.isOnboarding {
		m.input.Placeholder = m.modelType.Placeholder()
	}

	return nil
}

// allowedModelGroups bỏ model không được chính sách cho phép, và nhóm rỗng.
func allowedModelGroups(t *styles.Styles, groups []ModelGroup, pol execpolicy.Policy) []ModelGroup {
	out := make([]ModelGroup, 0, len(groups))
	for _, g := range groups {
		kept := NewModelGroup(t, g.Title, g.configured)
		for _, item := range g.Items {
			if pol.AllowsModel(string(item.prov.ID), item.model.ID) {
				kept.AppendItems(item)
			}
		}
		if len(kept.Items) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

// modelGroupsContain cho biết có dòng mang id trong các nhóm.
func modelGroupsContain(groups []ModelGroup, id string) bool {
	for _, g := range groups {
		for _, item := range g.Items {
			if item.ID() == id {
				return true
			}
		}
	}
	return false
}

func modelKey(providerID, modelID string) string {
	if providerID == "" || modelID == "" {
		return ""
	}
	return providerID + ":" + modelID
}

// providerName là tên hiển thị của provider đang lọc ("" khi không lọc).
func (m *Models) providerName() string {
	if m.providerID == "" {
		return ""
	}
	if pc, ok := m.com.Config().Providers.Get(m.providerID); ok && pc.Name != "" {
		return pc.Name
	}
	for _, p := range m.providers {
		if string(p.ID) == m.providerID && p.Name != "" {
			return p.Name
		}
	}
	return m.providerID
}
