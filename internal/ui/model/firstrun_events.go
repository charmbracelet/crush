package model

import (
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/google/uuid"
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
	// onboarding is the finished attempt, carried by the first-turn events.
	onboarding event.Onboarding
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

// onboardingAttemptPath is where an onboarding attempt is kept between
// launches.
func onboardingAttemptPath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), event.OnboardingFileName)
}

// onboardingEvent returns what telemetry knows about the current onboarding
// attempt. It starts an attempt when there is none, such as when a provider
// is lost mid-session.
func (m *UI) onboardingEvent() event.Onboarding {
	if m.firstRun.onboarding.ID == "" {
		m.firstRun.onboarding.ID = uuid.NewString()
		m.firstRun.onboarding.Launch = 1
	}
	return m.firstRun.onboarding
}

// onProviderChosen reports a provider card choice and remembers which
// provider and path the user took.
func (m *UI) onProviderChosen(c dialog.ProviderChoice) {
	o := &m.firstRun.onboarding
	switch {
	case c.ID == dialog.ChoiceMoreProviders:
		o.Provider, o.Via = "", onboardingViaMoreProviders
	case c.NeedsKey:
		o.Provider, o.Via = c.ID, onboardingViaCardKey
	default:
		o.Provider, o.Via = c.ID, onboardingViaCard
	}
	event.OnboardingProviderChosen(m.onboardingEvent(), providerChoiceMethod(c))
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
	o := &m.firstRun.onboarding
	o.Provider, o.Model = msg.Model.Provider, msg.Model.Model
	if o.Via == "" {
		// Reached without the card, such as through ctrl+l.
		o.Via = onboardingViaMoreProviders
	}
	if pc, ok := m.com.Config().Providers.Get(msg.Model.Provider); ok {
		o.Auth = authKind(pc)
	}
	var duration time.Duration
	if !m.firstRun.startedAt.IsZero() {
		duration = now.Sub(m.firstRun.startedAt)
	}
	held := m.firstRun.held != nil
	event.OnboardingCompleted(m.onboardingEvent(), held, duration)
	event.FinishOnboarding(onboardingAttemptPath())

	m.firstRun.firstTurn = &firstTurn{
		onboarding:  m.onboardingEvent(),
		completedAt: now,
		held:        held,
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
	ended := func(providerID, step, outcome string) {
		if providerID != "" {
			m.firstRun.onboarding.Provider = providerID
		}
		event.OnboardingSignInEnded(m.onboardingEvent(), step, outcome)
	}
	switch d := m.dialog.DialogLast().(type) {
	case *dialog.OAuth:
		ended(d.ProviderID(), "sign in", outcome(d.Failed()))
	case *dialog.APIKeyInput:
		ended(d.ProviderID(), "api key", outcome(d.Failed()))
	case *dialog.AuthMethod:
		ended(d.ProviderID(), "auth method", signInCancelled)
	case *dialog.Models:
		ended("", "models", signInCancelled)
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
	event.OnboardingFirstPromptSent(ft.onboarding, ft.held, ft.sentAt.Sub(ft.completedAt))
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
	event.OnboardingFirstTurnFinished(ft.onboarding, outcome, category, time.Since(ft.sentAt))
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
