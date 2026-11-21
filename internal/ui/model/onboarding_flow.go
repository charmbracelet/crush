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

// openOnboardingDemo opens the final setup screen, which offers a live first
// turn instead of a shortcut list.
func (m *UI) openOnboardingDemo() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.OnboardingDemoID) {
		m.dialog.BringToFront(dialog.OnboardingDemoID)
		return nil
	}

	m.dialog.OpenDialog(dialog.NewOnboardingDemo(m.com))
	return nil
}

// openOnboardingStepDialog reopens the dialog belonging to the current setup
// step. It is the fallback when a screen is dismissed during onboarding.
func (m *UI) openOnboardingStepDialog() tea.Cmd {
	switch m.onboarding.step {
	case dialog.OnboardingStepModel:
		return m.openModelShortlist(m.onboarding.provider)
	case dialog.OnboardingStepDemo:
		return m.openOnboardingDemo()
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

// finishOnboarding hands control back to the normal UI. When the user asked
// for the demo, the first turn is a live tour: the project-init prompt when
// the project has not been initialized yet (so the first turn also writes the
// context file), otherwise a plain exploration prompt.
func (m *UI) finishOnboarding(runDemo bool) tea.Cmd {
	m.setState(uiLanding, uiFocusEditor)
	m.textarea.Focus()
	if !runDemo {
		return nil
	}

	needsInit, err := m.com.Workspace.ProjectNeedsInitialization()
	if err != nil {
		slog.Warn("Failed to check project initialization", "error", err)
	}
	if needsInit {
		return m.initializeProject()
	}
	return m.sendMessage("What does this project do?")
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
