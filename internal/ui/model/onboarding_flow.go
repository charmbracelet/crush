package model

import (
	"log/slog"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// openProviderSelect opens the first setup step: the provider selector.
func (m *UI) openProviderSelect() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.ProviderSelectID) {
		m.dialog.BringToFront(dialog.ProviderSelectID)
		return nil
	}

	selector, err := dialog.NewProviderSelect(m.com)
	if err != nil {
		return util.ReportError(err)
	}

	m.dialog.OpenDialogWithGrace(selector)
	return nil
}

// openModelShortlist opens the curated model step for a provider. Providers
// without a usable catalog fall back to the full list, which can discover
// models on demand.
func (m *UI) openModelShortlist(provider catwalk.Provider) tea.Cmd {
	if m.dialog.ContainsDialog(dialog.ModelShortlistID) {
		m.dialog.BringToFront(dialog.ModelShortlistID)
		return nil
	}

	shortlist, err := dialog.NewModelShortlist(m.com, provider, config.SelectedModelTypeLarge)
	if err != nil {
		slog.Warn("Falling back to the full model list", "error", err)
		return m.openModelsDialogForProvider(string(provider.ID))
	}

	m.dialog.OpenDialogWithGrace(shortlist)
	return nil
}

// openOnboardingHelp opens the final setup screen that teaches the keys worth
// knowing.
func (m *UI) openOnboardingHelp() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.OnboardingHelpID) {
		m.dialog.BringToFront(dialog.OnboardingHelpID)
		return nil
	}

	m.dialog.OpenDialog(dialog.NewOnboardingHelp(m.com, m.onboardingKeyHints()))
	return nil
}

// openOnboardingStepDialog reopens the dialog belonging to the current setup
// step. It is the fallback when a screen is dismissed during onboarding.
func (m *UI) openOnboardingStepDialog() tea.Cmd {
	switch m.onboarding.step {
	case dialog.OnboardingStepModel:
		return m.openModelShortlist(m.onboarding.provider)
	case dialog.OnboardingStepHelp:
		return m.openOnboardingHelp()
	default:
		return m.openProviderSelect()
	}
}

// startProviderAuth begins the credential flow for a provider picked on the
// provider selector. Setup has no model yet, so providers that support both a
// subscription login and an API key go to the method picker instead of
// assuming either.
func (m *UI) startProviderAuth(provider catwalk.Provider) tea.Cmd {
	providerID := string(provider.ID)
	model := config.SelectedModel{Provider: providerID}

	switch provider.ID {
	case catwalk.InferenceProviderCopilot:
		// A Copilot token may already be on disk (signed in through the
		// GitHub CLI or an editor). Importing it skips the device flow
		// entirely.
		if _, imported := m.com.Workspace.ImportCopilot(); imported {
			m.onboarding.step = dialog.OnboardingStepModel
			m.onboarding.provider = provider
			return m.openModelShortlist(provider)
		}
	case catwalk.InferenceProviderOpenAI, catwalk.InferenceProviderXAI:
		providerConfig, _ := m.com.Config().Providers.Get(providerID)
		if providerConfig.OAuthToken == nil && !providerConfig.HasAPIKey(m.com.Workspace.Resolver()) {
			method := dialog.NewAuthMethod(m.com, provider, model, config.SelectedModelTypeLarge)
			if m.dialog.ContainsDialog(method.ID()) {
				m.dialog.BringToFront(method.ID())
				return nil
			}
			m.dialog.OpenDialogWithGrace(method)
			return nil
		}
	}

	return m.openAuthenticationDialog(provider, model, config.SelectedModelTypeLarge)
}

// finishOnboarding hands control back to the normal UI. When the project has
// not been initialized yet, the initialize prompt runs as the last setup step
// instead of waiting for a later launch.
func (m *UI) finishOnboarding() tea.Cmd {
	needsInit, err := m.com.Workspace.ProjectNeedsInitialization()
	if err != nil {
		slog.Warn("Failed to check project initialization", "error", err)
	}
	if needsInit {
		m.setState(uiInitialize, uiFocusEditor)
		return nil
	}

	m.setState(uiLanding, uiFocusEditor)
	m.textarea.Focus()
	return nil
}

// onboardingKeyHints builds the usage screen content from the live key
// bindings, so the hints cannot drift from what the keys do.
func (m *UI) onboardingKeyHints() []dialog.KeyHint {
	return []dialog.KeyHint{
		keyHint(m.keyMap.Editor.SendMessage, "send"),
		keyHint(m.keyMap.Editor.Newline, "new line"),
		keyHint(m.keyMap.Editor.MentionFile, "pull a file into the conversation"),
		keyHint(m.keyMap.Editor.Commands, "slash commands"),
		keyHint(m.keyMap.Commands, "palette: models, sessions, init"),
		keyHint(m.keyMap.Chat.Cancel, "interrupt the agent"),
		keyHint(m.keyMap.ShiftTab, "switch mode"),
	}
}

// keyHint pairs a binding's key with a description suited to onboarding. An
// empty description falls back to the binding's own help text.
func keyHint(binding key.Binding, desc string) dialog.KeyHint {
	help := binding.Help()
	if desc == "" {
		desc = help.Desc
	}
	return dialog.KeyHint{Key: help.Key, Desc: desc}
}
