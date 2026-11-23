package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// Ways a provider can be reached during first run, reported as "via" on
// the "onboarding completed" event.
const (
	onboardingViaCard          = "card"
	onboardingViaCardKey       = "card key"
	onboardingViaMoreProviders = "more providers"
)

// firstTurn follows the first prompt after onboarding so its outcome can be
// reported.
type firstTurn struct {
	// completedAt is when onboarding finished.
	completedAt time.Time
	// held reports whether a prompt was waiting for the provider. If so,
	// it is the first prompt.
	held bool
	// sessionID and sentAt are set once the first prompt goes out.
	sessionID string
	sentAt    time.Time
}

// providerChoiceMethod names how a provider card row signs in.
func providerChoiceMethod(c dialog.ProviderChoice) string {
	switch {
	case c.ID == dialog.ChoiceMoreProviders:
		return "more providers"
	case c.NeedsKey:
		return "api key"
	default:
		return "sign in"
	}
}

// onProviderChosen reports a provider card choice and remembers which path
// the user took.
func (m *UI) onProviderChosen(c dialog.ProviderChoice) {
	switch {
	case c.ID == dialog.ChoiceMoreProviders:
		m.firstRun.via = onboardingViaMoreProviders
	case c.NeedsKey:
		m.firstRun.via = onboardingViaCardKey
	default:
		m.firstRun.via = onboardingViaCard
	}
	event.OnboardingProviderChosen(c.ID, providerChoiceMethod(c))
}

// authKind names the kind of credential a provider uses, without revealing
// it.
func authKind(pc config.ProviderConfig) string {
	switch {
	case pc.ExternalAuth == config.ExternalAuthAnt:
		return "ant login"
	case pc.OAuthToken != nil:
		return "oauth"
	case strings.HasPrefix(pc.APIKey, "$"):
		return "env var"
	case pc.APIKey != "":
		return "api key"
	default:
		return "other"
	}
}

// setupSource names the kind of credential Crush starts with: that of the
// large model's provider, or of the first enabled provider when no model is
// picked yet. It is "none" when no provider is set up.
func setupSource(cfg *config.Config) string {
	if cfg == nil || !cfg.IsConfigured() {
		return "none"
	}
	if large, ok := cfg.Models[config.SelectedModelTypeLarge]; ok {
		if pc, ok := cfg.Providers.Get(large.Provider); ok {
			return authKind(pc)
		}
	}
	return authKind(cfg.EnabledProviders()[0])
}

// trackOnboardingCompleted reports a finished onboarding and starts following
// the first turn. It must run before the held prompt is released, since
// releasing it sends it right away.
func (m *UI) trackOnboardingCompleted(msg dialog.ActionSelectModel) {
	now := time.Now()
	props := event.OnboardingCompletedProps{
		Provider:   msg.Model.Provider,
		Model:      msg.Model.Model,
		Via:        m.firstRun.via,
		HeldPrompt: m.firstRun.held != nil,
	}
	if props.Via == "" {
		// Reached without the card, such as through ctrl+l.
		props.Via = onboardingViaMoreProviders
	}
	if pc, ok := m.com.Config().Providers.Get(msg.Model.Provider); ok {
		props.Auth = authKind(pc)
	}
	if !m.firstRun.startedAt.IsZero() {
		props.Duration = now.Sub(m.firstRun.startedAt)
	}
	event.OnboardingCompleted(props)

	m.firstRun.firstTurn = &firstTurn{
		completedAt: now,
		held:        props.HeldPrompt,
	}
}

// Outcomes of a sign-in step that did not set up a provider.
const (
	signInCancelled        = "cancelled"
	signInFailed           = "failed"
	signInModelUnavailable = "model unavailable"
)

// trackSignInClosed reports a first-run dialog closed without setting up a
// provider. It must run before the dialog is removed.
func (m *UI) trackSignInClosed() {
	if !m.isOnboarding() {
		return
	}
	outcome := func(failed bool) string {
		if failed {
			return signInFailed
		}
		return signInCancelled
	}
	switch d := m.dialog.DialogLast().(type) {
	case *dialog.OAuth:
		event.OnboardingSignInEnded(d.ProviderID(), "sign in", outcome(d.Failed()))
	case *dialog.APIKeyInput:
		event.OnboardingSignInEnded(d.ProviderID(), "api key", outcome(d.Failed()))
	case *dialog.AuthMethod:
		event.OnboardingSignInEnded(d.ProviderID(), "auth method", signInCancelled)
	case *dialog.Models:
		event.OnboardingSignInEnded("", "models", signInCancelled)
	}
}

// trackFirstPromptSent reports the first prompt sent after onboarding.
func (m *UI) trackFirstPromptSent(sessionID string) {
	ft := m.firstRun.firstTurn
	if ft == nil || ft.sessionID != "" || sessionID == "" {
		return
	}
	ft.sessionID = sessionID
	ft.sentAt = time.Now()
	event.OnboardingFirstPromptSent(ft.held, ft.sentAt.Sub(ft.completedAt))
}

// trackFirstTurnFinished reports how the first turn after onboarding ended.
func (m *UI) trackFirstTurnFinished(msg tea.Msg) {
	ft := m.firstRun.firstTurn
	if ft == nil || ft.sessionID == "" {
		return
	}
	done, ok := msg.(pubsub.Event[notify.RunComplete])
	if !ok || done.Payload.SessionID != ft.sessionID {
		return
	}
	outcome, category := "done", ""
	switch {
	case done.Payload.Cancelled:
		outcome = "cancelled"
	case done.Payload.Error != "":
		outcome, category = "error", errorCategory(done.Payload.Error)
	}
	event.OnboardingFirstTurnFinished(outcome, category, time.Since(ft.sentAt))
	m.firstRun.firstTurn = nil
}

// errorCategories map words found in a run error to a coarse class, checked
// in order. Billing comes before rate limits because providers report an
// exhausted balance with a 429 too.
var errorCategories = []struct {
	category string
	needles  []string
}{
	{"billing", []string{"insufficient_quota", "insufficient quota", "billing", "credit", "payment", "402"}},
	{"auth", []string{"401", "403", "unauthorized", "forbidden", "invalid api key", "invalid x-api-key", "invalid_api_key", "authentication", "permission denied"}},
	{"rate limit", []string{"429", "rate limit", "rate_limit", "too many requests", "overloaded", "529"}},
	{"model", []string{"404", "model not found", "model_not_found", "does not exist", "not supported", "unknown model"}},
	{"context", []string{"context length", "context window", "too long", "maximum context", "too many tokens"}},
	{"network", []string{"timeout", "timed out", "deadline exceeded", "connection refused", "connection reset", "no such host", "dial tcp", "tls", "eof", "network"}},
	{"server", []string{"500", "502", "503", "504", "internal server error", "bad gateway", "service unavailable"}},
}

// errorCategory sorts a run error into a coarse class for telemetry, so the
// error text itself is never sent.
func errorCategory(msg string) string {
	msg = strings.ToLower(msg)
	for _, c := range errorCategories {
		for _, n := range c.needles {
			if containsNeedle(msg, n) {
				return c.category
			}
		}
	}
	return "other"
}

// containsNeedle reports whether s contains n. A numeric needle, such as a
// status code, must stand on its own so "401" does not match "140123 tokens".
func containsNeedle(s, n string) bool {
	if strings.Trim(n, "0123456789") != "" {
		return strings.Contains(s, n)
	}
	isDigit := func(b byte) bool { return b >= '0' && b <= '9' }
	for i := 0; ; {
		j := strings.Index(s[i:], n)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(n)
		if (start == 0 || !isDigit(s[start-1])) && (end == len(s) || !isDigit(s[end])) {
			return true
		}
		i = start + 1
	}
}
