package dialog

import (
	"cmp"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
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
	// ProviderSelectID is the identifier for the first-run provider
	// selector.
	ProviderSelectID = "provider_select"

	providerSelectDialogMaxWidth = 64
	providerSelectDialogHeight   = 22
	// providerSelectChrome is the number of lines the dialog spends on
	// everything but the list: the step label, the two copy lines, the
	// input and its margins, and the help footer.
	providerSelectChrome = 8
	// providerPromotedBadge marks the provider promoted on top of the
	// list. It says why Hyper sits first (it is optimized for Crush)
	// rather than implying an entitlement: Hyper needs a Charm
	// subscription and metered credits.
	providerPromotedBadge = "Optimized for Crush"
	// providerSignInBadge marks the providers that authenticate with an
	// account instead of an API key.
	providerSignInBadge = "Sign in"
	// providerAPIKeyBadge marks the providers that need a key. It makes
	// the sign-in rows stand out as the exception rather than the norm.
	providerAPIKeyBadge = "API key"
	// providerFallbackBlurb is shown for providers without a tailored
	// description.
	providerFallbackBlurb = "Bring your own API key."
)

// providerBlurbs describes, in one line, how a provider is paid for. It is
// keyed by provider ID because the catalog carries no description field.
var providerBlurbs = map[string]string{
	"hyper":      "Sign in with your Charm subscription. No API key needed.",
	"openai":     "Sign in with ChatGPT, or paste an API key.",
	"xai":        "Sign in with Grok, or paste an API key.",
	"copilot":    "Sign in with GitHub to use your Copilot subscription.",
	"anthropic":  "Paste a key from console.anthropic.com.",
	"google":     "Paste a key from aistudio.google.com.",
	"gemini":     "Paste a key from aistudio.google.com.",
	"openrouter": "One key for models from many providers.",
	"vercel":     "Use the Vercel AI Gateway.",
	"ollama":     "Runs models on your machine. No key needed.",
	"lmstudio":   "Runs models on your machine. No key needed.",
	"llamacpp":   "Runs models on your machine. No key needed.",
	"bedrock":    "Use your AWS Bedrock credentials.",
	"azure":      "Use your Azure OpenAI deployment.",
	"vertexai":   "Use your Google Cloud Vertex AI project.",
	"groq":       "Paste a key from console.groq.com.",
	"zai":        "Paste a key from z.ai.",
	"deepseek":   "Paste a key from platform.deepseek.com.",
}

// ProviderSelect is the first onboarding screen: a short, described list of
// the providers Crush supports, with Hyper promoted on top.
type ProviderSelect struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
	}
}

// ProviderItem is one provider row in the selector.
type ProviderItem struct {
	*list.Versioned
	provider catwalk.Provider
	blurb    string
	badge    string
	t        *styles.Styles
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var (
	_ Dialog   = (*ProviderSelect)(nil)
	_ ListItem = (*ProviderItem)(nil)
)

// NewProviderSelect creates the provider selector from the provider catalog.
func NewProviderSelect(com *common.Common) (*ProviderSelect, error) {
	items, err := newProviderItems(com)
	if err != nil {
		return nil, err
	}
	return newProviderSelect(com, items), nil
}

// newProviderSelect builds the dialog from prepared rows.
func newProviderSelect(com *common.Common, items []list.FilterableItem) *ProviderSelect {
	m := &ProviderSelect{com: com}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.list = list.NewFilterableList(items...)
	// Described rows read better with a blank line between them.
	m.list.SetGap(1)
	m.list.Focus()
	m.list.SetSelected(0)

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Find your provider"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

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

	return m
}

// ID implements Dialog.
func (m *ProviderSelect) ID() string {
	return ProviderSelectID
}

// HandleMsg implements Dialog.
func (m *ProviderSelect) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, CloseKey):
		// Setup cannot be skipped: escaping the selector would leave Crush
		// without a provider, so the key is swallowed.
		return nil
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
		item, ok := m.list.SelectedItem().(*ProviderItem)
		if !ok {
			break
		}
		return ActionSelectProvider{Provider: item.provider}
	default:
		prevValue := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if value := m.input.Value(); value != prevValue {
			m.list.SetFilter(value)
			m.list.ScrollToTop()
			m.list.SelectFirst()
		}
		return ActionCmd{cmd}
	}
	return nil
}

// ShortHelp returns the short help view.
func (m *ProviderSelect) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.UpDown, m.keyMap.Select}
}

// FullHelp returns the full help view.
func (m *ProviderSelect) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// Draw implements Dialog.
func (m *ProviderSelect) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(providerSelectDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	height := max(1, min(providerSelectDialogHeight, area.Dy()))
	chrome := providerSelectChrome + t.Dialog.View.GetVerticalFrameSize()

	m.input.SetWidth(dialogInputTextWidth(t, m.input, innerWidth))
	listHeight, listTotalHeight := sizeOnboardingList(t, m.list, innerWidth, height, chrome)

	visibleCount := len(m.list.FilteredItems())
	if m.list.Height() >= visibleCount {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	heading := t.Dialog.PrimaryText.Render("Choose how to run your models.")
	subheading := t.Dialog.SecondaryText.Render("You can change this anytime with ctrl+p.")

	rc := NewRenderContext(t, width)
	rc.Title = "Welcome"
	rc.TitleInfo = onboardingProgress(t, OnboardingStepProvider)
	rc.AddPart(heading)
	rc.AddPart(subheading)
	rc.AddPart(t.Dialog.InputPrompt.Render(m.input.View()))

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	cur := m.inputCursor(t, heading, subheading)
	DrawCenterCursor(scr, area, rc.Render(), cur)
	return cur
}

// inputCursor places the text cursor over the rendered input. The title and
// copy lines sit above it and the prompt is rendered before the text, so
// neither offset can be derived from InputCursor alone.
func (m *ProviderSelect) inputCursor(t *styles.Styles, above ...string) *tea.Cursor {
	cur := m.input.Cursor()
	if cur == nil {
		return nil
	}

	// The textinput cursor already accounts for the prompt, so only the
	// frame and the prompt margin are missing.
	cur.X += t.Dialog.View.GetBorderLeftSize() + t.Dialog.InputPrompt.GetMarginLeft()

	offsetY := t.Dialog.View.GetBorderTopSize() + titleContentHeight
	for _, part := range above {
		offsetY += lipgloss.Height(part)
	}
	cur.Y += offsetY + t.Dialog.InputPrompt.GetMarginTop()
	return cur
}

// newProviderItems builds the selector rows from the provider catalog. The
// catalog already lists Hyper first.
func newProviderItems(com *common.Common) ([]list.FilterableItem, error) {
	cfg := com.Config()
	if cfg == nil {
		return nil, fmt.Errorf("configuration not found")
	}

	providers, err := config.Providers(cfg)
	if err != nil && len(providers) == 0 {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}
	if err != nil {
		// The catalog is advisory: a stale cache or an unreachable
		// upstream still gives us a usable list.
		slog.Warn("Listing the previously known providers", "error", err)
	}

	items := make([]list.FilterableItem, 0, len(providers))
	for _, provider := range providers {
		if providerConfig, ok := cfg.Providers.Get(string(provider.ID)); ok && providerConfig.Disable {
			continue
		}
		items = append(items, NewProviderItem(com.Styles, provider))
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no providers available")
	}
	return items, nil
}

// NewProviderItem creates a row for a provider.
func NewProviderItem(t *styles.Styles, provider catwalk.Provider) *ProviderItem {
	item := &ProviderItem{
		Versioned: list.NewVersioned(),
		provider:  provider,
		blurb:     cmp.Or(providerBlurbs[string(provider.ID)], providerFallbackBlurb),
		t:         t,
		cache:     make(map[int]string),
	}
	item.badge = providerBadge(provider)
	return item
}

// providerBadge labels how a provider authenticates. Hyper says why it is
// promoted instead, since the row is already first and its blurb explains the
// sign-in.
func providerBadge(provider catwalk.Provider) string {
	switch provider.ID {
	case "hyper":
		return providerPromotedBadge
	case catwalk.InferenceProviderOpenAI,
		catwalk.InferenceProviderXAI,
		catwalk.InferenceProviderCopilot:
		// These providers can authenticate with an account instead of an
		// API key.
		return providerSignInBadge
	default:
		return providerAPIKeyBadge
	}
}

// Filter implements ListItem.
func (p *ProviderItem) Filter() string {
	return p.name() + " " + string(p.provider.ID)
}

// ID implements ListItem.
func (p *ProviderItem) ID() string {
	return string(p.provider.ID)
}

// Render implements ListItem.
func (p *ProviderItem) Render(width int) string {
	if p.cache == nil {
		p.cache = make(map[int]string)
	}
	if cached, ok := p.cache[width]; ok {
		return cached
	}

	titleStyle := p.t.Dialog.NormalItem
	blurbStyle := p.t.Dialog.Onboarding.Blurb
	if p.focused {
		titleStyle = p.t.Dialog.SelectedItem.Bold(true)
		blurbStyle = p.t.Dialog.SelectedItem
	}

	lineWidth := max(0, width-titleStyle.GetHorizontalFrameSize())

	badgeStyle := p.t.Dialog.Onboarding.BadgeBlurred
	if p.focused {
		badgeStyle = p.t.Dialog.Onboarding.Badge
	}

	var badge string
	if p.badge != "" && lipgloss.Width(p.badge)+2 <= lineWidth/2 {
		badge = badgeStyle.Render(" " + p.badge + " ")
	}

	title := ansi.Truncate(p.name(), max(0, lineWidth-lipgloss.Width(badge)), "…")
	gap := strings.Repeat(" ", max(0, lineWidth-lipgloss.Width(title)-lipgloss.Width(badge)))
	blurb := padRight(ansi.Truncate(p.blurb, lineWidth, "…"), lineWidth)

	content := titleStyle.Render(title+gap+badge) + "\n" + blurbStyle.Render(blurb)
	p.cache[width] = content
	return content
}

// SetFocused implements ListItem.
func (p *ProviderItem) SetFocused(focused bool) {
	if p.focused == focused {
		return
	}
	p.cache = nil
	p.focused = focused
	if p.Versioned != nil {
		p.Bump()
	}
}

// SetMatch implements ListItem.
func (p *ProviderItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(p.m, m) {
		return
	}
	p.cache = nil
	p.m = m
	if p.Versioned != nil {
		p.Bump()
	}
}

// Finished implements list.Item. Provider rows are render-stable outside
// of explicit SetFocused / SetMatch calls.
func (p *ProviderItem) Finished() bool {
	return true
}

func (p *ProviderItem) name() string {
	return cmp.Or(p.provider.Name, string(p.provider.ID))
}
