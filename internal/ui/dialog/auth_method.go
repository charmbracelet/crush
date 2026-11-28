package dialog

import (
	"cmp"
	"image"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// AuthMethodID is the identifier for the auth method selection dialog.
const AuthMethodID = "auth_method"

const (
	defaultAuthMethodDialogMaxWidth = 72
	authMethodCardGap               = 1
	// authMethodCardMargin is the blank space between the dialog frame and
	// the pair of cards on each side.
	authMethodCardMargin = 1
	// authMethodCardHeight is the total card height, border included. The
	// odd content height lets the one-line "API Key" label center exactly;
	// the two-line OAuth label lands within half a row of center.
	authMethodCardHeight = 13
	// authMethodMinCardHeight is the smallest card height, border included,
	// that still shows the two-line OAuth label; cards shrink toward it to
	// fit short screens.
	authMethodMinCardHeight = 4
	// authMethodMinCardWidth is the smallest card width that keeps the
	// two-line OAuth label legible; below it the cards stack vertically.
	authMethodMinCardWidth = 20
	// authMethodDoubleClickThreshold bounds the delay between two clicks on
	// a card for the second one to count as confirmation.
	authMethodDoubleClickThreshold = 400 * time.Millisecond
)

// AuthMethod asks how to authenticate a provider that supports both
// OAuth and an API key, before starting either flow.
type AuthMethod struct {
	com          *common.Common
	isOnboarding bool
	provider     catwalk.Provider
	model        config.SelectedModel
	modelType    config.SelectedModelType

	selected       int
	oauthCardArea  image.Rectangle
	apiKeyCardArea image.Rectangle
	lastClickCard  int
	lastClickTime  time.Time
	help           help.Model
	keyMap         struct {
		Choose key.Binding
		Select key.Binding
		Close  key.Binding
	}
}

var _ Dialog = (*AuthMethod)(nil)

// NewAuthMethod creates a dialog that chooses between OAuth and API key
// authentication for the given provider.
func NewAuthMethod(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) *AuthMethod {
	m := &AuthMethod{
		com:           com,
		isOnboarding:  isOnboarding,
		provider:      provider,
		model:         model,
		modelType:     modelType,
		lastClickCard: -1,
	}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	// Vim-style h/l work too, but stay out of the help bar.
	m.keyMap.Choose = key.NewBinding(
		key.WithKeys("left", "right", "up", "down", "tab", "shift+tab", "h", "l"),
		key.WithHelp("←/→", "choose"),
	)
	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "accept"),
	)
	m.keyMap.Close = backKey

	return m
}

// ID implements Dialog.
func (m *AuthMethod) ID() string {
	return AuthMethodID
}

// HandleMsg implements Dialog.
func (m *AuthMethod) HandleMsg(msg tea.Msg) Action {
	if click, ok := msg.(tea.MouseClickMsg); ok {
		return m.handleMouseClick(click)
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, m.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, m.keyMap.Choose):
		m.selected = 1 - m.selected
		return nil
	case key.Matches(keyMsg, m.keyMap.Select):
		return ActionSelectAuthMethod{
			Provider:  m.provider,
			Model:     m.model,
			ModelType: m.modelType,
			UseOAuth:  m.selected == 0,
		}
	}
	return nil
}

// handleMouseClick focuses the clicked card; a second click on the same
// card within the double-click threshold confirms the choice.
func (m *AuthMethod) handleMouseClick(msg tea.MouseClickMsg) Action {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	point := image.Pt(msg.X, msg.Y)
	card := -1
	switch {
	case point.In(m.oauthCardArea):
		card = 0
	case point.In(m.apiKeyCardArea):
		card = 1
	}
	if card < 0 {
		m.lastClickCard = -1
		return nil
	}
	now := time.Now()
	confirm := m.lastClickCard == card && now.Sub(m.lastClickTime) <= authMethodDoubleClickThreshold
	m.lastClickCard = card
	m.lastClickTime = now
	m.selected = card
	if !confirm {
		return nil
	}
	return ActionSelectAuthMethod{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
		UseOAuth:  m.selected == 0,
	}
}

// cardLayout is how the two auth cards share the dialog: side by side
// or stacked, each w by h cells, with gap cells between them when side
// by side.
type cardLayout struct {
	stacked bool
	w, h    int
	gap     int
}

// authMethodLayout fits the cards to innerWidth, inset by
// authMethodCardMargin on both sides, at full height.
func authMethodLayout(innerWidth int) cardLayout {
	l := cardLayout{h: authMethodCardHeight}
	space := innerWidth - 2*authMethodCardMargin
	l.w = max(0, (space-authMethodCardGap)/2)
	if l.w < authMethodMinCardWidth {
		l.stacked = true
		l.w = max(0, space)
		return l
	}
	// The gap takes up any odd leftover cell so the margins on both
	// sides match and the pair sits exactly centered.
	l.gap = space - 2*l.w
	return l
}

// Draw implements Dialog.
func (m *AuthMethod) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width, innerWidth := authWidths(t, area, defaultAuthMethodDialogMaxWidth)

	prompt := t.Dialog.AuthMethod.Prompt.Width(innerWidth).Render("How would you like to authenticate?")
	if !m.isOnboarding {
		// Keep a single blank line under the title.
		prompt = "\n" + prompt
	}
	prompt += "\n"

	// The OAuth card names the account the sign-in uses, which is
	// provider-specific: ChatGPT for OpenAI, Grok for xAI.
	oauthCardLabel := "ChatGPT Account\nwith Subscription"
	if m.provider.ID == catwalk.InferenceProviderXAI {
		oauthCardLabel = "Grok Account\nwith Subscription"
	}

	rc := NewRenderContext(t, width)
	rc.IsOnboarding = m.isOnboarding
	if !m.isOnboarding {
		rc.Title = "Authenticate with " + cmp.Or(m.provider.Name, string(m.provider.ID))
	}
	rc.AddPart(prompt)
	rc.Help = "\n" + renderDialogHelp(t, &m.help, m, innerWidth)
	cardsAt := nextPartOrigin(rc)

	layout := authMethodLayout(innerWidth)
	render := func(l cardLayout) string {
		oauthCard := m.renderCard(oauthCardLabel, m.selected == 0, l.w, l.h)
		apiKeyCard := m.renderCard("API Key", m.selected == 1, l.w, l.h)
		cards := lipgloss.JoinVertical(lipgloss.Left, oauthCard, apiKeyCard)
		if !l.stacked {
			cards = lipgloss.JoinHorizontal(lipgloss.Top, oauthCard, strings.Repeat(" ", l.gap), apiKeyCard)
		}
		withCards := *rc
		withCards.Parts = append(slices.Clip(rc.Parts),
			lipgloss.NewStyle().Padding(0, authMethodCardMargin).Render(cards))
		return withCards.Render()
	}

	// Shrink the cards to fit a short screen; stacked cards share the cut.
	view := render(layout)
	if over := lipgloss.Height(view) - area.Dy(); over > 0 {
		cards := 1
		if layout.stacked {
			cards = 2
		}
		layout.h = max(authMethodMinCardHeight, layout.h-(over+cards-1)/cards)
		view = render(layout)
	}

	viewWidth, viewHeight := lipgloss.Size(view)
	viewWidth = min(viewWidth, area.Dx())
	viewHeight = min(viewHeight, area.Dy())
	origin := common.CenterRect(area, viewWidth, viewHeight).Min
	if m.isOnboarding {
		origin = common.BottomLeftRect(area, viewWidth, viewHeight).Min
	}
	m.updateCardAreas(origin.Add(cardsAt).Add(image.Pt(authMethodCardMargin, 0)), layout)

	authDraw(scr, area, view, nil, m.isOnboarding)
	return nil
}

// updateCardAreas records where each card lands on screen, with the first
// card's top-left corner at at, so mouse clicks can be mapped back to a
// choice.
func (m *AuthMethod) updateCardAreas(at image.Point, l cardLayout) {
	m.oauthCardArea = image.Rect(at.X, at.Y, at.X+l.w, at.Y+l.h)
	next := image.Pt(l.w+l.gap, 0)
	if l.stacked {
		next = image.Pt(0, l.h)
	}
	m.apiKeyCardArea = m.oauthCardArea.Add(next)
}

// renderCard renders one auth option as a bordered card with its label
// centered both ways. The selected card gets the focused frame.
func (m *AuthMethod) renderCard(label string, focused bool, width, height int) string {
	t := m.com.Styles
	style := t.Dialog.AuthMethod.CardBlurred
	if focused {
		style = t.Dialog.AuthMethod.CardFocused
	}
	return style.
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(label)
}

// FullHelp implements help.KeyMap.
func (m *AuthMethod) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp implements help.KeyMap.
func (m *AuthMethod) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Choose,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}
