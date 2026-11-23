package model

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
)

// firstRunState holds what the first-run experience needs to remember. There
// is no separate onboarding screen: a user without a provider lands in the
// regular editor, and only the first send asks them to pick one.
type firstRunState struct {
	// noProvider is true until a provider and model are set up.
	noProvider bool
	// held is the message the user sent before a model was ready. It goes
	// out as soon as one is.
	held *heldMessage

	// startedAt is when the UI opened without a provider. It is zero when a
	// provider was already set up.
	startedAt time.Time
	// via is the path that led to the provider, for telemetry.
	via string
	// firstTurn follows the first prompt after onboarding, for telemetry.
	firstTurn *firstTurn
}

// heldMessage is a prompt waiting for a model.
type heldMessage struct {
	content     string
	attachments []message.Attachment
}

// featuredProviders are the providers the first-run card lists, in order.
// The detail says how each one signs in; it must match what Crush supports.
var featuredProviders = []struct {
	id     catwalk.InferenceProvider
	name   string
	detail string
	// needsKey marks providers that only need an API key, which the card
	// asks for in place.
	needsKey bool
}{
	{"hyper", "Hyper", "Charm account · optimized for Crush", false},
	{catwalk.InferenceProviderAnthropic, "Anthropic", "paste an API key", true},
	{catwalk.InferenceProviderOpenAI, "OpenAI", "ChatGPT sign-in or API key", false},
	{catwalk.InferenceProviderCopilot, "GitHub Copilot", "sign in with GitHub", false},
}

// isOnboarding reports whether no provider is set up yet.
func (m *UI) isOnboarding() bool {
	return m.firstRun.noProvider
}

// providerCardChoices lists the rows of the first-run provider card: the
// featured providers the catalog actually offers, then a way to see the rest.
func providerCardChoices(providers []catwalk.Provider) []dialog.ProviderChoice {
	available := make(map[catwalk.InferenceProvider]bool, len(providers))
	for _, p := range providers {
		available[p.ID] = true
	}

	choices := make([]dialog.ProviderChoice, 0, len(featuredProviders)+1)
	for _, fp := range featuredProviders {
		if !available[fp.id] {
			continue
		}
		choices = append(choices, dialog.ProviderChoice{
			ID:       string(fp.id),
			Name:     fp.name,
			Detail:   fp.detail,
			NeedsKey: fp.needsKey,
		})
	}
	choices = append(choices, dialog.ProviderChoice{
		ID:     dialog.ChoiceMoreProviders,
		Name:   "More providers…",
		Detail: "",
	})
	return choices
}

// defaultSelection returns the model a provider starts with: its default
// large model, or its first model when the catalog names none.
func defaultSelection(p catwalk.Provider) (config.SelectedModel, bool) {
	var picked *catwalk.Model
	for i := range p.Models {
		if p.Models[i].ID == p.DefaultLargeModelID {
			picked = &p.Models[i]
			break
		}
	}
	if picked == nil && len(p.Models) > 0 {
		picked = &p.Models[0]
	}
	if picked == nil {
		return config.SelectedModel{}, false
	}
	return config.SelectedModel{
		Model:           picked.ID,
		Provider:        string(p.ID),
		ReasoningEffort: picked.Reasoning.DefaultEffortLevel,
		MaxTokens:       picked.DefaultMaxTokens,
	}, true
}

// holdMessage keeps a prompt until a model is ready and asks which provider
// to use.
func (m *UI) holdMessage(content string, attachments []message.Attachment) tea.Cmd {
	m.firstRun.held = &heldMessage{content: content, attachments: attachments}
	event.OnboardingPromptHeld(len(attachments) > 0)
	return m.openProviderCard()
}

// providerCard returns the first-run card when it is the active inline
// editor.
func (m *UI) providerCard() *dialog.ProviderInline {
	card, _ := m.activeInline.(*dialog.ProviderInline)
	return card
}

// openProviderCard replaces the editor with the first-run provider card. Like
// the question form, it takes over the editor area while it needs an answer,
// so no text input competes with it.
func (m *UI) openProviderCard() tea.Cmd {
	if m.providerCard() != nil {
		return nil
	}

	providers, err := config.Providers(m.com.Config())
	if err != nil || len(providers) == 0 {
		// Without a catalog the card has nothing to list, so go straight
		// to the full model list, which reports the problem itself.
		return m.openModelsDialog()
	}

	return m.showProviderCard(providerCardChoices(providers))
}

// showProviderCard puts the card with the given rows in the editor area.
func (m *UI) showProviderCard(choices []dialog.ProviderChoice) tea.Cmd {
	title := "Pick a provider to start"
	if m.firstRun.held != nil {
		title = "Pick a provider to send this"
	}
	card := dialog.NewProviderInline(m.com, title, choices)
	card.OnChoose = m.onProviderChosen
	card.OnSelect = m.handleProviderChoice
	card.OnSubmitKey = m.submitProviderKey
	card.OnCancel = func() tea.Cmd {
		event.OnboardingCardDismissed(m.firstRun.held != nil)
		m.restoreHeldMessage()
		return nil
	}

	m.activeInline = card
	m.textarea.Blur()
	m.focus = uiFocusEditor
	card.SetFocused(true)
	m.updateLayoutAndSize()
	return nil
}

// closeProviderCard removes the card and gives the editor back.
func (m *UI) closeProviderCard() tea.Cmd {
	if m.providerCard() == nil {
		return nil
	}
	m.activeInline = nil
	m.updateLayoutAndSize()
	return m.textarea.Focus()
}

// providerByID looks a provider up in the catalog.
func (m *UI) providerByID(id string) (catwalk.Provider, error) {
	providers, err := config.Providers(m.com.Config())
	if err != nil {
		return catwalk.Provider{}, err
	}
	for _, p := range providers {
		if string(p.ID) == id {
			return p, nil
		}
	}
	return catwalk.Provider{}, fmt.Errorf("provider %q is not available", id)
}

// handleProviderChoice starts the sign-in flow for a card row that is not
// key-only. The card stays in the editor area while the flow's dialog is open,
// so backing out of the dialog lands back on the card.
func (m *UI) handleProviderChoice(choice dialog.ProviderChoice) tea.Cmd {
	if choice.ID == dialog.ChoiceMoreProviders {
		return m.openModelsDialog()
	}

	p, err := m.providerByID(choice.ID)
	if err != nil {
		return util.ReportError(err)
	}
	selected, ok := defaultSelection(p)
	if !ok {
		return util.ReportError(fmt.Errorf("%s has no models to choose from", cmp.Or(p.Name, choice.ID)))
	}
	return m.handleSelectModel(dialog.ActionSelectModel{
		Provider:  p,
		Model:     selected,
		ModelType: config.SelectedModelTypeLarge,
	})
}

// providerKeyResultMsg reports the outcome of checking and saving a key
// entered in the card.
type providerKeyResultMsg struct {
	provider catwalk.Provider
	model    config.SelectedModel
	// rejected means the provider did not accept the key.
	rejected bool
	// err is a failure to check or save the key, other than a rejection.
	err error
}

// submitProviderKey checks a key against the provider and, if it works,
// saves it. Both steps are I/O, so they run in a command; the card learns the
// outcome from a providerKeyResultMsg.
func (m *UI) submitProviderKey(choice dialog.ProviderChoice, apiKey string) tea.Cmd {
	p, err := m.providerByID(choice.ID)
	if err != nil {
		return func() tea.Msg { return providerKeyResultMsg{err: err} }
	}
	selected, ok := defaultSelection(p)
	if !ok {
		err := fmt.Errorf("%s has no models to choose from", cmp.Or(p.Name, choice.ID))
		return func() tea.Msg { return providerKeyResultMsg{provider: p, err: err} }
	}
	ws := m.com.Workspace
	return func() tea.Msg {
		result := providerKeyResultMsg{provider: p, model: selected}
		check := config.ProviderConfig{
			ID:      string(p.ID),
			Name:    p.Name,
			APIKey:  apiKey,
			Type:    p.Type,
			BaseURL: p.APIEndpoint,
		}
		if err := check.TestConnection(ws.Resolver()); err != nil {
			result.rejected = true
			return result
		}
		if err := ws.SetProviderAPIKey(config.ScopeGlobal, string(p.ID), apiKey); err != nil {
			result.err = fmt.Errorf("failed to save API key: %w", err)
		}
		return result
	}
}

// handleProviderKeyResult applies the outcome of a key submitted in the card.
func (m *UI) handleProviderKeyResult(msg providerKeyResultMsg) tea.Cmd {
	card := m.providerCard()
	if card == nil {
		// The user left the card while the key was being checked.
		return nil
	}
	switch {
	case msg.err != nil:
		event.OnboardingAPIKeyFailed(string(msg.provider.ID), "error")
		card.KeyResult(false)
		return util.ReportError(msg.err)
	case msg.rejected:
		event.OnboardingAPIKeyFailed(string(msg.provider.ID), "rejected")
		card.KeyResult(false)
		return nil
	}
	card.KeyResult(true)
	return m.handleSelectModel(dialog.ActionSelectModel{
		Provider:  msg.provider,
		Model:     msg.model,
		ModelType: config.SelectedModelTypeLarge,
	})
}

// restoreHeldMessage puts a held prompt back into the editor so nothing the
// user typed is lost when they back out of the provider card.
func (m *UI) restoreHeldMessage() {
	held := m.firstRun.held
	m.firstRun.held = nil
	if held == nil {
		return
	}
	m.textarea.SetValue(held.content)
	m.textarea.MoveToEnd()
	for _, a := range held.attachments {
		m.changeAttachments(func() bool { return m.attachments.Update(a) })
	}
}

// releaseHeldMessage sends the held prompt, if any, once a model is ready.
func (m *UI) releaseHeldMessage() tea.Cmd {
	held := m.firstRun.held
	m.firstRun.held = nil
	if held == nil {
		return nil
	}
	return m.sendMessage(held.content, held.attachments...)
}

// Copy for the launch screen.
const (
	introLine1 = "Crush can configure itself."
	introLine2 = "Ask it to add an MCP server, set up an LSP or change a permission."

	readyLine = "Ready when you are. You'll pick a provider once you hit send."
)

// welcomeView renders the intro shown below the path while no provider is
// set up. It is left out while a provider is being asked for, so the
// question below has the screen to itself.
func (m *UI) welcomeView(width int) string {
	if m.firstRun.held != nil {
		return ""
	}
	muted := m.com.Styles.Initialize.Content
	return lipgloss.JoinVertical(lipgloss.Left,
		ansi.Truncate(muted.Render(introLine1), width, "…"),
		ansi.Truncate(muted.Render(introLine2), width, "…"),
	)
}

// firstRunPromptView renders the line anchored just above the editor while no
// provider is set up: the instruction on a fresh launch, or the message that
// is waiting for a provider. The ◇ icon is left out on purpose; it marks a
// model name or a model's response.
func (m *UI) firstRunPromptView(width int) string {
	t := m.com.Styles
	text := t.Initialize.Header
	muted := t.Initialize.Content
	accent := t.Initialize.Accent

	if held := m.firstRun.held; held != nil {
		first, _, _ := strings.Cut(held.content, "\n")
		return lipgloss.JoinVertical(lipgloss.Left,
			ansi.Truncate(accent.Render("▌")+" "+text.Render(first), width, "…"),
			ansi.Truncate(muted.Render("  held · sends once your provider is set up"), width, "…"),
		)
	}
	return ansi.Truncate(text.Render(readyLine), width, "…")
}

// anchorBottom lays out a block of the given height with top flush to the
// top (after one row of padding) and bottom flush to the bottom. When both do
// not fit, the top block is dropped so the bottom one is never the part that
// is cut.
func anchorBottom(width, height int, top, bottom string) string {
	layout := func(top string) (string, bool) {
		lines := []string{""}
		if top != "" {
			lines = append(lines, strings.Split(top, "\n")...)
		}
		var tail []string
		if bottom != "" {
			tail = strings.Split(bottom, "\n")
		}
		gap := height - len(lines) - len(tail)
		if gap < 0 {
			return "", false
		}
		lines = append(lines, make([]string, gap)...)
		lines = append(lines, tail...)
		return strings.Join(lines, "\n"), true
	}

	body, ok := layout(top)
	if !ok {
		body, _ = layout("")
	}
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(body)
}

// frontDialogID returns the ID of the dialog on top of the stack, or "".
func (m *UI) frontDialogID() string {
	if d := m.dialog.DialogLast(); d != nil {
		return d.ID()
	}
	return ""
}

// firstRunDialogIDs are the dialogs that make up the first-run flow. They are
// drawn unframed, at the bottom of the area they are given.
var firstRunDialogIDs = map[string]bool{
	dialog.APIKeyInputID: true,
	dialog.AuthMethodID:  true,
	dialog.OAuthID:       true,
	dialog.ModelsID:      true,
}

// firstRunDialogFront reports whether a first-run dialog is on top. First run
// shows one prompt at a time, so while a step is open the landing page, the
// provider card it came from and the status line are all hidden. The dialogs
// are unframed and sit at the bottom of the screen, where anything left
// showing beside them would read as part of the step. Closing the dialog
// brings the card back.
func (m *UI) firstRunDialogFront() bool {
	return m.isOnboarding() && firstRunDialogIDs[m.frontDialogID()]
}
