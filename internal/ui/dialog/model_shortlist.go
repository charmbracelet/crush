package dialog

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const (
	// ModelShortlistID is the identifier for the curated model step shown
	// during onboarding.
	ModelShortlistID = "model_shortlist"

	modelShortlistDialogMaxWidth = 64
	modelShortlistDialogHeight   = 20
	// modelShortlistChrome is the number of lines the dialog spends on
	// everything but the list: the step label, two copy lines, and the help
	// footer.
	modelShortlistChrome = 5
	// shortlistMaxEntries caps how many curated models are offered. The
	// full catalog stays one row below.
	shortlistMaxEntries = 3
)

// Tags describe a curated model's trade-off, relative to the provider's
// other models.
const (
	shortlistTagCapable  = "Most capable"
	shortlistTagBalanced = "Balanced"
	shortlistTagFastest  = "Fastest"
)

// ShortlistEntry is one curated model and the tag explaining why it was
// picked.
type ShortlistEntry struct {
	Model catwalk.Model
	Tag   string
}

// ModelShortlist is the second onboarding screen. Rather than asking the
// user to parse the whole catalog, it offers the provider's most capable,
// balanced, and fastest models, plus a way into the full list.
type ModelShortlist struct {
	com       *common.Common
	provider  catwalk.Provider
	modelType config.SelectedModelType
	help      help.Model
	list      *list.FilterableList

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
	}
}

// ShortlistItem is one row in the shortlist. It renders either a curated
// model or the "browse all" row.
type ShortlistItem struct {
	*list.Versioned
	provider    catwalk.Provider
	model       catwalk.Model
	modelType   config.SelectedModelType
	tag         string
	browse      bool
	browseCount int
	t           *styles.Styles
	m           fuzzy.Match
	cache       map[int]string
	focused     bool
}

var (
	_ Dialog   = (*ModelShortlist)(nil)
	_ ListItem = (*ShortlistItem)(nil)
)

// NewModelShortlist creates the curated model step for a provider.
func NewModelShortlist(com *common.Common, provider catwalk.Provider, modelType config.SelectedModelType) (*ModelShortlist, error) {
	models := shortlistModels(com.Config(), provider)
	entries := PickShortlist(models)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no models available for %s", providerName(provider))
	}

	items := make([]list.FilterableItem, 0, len(entries)+1)
	for _, entry := range entries {
		items = append(items, NewShortlistItem(com.Styles, provider, entry, modelType))
	}
	if len(models) > len(entries) {
		items = append(items, newBrowseAllItem(com.Styles, provider, len(models), modelType))
	}

	m := &ModelShortlist{
		com:       com,
		provider:  provider,
		modelType: modelType,
	}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.list = list.NewFilterableList(items...)
	// Described rows read better with a blank line between them.
	m.list.SetGap(1)
	m.list.Focus()
	m.list.SetSelected(0)

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "select"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)

	return m, nil
}

// ID implements Dialog.
func (m *ModelShortlist) ID() string {
	return ModelShortlistID
}

// HandleMsg implements Dialog.
func (m *ModelShortlist) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, m.keyMap.Previous):
		m.list.Focus()
		if m.list.IsSelectedFirst() {
			m.list.SelectLast()
		} else {
			m.list.SelectPrev()
		}
		m.list.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.Next):
		m.list.Focus()
		if m.list.IsSelectedLast() {
			m.list.SelectFirst()
		} else {
			m.list.SelectNext()
		}
		m.list.ScrollToSelected()
	case key.Matches(keyMsg, m.keyMap.Select):
		item, ok := m.list.SelectedItem().(*ShortlistItem)
		if !ok {
			break
		}
		if item.browse {
			return ActionBrowseModels{Provider: item.provider}
		}
		return ActionSelectModel{
			Provider:  item.provider,
			Model:     item.selectedModel(),
			ModelType: item.modelType,
		}
	}
	return nil
}

// ShortHelp returns the short help view.
func (m *ModelShortlist) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.UpDown, m.keyMap.Select}
}

// FullHelp returns the full help view.
func (m *ModelShortlist) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// Draw implements Dialog.
func (m *ModelShortlist) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(modelShortlistDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	height := max(1, min(modelShortlistDialogHeight, area.Dy()))
	chrome := modelShortlistChrome + t.Dialog.View.GetVerticalFrameSize()

	listHeight, listTotalHeight := sizeOnboardingList(t, m.list, innerWidth, height, chrome)

	if m.list.Height() >= len(m.list.FilteredItems()) {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	rc := NewRenderContext(t, width)
	rc.Title = "Welcome"
	rc.TitleInfo = onboardingProgress(t, OnboardingStepModel)
	rc.AddPart(t.Dialog.PrimaryText.Render("Pick a starting model."))
	rc.AddPart(t.Dialog.SecondaryText.Render("Switch anytime with ctrl+p."))

	listView := "\n" + t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	DrawCenter(scr, area, rc.Render())
	return nil
}

// NewShortlistItem creates a row for a curated model.
func NewShortlistItem(t *styles.Styles, provider catwalk.Provider, entry ShortlistEntry, modelType config.SelectedModelType) *ShortlistItem {
	return &ShortlistItem{
		Versioned: list.NewVersioned(),
		provider:  provider,
		model:     entry.Model,
		modelType: modelType,
		tag:       entry.Tag,
		t:         t,
		cache:     make(map[int]string),
	}
}

// newBrowseAllItem creates the row that opens the full catalog.
func newBrowseAllItem(t *styles.Styles, provider catwalk.Provider, count int, modelType config.SelectedModelType) *ShortlistItem {
	return &ShortlistItem{
		Versioned:   list.NewVersioned(),
		provider:    provider,
		modelType:   modelType,
		browse:      true,
		browseCount: count,
		t:           t,
		cache:       make(map[int]string),
	}
}

// selectedModel returns the model configuration a choice implies.
func (s *ShortlistItem) selectedModel() config.SelectedModel {
	return config.SelectedModel{
		Model:           s.model.ID,
		Provider:        string(s.provider.ID),
		ReasoningEffort: s.model.Reasoning.DefaultEffortLevel,
		MaxTokens:       s.model.DefaultMaxTokens,
	}
}

// Filter implements ListItem.
func (s *ShortlistItem) Filter() string {
	if s.browse {
		return "browse all models"
	}
	return s.model.Name
}

// ID implements ListItem.
func (s *ShortlistItem) ID() string {
	if s.browse {
		return "browse:" + string(s.provider.ID)
	}
	return modelKey(string(s.provider.ID), s.model.ID)
}

// Render implements ListItem.
func (s *ShortlistItem) Render(width int) string {
	if s.cache == nil {
		s.cache = make(map[int]string)
	}
	if cached, ok := s.cache[width]; ok {
		return cached
	}

	titleStyle := s.t.Dialog.NormalItem
	blurbStyle := s.t.Dialog.Onboarding.Blurb
	if s.focused {
		titleStyle = s.t.Dialog.SelectedItem.Bold(true)
		blurbStyle = s.t.Dialog.SelectedItem
	}

	lineWidth := max(0, width-titleStyle.GetHorizontalFrameSize())

	var tag string
	if s.tag != "" && lipgloss.Width(s.tag)+2 <= lineWidth/2 {
		tag = s.t.Dialog.Onboarding.Badge.Render(" " + s.tag + " ")
	}

	name := s.title()
	name = ansi.Truncate(name, max(0, lineWidth-lipgloss.Width(tag)), "…")
	gap := strings.Repeat(" ", max(0, lineWidth-lipgloss.Width(name)-lipgloss.Width(tag)))
	blurb := padRight(ansi.Truncate(s.blurb(), lineWidth, "…"), lineWidth)

	content := titleStyle.Render(name+gap+tag) + "\n" + blurbStyle.Render(blurb)
	s.cache[width] = content
	return content
}

// SetFocused implements ListItem.
func (s *ShortlistItem) SetFocused(focused bool) {
	if s.focused == focused {
		return
	}
	s.cache = nil
	s.focused = focused
	if s.Versioned != nil {
		s.Bump()
	}
}

// SetMatch implements ListItem.
func (s *ShortlistItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(s.m, m) {
		return
	}
	s.cache = nil
	s.m = m
	if s.Versioned != nil {
		s.Bump()
	}
}

// Finished implements list.Item. Shortlist rows are render-stable outside
// of explicit SetFocused / SetMatch calls.
func (s *ShortlistItem) Finished() bool {
	return true
}

func (s *ShortlistItem) title() string {
	if s.browse {
		return fmt.Sprintf("Browse all %d models…", s.browseCount)
	}
	return cmp.Or(s.model.Name, s.model.ID)
}

func (s *ShortlistItem) blurb() string {
	if s.browse {
		return "Every model " + providerName(s.provider) + " supports."
	}
	return shortlistBlurb(s.model)
}

// PickShortlist chooses up to three models that represent a provider's
// trade-offs. When the provider offers few models, or none of them price
// per token, every candidate is returned in catalog order without a tag:
// labeling a lone model "fastest" would be noise, not information.
func PickShortlist(models []catwalk.Model) []ShortlistEntry {
	candidates := make([]catwalk.Model, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		if model.ID == "" || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		candidates = append(candidates, model)
	}
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= shortlistMaxEntries || !hasPricing(candidates) {
		entries := make([]ShortlistEntry, 0, min(shortlistMaxEntries, len(candidates)))
		for _, model := range candidates[:min(shortlistMaxEntries, len(candidates))] {
			entries = append(entries, ShortlistEntry{Model: model})
		}
		return entries
	}

	capable := pickModel(candidates, func(a, b catwalk.Model) bool {
		if a.ContextWindow != b.ContextWindow {
			return a.ContextWindow > b.ContextWindow
		}
		if price(a) != price(b) {
			return price(a) > price(b)
		}
		return a.ID < b.ID
	})

	// The most capable model is often also the cheapest (a small, long
	// context model), so the other picks are chosen from the rest rather
	// than risking the same model twice.
	rest := modelsExcept(candidates, capable.ID)
	if len(rest) == 0 {
		return []ShortlistEntry{{Model: capable, Tag: shortlistTagCapable}}
	}

	fastest := pickModel(rest, func(a, b catwalk.Model) bool {
		if price(a) != price(b) {
			return price(a) < price(b)
		}
		if a.ContextWindow != b.ContextWindow {
			return a.ContextWindow < b.ContextWindow
		}
		return a.ID < b.ID
	})

	entries := []ShortlistEntry{
		{Model: capable, Tag: shortlistTagCapable},
	}

	if balanced, ok := medianModel(modelsExcept(rest, fastest.ID)); ok {
		entries = append(entries, ShortlistEntry{Model: balanced, Tag: shortlistTagBalanced})
	}

	entries = append(entries, ShortlistEntry{Model: fastest, Tag: shortlistTagFastest})
	return entries
}

// modelsExcept returns the models whose ID is not one of the excluded IDs,
// preserving order.
func modelsExcept(models []catwalk.Model, ids ...string) []catwalk.Model {
	kept := make([]catwalk.Model, 0, len(models))
	for _, model := range models {
		if slices.Contains(ids, model.ID) {
			continue
		}
		kept = append(kept, model)
	}
	return kept
}

// pickModel returns the model that wins the given comparison, breaking
// remaining ties by ID so the result never depends on slice order.
func pickModel(models []catwalk.Model, better func(a, b catwalk.Model) bool) catwalk.Model {
	best := models[0]
	for _, model := range models[1:] {
		if better(model, best) {
			best = model
		}
	}
	return best
}

// medianModel picks the model at the median price, preferring the largest
// context window when several share that price.
func medianModel(models []catwalk.Model) (catwalk.Model, bool) {
	if len(models) == 0 {
		return catwalk.Model{}, false
	}

	sorted := make([]catwalk.Model, len(models))
	copy(sorted, models)
	slices.SortFunc(sorted, func(a, b catwalk.Model) int {
		if diff := cmp.Compare(price(a), price(b)); diff != 0 {
			return diff
		}
		return cmp.Compare(a.ID, b.ID)
	})

	medianPrice := price(sorted[len(sorted)/2])
	median := sorted[len(sorted)/2]
	for _, model := range sorted {
		if price(model) != medianPrice {
			continue
		}
		if model.ContextWindow > median.ContextWindow {
			median = model
		}
	}
	return median, true
}

func hasPricing(models []catwalk.Model) bool {
	for _, model := range models {
		if price(model) > 0 {
			return true
		}
	}
	return false
}

// price is the combined per-million-token cost of a model.
func price(model catwalk.Model) float64 {
	return model.Pricing.Input + model.Pricing.Output
}

// shortlistModels gathers the models a provider currently offers, honoring
// an account login (which swaps the catalog for the subscription's models)
// and any user-defined models in the provider config.
func shortlistModels(cfg *config.Config, provider catwalk.Provider) []catwalk.Model {
	models := make([]catwalk.Model, 0, len(provider.Models))
	index := make(map[string]int, len(provider.Models))
	for _, model := range provider.Models {
		if model.ID == "" {
			continue
		}
		if _, ok := index[model.ID]; ok {
			continue
		}
		index[model.ID] = len(models)
		models = append(models, model)
	}

	if cfg == nil {
		return models
	}
	providerConfig, ok := cfg.Providers.Get(string(provider.ID))
	if !ok {
		return models
	}

	// A signed-in ChatGPT or Grok account serves only the models the
	// subscription grants; the API catalog would 404.
	if providerConfig.OAuthToken != nil {
		var subscription []catwalk.Model
		switch provider.ID {
		case catwalk.InferenceProviderOpenAI:
			subscription = providerConfig.ChatGPTModels
		case catwalk.InferenceProviderXAI:
			subscription = providerConfig.GrokModels
		}
		if subscription != nil {
			models = models[:0]
			index = make(map[string]int, len(subscription))
			for _, model := range subscription {
				if model.ID == "" {
					continue
				}
				if _, ok := index[model.ID]; ok {
					continue
				}
				index[model.ID] = len(models)
				models = append(models, model)
			}
			return models
		}
	}

	for _, model := range providerConfig.Models {
		if model.ID == "" {
			continue
		}
		if idx, ok := index[model.ID]; ok {
			if model.Name != "" {
				models[idx].Name = model.Name
			}
			continue
		}
		index[model.ID] = len(models)
		models = append(models, model)
	}
	return models
}

// shortlistBlurb summarizes a model's context window and price.
func shortlistBlurb(model catwalk.Model) string {
	parts := make([]string, 0, 2)
	if model.ContextWindow > 0 {
		parts = append(parts, shortTokenCount(model.ContextWindow)+" context")
	}
	if price(model) > 0 {
		parts = append(parts,
			fmt.Sprintf("$%.2f in · $%.2f out per M tok", model.Pricing.Input, model.Pricing.Output),
		)
	} else {
		parts = append(parts, "no per-token cost")
	}
	return strings.Join(parts, " · ")
}

// shortTokenCount renders a token count compactly (131072 -> "131K").
func shortTokenCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1_000_000, 'f', 1, 64) + "M"
	case n >= 1_000:
		return strconv.FormatInt(n/1_000, 10) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// providerName returns a provider's display name, falling back to its ID.
func providerName(provider catwalk.Provider) string {
	return cmp.Or(provider.Name, string(provider.ID))
}
